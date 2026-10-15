package insight

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"bugun/internal/store/db"
)

// row builds a daily row; wobble keeps the series from being perfectly flat.
func row(province int16, day time.Time, median float64) db.DailyForAnomaliesRow {
	return db.DailyForAnomaliesRow{ProductID: 1, TradeType: "ulgurji", ProvinceID: province, Day: pgtype.Date{Time: day, Valid: true}, Median: median}
}

func wobble(i int, province int16) float64 {
	return 1 + 0.02*math.Sin(float64(i)*1.7+float64(province))
}

func TestDetectAnomalies(t *testing.T) {
	end := time.Date(2026, 9, 26, 0, 0, 0, 0, time.UTC)
	from := end.AddDate(0, 0, -30)
	spike := end.AddDate(0, 0, -20) // province 1: one wrong day (a typo ×2)
	level := end.AddDate(0, 0, -15) // province 2: a real new level that lasts
	cross := end.AddDate(0, 0, -10) // province 3: jumps 40% while others stay
	recent := end.AddDate(0, 0, -1) // province 4: a jump too recent to judge
	var rows []db.DailyForAnomaliesRow
	for p := int16(1); p <= 7; p++ {
		for i := 60; i >= 0; i-- {
			d := end.AddDate(0, 0, -i)
			v := 5000 * wobble(i, p)
			switch {
			case p == 1 && d.Equal(spike):
				v *= 2
			case p == 2 && !d.Before(level):
				v *= 1.4
			case p == 3 && d.Equal(cross):
				v *= 1.4
			case p == 4 && d.Equal(recent):
				v *= 1.8
			}
			rows = append(rows, row(p, d, v))
		}
	}
	got := map[string]Anomaly{}
	for _, a := range DetectAnomalies(rows, from, nil) {
		got[a.Layer+"|"+a.Day.Format(time.DateOnly)+"|"+string(rune('0'+a.ProvinceID))] = a
	}
	if a, ok := got["temporal|"+spike.Format(time.DateOnly)+"|1"]; !ok || !a.Settled || math.Abs(a.Value/a.Reference-2) > 0.1 {
		t.Errorf("typo day not flagged: %+v", got)
	}
	for k := range got {
		if k[len(k)-1] == '2' {
			t.Errorf("a lasting new level was flagged: %s", k)
		}
	}
	if _, ok := got["cross_section|"+cross.Format(time.DateOnly)+"|3"]; !ok {
		t.Errorf("lone jump not flagged across viloyatlar: %+v", got)
	}
	if a, ok := got["temporal|"+recent.Format(time.DateOnly)+"|4"]; !ok || a.Settled {
		t.Errorf("recent jump must be flagged provisionally: %+v", a)
	}
	// A holiday around the typo day explains it.
	holiday := []db.EventsSinceRow{{StartsOn: pgtype.Date{Time: spike.AddDate(0, 0, -2), Valid: true}, EndsOn: pgtype.Date{Time: spike.AddDate(0, 0, -1), Valid: true},
		Category: pgtype.Text{String: "meva", Valid: true}}}
	for _, a := range DetectAnomalies(rows, from, explainedBy(holiday, "meva")) {
		if a.ProvinceID == 1 && a.Day.Equal(spike) {
			t.Errorf("a day next to a holiday was flagged: %+v", a)
		}
	}
	if len(DetectAnomalies(rows, from, explainedBy(holiday, "sabzavot"))) != len(got) {
		t.Error("another category's holiday must not explain the day")
	}
	// Days before `from` are only reference.
	for _, a := range got {
		if a.Day.Before(from) {
			t.Errorf("flag before from: %+v", a)
		}
	}
	// Configured thresholds: a 40% jump is not enough when 50% is asked for,
	// and 7 viloyatlar are too few when 8 are needed; the ×2 typo still is.
	strict := DefaultAnomalyRules
	strict.MinMove, strict.MinCrossMove, strict.MinProvinces = 0.5, 0.5, 8
	for _, a := range strict.Detect(rows, from, nil) {
		if a.Layer == "cross_section" || a.ProvinceID == 3 {
			t.Errorf("flagged under stricter rules: %+v", a)
		}
	}
	if n := len(strict.Detect(rows, from, nil)); n == 0 || n >= len(got) {
		t.Errorf("stricter rules flagged %d days, want fewer than %d but not none", n, len(got))
	}
}

// A settled flag keeps the day out of price_daily_clean; a dismissed one
// brings it back; the refresh never overrides an admin's decision.
func TestAnomalyView(t *testing.T) {
	svc, pool, q := setup(t)
	ctx := context.Background()
	day := Day(time.Now().AddDate(0, 0, -9)).Time
	clean := func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM price_anomalies WHERE product_id = 17 AND province_id = 1 AND trade_type = 'chakana'`)
		_, _ = pool.Exec(context.Background(), `DELETE FROM price_daily WHERE product_id = 17 AND province_id = 1 AND trade_type = 'chakana' AND synthetic`)
	}
	clean()
	t.Cleanup(clean)
	for i := 40; i >= 1; i-- {
		d := Day(time.Now().AddDate(0, 0, -i)).Time
		v := 3000 * wobble(i, 1)
		if d.Equal(day) {
			v = 9000
		}
		if _, err := pool.Exec(ctx, `INSERT INTO price_daily (product_id, trade_type, province_id, day, median, p25, p75, min_price, max_price, offers, synthetic)
			VALUES (17, 'chakana', 1, $1, $2, $2, $2, $3, $3, 5, TRUE) ON CONFLICT DO NOTHING`, d, v, int64(v)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := svc.RefreshAnomalies(ctx, 30); err != nil {
		t.Fatal(err)
	}
	inClean := func() bool {
		var n int
		if err := pool.QueryRow(ctx, `SELECT count(*) FROM price_daily_clean WHERE product_id = 17 AND province_id = 1 AND trade_type = 'chakana' AND day = $1`, day).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n == 1
	}
	if inClean() {
		t.Fatal("the 9 000 day is still in the clean history")
	}
	n, err := q.DecideAnomaly(ctx, db.DecideAnomalyParams{Status: "dismissed", ProductID: 17, TradeType: "chakana", ProvinceID: 1,
		Day: pgtype.Date{Time: day, Valid: true}, Layer: "temporal"})
	if err != nil || n != 1 {
		t.Fatalf("dismiss: %d %v", n, err)
	}
	if !inClean() {
		t.Fatal("a dismissed day must come back")
	}
	if _, err := svc.RefreshAnomalies(ctx, 30); err != nil {
		t.Fatal(err)
	}
	if !inClean() {
		t.Fatal("the refresh overrode the admin's decision")
	}
}
