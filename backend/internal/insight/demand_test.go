package insight

import (
	"context"
	"testing"
)

func TestTopDemand(t *testing.T) {
	svc, pool, _ := setup(t)
	ctx := context.Background()
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, sql, args...); err != nil {
			t.Fatal(err)
		}
	}
	exec(`DELETE FROM demand_events`)
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM demand_events`) })
	add := func(product int16, kind string, n, daysAgo int) {
		exec(`INSERT INTO demand_events (kind, product_id, province_id, viewer_key, created_at)
			SELECT $1, $2::smallint, 1, int4send(g + $5::int * 1000 + $2::int * 100000), now() - make_interval(days => $4::int)
			FROM generate_series(1, $3) g`, kind, product, n, daysAgo, daysAgo)
	}
	add(21, "view", 10, 1)   // 10 this week
	add(21, "rfq", 2, 2)     // +10
	add(21, "view", 4, 10)   // 4 a week before: usual 1 a week over 4 weeks
	add(22, "search", 20, 1) // 40, steady:
	for _, d := range []int{8, 15, 22, 29} {
		add(22, "search", 20, d)
	}
	add(23, "view", 3, 1) // too little
	td, err := svc.TopDemand(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(td.Top) != 2 || td.Top[0].Product.ID != 22 || td.Top[0].Score != 40 || td.Top[1].Score != 20 {
		t.Fatalf("top %+v", td.Top)
	}
	if len(td.Rising) != 1 || td.Rising[0].Product.ID != 21 || td.Rising[0].Growth == nil || *td.Rising[0].Growth != 19 {
		t.Errorf("rising %+v (steady demand is not rising)", td.Rising)
	}
	if country, err := svc.TopDemand(ctx, 0); err != nil || len(country.Top) != 2 {
		t.Errorf("country: %+v %v", country, err)
	}
}
