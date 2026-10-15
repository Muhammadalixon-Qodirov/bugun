package insight

import (
	"context"
	"testing"
	"time"
)

// Two years of a known daily history in one viloyat and a dearer one
// elsewhere: series, records per year, changes and the demo share follow.
func TestAnalytics(t *testing.T) {
	svc, pool, _ := setup(t)
	ctx := context.Background()
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, sql, args...); err != nil {
			t.Fatal(err)
		}
	}
	const product = 18
	exec(`DELETE FROM price_daily WHERE product_id = $1`, product)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM price_daily WHERE product_id = $1`, product)
	})
	today := Day(time.Now()).Time
	// Viloyat 1: 5 000 every day, 6 000 in the last week; one spike of 9 000
	// 200 days ago. Viloyat 100: always 7 000 (so the country maximum is there).
	for i := 0; i < 730; i++ {
		day := today.AddDate(0, 0, -i)
		price := 5000.0
		switch {
		case i < 7:
			price = 6000
		case i == 200:
			price = 9000
		}
		exec(`INSERT INTO price_daily (product_id, trade_type, province_id, day, median, p25, p75, min_price, max_price, offers, demo_share, synthetic)
			VALUES ($1, 'ulgurji', 1, $2, $3, $3, $3, $4, $4, 5, 1, true),
			       ($1, 'ulgurji', 100, $2, 7000, 7000, 7000, 7000, 7000, 5, 0, false)`, product, date(day), price, int64(price))
	}

	a, err := svc.Analytics(ctx, Params{ProductID: product, Province: 1, Trade: "ulgurji", Period: "1y"})
	if err != nil {
		t.Fatal(err)
	}
	if a.Scope.ProvinceName == "" || a.Scope.Bucket != "day" || len(a.Series) != 365 {
		t.Errorf("scope %+v, %d points", a.Scope, len(a.Series))
	}
	if a.Changes.D7 == nil || *a.Changes.D7 != 0.2 || a.Changes.Y1 == nil || *a.Changes.Y1 != 0.2 {
		t.Errorf("changes %+v", a.Changes)
	}
	if a.Records.Max == nil || a.Records.Max.Price != 9000 || a.Records.Max.Date != today.AddDate(0, 0, -200).Format(time.DateOnly) {
		t.Errorf("max record %+v", a.Records.Max)
	}
	if a.Quality.DemoShare != 1 || a.Quality.HistoryDemoShare != 1 {
		t.Errorf("demo share %v, history %v, want 1 for a generated history", a.Quality.DemoShare, a.Quality.HistoryDemoShare)
	}
	if a.Forecast != nil && a.Forecast.DemoShare != 1 {
		t.Errorf("forecast demo share %v, want 1", a.Forecast.DemoShare)
	}

	// Comparing the synthetic viloyat with the live one marks each line.
	cmp, err := svc.Compare(ctx, []LineSpec{{ProductID: product, Province: 1}, {ProductID: product, Province: 100}}, "ulgurji", "90d")
	if err != nil {
		t.Fatal(err)
	}
	if len(cmp.Lines) != 2 || cmp.Lines[0].DemoShare != 1 || cmp.Lines[1].DemoShare != 0 || cmp.DemoShare != 0.5 {
		t.Errorf("compare demo shares: overall %v, lines %+v", cmp.DemoShare, cmp.Lines)
	}

	// The country: weekly buckets over 3 years, the maximum in the dear viloyat,
	// half of it demo.
	c, err := svc.Analytics(ctx, Params{ProductID: product, Trade: "ulgurji", Period: "3y"})
	if err != nil {
		t.Fatal(err)
	}
	if c.Scope.Bucket != "week" || len(c.Series) < 100 || len(c.Series) > 110 {
		t.Errorf("3y: bucket %s, %d points", c.Scope.Bucket, len(c.Series))
	}
	if c.Records.Max == nil || c.Records.Max.Price != 9000 || c.Records.Min == nil || c.Records.Min.ProvinceID != 1 {
		t.Errorf("country records %+v %+v", c.Records.Max, c.Records.Min)
	}
	if len(c.Records.ByYear) < 2 || c.Quality.DemoShare != 0.5 {
		t.Errorf("years %d, demo share %v", len(c.Records.ByYear), c.Quality.DemoShare)
	}
	// Regional: two viloyatlar ranked, the chosen one as the reference, and
	// buying in Namangan to sell in Andijon pays after the road.
	r, err := svc.Analytics(ctx, Params{ProductID: product, Province: 1, Trade: "ulgurji", Period: "30d", Transport: 60})
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Provinces) != 2 || r.Provinces[0].ID != 1 || r.Provinces[1].Diff == nil || *r.Provinces[1].Diff <= 0.2 {
		t.Errorf("provinces %+v", r.Provinces)
	}
	if len(r.Arbitrage) != 1 || r.Arbitrage[0].BuyID != 1 || r.Arbitrage[0].SellID != 100 || r.Arbitrage[0].Net <= 0 || r.Arbitrage[0].Net >= r.Arbitrage[0].Spread {
		t.Errorf("arbitrage %+v", r.Arbitrage)
	}
	expensive, err := svc.Analytics(ctx, Params{ProductID: product, Province: 1, Trade: "ulgurji", Period: "30d", Transport: 5000})
	if err != nil {
		t.Fatal(err)
	}
	if len(expensive.Arbitrage) != 0 {
		t.Errorf("arbitrage with a road dearer than the gap: %+v", expensive.Arbitrage)
	}
	if _, err := svc.Analytics(ctx, Params{ProductID: product, Trade: "ulgurji", Period: "5y"}); err == nil {
		t.Error("unknown period accepted")
	}
}

// Dollar rates beside a price that follows them.
func TestAnalyticsFx(t *testing.T) {
	svc, pool, _ := setup(t)
	ctx := context.Background()
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, sql, args...); err != nil {
			t.Fatal(err)
		}
	}
	const product = 24
	exec(`DELETE FROM price_daily WHERE product_id = $1`, product)
	exec(`DELETE FROM fx_daily`)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM price_daily WHERE product_id = $1`, product)
		_, _ = pool.Exec(context.Background(), `DELETE FROM fx_daily`)
	})
	today := Day(time.Now()).Time
	for i := 0; i < 90; i++ {
		day := today.AddDate(0, 0, -i)
		usd := 12000 + 10*float64(90-i) // rising
		exec(`INSERT INTO fx_daily (day, ccy, rate) VALUES ($1, 'USD', $2), ($1, 'RUB', 140)`, date(day), usd)
		exec(`INSERT INTO price_daily (product_id, trade_type, province_id, day, median, p25, p75, min_price, max_price, offers, demo_share, synthetic)
			VALUES ($1, 'ulgurji', 1, $2, $3, $3, $3, 1, 1, 5, 0, false)`, product, date(day), usd/3)
	}
	a, err := svc.Analytics(ctx, Params{ProductID: product, Province: 1, Trade: "ulgurji", Period: "90d", Transport: 60})
	if err != nil {
		t.Fatal(err)
	}
	if a.Fx == nil || len(a.Fx.Rates) != 2 || a.Fx.Rates[0].Ccy != "USD" || len(a.Fx.Rates[0].Points) != 90 || a.Fx.Rates[0].Change == nil || *a.Fx.Rates[0].Change <= 0 {
		t.Fatalf("fx %+v", a.Fx)
	}
	if a.Fx.Correlation == nil || *a.Fx.Correlation < 0.99 {
		t.Errorf("correlation %v, want ~1 for a price that follows the dollar", a.Fx.Correlation)
	}
	if a.Fx.Rates[1].Points != nil {
		t.Error("points for a currency other than the dollar")
	}
}
