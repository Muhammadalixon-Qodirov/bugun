package httpapi

import (
	"context"
	"net/http"
	"testing"

	"bugun/internal/store/db"
)

// Benchmarks from a connector are listed per source and feed the review
// check's reference price, without touching medians.
func TestProductReferences(t *testing.T) {
	a := newTestAPI(t)
	ctx := context.Background()
	wheat, err := a.q.GetProductBySlug(ctx, "bugdoy")
	if err != nil {
		t.Fatal(err)
	}
	clean := func() {
		_, _ = a.pool.Exec(context.Background(), `DELETE FROM reference_prices WHERE product_id = $1`, wheat.ID)
	}
	clean()
	t.Cleanup(clean)

	var out struct {
		References []ReferencePrice `json:"references"`
	}
	if code := a.call("", http.MethodGet, "/v1/products/bugdoy/references", nil, &out); code != 200 || len(out.References) != 0 {
		t.Fatalf("before any quote: %d %+v", code, out)
	}
	// An older and a newer national quote.
	if _, err := a.pool.Exec(ctx, `INSERT INTO reference_prices (source, product_id, price_som, quote_name, quote_unit, quote_som, quoted_on)
		VALUES ('uzex', $1, 3100, 'Bug''doy 3 sinf', 'tonna', 3100000, (now() AT TIME ZONE 'Asia/Tashkent')::date - 3),
		       ('uzex', $1, 3223, 'Bug''doy 3 sinf', 'tonna', 3222925.54, (now() AT TIME ZONE 'Asia/Tashkent')::date)`, wheat.ID); err != nil {
		t.Fatal(err)
	}
	if code := a.call("", http.MethodGet, "/v1/products/bugdoy/references?region=2", nil, &out); code != 200 || len(out.References) != 1 {
		t.Fatalf("references: %d %+v", code, out)
	}
	if r := out.References[0]; r.Source != "uzex" || r.PriceSom != 3223 || r.QuoteSom != 3222925.54 || !r.National || r.QuoteUnit != "tonna" {
		t.Errorf("reference = %+v", r)
	}
	if code := a.call("", http.MethodGet, "/v1/products/nope/references", nil, nil); code != http.StatusNotFound {
		t.Errorf("unknown product: %d", code)
	}

	// The national quotes are the benchmark in every viloyat.
	ref, err := a.q.ReferencePrice(ctx, db.ReferencePriceParams{ProvinceID: 2, ProductID: wheat.ID, TradeType: "ulgurji"})
	if err != nil || ref != (3100+3223)/2.0 {
		t.Errorf("ReferencePrice = %v, %v", ref, err)
	}
}
