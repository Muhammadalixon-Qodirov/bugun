package main

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"bugun/internal/advice"
	"bugun/internal/ailog"
	"bugun/internal/analyst"
	"bugun/internal/ask"
	"bugun/internal/bot"
	"bugun/internal/catalogmatch"
	"bugun/internal/config"
	"bugun/internal/httpapi"
	"bugun/internal/insight"
	"bugun/internal/jobs"
	"bugun/internal/llm"
	"bugun/internal/moderation"
	"bugun/internal/photoprice"
	"bugun/internal/pricing"
	"bugun/internal/secure"
	"bugun/internal/store/db"
)

// aiParts are the optional model features (spec §9). Every model client is
// nil unless its settings are complete, and each feature then falls back to
// its deterministic or rule-based path.
type aiParts struct {
	cfg  config.Config
	log  *ailog.Logger
	chat *llm.Client // LLM_*: channel posts, analyst, moderation, advice
	// chatChain is chat and then LLM_FALLBACK_MODELS, for the assistant.
	chatChain llm.Chain
	stt       *llm.Client
	vision    *llm.Client
	emb       *llm.Client

	Moderation *moderation.Checker
	matcher    *catalogmatch.Matcher
	analyst    *analyst.Service
	photos     *photoprice.Reader
	advice     *advice.Phraser
}

// newAI builds the model clients; they all record to ai_log.
func newAI(cfg config.Config, pool *pgxpool.Pool, sealer *secure.Sealer) *aiParts {
	a := &aiParts{cfg: cfg, log: ailog.New(db.New(pool), sealer.Hash, cfg.ModelPriceTable())}
	a.chat = llm.New(cfg.LLMBaseURL, cfg.LLMModel, cfg.LLMAPIKey, cfg.LLMMaxPerHour).WithTimeout(cfg.AI.LLMTimeout).WithRecorder(a.log)
	a.chatChain = llm.NewChain(a.chat)
	if a.chat != nil {
		for _, m := range cfg.LLMFallbackModels {
			if m = strings.TrimSpace(m); m != "" && m != cfg.LLMModel {
				a.chatChain = append(a.chatChain, llm.New(cfg.LLMBaseURL, m, cfg.LLMAPIKey, cfg.LLMMaxPerHour).WithTimeout(cfg.AI.LLMTimeout).WithRecorder(a.log))
			}
		}
	}
	a.stt = llm.New(cfg.AI.STTBaseURL, cfg.AI.STTModel, cfg.AI.STTAPIKey, cfg.AI.STTMaxPerHour).WithTimeout(time.Minute).WithRecorder(a.log)
	base, key := cfg.VisionEndpoint()
	a.vision = llm.New(base, cfg.AI.VisionModel, key, cfg.LLMMaxPerHour).WithTimeout(time.Minute).WithRecorder(a.log)
	base, key = cfg.EmbeddingsEndpoint()
	a.emb = llm.New(base, cfg.AI.EmbeddingsModel, key, cfg.LLMMaxPerHour).WithTimeout(cfg.AI.LLMTimeout).WithRecorder(a.log)

	var mod *llm.Client
	if cfg.AI.ModerationEnabled {
		mod = a.chat
	}
	a.Moderation = moderation.New(mod, cfg.AI.ModerationTimeout)
	slog.Info("model features",
		"chat", a.chat != nil, "chat_models", len(a.chatChain), "analyst", cfg.AI.AnalystEnabled && a.chat != nil, "moderation", mod != nil,
		"advice", cfg.AI.AdviceTextEnabled && a.chat != nil, "stt", a.stt != nil, "vision", a.vision != nil, "embeddings", a.emb != nil)
	return a
}

// wire builds the features on top of the core services. Catalog embeddings
// are computed in the background; matching works without them meanwhile.
func (a *aiParts) wire(ctx context.Context, pool *pgxpool.Pool, prices *pricing.Service, asker *ask.Service, insights *insight.Service, background *sync.WaitGroup) error {
	cfg := a.cfg
	m, err := catalogmatch.Load(ctx, db.New(pool), catalogmatch.Thresholds{
		MinConfidence: cfg.AI.MatchMinConfidence, MinCandidate: cfg.AI.MatchMinCandidate, Margin: catalogmatch.DefaultThresholds.Margin,
	})
	if err != nil {
		return err
	}
	a.matcher = m
	prices.SetMatcher(m)
	if a.emb != nil {
		m.SetEmbeddings(a.emb)
		background.Go(func() {
			if err := m.SyncEmbeddings(ctx); err != nil {
				slog.Warn("catalog embeddings", "err", err)
			}
		})
	}
	opts := []analyst.Option{analyst.WithMatcher(m)}
	if cfg.AI.AnalystEnabled && a.chat != nil {
		opts = append(opts, analyst.WithModel(a.chatChain, cfg.AI.AnalystMaxRounds, cfg.AI.AnalystPerMinute))
	}
	a.analyst = analyst.New(db.New(pool), asker, prices, insights, opts...)
	a.photos = photoprice.New(a.vision, prices, cfg.MediaLimits())
	var adv *llm.Client
	if cfg.AI.AdviceTextEnabled {
		adv = a.chat
	}
	a.advice = advice.New(adv)
	return nil
}

// Chat is the chat model for channel posts, or nil.
func (a *aiParts) Chat() *llm.Client { return a.chat }

func (a *aiParts) HTTP() *httpapi.AIDeps {
	cfg := a.cfg
	return &httpapi.AIDeps{
		Analyst: a.analyst, STT: a.stt, STTLanguage: cfg.AI.STTLanguage, STTMaxSeconds: cfg.AI.STTMaxSeconds,
		Photos: a.photos, Matcher: a.matcher, Advice: a.advice, Log: a.log,
		Features: httpapi.AIFeatures{
			Voice: a.stt != nil, VoiceMaxSecond: cfg.AI.STTMaxSeconds, Photo: a.vision != nil,
			AnalystModel: a.analyst.ModelEnabled(), Embeddings: a.emb != nil,
			Moderation: cfg.AI.ModerationEnabled && a.chat != nil, AdviceModel: cfg.AI.AdviceTextEnabled && a.chat != nil,
		},
	}
}

func (a *aiParts) Bot() bot.AI {
	return bot.AI{Analyst: a.analyst, STT: a.stt, STTLanguage: a.cfg.AI.STTLanguage, STTMaxSeconds: a.cfg.AI.STTMaxSeconds,
		Photos: a.photos, MaxPhotoBytes: a.cfg.MediaMaxUploadBytes}
}

// Jobs: ai_log rows older than AI_LOG_RETENTION_DAYS go at night (04:45),
// old conversations at 04:50.
func (a *aiParts) Jobs() []jobs.Job {
	return []jobs.Job{{
		Name: "ai_log_cleanup", Slot: jobs.Daily(a.cfg.AI.ScheduleAILogCleanup...), Window: 20 * time.Hour,
		Run: func(ctx context.Context) (int, error) { return a.log.Prune(ctx, a.cfg.AI.LogRetentionDays) },
	}, {
		// Conversations are kept 30 days, unanswered questions 180 (04:50).
		Name: "chat_cleanup", Slot: jobs.Daily(jobs.Clock{Hour: 4, Minute: 50}), Window: 20 * time.Hour,
		Run: a.analyst.CleanChats,
	}}
}
