package insight

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"bugun/internal/store"
	"bugun/internal/store/db"
	"bugun/internal/tz"
)

func setup(t *testing.T) (*Service, *pgxpool.Pool, *db.Queries) {
	t.Helper()
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
	return NewService(pool), pool, db.New(pool)
}

// A price counts on every Tashkent day it was live; held prices and
// benchmarks never count.
func TestRebuildDaily(t *testing.T) {
	svc, pool, _ := setup(t)
	ctx := context.Background()
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, sql, args...); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM price_observations WHERE raw_text = 'daily-test'`)
	})
	// Three days ago 10:00 Tashkent, live for two days: 4000, 5000 and 6000
	// (seller), plus a held 90000 and a benchmark 1000 that must not count.
	base := time.Now().In(tz.Tashkent).AddDate(0, 0, -3)
	start := time.Date(base.Year(), base.Month(), base.Day(), 10, 0, 0, 0, tz.Tashkent)
	for _, p := range []struct {
		price  int
		source string
		status string
	}{{4000, "seller", "expired"}, {5000, "seller", "expired"}, {6000, "seller", "expired"}, {90000, "seller", "review"}, {1000, "uzex", "expired"}} {
		exec(`INSERT INTO price_observations (product_id, price_som, unit, trade_type, region_id, source, observed_at, valid_until, status, raw_text)
			VALUES (17, $1, 'kg', 'ulgurji', 2, $2, $3, $4, $5, 'daily-test')`, p.price, p.source, start, start.Add(48*time.Hour), p.status)
	}
	if _, err := svc.RebuildDaily(ctx, start.AddDate(0, 0, -1), time.Now()); err != nil {
		t.Fatal(err)
	}
	rows, err := pool.Query(ctx, `
		SELECT day, offers, min_price, max_price FROM price_daily d
		WHERE product_id = 17 AND trade_type = 'ulgurji' AND province_id = (SELECT COALESCE(parent_id, id) FROM regions WHERE id = 2)
		ORDER BY day`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var days []string
	for rows.Next() {
		var day time.Time
		var offers int
		var lo, hi int64
		if err := rows.Scan(&day, &offers, &lo, &hi); err != nil {
			t.Fatal(err)
		}
		if lo != 4000 || hi != 6000 {
			t.Errorf("%s: %d–%d, want 4 000–6 000 (held and benchmark left out)", day.Format(time.DateOnly), lo, hi)
		}
		days = append(days, day.Format(time.DateOnly))
	}
	// Live from day 0 10:00 to day 2 10:00: three calendar days.
	if want := []string{start.Format(time.DateOnly), start.AddDate(0, 0, 1).Format(time.DateOnly), start.AddDate(0, 0, 2).Format(time.DateOnly)}; len(days) != 3 || days[0] != want[0] || days[2] != want[2] {
		t.Errorf("days %v, want %v", days, want)
	}
	// Rebuilding the same range gives the same rows, not duplicates.
	if _, err := svc.RebuildDaily(ctx, start.AddDate(0, 0, -1), time.Now()); err != nil {
		t.Fatal(err)
	}
	// Generated history survives a rebuild; a day with 3 real prices replaces it.
	exec(`DELETE FROM price_daily WHERE product_id = 17`)
	for _, d := range []time.Time{start.AddDate(0, 0, -1), start} {
		exec(`INSERT INTO price_daily (product_id, trade_type, province_id, day, median, p25, p75, min_price, max_price, offers, demo_share, synthetic)
			VALUES (17, 'ulgurji', (SELECT COALESCE(parent_id, id) FROM regions WHERE id = 2), $1, 7777, 7000, 8000, 6000, 9000, 9, 1, true)`, Day(d))
	}
	if _, err := svc.RebuildDaily(ctx, start.AddDate(0, 0, -1), time.Now()); err != nil {
		t.Fatal(err)
	}
	var kept, replaced float64
	if err := pool.QueryRow(ctx, `SELECT median FROM price_daily WHERE product_id = 17 AND day = $1`, Day(start.AddDate(0, 0, -1))).Scan(&kept); err != nil || kept != 7777 {
		t.Errorf("generated day without real prices: %v %v, want kept 7 777", kept, err)
	}
	if err := pool.QueryRow(ctx, `SELECT median FROM price_daily WHERE product_id = 17 AND day = $1 AND NOT synthetic`, Day(start)).Scan(&replaced); err != nil || replaced != 5000 {
		t.Errorf("day with real prices: %v %v, want the real median 5 000", replaced, err)
	}
	exec(`DELETE FROM price_daily WHERE product_id = 17`)
}
