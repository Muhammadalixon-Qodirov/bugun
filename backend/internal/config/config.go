// Package config loads runtime settings from the environment (.env in development).
package config

import (
	"errors"
	"fmt"
	"net/netip"
	"strings"
	"time"

	"github.com/caarlos0/env/v11"
	"github.com/joho/godotenv"

	"bugun/internal/analytics"
	"bugun/internal/insight"
	"bugun/internal/jobs"
	"bugun/internal/market"
	"bugun/internal/media"
	"bugun/internal/rfq"
)

type Config struct {
	Env         string `env:"APP_ENV" envDefault:"development"`
	HTTPAddr    string `env:"HTTP_ADDR" envDefault:":8080"`
	DatabaseURL string `env:"DATABASE_URL,required"`

	// Public site URL: login returns here and CORS allows it.
	WebURL string `env:"WEB_URL" envDefault:"http://localhost:3000"`
	// Public API URL used for the Telegram login callback. Empty means the site
	// proxies the API under WEB_URL/api (local development).
	APIURL string `env:"API_URL"`
	// Extra browser origins allowed by CORS, comma separated (WEB_URL is always allowed).
	CORSOrigins []string `env:"CORS_ORIGINS" envSeparator:","`
	// Cookie domain shared by the site and the API, e.g. "e-investment.uz".
	// Empty keeps the cookie on the exact host (local development).
	CookieDomain string `env:"COOKIE_DOMAIN"`
	// Shared with the site's server so its requests can pass the visitor's IP
	// for rate limiting. Empty disables forwarding.
	InternalSecret string `env:"INTERNAL_SECRET"`

	// Telegram user ids with access to the admin pages (review queue, role
	// requests), comma separated. Users with role "admin" have access too.
	AdminTelegramIDs []int64 `env:"ADMIN_TELEGRAM_IDS" envSeparator:","`

	SessionTTL time.Duration `env:"SESSION_TTL" envDefault:"720h"`

	// Listing photos on local disk (kept out of the repository).
	MediaDir string `env:"MEDIA_DIR" envDefault:"data/media"`

	// Encrypts personal data at rest (phone numbers). 32 bytes, base64.
	DataKey string `env:"DATA_ENCRYPTION_KEY,required"`

	// Only one process may poll a bot token; turn it off where another copy runs.
	BotEnabled bool `env:"BOT_ENABLED" envDefault:"true"`

	TelegramBotToken     string `env:"TELEGRAM_BOT_TOKEN"`
	TelegramBotUsername  string `env:"TELEGRAM_BOT_USERNAME"`
	TelegramClientID     string `env:"TELEGRAM_CLIENT_ID"`
	TelegramClientSecret string `env:"TELEGRAM_CLIENT_SECRET"`

	// Optional language model for channel posts the rules cannot read
	// (ML_REJA.md 8): any OpenAI-compatible chat API. Off unless all three
	// are set. Posts are sent to that service.
	LLMBaseURL    string `env:"LLM_BASE_URL"`
	LLMModel      string `env:"LLM_MODEL"`
	LLMAPIKey     string `env:"LLM_API_KEY"`
	LLMMaxPerHour int    `env:"LLM_MAX_PER_HOUR" envDefault:"100"`
	// Other models on the same API and key, tried in order when the main one
	// is rate limited or down (the chat assistant; a free tier limits each
	// model separately). Comma separated, empty for none.
	LLMFallbackModels []string `env:"LLM_FALLBACK_MODELS" envSeparator:","`

	// Optional model features (speech, photos, analyst, matching,
	// moderation, advice text) and the ai_log: see ai.go.
	AI AI

	// Run database migrations when the API starts. Turn off where a separate
	// step (cmd/migrate) applies them before the new version starts.
	MigrateOnStart bool `env:"MIGRATE_ON_START" envDefault:"true"`

	// HTTP server. REQUEST_TIMEOUT cuts off a slow handler; WRITE_TIMEOUT
	// bounds a whole response (reports and CSV included) and must be longer.
	RequestTimeout     time.Duration `env:"REQUEST_TIMEOUT" envDefault:"15s"`
	HTTPWriteTimeout   time.Duration `env:"HTTP_WRITE_TIMEOUT" envDefault:"60s"`
	HTTPIdleTimeout    time.Duration `env:"HTTP_IDLE_TIMEOUT" envDefault:"120s"`
	RateLimitPerMinute int           `env:"RATE_LIMIT_PER_MINUTE" envDefault:"120"`
	// Proxies whose X-Forwarded-For / X-Real-IP are believed, as CIDRs. A
	// request from anywhere else is keyed by its own address.
	TrustedProxies []netip.Prefix `env:"TRUSTED_PROXIES" envSeparator:"," envDefault:"127.0.0.1/32,::1/128"`

	// Telegram bot: messages and button presses one user may send a minute.
	BotMessagesPerMinute int `env:"BOT_MESSAGES_PER_MINUTE" envDefault:"20"`

	// Optional external uptime check (e.g. a healthchecks.io ping URL): the
	// scheduler requests it every HEARTBEAT_INTERVAL while it is alive.
	HeartbeatURL      string        `env:"HEARTBEAT_URL"`
	HeartbeatInterval time.Duration `env:"HEARTBEAT_INTERVAL" envDefault:"5m"`

	// A running job refreshes its lease every JOB_HEARTBEAT; at start-up an
	// unfinished run is released only when its lease is older than
	// JOB_LEASE_TIMEOUT, so a second instance never frees a live run.
	JobHeartbeat    time.Duration `env:"JOB_HEARTBEAT" envDefault:"1m"`
	JobLeaseTimeout time.Duration `env:"JOB_LEASE_TIMEOUT" envDefault:"5m"`

	// Schedules, Asia/Tashkent wall-clock time: HH:MM, comma separated for
	// several runs a day; the weekly one as "Mon 09:00".
	ScheduleAutoPricing     []jobs.Clock   `env:"SCHEDULE_AUTO_PRICING" envSeparator:"," envDefault:"07:30,12:00,16:30"`
	ScheduleAnomalies       []jobs.Clock   `env:"SCHEDULE_ANOMALIES" envSeparator:"," envDefault:"03:30"`
	ScheduleSellerScores    []jobs.Clock   `env:"SCHEDULE_SELLER_SCORES" envSeparator:"," envDefault:"04:00"`
	ScheduleSessionsCleanup []jobs.Clock   `env:"SCHEDULE_SESSIONS_CLEANUP" envSeparator:"," envDefault:"04:30"`
	ScheduleFX              []jobs.Clock   `env:"SCHEDULE_FX" envSeparator:"," envDefault:"10:15"`
	ScheduleWeather         []jobs.Clock   `env:"SCHEDULE_WEATHER" envSeparator:"," envDefault:"06:30"`
	ScheduleSellerDigest    []jobs.Clock   `env:"SCHEDULE_SELLER_DIGEST" envSeparator:"," envDefault:"19:00"`
	ScheduleWatchAlerts     []jobs.Clock   `env:"SCHEDULE_WATCH_ALERTS" envSeparator:"," envDefault:"08:00,17:00"`
	ScheduleRatingPrompts   []jobs.Clock   `env:"SCHEDULE_RATING_PROMPTS" envSeparator:"," envDefault:"11:00"`
	ScheduleWeeklyReminder  jobs.WeekClock `env:"SCHEDULE_WEEKLY_REMINDER" envDefault:"Mon 09:00"`

	// Daily price anomalies (ML_REJA.md 4).
	AnomalyZ            float64 `env:"ANOMALY_Z" envDefault:"3.5"`
	AnomalyMinMove      float64 `env:"ANOMALY_MIN_MOVE" envDefault:"0.25"`
	AnomalyMinCrossMove float64 `env:"ANOMALY_MIN_CROSS_MOVE" envDefault:"0.3"`
	AnomalyMinProvinces int     `env:"ANOMALY_MIN_PROVINCES" envDefault:"5"`

	// Automatic pricing (DINAMIK_NARX.md): goal factors against the market
	// median, the daily step, the pause after a price set by hand and the
	// market views needed to compare contact rates.
	RepriceGoalFast         float64       `env:"REPRICE_GOAL_FAST" envDefault:"0.97"`
	RepriceGoalBalanced     float64       `env:"REPRICE_GOAL_BALANCED" envDefault:"1.00"`
	RepriceGoalProfit       float64       `env:"REPRICE_GOAL_PROFIT" envDefault:"1.03"`
	RepriceDailyStep        float64       `env:"REPRICE_DAILY_STEP" envDefault:"0.05"`
	AutoPriceManualPause    time.Duration `env:"AUTO_PRICE_MANUAL_PAUSE" envDefault:"24h"`
	AutoPriceMinMarketViews int           `env:"AUTO_PRICE_MIN_MARKET_VIEWS" envDefault:"100"`

	// Buyer requests (spec §8.4): open requests per buyer and the answer
	// deadline in days.
	RFQMaxOpen     int `env:"RFQ_MAX_OPEN" envDefault:"3"`
	RFQDefaultDays int `env:"RFQ_DEFAULT_DAYS" envDefault:"3"`
	RFQMinDays     int `env:"RFQ_MIN_DAYS" envDefault:"1"`
	RFQMaxDays     int `env:"RFQ_MAX_DAYS" envDefault:"7"`

	// Listing photos.
	MediaMaxUploadBytes int64 `env:"MEDIA_MAX_UPLOAD_BYTES" envDefault:"10485760"`
	MediaMaxPixels      int   `env:"MEDIA_MAX_PIXELS" envDefault:"40000000"`
	MaxPhotosPerListing int   `env:"MAX_PHOTOS_PER_LISTING" envDefault:"5"`

	// ---- Analytics parameters, interval jobs and optional integrations.
	// Checked in settings.go (validateSettings).

	// Forecast (insight.ForecastRules).
	ForecastWeeks            int     `env:"FORECAST_WEEKS" envDefault:"4"`
	ForecastTrendDamping     float64 `env:"FORECAST_TREND_DAMPING" envDefault:"0.8"`
	ForecastMaxTrendEffect   float64 `env:"FORECAST_MAX_TREND_EFFECT" envDefault:"0.15"`
	ForecastMinBacktests     int     `env:"FORECAST_MIN_BACKTESTS" envDefault:"20"`
	ForecastIntervalQuantile float64 `env:"FORECAST_INTERVAL_QUANTILE" envDefault:"0.8"`

	// Similar viloyat groups (insight.ClusterRules).
	ClusterProfileCoverage  float64 `env:"CLUSTER_PROFILE_COVERAGE" envDefault:"0.7"`
	ClusterProvinceCoverage float64 `env:"CLUSTER_PROVINCE_COVERAGE" envDefault:"0.5"`
	ClusterMinProvinces     int     `env:"CLUSTER_MIN_PROVINCES" envDefault:"6"`
	ClusterMinProducts      int     `env:"CLUSTER_MIN_PRODUCTS" envDefault:"5"`
	ClusterMaxGroups        int     `env:"CLUSTER_MAX_GROUPS" envDefault:"5"`
	ClusterTop              int     `env:"CLUSTER_TOP" envDefault:"3"`
	ClusterNotableGap       float64 `env:"CLUSTER_NOTABLE_GAP" envDefault:"0.03"`

	// Jobs that run on an interval rather than at clock times; each must
	// divide a day evenly.
	SchedulePriceDailyEvery  time.Duration `env:"SCHEDULE_PRICE_DAILY_EVERY" envDefault:"1h"`
	ScheduleDealPromptsEvery time.Duration `env:"SCHEDULE_DEAL_PROMPTS_EVERY" envDefault:"15m"`
	ScheduleMonitoringEvery  time.Duration `env:"SCHEDULE_MONITORING_EVERY" envDefault:"1h"`

	// Where listing photos are kept: "disk" (MEDIA_DIR) or "s3" (any
	// S3-compatible service). S3_PUBLIC_BASE_URL, when set, is a public
	// address of the bucket: photos of live listings are redirected there,
	// all others still stream through the API.
	MediaStorage    string `env:"MEDIA_STORAGE" envDefault:"disk"`
	S3Endpoint      string `env:"S3_ENDPOINT"`
	S3Region        string `env:"S3_REGION"`
	S3Bucket        string `env:"S3_BUCKET"`
	S3AccessKey     string `env:"S3_ACCESS_KEY"`
	S3SecretKey     string `env:"S3_SECRET_KEY"`
	S3UsePathStyle  bool   `env:"S3_USE_PATH_STYLE" envDefault:"false"`
	S3PublicBaseURL string `env:"S3_PUBLIC_BASE_URL"`

	// Telegram Mini App sign-in: how old the Mini App's init data may be.
	TelegramWebAppMaxAge time.Duration `env:"TELEGRAM_WEBAPP_MAX_AGE" envDefault:"1h"`

	// Optional error tracking (Sentry). Off while SENTRY_DSN is empty.
	SentryDSN              string  `env:"SENTRY_DSN"`
	SentryEnvironment      string  `env:"SENTRY_ENVIRONMENT"`
	SentryTracesSampleRate float64 `env:"SENTRY_TRACES_SAMPLE_RATE" envDefault:"0"`

	// Live UzEx exchange quotes as official benchmarks. Off by default.
	UzExEnabled  bool         `env:"UZEX_ENABLED" envDefault:"false"`
	UzExURL      string       `env:"UZEX_URL" envDefault:"https://uzex.uz/Quote/GetQuotes"`
	ScheduleUzEx []jobs.Clock `env:"SCHEDULE_UZEX" envSeparator:"," envDefault:"18:30"`
}

