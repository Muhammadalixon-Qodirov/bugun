// Package monitor watches the platform itself (ML_REJA.md 3, after HUDUD's
// ml/monitoring.py): whether prices keep coming in from every source as
// usual, whether scheduled jobs run and succeed, and whether the exchange
// rates and weather are fresh. Each problem is a signal; signals become
// alerts, one open alert per kind and subject, and serious new ones go to
// the admins.
package monitor

import (
	"context"
	"fmt"
	"html"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"bugun/internal/ml"
	"bugun/internal/store/db"
	"bugun/internal/tz"
)

// Signal is one problem the check found.
type Signal struct {
	Kind     string
	Subject  string // the source or job; empty when there is only one
	Level    string // medium, high, critical
	Value    *float64
	Baseline *float64
	Message  string
}

func (s Signal) key() string { return s.Kind + "|" + s.Subject }

const (
	// minFlowBaseline: a source with fewer prices on a usual day is too
	// quiet for its silence to mean anything.
	minFlowBaseline = 10
	dropShare       = 0.3 // under 30% of a usual day is a drop
	spikeTimes      = 5   // over 5 times a usual day is a spike
	minSpike        = 50
	// minReviewSample and the review limits: many prices held for review at
	// once usually mean the parser or a reference price broke.
	minReviewSample = 20
	reviewFloor     = 0.3
	reviewTimes     = 3
	stuckAfter      = 2 * time.Hour
	fxStaleDays     = 4 // the Central Bank does not publish on weekends and holidays
	weatherStale    = 3
)

func ptr(x float64) *float64 { return &x }

// FlowSignals compares the last 24 hours of every source with the 14 days
// before, and the share of prices held for review with its usual share.
func FlowSignals(rows []db.SourceDailyCountsRow) []Signal {
	type day struct{ n, review int32 }
	bySource := map[string]*[15]day{}
	for _, r := range rows {
		if r.DaysAgo < 0 || r.DaysAgo > 14 {
			continue
		}
		if bySource[r.Source] == nil {
			bySource[r.Source] = &[15]day{}
		}
		bySource[r.Source][r.DaysAgo] = day{r.N, r.Review}
	}
	sources := make([]string, 0, len(bySource))
	for s := range bySource {
		sources = append(sources, s)
	}
	sort.Strings(sources)
	var out []Signal
	var lastN, lastReview, histN, histReview float64
	for _, src := range sources {
		days := bySource[src]
		hist := make([]float64, 14)
		for i := 1; i <= 14; i++ {
			hist[i-1] = float64(days[i].n)
			histN += float64(days[i].n)
			histReview += float64(days[i].review)
		}
		last := float64(days[0].n)
		lastN += last
		lastReview += float64(days[0].review)
		base := ml.Median(hist)
		switch {
		case base >= minFlowBaseline && last == 0:
			out = append(out, Signal{Kind: "source_silent", Subject: src, Level: "critical", Value: ptr(0), Baseline: ptr(base),
				Message: fmt.Sprintf("«%s» manbasidan oxirgi 24 soatda birorta ham narx kelmadi (odatda kuniga %.0f ta).", src, base)})
		case base >= minFlowBaseline && last < dropShare*base:
			out = append(out, Signal{Kind: "source_drop", Subject: src, Level: "high", Value: ptr(last), Baseline: ptr(base),
				Message: fmt.Sprintf("«%s» manbasidan narxlar keskin kamaydi: oxirgi 24 soatda %.0f ta, odatda %.0f ta.", src, last, base)})
		case last >= minSpike && last > spikeTimes*max(base, 1):
			out = append(out, Signal{Kind: "source_spike", Subject: src, Level: "high", Value: ptr(last), Baseline: ptr(base),
				Message: fmt.Sprintf("«%s» manbasidan narxlar juda koʻpaydi: oxirgi 24 soatda %.0f ta, odatda %.0f ta. Spam yoki ikki marta yuklash boʻlishi mumkin.", src, last, base)})
		}
	}
	if lastN >= minReviewSample {
		share := lastReview / lastN
		usual := 0.0
		if histN > 0 {
			usual = histReview / histN
		}
		if share > max(reviewFloor, reviewTimes*usual) {
			out = append(out, Signal{Kind: "review_share", Level: "high", Value: ptr(share), Baseline: ptr(usual),
				Message: fmt.Sprintf("Oxirgi 24 soatda narxlarning %.0f%% i tekshiruvga tushdi (odatda %.0f%%). Parser yoki etalon narx buzilgan boʻlishi mumkin.", share*100, usual*100)})
		}
	}
	return out
}

