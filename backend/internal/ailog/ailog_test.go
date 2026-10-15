package ailog_test

import (
	"context"
	"crypto/sha256"
	"math"
	"testing"
	"time"

	"bugun/internal/ailog"
	"bugun/internal/aitest"
	"bugun/internal/llm"
	"bugun/internal/store/db"
)

func TestPricesAndCost(t *testing.T) {
	p, err := ailog.ParsePrices("gpt-small=0.15:0.6, whisper-1=0:0:0.006, llama3:8b=1:2")
	if err != nil {
		t.Fatal(err)
	}
	if p["llama3:8b"].OutputPerMillion != 2 {
		t.Errorf("a model name may hold a colon: %+v", p)
	}
	cost := ailog.Cost(p, llm.Call{Model: "gpt-small", InputTokens: 1_000_000, OutputTokens: 500_000})
	if math.Abs(cost-0.45) > 1e-9 {
		t.Errorf("cost %v", cost)
	}
	if c := ailog.Cost(p, llm.Call{Model: "whisper-1", AudioSeconds: 90}); math.Abs(c-0.009) > 1e-9 {
		t.Errorf("audio cost %v", c)
	}
	if ailog.Cost(p, llm.Call{Model: "unknown", InputTokens: 10}) != 0 {
		t.Error("unknown models cost 0")
	}
	for _, bad := range []string{"x", "m=1", "m=a:b", "m=-1:0", "=1:2"} {
		if _, err := ailog.ParsePrices(bad); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
}

func TestRecordUsageAndPrune(t *testing.T) {
	pool := aitest.DB(t)
	ctx := context.Background()
	if _, err := pool.Exec(ctx, `DELETE FROM ai_log`); err != nil {
		t.Fatal(err)
	}
	q := db.New(pool)
	u := aitest.User(t, q, 900600001, "entrepreneur")
	hash := func(s string) []byte { h := sha256.Sum256([]byte("key" + s)); return h[:] }
	l := ailog.New(q, hash, map[string]ailog.Price{"chat": {InputPerMillion: 1, OutputPerMillion: 2}})

	l.Record(ctx, llm.Call{Feature: "analyst", Model: "chat", InputTokens: 1000, OutputTokens: 500, Latency: 800 * time.Millisecond, OK: true, UserID: &u.ID, Input: []byte("Namanganda piyoz qancha?")})
	l.Record(ctx, llm.Call{Feature: "analyst", Model: "chat", Latency: 200 * time.Millisecond, ErrorCode: "timeout"})
	l.Record(ctx, llm.Call{Feature: "stt", Model: "whisper", AudioSeconds: 12.5, Latency: time.Second, OK: true})

	var hashes, raw int
	if err := pool.QueryRow(ctx, `SELECT count(input_hash), count(*) FILTER (WHERE input_hash LIKE '%piyoz%') FROM ai_log`).Scan(&hashes, &raw); err != nil {
		t.Fatal(err)
	}
	if hashes != 1 || raw != 0 {
		t.Errorf("only a digest of the input is kept: %d hashes, %d raw", hashes, raw)
	}

	us, err := l.Usage(ctx, 7)
	if err != nil {
		t.Fatal(err)
	}
	if us.Total.Calls != 3 || us.Total.Errors != 1 || us.Total.InputTokens != 1000 || us.Total.AudioSeconds != 12.5 || math.Abs(us.Total.CostUSD-0.002) > 1e-9 {
		t.Errorf("total %+v", us.Total)
	}
	if len(us.ByFeature) != 2 || us.ByFeature[0].Feature != "analyst" || us.ByFeature[0].Calls != 2 || us.ByFeature[0].AvgLatencyMs != 500 {
		t.Errorf("by feature %+v", us.ByFeature)
	}
	if len(us.ByModel) != 2 || len(us.ByDay) != 1 || len(us.Rows) != 2 {
		t.Errorf("by model %+v, by day %+v, rows %+v", us.ByModel, us.ByDay, us.Rows)
	}
	if len(us.Errors) != 1 || us.Errors[0].Code != "timeout" || us.Errors[0].Calls != 1 {
		t.Errorf("errors %+v", us.Errors)
	}

	if _, err := pool.Exec(ctx, `UPDATE ai_log SET created_at = now() - interval '100 days' WHERE feature = 'stt'`); err != nil {
		t.Fatal(err)
	}
	n, err := l.Prune(ctx, 90)
	if err != nil || n != 1 {
		t.Errorf("prune removed %d (%v), want 1", n, err)
	}
}