func (c Config) IsProduction() bool { return c.Env == "production" }

// PublicAPIURL is where browsers reach the API.
func (c Config) PublicAPIURL() string {
	if c.APIURL != "" {
		return strings.TrimRight(c.APIURL, "/")
	}
	return strings.TrimRight(c.WebURL, "/") + "/api"
}

// Load reads .env if present (ignored in production), then the process environment.
func Load() (Config, error) {
	_ = godotenv.Load()
	return parse(env.Options{})
}

// parse reads the settings from opts.Environment (the process environment
// when nil) and checks them.
func parse(opts env.Options) (Config, error) {
	var c Config
	if err := env.ParseWithOptions(&c, opts); err != nil {
		return c, fmt.Errorf("config: %w", err)
	}
	if err := c.validate(); err != nil {
		return c, fmt.Errorf("config: %w", err)
	}
	return c, nil
}

// validate rejects settings that parse but make no sense, so a typo stops the
// start instead of silently changing behaviour.
func (c Config) validate() error {
	var errs []error
	check := func(ok bool, format string, args ...any) {
		if !ok {
			errs = append(errs, fmt.Errorf(format, args...))
		}
	}
	positive := func(name string, d time.Duration) { check(d > 0, "%s must be positive, got %s", name, d) }
	positive("REQUEST_TIMEOUT", c.RequestTimeout)
	positive("HTTP_WRITE_TIMEOUT", c.HTTPWriteTimeout)
	positive("HTTP_IDLE_TIMEOUT", c.HTTPIdleTimeout)
	positive("HEARTBEAT_INTERVAL", c.HeartbeatInterval)
	positive("JOB_HEARTBEAT", c.JobHeartbeat)
	positive("AUTO_PRICE_MANUAL_PAUSE", c.AutoPriceManualPause)
	check(c.HTTPWriteTimeout > c.RequestTimeout, "HTTP_WRITE_TIMEOUT (%s) must be longer than REQUEST_TIMEOUT (%s)", c.HTTPWriteTimeout, c.RequestTimeout)
	check(c.JobLeaseTimeout >= 2*c.JobHeartbeat, "JOB_LEASE_TIMEOUT (%s) must be at least twice JOB_HEARTBEAT (%s)", c.JobLeaseTimeout, c.JobHeartbeat)
	check(c.RateLimitPerMinute > 0, "RATE_LIMIT_PER_MINUTE must be positive")
	check(c.BotMessagesPerMinute > 0, "BOT_MESSAGES_PER_MINUTE must be positive")
	for name, times := range map[string][]jobs.Clock{
		"SCHEDULE_AUTO_PRICING": c.ScheduleAutoPricing, "SCHEDULE_ANOMALIES": c.ScheduleAnomalies,
		"SCHEDULE_SELLER_SCORES": c.ScheduleSellerScores, "SCHEDULE_SESSIONS_CLEANUP": c.ScheduleSessionsCleanup,
		"SCHEDULE_FX": c.ScheduleFX, "SCHEDULE_WEATHER": c.ScheduleWeather, "SCHEDULE_SELLER_DIGEST": c.ScheduleSellerDigest,
		"SCHEDULE_WATCH_ALERTS": c.ScheduleWatchAlerts, "SCHEDULE_RATING_PROMPTS": c.ScheduleRatingPrompts,
	} {
		check(len(times) > 0, "%s needs at least one time", name)
	}
	check(c.AnomalyZ > 0, "ANOMALY_Z must be positive")
	check(c.AnomalyMinMove >= 0 && c.AnomalyMinCrossMove >= 0, "ANOMALY_MIN_MOVE and ANOMALY_MIN_CROSS_MOVE must not be negative")
	check(c.AnomalyMinProvinces >= 2, "ANOMALY_MIN_PROVINCES must be at least 2")
	for name, g := range map[string]float64{"REPRICE_GOAL_FAST": c.RepriceGoalFast, "REPRICE_GOAL_BALANCED": c.RepriceGoalBalanced, "REPRICE_GOAL_PROFIT": c.RepriceGoalProfit} {
		check(g >= 0.5 && g <= 1.5, "%s must be between 0.5 and 1.5, got %v", name, g)
	}
	check(c.RepriceDailyStep > 0 && c.RepriceDailyStep < 1, "REPRICE_DAILY_STEP must be between 0 and 1, got %v", c.RepriceDailyStep)
	check(c.AutoPriceMinMarketViews >= 0, "AUTO_PRICE_MIN_MARKET_VIEWS must not be negative")
	check(c.RFQMaxOpen > 0, "RFQ_MAX_OPEN must be positive")
	check(c.RFQMinDays >= 1 && c.RFQMinDays <= c.RFQDefaultDays && c.RFQDefaultDays <= c.RFQMaxDays,
		"RFQ days must satisfy 1 <= RFQ_MIN_DAYS (%d) <= RFQ_DEFAULT_DAYS (%d) <= RFQ_MAX_DAYS (%d)", c.RFQMinDays, c.RFQDefaultDays, c.RFQMaxDays)
	check(c.MediaMaxUploadBytes >= 1<<20, "MEDIA_MAX_UPLOAD_BYTES must be at least 1 MB")
	check(c.MediaMaxPixels >= 1_000_000, "MEDIA_MAX_PIXELS must be at least 1000000")
	check(c.MaxPhotosPerListing > 0, "MAX_PHOTOS_PER_LISTING must be positive")
	errs = append(errs, c.validateSettings()...)
	errs = append(errs, c.AI.validate()...)
	return errors.Join(errs...)
}

