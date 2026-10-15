package monitor

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"bugun/internal/store"
	"bugun/internal/store/db"
)

func days(src string, counts ...int32) []db.SourceDailyCountsRow {
	var out []db.SourceDailyCountsRow
	for i, n := range counts {
		out = append(out, db.SourceDailyCountsRow{Source: src, DaysAgo: int32(i), N: n})
	}
	return out
}

func kinds(ss []Signal) map[string]Signal {
	m := map[string]Signal{}
	for _, s := range ss {
		m[s.key()] = s
	}
	return m
}

func TestFlowSignals(t *testing.T) {
	usual := []int32{40, 42, 38, 41, 39, 40, 43, 37, 40, 41, 40, 39, 42, 40}
	rows := days("seller", append([]int32{0}, usual...)...)
	rows = append(rows, days("telegram_channel", append([]int32{9}, usual...)...)...)
	rows = append(rows, days("rfq_offer", 300, 20, 22, 18, 20)...)
	rows = append(rows, days("olx", 0, 3, 2, 4)...) // too quiet to matter
	got := kinds(FlowSignals(rows))
	if s, ok := got["source_silent|seller"]; !ok || s.Level != "critical" || *s.Baseline != 40 {
		t.Errorf("seller silent: %+v", s)
	}
	if s, ok := got["source_drop|telegram_channel"]; !ok || *s.Value != 9 {
		t.Errorf("channel drop: %+v", s)
	}
	if _, ok := got["source_spike|rfq_offer"]; !ok {
		t.Errorf("rfq spike missing: %+v", got)
	}
	if len(got) != 3 {
		t.Errorf("signals %+v", got)
	}
	// A normal day raises nothing.
	if s := FlowSignals(days("seller", append([]int32{41}, usual...)...)); len(s) != 0 {
		t.Errorf("normal day: %+v", s)
	}
}

func TestReviewShare(t *testing.T) {
	var rows []db.SourceDailyCountsRow
	for d := int32(0); d <= 14; d++ {
		r := db.SourceDailyCountsRow{Source: "seller", DaysAgo: d, N: 50, Review: 2}
		if d == 0 {
			r.Review = 25
		}
		rows = append(rows, r)
	}
	if s, ok := kinds(FlowSignals(rows))["review_share|"]; !ok || *s.Value != 0.5 {
		t.Fatalf("review share: %+v", s)
	}
}

func TestJobAndFreshnessSignals(t *testing.T) {
	now := time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)
	done := now.Add(-time.Hour)
	old := now.Add(-5 * time.Hour)
	rows := []db.LatestJobRunsRow{
		{Job: "price_daily", StartedAt: old, FinishedAt: &old},                                                         // overdue
		{Job: "fx_daily", StartedAt: now.Add(-3 * time.Hour)},                                                          // stuck
		{Job: "weather_daily", StartedAt: done, FinishedAt: &done, Error: pgtype.Text{String: "timeout", Valid: true}}, // failed
		{Job: "watch_alerts", StartedAt: done, FinishedAt: &done},                                                      // fine
	}
	got := kinds(JobSignals(rows, now, map[string]time.Duration{"price_daily": 3 * time.Hour, "watch_alerts": 18 * time.Hour}))
	for _, k := range []string{"job_overdue|price_daily", "job_stuck|fx_daily", "job_failed|weather_daily"} {
		if _, ok := got[k]; !ok {
			t.Errorf("missing %s in %+v", k, got)
		}
	}
	if len(got) != 3 {
		t.Errorf("signals %+v", got)
	}

	today := time.Date(2026, 9, 26, 0, 0, 0, 0, time.UTC)
	fresh := FreshnessSignals(db.DataFreshnessRow{
		Fx:      pgtype.Date{Time: today.AddDate(0, 0, -6), Valid: true},
		Weather: pgtype.Date{Time: today, Valid: true},
	}, today)
	if len(fresh) != 1 || fresh[0].Kind != "fx_stale" || *fresh[0].Value != 6 {
		t.Errorf("freshness %+v", fresh)
	}
	if m := FreshnessSignals(db.DataFreshnessRow{}, today); len(m) != 2 || m[0].Level != "medium" {
		t.Errorf("missing data %+v", m)
	}
}