// JobSignals finds jobs whose last run failed, hangs, or is overdue; gaps
// holds the longest normal pause between runs of each job.
func JobSignals(rows []db.LatestJobRunsRow, now time.Time, gaps map[string]time.Duration) []Signal {
	var out []Signal
	for _, r := range rows {
		switch {
		case r.FinishedAt == nil && now.Sub(r.StartedAt) > stuckAfter:
			out = append(out, Signal{Kind: "job_stuck", Subject: r.Job, Level: "critical",
				Message: fmt.Sprintf("«%s» vazifasi %s dan beri tugamayapti.", r.Job, r.StartedAt.In(tz.Tashkent).Format("02.01 15:04"))})
		case r.Error.Valid:
			msg := r.Error.String
			if len(msg) > 200 {
				msg = msg[:200] + "…"
			}
			out = append(out, Signal{Kind: "job_failed", Subject: r.Job, Level: "high",
				Message: fmt.Sprintf("«%s» vazifasi oxirgi marta xato bilan tugadi: %s", r.Job, msg)})
		}
		if gap, ok := gaps[r.Job]; ok && now.Sub(r.StartedAt) > gap {
			out = append(out, Signal{Kind: "job_overdue", Subject: r.Job, Level: "high",
				Message: fmt.Sprintf("«%s» vazifasi %s dan beri ishga tushmadi.", r.Job, r.StartedAt.In(tz.Tashkent).Format("02.01 15:04"))})
		}
	}
	return out
}

// Chat assistant limits: with at least this many replies a day, more than
// this share left unanswered means a gap (a missing product or function).
const (
	minChatAnswers     = 20
	maxUnansweredShare = 0.3
)

// ChatSignals watches the share of questions the assistant could not answer.
func ChatSignals(c db.ChatHealthRow) []Signal {
	if c.Answers < minChatAnswers {
		return nil
	}
	share := float64(c.Unanswered) / float64(c.Answers)
	if share <= maxUnansweredShare {
		return nil
	}
	return []Signal{{Kind: "chat_unanswered", Level: "high", Value: ptr(share), Baseline: ptr(maxUnansweredShare),
		Message: fmt.Sprintf("Yordamchi oxirgi 24 soatda savollarning %.0f%% iga javob bera olmadi (%d / %d). Roʻyxat: admin panel → Yordamchi.", share*100, c.Unanswered, c.Answers)}}
}

// FreshnessSignals checks that exchange rates and weather keep arriving.
func FreshnessSignals(f db.DataFreshnessRow, today time.Time) []Signal {
	var out []Signal
	check := func(kind, what string, d pgtype.Date, stale int) {
		switch {
		case !d.Valid:
			out = append(out, Signal{Kind: kind + "_missing", Level: "medium", Message: what + " hali yuklanmagan."})
		case today.Sub(d.Time) > time.Duration(stale)*24*time.Hour:
			days := float64(int(today.Sub(d.Time).Hours() / 24))
			out = append(out, Signal{Kind: kind + "_stale", Level: "high", Value: ptr(days),
				Message: fmt.Sprintf("%s eskirgan: oxirgi kun %s (%.0f kun oldin).", what, d.Time.Format("02.01.2006"), days)})
		}
	}
	check("fx", "Markaziy bank kursi", f.Fx, fxStaleDays)
	check("weather", "Ob-havo maʼlumoti", f.Weather, weatherStale)
	return out
}