// AnomalyRules are the thresholds of the daily anomaly check.
func (c Config) AnomalyRules() insight.AnomalyRules {
	return insight.AnomalyRules{Z: c.AnomalyZ, MinMove: c.AnomalyMinMove, MinCrossMove: c.AnomalyMinCrossMove, MinProvinces: c.AnomalyMinProvinces}
}

// AutoPriceRules are the rules of automatic pricing.
func (c Config) AutoPriceRules() market.AutoPriceRules {
	return market.AutoPriceRules{
		RepriceRules: analytics.RepriceRules{
			Goals:     map[string]float64{"fast": c.RepriceGoalFast, "balanced": c.RepriceGoalBalanced, "profit": c.RepriceGoalProfit},
			DailyStep: c.RepriceDailyStep,
		},
		ManualPause: c.AutoPriceManualPause, MinMarketViews: c.AutoPriceMinMarketViews,
	}
}

// RFQLimits bound buyer requests.
func (c Config) RFQLimits() rfq.Limits {
	return rfq.Limits{MaxOpen: c.RFQMaxOpen, DefaultDays: c.RFQDefaultDays, MinDays: c.RFQMinDays, MaxDays: c.RFQMaxDays}
}

// MediaLimits bound photo uploads.
func (c Config) MediaLimits() media.Limits {
	return media.Limits{MaxBytes: c.MediaMaxUploadBytes, MaxPixels: c.MediaMaxPixels}
}
