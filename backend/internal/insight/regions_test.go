package insight

import (
	"context"
	"testing"
	"time"
)

// Two products priced in Namangan (1) and Toshkent shahri (110): the page
// compares with the country, finds the opportunity and builds the index.
func TestRegionOverview(t *testing.T) {
	svc, pool, _ := setup(t)
	ctx := context.Background()
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, sql, args...); err != nil {
			t.Fatal(err)
		}
	}
	exec(`DELETE FROM price_daily`)
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM price_daily`) })
	today := Day(time.Now()).Time
	// Product 1 (sabzavot): 4 000 in Namangan, 6 000 in Toshkent, rising 10%
	// over the year in both. Product 2: 3 000 everywhere.
	for i := 0; i < 371; i++ {
		day := today.AddDate(0, 0, -i)
		grow := 1 + 0.1*float64(371-i)/371
		for _, r := range []struct {
			product, province int16
			price             float64
		}{{1, 1, 4000 * grow}, {1, 110, 6000 * grow}, {2, 1, 3000}, {2, 110, 3000}} {
			exec(`INSERT INTO price_daily (product_id, trade_type, province_id, day, median, p25, p75, min_price, max_price, offers, demo_share, synthetic)
				VALUES ($1, 'ulgurji', $2, $3, $4, $4, $4, 1, 1, 5, 1, true)`, r.product, r.province, date(day), r.price)
		}
	}
	o, err := svc.RegionOverview(ctx, 1, "ulgurji")
	if err != nil {
		t.Fatal(err)
	}
	if o.ProvinceName == "" || len(o.Products) != 2 || o.DemoShare != 1 {
		t.Fatalf("overview: %s, %d products, demo %v", o.ProvinceName, len(o.Products), o.DemoShare)
	}
	var p1 *RegionProduct
	for i := range o.Products {
		if o.Products[i].ID == 1 {
			p1 = &o.Products[i]
		}
	}
	if p1 == nil || p1.VsCountry == nil || *p1.VsCountry != -0.2 {
		t.Errorf("product 1 against the country (5 000): %+v", p1)
	}
	if len(o.Opportunities) != 1 || o.Opportunities[0].ProductID != 1 || o.Opportunities[0].SellInID != 110 || o.Opportunities[0].NetPct < 0.4 {
		t.Errorf("opportunities %+v", o.Opportunities)
	}
	h, err := svc.Heatmap(ctx, "ulgurji", "sabzavot")
	if err != nil {
		t.Fatal(err)
	}
	if len(h.Provinces) != 14 || len(h.Rows) != 2 || h.Rows[0].Product.ID != 1 && h.Rows[1].Product.ID != 1 {
		t.Fatalf("heatmap: %d provinces, %d rows", len(h.Provinces), len(h.Rows))
	}
	filled := 0
	for _, r := range h.Rows {
		for i, c := range r.Cells {
			if c != nil {
				filled++
				if id := h.Provinces[i].ID; id != 1 && id != 110 {
					t.Errorf("cell in viloyat %d without prices", id)
				}
			}
		}
	}
	if filled != 4 || len(h.Up) != 1 || h.Up[0].Product.ID != 1 || len(h.Down) != 0 {
		t.Errorf("heatmap cells %d, up %+v, down %+v", filled, h.Up, h.Down)
	}
	if meat, err := svc.Heatmap(ctx, "ulgurji", "gosht"); err != nil || len(meat.Rows) != 0 {
		t.Errorf("another category: %d rows %v", len(meat.Rows), err)
	}
	if o.IndexBasket < 1 || len(o.Index) < 50 || o.Index[0].Region != 100 || o.Index[len(o.Index)-1].Region < 104 {
		t.Errorf("index: basket %d, %d weeks, first %+v, last %+v", o.IndexBasket, len(o.Index), o.Index[0], o.Index[len(o.Index)-1])
	}
}
