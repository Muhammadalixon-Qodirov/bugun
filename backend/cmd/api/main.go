// Command api runs the Bugun HTTP API.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"bugun/internal/ask"
	"bugun/internal/bot"
	"bugun/internal/channels"
	"bugun/internal/config"
	"bugun/internal/deals"
	"bugun/internal/errtrack"
	"bugun/internal/external"
	"bugun/internal/httpapi"
	"bugun/internal/insight"
	"bugun/internal/jobs"
	"bugun/internal/market"
	"bugun/internal/media"
	"bugun/internal/monitor"
	"bugun/internal/pricing"
	"bugun/internal/rfq"
	"bugun/internal/secure"
	"bugun/internal/store"
	"bugun/internal/store/db"
	"bugun/internal/watch"
)

func main() {
	if err := run(); err != nil {
		slog.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run() error {
	// API output is UTC regardless of the host's zone; clients render Asia/Tashkent.
	time.Local = time.UTC

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	setupLogger(cfg)

	// Optional error tracking; everything below works the same without it.
	if on, err := errtrack.Init(errtrack.Options{DSN: cfg.SentryDSN, Environment: cfg.SentryEnv(), TracesSampleRate: cfg.SentryTracesSampleRate}); err != nil {
		return err
	} else if on {
		slog.Info("error tracking on", "environment", cfg.SentryEnv())
		defer errtrack.Flush(2 * time.Second)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	sealer, err := secure.NewSealer(cfg.DataKey)
	if err != nil {
		return err
	}

	pool, err := store.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	if cfg.MigrateOnStart {
		if err := store.Migrate(ctx, pool); err != nil {
			return err
		}
	} else {
		slog.Info("migrations left to cmd/migrate (MIGRATE_ON_START=false)")
	}

	photos, err := media.NewStore(ctx, cfg.MediaStorage, cfg.MediaDir, cfg.S3())
	if err != nil {
		return err
	}

	prices, err := pricing.NewService(ctx, pool)
	if err != nil {
		return err
	}
	rfqs := rfq.NewService(pool, nil, rfq.WithLimits(cfg.RFQLimits()))
	rfqs.SetDemandKey(sealer.Hash)
	dealSvc := deals.NewService(pool)
	watches := watch.NewService(pool)
	insights := insight.NewService(pool, insight.WithAnomalyRules(cfg.AnomalyRules()),
		insight.WithForecastRules(cfg.ForecastRules()), insight.WithClusterRules(cfg.ClusterRules()))
	// Optional model features (spec §9), all recorded in ai_log: see ai.go.
	models := newAI(cfg, pool, sealer)
	// Texts the rules cannot fully read go to a language model when one is
	// configured (MATN_TUSHUNISH.md 3); the site, the bot and channels share it.
	if model := models.Chat(); model != nil {
		prices.SetExtractor(model)
		slog.Info("price texts: language model on", "model", cfg.LLMModel)
	}
	listings := market.NewService(pool, market.WithAutoPrice(cfg.AutoPriceRules()), market.WithMaxPhotos(cfg.MaxPhotosPerListing),
		market.WithContentChecker(models.Moderation))
	asker, err := ask.NewService(ctx, db.New(pool), prices)
	if err != nil {
		return err
	}
	asker.SetDemandKey(sealer.Hash)

	// Background work (the bot, the scheduler) that shutdown waits for
	// before the database pool closes.
	var background sync.WaitGroup
	if err := models.wire(ctx, pool, prices, asker, insights, &background); err != nil {
		return err
	}

	// Clock times below are the defaults of the SCHEDULE_* settings, Tashkent time.
	// Automatic pricing (DINAMIK_NARX.md) before the bazaar opens, at midday
	// and in the afternoon (07:30, 12:00, 16:30); a restart up to 3 hours late
	// still runs the slot.
	var notifyPrices func(context.Context, []market.AutoChange) // the bot, when it runs
	schedule := []jobs.Job{{
		Name: "auto_pricing", Slot: jobs.Daily(cfg.ScheduleAutoPricing...),
		Window: 3 * time.Hour, MaxGap: 18 * time.Hour,
		Run: func(ctx context.Context) (int, error) {
			changes, err := listings.Reprice(ctx)
			if notifyPrices != nil && len(changes) > 0 {
				notifyPrices(ctx, changes)
			}
			return len(changes), err
		},
	}, {
		// Daily price history for analytics: yesterday and today, every hour
		// (SCHEDULE_PRICE_DAILY_EVERY),
		// then the last week's suspicious days (ML_REJA.md 4) on the fresh rows.
		Name: "price_daily", Slot: jobs.Every(cfg.SchedulePriceDailyEvery), Window: cfg.SchedulePriceDailyEvery, MaxGap: 3 * cfg.SchedulePriceDailyEvery,
		Run: func(ctx context.Context) (int, error) {
			n, err := insights.RefreshRecent(ctx)
			if err != nil {
				return n, err
			}
			_, err = insights.RefreshAnomalies(ctx, insight.AnomalyHourDays)
			return n, err
		},
	}, {
		// The whole last year's suspicious days at night (03:30).
		Name: "price_anomalies_year", Slot: jobs.Daily(cfg.ScheduleAnomalies...), Window: 20 * time.Hour, MaxGap: 30 * time.Hour,
		Run: func(ctx context.Context) (int, error) {
			return insights.RefreshAnomalies(ctx, insight.AnomalyNightDays)
		},
	}, {
		// Price texts are kept 180 days (MATN_TUSHUNISH.md 1).
		Name: "parse_log_cleanup", Slot: jobs.Daily(jobs.Clock{Hour: 3}), Window: 20 * time.Hour, MaxGap: 30 * time.Hour,
		Run: prices.CleanParseLog,
	}, {
		// The boosted-tree forecast (PROGNOZ_ML.md): trained nightly, then
		// checked on the past year (04:40).
		Name: "forecast_ml", Slot: jobs.Daily(jobs.Clock{Hour: 4, Minute: 40}), Window: 20 * time.Hour, MaxGap: 30 * time.Hour,
		Run: func(ctx context.Context) (int, error) {
			n, err := insights.TrainForecastModel(ctx)
			if err != nil {
				return n, err
			}
			_, err = insights.BacktestForecastModel(ctx)
			return n, err
		},
	}, {
		// Unusual sellers (ML_REJA.md 5), after the night's anomaly check (04:00).
		Name: "seller_scores", Slot: jobs.Daily(cfg.ScheduleSellerScores...), Window: 20 * time.Hour, MaxGap: 30 * time.Hour,
		Run: insights.RefreshSellerScores,
	}, {
		// Expired sessions and unfinished logins, at night (04:30).
		Name: "sessions_cleanup", Slot: jobs.Daily(cfg.ScheduleSessionsCleanup...), Window: 20 * time.Hour,
		Run: func(ctx context.Context) (int, error) {
			q := db.New(pool)
			sessions, err := q.DeleteExpiredSessions(ctx)
			if err != nil {
				return 0, err
			}
			states, err := q.DeleteExpiredOAuthStates(ctx)
			return int(sessions + states), err
		},
	}, {
		// Central Bank rates once a day (10:15); the first run fills three years back.
		Name: "fx_daily", Slot: jobs.Daily(cfg.ScheduleFX...), Window: 20 * time.Hour, MaxGap: 30 * time.Hour,
		Run: (&external.FX{Q: db.New(pool), Client: &http.Client{Timeout: 15 * time.Second}, Base: external.CBUBase, Pause: time.Second}).Sync,
	}, {
		// Weather at the viloyat centres (06:30); the first run loads three years.
		Name: "weather_daily", Slot: jobs.Daily(cfg.ScheduleWeather...), Window: 20 * time.Hour, MaxGap: 30 * time.Hour,
		Run: (&external.Weather{Q: db.New(pool), Client: &http.Client{Timeout: 60 * time.Second},
			Archive: external.OpenMeteoArchive, Forecast: external.OpenMeteoForecast, Pause: time.Second}).Sync,
	}}
	// Live UzEx exchange quotes as official benchmarks (off unless UZEX_ENABLED).
	references := map[string]time.Duration{}
	if cfg.UzExEnabled {
		schedule = append(schedule, jobs.Job{
			Name: "uzex_quotes", Slot: jobs.Daily(cfg.ScheduleUzEx...), Window: 20 * time.Hour, MaxGap: 30 * time.Hour,
			Run: (&external.UzEx{Q: db.New(pool), Client: &http.Client{Timeout: 30 * time.Second}, URL: cfg.UzExURL}).Sync,
		})
		references["uzex"] = 4 * 24 * time.Hour // a long weekend without a successful fetch
		slog.Info("uzex quotes on", "url", cfg.UzExURL)
	}
	schedule = append(schedule, models.Jobs()...)
	var notifyAdmins func(context.Context, []db.MonitoringAlert) // the bot, when it runs
	if cfg.BotEnabled && cfg.TelegramBotToken != "" {
		b, err := bot.New(cfg.TelegramBotToken, db.New(pool), prices, listings, rfqs, dealSvc, watches, channels.NewService(pool, prices), asker, cfg.WebURL, cfg.BotMessagesPerMinute)
		if err != nil {
			return err
		}
		rfqs.SetNotifier(b)
		b.SetAI(models.Bot())
		notifyPrices = b.PricesChanged
		notifyAdmins = func(ctx context.Context, alerts []db.MonitoringAlert) {
			b.MonitoringAlerts(ctx, cfg.AdminTelegramIDs, alerts)
		}
		// Monday 09:00 Tashkent (spec §8.3); a restart until 15:00 still sends it.
		weekly := cfg.ScheduleWeeklyReminder
		schedule = append(schedule, jobs.Job{
			Name: "weekly_reminder", Slot: jobs.Weekly(weekly.Day, weekly.Hour, weekly.Minute), Window: 6 * time.Hour, MaxGap: 8 * 24 * time.Hour, Run: b.WeeklyReminder,
		}, jobs.Job{
			// Deal follow-ups (QIZIQISH_VA_SAVDO.md); nothing goes out at night.
			Name: "deal_prompts", Slot: jobs.Every(cfg.ScheduleDealPromptsEvery), Window: cfg.ScheduleDealPromptsEvery, MaxGap: 4 * cfg.ScheduleDealPromptsEvery, Run: b.SendDealPrompts,
		}, jobs.Job{
			// Sellers whose contact was opened today (19:00); a late restart must stay before 20:00.
			Name: "seller_deal_digest", Slot: jobs.Daily(cfg.ScheduleSellerDigest...), Window: time.Hour, MaxGap: 30 * time.Hour, Run: b.SellerDealDigest,
		}, jobs.Job{
			// Cheaper prices for followers, morning and afternoon (08:00, 17:00).
			Name: "watch_alerts", Slot: jobs.Daily(cfg.ScheduleWatchAlerts...), Window: 2 * time.Hour, MaxGap: 18 * time.Hour, Run: b.WatchAlerts,
		}, jobs.Job{
			// The assistant's day for the admins (CHATBOT_REJA.md §6).
			Name: "chat_digest", Slot: jobs.Daily(jobs.Clock{Hour: 20}), Window: 3 * time.Hour, MaxGap: 30 * time.Hour,
			Run: func(ctx context.Context) (int, error) { return b.ChatDigest(ctx, cfg.AdminTelegramIDs) },
		}, jobs.Job{
			// Parser figures for the admins (MATN_TUSHUNISH.md 1).
			Name: "parser_report", Slot: jobs.Weekly(time.Monday, 9, 30), Window: 6 * time.Hour, MaxGap: 8 * 24 * time.Hour,
			Run: func(ctx context.Context) (int, error) { return b.ParserReport(ctx, cfg.AdminTelegramIDs) },
		}, jobs.Job{
			// Ratings for deals confirmed three days ago (11:00).
			Name: "rating_prompts", Slot: jobs.Daily(cfg.ScheduleRatingPrompts...), Window: 3 * time.Hour, MaxGap: 30 * time.Hour, Run: b.RatingPrompts,
		})
		background.Go(func() { b.Run(ctx) })
	} else {
		slog.Warn("telegram bot is disabled (BOT_ENABLED=false or no token)")
	}
	// Monitoring (ML_REJA.md 3) watches every job above for being overdue.
	gaps := map[string]time.Duration{}
	for _, j := range schedule {
		if j.MaxGap > 0 {
			gaps[j.Name] = j.MaxGap
		}
	}
	watcher := monitor.NewService(pool, gaps)
	watcher.Notify = notifyAdmins
	watcher.References = references
	schedule = append(schedule, jobs.Job{Name: "monitoring", Slot: jobs.Every(cfg.ScheduleMonitoringEvery), Window: cfg.ScheduleMonitoringEvery, Run: watcher.Run})
	for i := range schedule { // failed runs go to error tracking too
		schedule[i].Run = errtrack.Job(schedule[i].Name, schedule[i].Run)
	}
	scheduler := jobs.NewScheduler(db.New(pool), schedule...).WithLease(cfg.JobHeartbeat, cfg.JobLeaseTimeout)
	if cfg.HeartbeatURL != "" {
		scheduler.WithHeartbeat(&jobs.Heartbeat{URL: cfg.HeartbeatURL, Every: cfg.HeartbeatInterval})
		slog.Info("uptime heartbeat on", "every", cfg.HeartbeatInterval)
	}
	background.Go(func() { scheduler.Run(ctx) })

	srv := &http.Server{
		Addr: cfg.HTTPAddr,
		Handler: httpapi.New(cfg, pool, httpapi.Deps{
			Sealer: sealer, Prices: prices, Listings: listings, RFQ: rfqs, Deals: dealSvc, Watches: watches, Insight: insights, Media: photos,
			AI: models.HTTP(),
		}),
		ReadHeaderTimeout: 5 * time.Second,
		// Longer than REQUEST_TIMEOUT, so a handler that is cut off still
		// gets its 503 out, and room for PDF reports and CSV downloads.
		WriteTimeout: cfg.HTTPWriteTimeout,
		IdleTimeout:  cfg.HTTPIdleTimeout,
	}
	errc := make(chan error, 1)
	go func() {
		slog.Info("api listening", "addr", cfg.HTTPAddr, "env", cfg.Env)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errc <- err
		}
		close(errc)
	}()

	var serveErr error
	select {
	case serveErr = <-errc:
		stop() // the server failed: stop the bot and the scheduler too
	case <-ctx.Done():
	}
	slog.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	err = srv.Shutdown(shutdownCtx)
	// The bot and the scheduler saw ctx end; wait for them (a job finishing
	// its run, the bot its last update) so nothing touches the pool after
	// the deferred Close. Give up at the same deadline.
	waited := make(chan struct{})
	go func() { background.Wait(); close(waited) }()
	select {
	case <-waited:
	case <-shutdownCtx.Done():
		slog.Warn("background work still running at shutdown deadline")
	}
	return errors.Join(serveErr, err)
}

func setupLogger(cfg config.Config) {
	var h slog.Handler
	if cfg.IsProduction() {
		h = slog.NewJSONHandler(os.Stdout, nil)
	} else {
		h = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug})
	}
	slog.SetDefault(slog.New(h))
}
