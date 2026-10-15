// Package ailog records every model call in ai_log (spec §16): the feature,
// model, tokens, audio length, latency, estimated cost and outcome. It never
// stores what was sent or answered: only a short keyed digest of the input.
package ailog

import (
	"context"
	"encoding/hex"
	"fmt"
	"log/slog"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"bugun/internal/llm"
	"bugun/internal/store/db"
	"bugun/internal/tz"
)

// Price is what a model costs, in US dollars.
type Price struct {
	InputPerMillion  float64 // per 1M input tokens
	OutputPerMillion float64 // per 1M output tokens
	PerAudioMinute   float64
}

// ParsePrices reads AI_MODEL_PRICES: "model=in:out[:audio_minute]" entries,
// comma separated, e.g. "gpt-4o-mini=0.15:0.6,whisper-1=0:0:0.006".
func ParsePrices(s string) (map[string]Price, error) {
	out := map[string]Price{}
	for _, entry := range strings.Split(s, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		i := strings.LastIndex(entry, "=")
		if i <= 0 {
			return nil, fmt.Errorf("AI_MODEL_PRICES: %q is not model=in:out[:audio]", entry)
		}
		parts := strings.Split(entry[i+1:], ":")
		if len(parts) < 2 || len(parts) > 3 {
			return nil, fmt.Errorf("AI_MODEL_PRICES: %q is not model=in:out[:audio]", entry)
		}
		var vals [3]float64
		for k, p := range parts {
			v, err := strconv.ParseFloat(strings.TrimSpace(p), 64)
			if err != nil || v < 0 || math.IsInf(v, 0) || math.IsNaN(v) {
				return nil, fmt.Errorf("AI_MODEL_PRICES: bad price in %q", entry)
			}
			vals[k] = v
		}
		out[strings.TrimSpace(entry[:i])] = Price{InputPerMillion: vals[0], OutputPerMillion: vals[1], PerAudioMinute: vals[2]}
	}
	return out, nil
}

// Cost estimates a call's price; unknown models cost 0.
func Cost(prices map[string]Price, c llm.Call) float64 {
	p, ok := prices[c.Model]
	if !ok {
		return 0
	}
	cost := float64(c.InputTokens)*p.InputPerMillion/1e6 + float64(c.OutputTokens)*p.OutputPerMillion/1e6 + c.AudioSeconds/60*p.PerAudioMinute
	return math.Round(cost*1e6) / 1e6
}

// Logger writes calls to ai_log. It implements llm.Recorder.
type Logger struct {
	q      *db.Queries
	hash   func(string) []byte // keyed digest (secure.Sealer.Hash)
	prices map[string]Price
}

func New(q *db.Queries, hash func(string) []byte, prices map[string]Price) *Logger {
	return &Logger{q: q, hash: hash, prices: prices}
}

// Record keeps one call. Failures are logged, never returned: a missing log
// row must not break the feature.
func (l *Logger) Record(ctx context.Context, c llm.Call) {
	p := db.InsertAILogParams{
		Feature: c.Feature, Model: c.Model, InputTokens: clampInt32(c.InputTokens), OutputTokens: clampInt32(c.OutputTokens),
		LatencyMs: clampInt32(int(c.Latency.Milliseconds())), CostUsd: Cost(l.prices, c), Ok: c.OK,
		ErrorCode: pgtype.Text{String: c.ErrorCode, Valid: c.ErrorCode != ""},
	}
	if c.AudioSeconds > 0 {
		p.AudioSeconds = pgtype.Float4{Float32: float32(c.AudioSeconds), Valid: true}
	}
	if c.UserID != nil {
		p.UserID = pgtype.UUID{Bytes: *c.UserID, Valid: true}
	}
	if len(c.Input) > 0 && l.hash != nil {
		p.InputHash = pgtype.Text{String: hex.EncodeToString(l.hash(string(c.Input)))[:16], Valid: true}
	}
	if err := l.q.InsertAILog(ctx, p); err != nil {
		slog.WarnContext(ctx, "ai log", "err", err)
	}
}

func clampInt32(n int) int32 {
	if n < 0 {
		return 0
	}
	if n > math.MaxInt32 {
		return math.MaxInt32
	}
	return int32(n)
}

// Prune removes calls older than days; the retention job runs it.
func (l *Logger) Prune(ctx context.Context, days int) (int, error) {
	n, err := l.q.DeleteOldAILog(ctx, time.Now().AddDate(0, 0, -days))
	return int(n), err
}

// Totals add up calls.
type Totals struct {
	Calls        int     `json:"calls"`
	Errors       int     `json:"errors"`
	InputTokens  int64   `json:"input_tokens"`
	OutputTokens int64   `json:"output_tokens"`
	AudioSeconds float64 `json:"audio_seconds"`
	CostUSD      float64 `json:"cost_usd" doc:"Estimated from AI_MODEL_PRICES; 0 for models without a price"`
	AvgLatencyMs float64 `json:"avg_latency_ms"`
}

