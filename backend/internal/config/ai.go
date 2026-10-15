package config

import (
	"fmt"
	"time"

	"bugun/internal/ailog"
	"bugun/internal/jobs"
)

// AI holds the settings of the optional model features (spec §9). Each is off
// until configured; without them the rule-based paths answer. They all speak
// OpenAI-compatible APIs, so any provider works. The chat model itself is
// LLM_BASE_URL / LLM_MODEL / LLM_API_KEY in Config.
type AI struct {
	// Timeout of one chat model call.
	LLMTimeout time.Duration `env:"LLM_TIMEOUT" envDefault:"20s"`

	// Model prices for the cost column of ai_log: "model=in:out[:audio_minute]"
	// in US dollars per 1M tokens and per audio minute, comma separated.
	ModelPrices string `env:"AI_MODEL_PRICES"`
	// ai_log rows are deleted after this many days, at SCHEDULE_AI_LOG_CLEANUP.
	LogRetentionDays     int          `env:"AI_LOG_RETENTION_DAYS" envDefault:"90"`
	ScheduleAILogCleanup []jobs.Clock `env:"SCHEDULE_AI_LOG_CLEANUP" envSeparator:"," envDefault:"04:45"`

	// AI-5 analyst: the chat model answers /savol and POST /v1/ask by calling
	// fixed functions over the data. Off: the rule-based answers.
	AnalystEnabled   bool `env:"ANALYST_ENABLED" envDefault:"false"`
	AnalystMaxRounds int  `env:"ANALYST_MAX_TOOL_ROUNDS" envDefault:"4"`
	// Model answers one user (or IP) may get a minute; above it the rules answer.
	AnalystPerMinute int `env:"ANALYST_PER_MINUTE" envDefault:"6"`

	// AI-1 speech to text: bot voice messages and the site's voice button.
	STTBaseURL    string `env:"STT_BASE_URL"`
	STTModel      string `env:"STT_MODEL"`
	STTAPIKey     string `env:"STT_API_KEY"`
	STTLanguage   string `env:"STT_LANGUAGE" envDefault:"uz"`
	STTMaxSeconds int    `env:"STT_MAX_SECONDS" envDefault:"60"`
	STTMaxPerHour int    `env:"STT_MAX_PER_HOUR" envDefault:"200"`

	// AI-3 prices from a photo: a vision-capable chat model. The base URL and
	// key default to LLM_*.
	VisionModel   string `env:"VISION_MODEL"`
	VisionBaseURL string `env:"VISION_BASE_URL"`
	VisionAPIKey  string `env:"VISION_API_KEY"`

	// AI-4 catalog matching: embeddings next to the dictionary and trigram
	// similarity. The base URL and key default to LLM_*.
	EmbeddingsModel   string `env:"EMBEDDINGS_MODEL"`
	EmbeddingsBaseURL string `env:"EMBEDDINGS_BASE_URL"`
	EmbeddingsAPIKey  string `env:"EMBEDDINGS_API_KEY"`
	// A match at or above this is taken; below it the seller picks from
	// candidates scoring at least MATCH_MIN_CANDIDATE.
	MatchMinConfidence float64 `env:"MATCH_MIN_CONFIDENCE" envDefault:"0.6"`
	MatchMinCandidate  float64 `env:"MATCH_MIN_CANDIDATE" envDefault:"0.3"`

	// AI-6: the chat model rewords the buy/sell advice in plain language,
	// with no number of its own. Off: the templated reasons.
	AdviceTextEnabled bool `env:"ADVICE_TEXT_ENABLED" envDefault:"false"`

	// AI-7: the chat model also screens listing texts; a flag only holds a
	// listing for review, and a failure or timeout never blocks it.
	ModerationEnabled bool          `env:"MODERATION_ENABLED" envDefault:"false"`
	ModerationTimeout time.Duration `env:"MODERATION_TIMEOUT" envDefault:"4s"`
}

// validate checks the AI settings.
func (a AI) validate() []error {
	var errs []error
	check := func(ok bool, format string, args ...any) {
		if !ok {
			errs = append(errs, fmt.Errorf(format, args...))
		}
	}
	check(a.LLMTimeout > 0, "LLM_TIMEOUT must be positive")
	_, err := ailog.ParsePrices(a.ModelPrices)
	check(err == nil, "%v", err)
	check(a.LogRetentionDays >= 1, "AI_LOG_RETENTION_DAYS must be at least 1")
	check(len(a.ScheduleAILogCleanup) > 0, "SCHEDULE_AI_LOG_CLEANUP needs at least one time")
	check(a.AnalystMaxRounds >= 1 && a.AnalystMaxRounds <= 10, "ANALYST_MAX_TOOL_ROUNDS must be between 1 and 10")
	check(a.AnalystPerMinute >= 1, "ANALYST_PER_MINUTE must be positive")
	check(a.STTMaxSeconds >= 5 && a.STTMaxSeconds <= 600, "STT_MAX_SECONDS must be between 5 and 600")
	check(a.STTMaxPerHour >= 1, "STT_MAX_PER_HOUR must be positive")
	check(a.MatchMinConfidence > 0 && a.MatchMinConfidence <= 1, "MATCH_MIN_CONFIDENCE must be in (0, 1]")
	check(a.MatchMinCandidate > 0 && a.MatchMinCandidate <= a.MatchMinConfidence, "MATCH_MIN_CANDIDATE must be in (0, MATCH_MIN_CONFIDENCE]")
	check(a.ModerationTimeout > 0, "MODERATION_TIMEOUT must be positive")
	return errs
}

// ModelPriceTable is AI_MODEL_PRICES parsed (validated at start).
func (c Config) ModelPriceTable() map[string]ailog.Price {
	p, _ := ailog.ParsePrices(c.AI.ModelPrices)
	return p
}

// VisionEndpoint is the vision model's base URL and key: its own, or LLM_*.
func (c Config) VisionEndpoint() (baseURL, key string) {
	return or(c.AI.VisionBaseURL, c.LLMBaseURL), or(c.AI.VisionAPIKey, c.LLMAPIKey)
}

// EmbeddingsEndpoint is the embeddings model's base URL and key: its own, or LLM_*.
func (c Config) EmbeddingsEndpoint() (baseURL, key string) {
	return or(c.AI.EmbeddingsBaseURL, c.LLMBaseURL), or(c.AI.EmbeddingsAPIKey, c.LLMAPIKey)
}

func or(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
