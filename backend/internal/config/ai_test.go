package config

import (
	"strings"
	"testing"
	"time"
)

func TestAIDefaults(t *testing.T) {
	c, err := load(t, nil)
	if err != nil {
		t.Fatal(err)
	}
	a := c.AI
	if a.AnalystEnabled || a.ModerationEnabled || a.AdviceTextEnabled || a.STTModel != "" || a.VisionModel != "" || a.EmbeddingsModel != "" {
		t.Errorf("every model feature must be off by default: %+v", a)
	}
	if a.LogRetentionDays != 90 || a.STTMaxSeconds != 60 || a.STTLanguage != "uz" || a.AnalystMaxRounds != 4 ||
		a.MatchMinConfidence != 0.6 || a.ModerationTimeout != 4*time.Second || len(a.ScheduleAILogCleanup) != 1 {
		t.Errorf("defaults: %+v", a)
	}
	if len(c.ModelPriceTable()) != 0 {
		t.Error("no prices by default")
	}
}

func TestAISettings(t *testing.T) {
	c, err := load(t, map[string]string{
		"LLM_BASE_URL": "https://llm.example/v1", "LLM_API_KEY": "k", "VISION_MODEL": "vision-1",
		"EMBEDDINGS_BASE_URL": "https://emb.example/v1", "AI_MODEL_PRICES": "small=0.15:0.6, whisper-1=0:0:0.006",
	})
	if err != nil {
		t.Fatal(err)
	}
	if base, key := c.VisionEndpoint(); base != "https://llm.example/v1" || key != "k" {
		t.Errorf("vision falls back to LLM_*: %s %s", base, key)
	}
	if base, key := c.EmbeddingsEndpoint(); base != "https://emb.example/v1" || key != "k" {
		t.Errorf("embeddings: %s %s", base, key)
	}
	if p := c.ModelPriceTable(); p["small"].OutputPerMillion != 0.6 || p["whisper-1"].PerAudioMinute != 0.006 {
		t.Errorf("prices: %+v", p)
	}
	for vars, want := range map[string]string{
		"AI_MODEL_PRICES=small=abc":     "AI_MODEL_PRICES",
		"STT_MAX_SECONDS=2":             "STT_MAX_SECONDS",
		"MATCH_MIN_CANDIDATE=0.9":       "MATCH_MIN_CANDIDATE",
		"ANALYST_MAX_TOOL_ROUNDS=0":     "ANALYST_MAX_TOOL_ROUNDS",
		"AI_LOG_RETENTION_DAYS=0":       "AI_LOG_RETENTION_DAYS",
		"MODERATION_TIMEOUT=0s":         "MODERATION_TIMEOUT",
		"SCHEDULE_AI_LOG_CLEANUP=25:00": "ScheduleAILogCleanup",
	} {
		k, v, _ := strings.Cut(vars, "=")
		if _, err := load(t, map[string]string{k: v}); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: err = %v", vars, err)
		}
	}
}