// ReferenceSignals checks that every enabled benchmark source (UzEx, ...)
// keeps delivering: stale holds the longest normal age of each source's
// latest quote.
func ReferenceSignals(rows []db.ReferenceFreshnessRow, now time.Time, stale map[string]time.Duration) []Signal {
	last := map[string]time.Time{}
	for _, r := range rows {
		last[r.Source] = r.FetchedAt
	}
	sources := make([]string, 0, len(stale))
	for src := range stale {
		sources = append(sources, src)
	}
	sort.Strings(sources)
	var out []Signal
	for _, src := range sources {
		t, ok := last[src]
		switch {
		case !ok:
			out = append(out, Signal{Kind: "reference_missing", Subject: src, Level: "medium",
				Message: fmt.Sprintf("«%s» etalon narxlari hali yuklanmagan.", src)})
		case now.Sub(t) > stale[src]:
			days := float64(int(now.Sub(t).Hours() / 24))
			out = append(out, Signal{Kind: "reference_stale", Subject: src, Level: "high", Value: ptr(days),
				Message: fmt.Sprintf("«%s» etalon narxlari eskirgan: oxirgi marta %s da olingan.", src, t.In(tz.Tashkent).Format("02.01 15:04"))})
		}
	}
	return out
}

// Service runs the checks and keeps the alerts.
type Service struct {
	q    *db.Queries
	gaps map[string]time.Duration
	// References are the enabled benchmark sources and how old their latest
	// quote may get; nil checks none.
	References map[string]time.Duration
	// Notify, when set, gets the serious alerts not sent yet; they count as
	// sent afterwards.
	Notify func(ctx context.Context, alerts []db.MonitoringAlert)
}

// NewService watches the jobs in gaps for being overdue.
func NewService(pool *pgxpool.Pool, gaps map[string]time.Duration) *Service {
	return &Service{q: db.New(pool), gaps: gaps}
}

// Check returns every problem seen now.
func (s *Service) Check(ctx context.Context, now time.Time) ([]Signal, error) {
	flows, err := s.q.SourceDailyCounts(ctx)
	if err != nil {
		return nil, err
	}
	jobs, err := s.q.LatestJobRuns(ctx)
	if err != nil {
		return nil, err
	}
	fresh, err := s.q.DataFreshness(ctx)
	if err != nil {
		return nil, err
	}
	t := now.In(tz.Tashkent)
	today := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
	out := FlowSignals(flows)
	out = append(out, JobSignals(jobs, now, s.gaps)...)
	out = append(out, FreshnessSignals(fresh, today)...)
	if len(s.References) > 0 {
		refs, err := s.q.ReferenceFreshness(ctx)
		if err != nil {
			return nil, err
		}
		out = append(out, ReferenceSignals(refs, now, s.References)...)
	}
	chat, err := s.q.ChatHealth(ctx)
	if err != nil {
		return nil, err
	}
	return append(out, ChatSignals(chat)...), nil
}

// Run checks, records the alerts, resolves those gone and notifies the
// admins. It returns how many problems are open.
func (s *Service) Run(ctx context.Context) (int, error) {
	signals, err := s.Check(ctx, time.Now())
	if err != nil {
		return 0, err
	}
	keys := make([]string, 0, len(signals))
	for _, sg := range signals {
		if _, err := s.q.UpsertAlert(ctx, db.UpsertAlertParams{
			Kind: sg.Kind, Subject: sg.Subject, Level: sg.Level, Value: float8(sg.Value), Baseline: float8(sg.Baseline), Message: sg.Message,
		}); err != nil {
			return 0, err
		}
		keys = append(keys, sg.key())
	}
	if _, err := s.q.ResolveAlertsExcept(ctx, keys); err != nil {
		return 0, err
	}
	if s.Notify != nil {
		alerts, err := s.q.AlertsToNotify(ctx)
		if err != nil {
			return 0, err
		}
		if len(alerts) > 0 {
			s.Notify(ctx, alerts)
			ids := make([]int64, len(alerts))
			for i, a := range alerts {
				ids[i] = a.ID
			}
			if err := s.q.MarkAlertsNotified(ctx, ids); err != nil {
				return 0, err
			}
		}
	}
	return len(signals), nil
}

func float8(x *float64) pgtype.Float8 {
	if x == nil {
		return pgtype.Float8{}
	}
	return pgtype.Float8{Float64: *x, Valid: true}
}

// Text is the admins' message for alerts.
func Text(alerts []db.MonitoringAlert) string {
	var sb strings.Builder
	sb.WriteString("⚠️ Tizim monitoringi:\n")
	for _, a := range alerts {
		mark := "🟠"
		if a.Level == "critical" {
			mark = "🔴"
		}
		sb.WriteString("\n" + mark + " " + html.EscapeString(a.Message))
	}
	return sb.String()
}
