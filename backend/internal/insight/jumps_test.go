package insight

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestJumpsOf(t *testing.T) {
	mon := time.Date(2026, 2, 2, 0, 0, 0, 0, time.UTC)
	weeks := []Point{{mon, 5000}, {mon.AddDate(0, 0, 7), 5100}, {mon.AddDate(0, 0, 14), 6600}, {mon.AddDate(0, 0, 21), 6500}, {mon.AddDate(0, 0, 42), 4000}}
	j := JumpsOf(weeks)
	// +29% in week 3 is a jump; the fall to 4 000 is after a gap, not a week-on-week move.
	if len(j) != 1 || j[0].Week != "2026-02-16" || j[0].Change != 0.294 {
		t.Errorf("jumps %+v", j)
	}
	days := []Point{{time.Date(2026, 2, 4, 0, 0, 0, 0, time.UTC), 10}, {time.Date(2026, 2, 8, 0, 0, 0, 0, time.UTC), 20}, {time.Date(2026, 2, 9, 0, 0, 0, 0, time.UTC), 40}}
	if w := weekly(days); len(w) != 2 || w[0].Day.Weekday() != time.Monday || w[0].Median != 15 || w[1].Median != 40 {
		t.Errorf("weekly %+v", w)
	}
}

// A frost event next to a price jump is shown with it.
func TestJumpsWithEvents(t *testing.T) {
	svc, pool, _ := setup(t)
	ctx := context.Background()
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, sql, args...); err != nil {
			t.Fatal(err)
		}
	}
	const product = 19
	exec(`DELETE FROM price_daily WHERE product_id = $1`, product)
	exec(`DELETE FROM market_events WHERE title LIKE 'test-%'`)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM price_daily WHERE product_id = $1`, product)
		_, _ = pool.Exec(context.Background(), `DELETE FROM market_events WHERE title LIKE 'test-%'`)
	})
	today := Day(time.Now()).Time
	jumpDay := today.AddDate(0, 0, -60)
	for i := 0; i < 120; i++ {
		day := today.AddDate(0, 0, -i)
		price := 5000.0
		if !day.Before(jumpDay) && day.Before(jumpDay.AddDate(0, 0, 14)) {
			price = 7000
		}
		exec(`INSERT INTO price_daily (product_id, trade_type, province_id, day, median, p25, p75, min_price, max_price, offers, demo_share, synthetic)
			VALUES ($1, 'ulgurji', 1, $2, $3, $3, $3, 1, 1, 5, 0, false)`, product, date(day), price)
	}
	cat := ""
	if err := pool.QueryRow(ctx, `SELECT category FROM products WHERE id = $1`, product).Scan(&cat); err != nil {
		t.Fatal(err)
	}
	exec(`INSERT INTO market_events (title, kind, starts_on, category) VALUES ('test-sovuq', 'frost', $1, $2)`, date(jumpDay.AddDate(0, 0, -2)), cat)
	exec(`INSERT INTO market_events (title, kind, starts_on, product_id) VALUES ('test-boshqa', 'export', $1, 1)`, date(jumpDay))
	exec(`INSERT INTO market_events (title, kind, starts_on) VALUES ('test-uzoq', 'fuel', $1)`, date(jumpDay.AddDate(0, 0, -20))) // in the period, far from the jump

	a, err := svc.Analytics(ctx, Params{ProductID: product, Province: 1, Trade: "ulgurji", Period: "90d", Transport: 60})
	if err != nil {
		t.Fatal(err)
	}
	// The holiday calendar may add its own rows depending on today's date;
	// only the test events are checked.
	var up *Jump
	for i := range a.Jumps {
		if a.Jumps[i].Change > 0 {
			up = &a.Jumps[i]
		}
	}
	var linked []string
	if up != nil {
		for _, e := range up.Events {
			if e.Kind != "holiday" {
				linked = append(linked, e.Title)
			}
		}
	}
	if len(linked) != 1 || linked[0] != "test-sovuq" {
		t.Fatalf("jumps %+v", a.Jumps)
	}
	titles := map[string]bool{}
	for _, e := range a.Events {
		titles[e.Title] = true
	}
	if !titles["test-sovuq"] || !titles["test-uzoq"] || titles["test-boshqa"] {
		t.Errorf("events of the period %+v: want the frost and the fuel, not another product's", a.Events)
	}
}

// A hard frost next to a price jump is linked to it; a mild winter day is not an extreme.
func TestJumpsWithWeather(t *testing.T) {
	svc, pool, _ := setup(t)
	ctx := context.Background()
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, sql, args...); err != nil {
			t.Fatal(err)
		}
	}
	const product = 25
	exec(`DELETE FROM price_daily WHERE product_id = $1`, product)
	exec(`DELETE FROM weather_daily`)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM price_daily WHERE product_id = $1`, product)
		_, _ = pool.Exec(context.Background(), `DELETE FROM weather_daily`)
	})
	today := Day(time.Now()).Time
	jumpDay := today.AddDate(0, 0, -40)
	for i := 0; i < 90; i++ {
		day := today.AddDate(0, 0, -i)
		price, tmin := 5000.0, -1.0
		if !day.Before(jumpDay) && day.Before(jumpDay.AddDate(0, 0, 14)) {
			price = 7000
		}
		if day.Equal(jumpDay.AddDate(0, 0, -1)) {
			tmin = -12
		}
		exec(`INSERT INTO price_daily (product_id, trade_type, province_id, day, median, p25, p75, min_price, max_price, offers, demo_share, synthetic)
			VALUES ($1, 'ulgurji', 1, $2, $3, $3, $3, 1, 1, 5, 0, false)`, product, date(day), price)
		exec(`INSERT INTO weather_daily (province_id, day, tmin, tmax, precip) VALUES (1, $1, $2, 10, 2)`, date(day), tmin)
	}
	a, err := svc.Analytics(ctx, Params{ProductID: product, Province: 1, Trade: "ulgurji", Period: "90d", Transport: 60})
	if err != nil {
		t.Fatal(err)
	}
	if a.Weather == nil || len(a.Weather.Points) != 90 || len(a.Weather.Extremes) != 1 || a.Weather.Extremes[0].Temp != -12 {
		t.Fatalf("weather %+v", a.Weather)
	}
	var linked bool
	for _, j := range a.Jumps {
		for _, e := range j.Events {
			if e.Kind == "frost" && strings.Contains(e.Title, "-12°C") {
				linked = true
			}
		}
	}
	if !linked {
		t.Errorf("frost not linked to the jump: %+v", a.Jumps)
	}
}