func (t *Totals) add(r db.AIUsageDailyRow) {
	// The mean latency is weighted by calls.
	calls := t.Calls + int(r.Calls)
	if calls > 0 {
		t.AvgLatencyMs = (t.AvgLatencyMs*float64(t.Calls) + r.AvgLatencyMs*float64(r.Calls)) / float64(calls)
	}
	t.Calls = calls
	t.Errors += int(r.Errors)
	t.InputTokens += r.InputTokens
	t.OutputTokens += r.OutputTokens
	t.AudioSeconds += r.AudioSeconds
	t.CostUSD += r.CostUsd
}

func (t Totals) rounded() Totals {
	t.AudioSeconds = math.Round(t.AudioSeconds*10) / 10
	t.CostUSD = math.Round(t.CostUSD*1e6) / 1e6
	t.AvgLatencyMs = math.Round(t.AvgLatencyMs)
	return t
}

type DayUsage struct {
	Day string `json:"day" doc:"Tashkent calendar day, YYYY-MM-DD"`
	Totals
}

type FeatureUsage struct {
	Feature string `json:"feature" doc:"analyst, stt, vision, match, moderation, channel_parse, advice, embeddings"`
	Totals
}

type ModelUsage struct {
	Model string `json:"model"`
	Totals
}

// Row is one day, feature and model.
type Row struct {
	Day     string `json:"day"`
	Feature string `json:"feature"`
	Model   string `json:"model"`
	Totals
	P95LatencyMs float64 `json:"p95_latency_ms"`
}

type ErrorCount struct {
	Feature string `json:"feature"`
	Code    string `json:"code" doc:"limit, timeout, http_<status>, bad_answer, network, canceled"`
	Calls   int    `json:"calls"`
}

// Usage is the admin view of model calls.
type Usage struct {
	Since     string         `json:"since" doc:"First Tashkent day counted"`
	Total     Totals         `json:"total"`
	ByDay     []DayUsage     `json:"by_day" doc:"Newest first"`
	ByFeature []FeatureUsage `json:"by_feature" doc:"Most calls first"`
	ByModel   []ModelUsage   `json:"by_model" doc:"Most calls first"`
	Rows      []Row          `json:"rows" doc:"Day × feature × model, newest first"`
	Errors    []ErrorCount   `json:"errors" doc:"Failed calls by code, most frequent first"`
}

// Usage adds up the calls of the last days (Tashkent days, today included).
func (l *Logger) Usage(ctx context.Context, days int) (Usage, error) {
	now := time.Now().In(tz.Tashkent)
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, tz.Tashkent).AddDate(0, 0, -(days - 1))
	rows, err := l.q.AIUsageDaily(ctx, start)
	if err != nil {
		return Usage{}, err
	}
	errs, err := l.q.AIUsageErrors(ctx, start)
	if err != nil {
		return Usage{}, err
	}
	return summarize(start, rows, errs), nil
}

func summarize(start time.Time, rows []db.AIUsageDailyRow, errs []db.AIUsageErrorsRow) Usage {
	u := Usage{Since: start.Format(time.DateOnly), ByDay: []DayUsage{}, ByFeature: []FeatureUsage{}, ByModel: []ModelUsage{}, Rows: []Row{}, Errors: []ErrorCount{}}
	byDay, byFeature, byModel := map[string]*Totals{}, map[string]*Totals{}, map[string]*Totals{}
	var dayOrder []string
	get := func(m map[string]*Totals, k string) *Totals {
		if m[k] == nil {
			m[k] = &Totals{}
		}
		return m[k]
	}
	for _, r := range rows {
		day := r.Day.Time.Format(time.DateOnly)
		if byDay[day] == nil {
			dayOrder = append(dayOrder, day)
		}
		get(byDay, day).add(r)
		get(byFeature, r.Feature).add(r)
		get(byModel, r.Model).add(r)
		u.Total.add(r)
		var t Totals
		t.add(r)
		u.Rows = append(u.Rows, Row{Day: day, Feature: r.Feature, Model: r.Model, Totals: t.rounded(), P95LatencyMs: math.Round(r.P95LatencyMs)})
	}
	u.Total = u.Total.rounded()
	for _, d := range dayOrder {
		u.ByDay = append(u.ByDay, DayUsage{Day: d, Totals: byDay[d].rounded()})
	}
	for f, t := range byFeature {
		u.ByFeature = append(u.ByFeature, FeatureUsage{Feature: f, Totals: t.rounded()})
	}
	sort.Slice(u.ByFeature, func(i, j int) bool {
		a, b := u.ByFeature[i], u.ByFeature[j]
		return a.Calls > b.Calls || (a.Calls == b.Calls && a.Feature < b.Feature)
	})
	for m, t := range byModel {
		u.ByModel = append(u.ByModel, ModelUsage{Model: m, Totals: t.rounded()})
	}
	sort.Slice(u.ByModel, func(i, j int) bool {
		a, b := u.ByModel[i], u.ByModel[j]
		return a.Calls > b.Calls || (a.Calls == b.Calls && a.Model < b.Model)
	})
	for _, e := range errs {
		u.Errors = append(u.Errors, ErrorCount{Feature: e.Feature, Code: e.ErrorCode, Calls: int(e.Calls)})
	}
	return u
}
