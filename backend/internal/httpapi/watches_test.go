package httpapi

import (
	"context"
	"strconv"
	"testing"
)

func TestWatchEndpoints(t *testing.T) {
	a := newTestAPI(t)
	ctx := context.Background()
	_, tok := a.signIn(900000541, "entrepreneur", 2)
	_, otherTok := a.signIn(900000542, "entrepreneur", 2)

	var out WatchesOutput
	if code := a.call(tok, "POST", "/v1/watches", map[string]any{"product_id": 9, "region_id": 2, "max_price": 5000}, &out.Body); code != 200 {
		t.Fatalf("add: status %d", code)
	}
	if len(out.Body.Watches) != 1 || out.Body.Watches[0].ProvinceID == 0 || out.Body.Watches[0].ProvinceID == 2 || *out.Body.Watches[0].MaxPrice != 5000 {
		t.Errorf("watch (region 2 is a tuman: its viloyat is kept): %+v", out.Body.Watches)
	}
	if code := a.call(tok, "POST", "/v1/watches", map[string]any{"product_id": 9999}, nil); code != 422 {
		t.Errorf("unknown product: status %d, want 422", code)
	}
	var n int
	if err := a.pool.QueryRow(ctx, `SELECT count(*) FROM demand_events WHERE kind = 'watch' AND product_id = 9 AND day = (now() AT TIME ZONE 'Asia/Tashkent')::date`).Scan(&n); err != nil || n < 1 {
		t.Errorf("watch demand signal: %d %v", n, err)
	}
	id := strconv.FormatInt(out.Body.Watches[0].ID, 10)
	if code := a.call(otherTok, "DELETE", "/v1/watches/"+id, nil, nil); code != 404 {
		t.Errorf("someone else's watch: status %d, want 404", code)
	}
	if code := a.call(tok, "DELETE", "/v1/watches/"+id, nil, nil); code != 204 {
		t.Errorf("delete: status %d", code)
	}
	if code := a.call(tok, "GET", "/v1/me/watches", nil, &out.Body); code != 200 || len(out.Body.Watches) != 0 {
		t.Errorf("after delete: status %d, %+v", code, out.Body.Watches)
	}
}