// Run keeps one open alert per problem, notifies serious ones once and
// resolves them when they are gone.
func TestRunKeepsAlerts(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	pool, err := store.Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err := store.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	clean := func() {
		_, _ = pool.Exec(ctx, `DELETE FROM monitoring_alerts`)
		_, _ = pool.Exec(ctx, `DELETE FROM job_runs WHERE job = 'monitor_test_job'`)
	}
	clean()
	t.Cleanup(clean)
	if _, err := pool.Exec(ctx, `INSERT INTO job_runs (job, slot, started_at, finished_at, error)
		VALUES ('monitor_test_job', now() - interval '1 hour', now() - interval '1 hour', now() - interval '59 minutes', 'boom <b>')`); err != nil {
		t.Fatal(err)
	}
	s := NewService(pool, nil)
	var sent [][]db.MonitoringAlert
	s.Notify = func(_ context.Context, a []db.MonitoringAlert) { sent = append(sent, a) }
	for i := 0; i < 2; i++ {
		if _, err := s.Run(ctx); err != nil {
			t.Fatal(err)
		}
	}
	var open int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM monitoring_alerts WHERE kind = 'job_failed' AND subject = 'monitor_test_job' AND resolved_at IS NULL`).Scan(&open); err != nil || open != 1 {
		t.Fatalf("open alerts %d %v", open, err)
	}
	found := 0
	for _, batch := range sent {
		for _, a := range batch {
			if a.Subject == "monitor_test_job" {
				found++
			}
		}
	}
	if found != 1 {
		t.Fatalf("notified %d times, want once", found)
	}
	if txt := Text(sent[0]); !strings.Contains(txt, "&lt;b&gt;") {
		t.Errorf("message not escaped: %s", txt)
	}
	// The job succeeds: the alert is resolved.
	if _, err := pool.Exec(ctx, `INSERT INTO job_runs (job, slot, finished_at) VALUES ('monitor_test_job', now(), now())`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Run(ctx); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM monitoring_alerts WHERE subject = 'monitor_test_job' AND resolved_at IS NULL`).Scan(&open); err != nil || open != 0 {
		t.Fatalf("still open %d %v", open, err)
	}
}

func TestReferenceSignals(t *testing.T) {
	now := time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)
	rows := []db.ReferenceFreshnessRow{
		{Source: "uzex", FetchedAt: now.Add(-5 * 24 * time.Hour)},
		{Source: "narxtahlil", FetchedAt: now.Add(-time.Hour)},
	}
	// Only enabled sources are checked.
	if got := ReferenceSignals(rows, now, nil); len(got) != 0 {
		t.Errorf("nothing enabled: %+v", got)
	}
	got := ReferenceSignals(rows, now, map[string]time.Duration{"uzex": 3 * 24 * time.Hour, "narxtahlil": 3 * 24 * time.Hour})
	if len(got) != 1 || got[0].Kind != "reference_stale" || got[0].Subject != "uzex" || *got[0].Value != 5 {
		t.Errorf("stale uzex: %+v", got)
	}
	got = ReferenceSignals(nil, now, map[string]time.Duration{"uzex": 3 * 24 * time.Hour})
	if len(got) != 1 || got[0].Kind != "reference_missing" {
		t.Errorf("never loaded: %+v", got)
	}
}

func TestChatSignals(t *testing.T) {
	if s := ChatSignals(db.ChatHealthRow{Answers: 10, Unanswered: 9}); len(s) != 0 {
		t.Errorf("too few replies to judge: %+v", s)
	}
	if s := ChatSignals(db.ChatHealthRow{Answers: 40, Unanswered: 8}); len(s) != 0 {
		t.Errorf("20%% unanswered is fine: %+v", s)
	}
	if s := ChatSignals(db.ChatHealthRow{Answers: 40, Unanswered: 16}); len(s) != 1 || s[0].Kind != "chat_unanswered" || *s[0].Value != 0.4 {
		t.Errorf("40%% unanswered: %+v", s)
	}
}
