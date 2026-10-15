package httpapi

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"bugun/internal/market"
	"bugun/internal/store/db"
)

func TestProductAnalyticsEndpoint(t *testing.T) {
	a := newTestAPI(t)
	var out AnalyticsOutput
	if code := a.call("", "GET", "/v1/products/olma/analytics?region=2&period=90d", nil, &out.Body); code != 200 {
		t.Fatalf("analytics: status %d", code)
	}
	if out.Body.Product.Slug != "olma" || out.Body.Scope.Period != "90d" || out.Body.Scope.ProvinceID == 0 || out.Body.Scope.ProvinceID == 2 || out.Body.Series == nil {
		t.Errorf("analytics: %+v %+v", out.Body.Product, out.Body.Scope)
	}
	for path, want := range map[string]int{
		"/v1/products/olma/analytics?period=5y":  422,
		"/v1/products/yoq/analytics":             404,
		"/v1/products/olma/analytics?region=999": 422,
	} {
		if code := a.call("", "GET", path, nil, nil); code != want {
			t.Errorf("%s: status %d, want %d", path, code, want)
		}
	}
}

// The supply block counts live listings, sellers and the stock they gave.
func TestAnalyticsSupply(t *testing.T) {
	a := newTestAPI(t)
	ctx := context.Background()
	for i, stock := range []float64{500, 300} {
		u, _ := a.signIn(900000571+int64(i), "entrepreneur", 2)
		st := stock
		if _, err := a.listings.Upsert(ctx, u, market.Input{ProductID: 20, PriceSom: 5000, TradeType: "ulgurji", Stock: &st}); err != nil {
			t.Fatal(err)
		}
	}
	var out AnalyticsOutput
	if code := a.call("", "GET", "/v1/products/"+slugOf(t, a, 20)+"/analytics?region=2&period=30d", nil, &out.Body); code != 200 {
		t.Fatalf("status %d", code)
	}
	if sp := out.Body.Supply; sp.Listings < 2 || sp.Sellers < 2 || sp.Stock == nil || *sp.Stock < 800 || sp.Stocked < 2 {
		t.Errorf("supply %+v", sp)
	}
}

func slugOf(t *testing.T, a *testAPI, id int16) string {
	t.Helper()
	p, err := a.q.GetProduct(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	return p.Slug
}

func TestCompareAndCSV(t *testing.T) {
	a := newTestAPI(t)
	var c CompareOutput
	if code := a.call("", "GET", "/v1/analytics/compare?products=olma,nok&period=90d", nil, &c.Body); code != 200 || len(c.Body.Lines) != 2 || c.Body.Lines[1].Product.Slug != "nok" {
		t.Errorf("two products: %d %+v", code, c.Body.Lines)
	}
	if code := a.call("", "GET", "/v1/analytics/compare?products=olma&regions=1,2&period=1y", nil, &c.Body); code != 200 || len(c.Body.Lines) != 2 || c.Body.Lines[0].ProvinceID == 0 || c.Body.Bucket != "week" {
		t.Errorf("two viloyatlar: %d %+v", code, c.Body)
	}
	for path, want := range map[string]int{
		"/v1/analytics/compare?products=olma,nok,uzum&regions=1,2": 422, // 6 lines
		"/v1/analytics/compare?products=yoq":                       422,
		"/v1/analytics/compare?products=olma&regions=x":            422,
	} {
		if code := a.call("", "GET", path, nil, nil); code != want {
			t.Errorf("%s: %d, want %d", path, code, want)
		}
	}

	res, err := http.Get(a.srv.URL + "/v1/products/olma/analytics.csv?period=30d")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	lines := strings.Split(strings.TrimSpace(string(body)), "\n")
	if res.StatusCode != 200 || !strings.HasPrefix(res.Header.Get("Content-Type"), "text/csv") ||
		!strings.Contains(res.Header.Get("Content-Disposition"), "bugun-olma-0-30d.csv") ||
		!strings.HasPrefix(string(body), "\ufeffmahsulot,hudud,") {
		t.Errorf("csv: %d %s %s %q", res.StatusCode, res.Header.Get("Content-Type"), res.Header.Get("Content-Disposition"), lines[0])
	}
}

// A reason a seller gave in the bot is counted on the analytics page.
func TestAnalyticsReasons(t *testing.T) {
	a := newTestAPI(t)
	ctx := context.Background()
	u, _ := a.signIn(900000581, "entrepreneur", 2)
	other, _ := a.signIn(900000582, "entrepreneur", 2)
	saved, err := a.listings.Upsert(ctx, u, market.Input{ProductID: 26, PriceSom: 5000, TradeType: "ulgurji"})
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range []string{"supply_low", "demand_up"} { // same day: the second replaces the first
		if err := a.q.RecordPriceReason(ctx, db.RecordPriceReasonParams{Direction: "up", Change: 0.2, Reason: r, ListingID: saved.ID, SellerID: u.ID}); err != nil {
			t.Fatal(err)
		}
	}
	// Another seller cannot answer for this listing.
	if err := a.q.RecordPriceReason(ctx, db.RecordPriceReasonParams{Direction: "down", Change: -0.2, Reason: "import", ListingID: saved.ID, SellerID: other.ID}); err != nil {
		t.Fatal(err)
	}
	var out AnalyticsOutput
	if code := a.call("", "GET", "/v1/products/"+slugOf(t, a, 26)+"/analytics?region=2&period=30d", nil, &out.Body); code != 200 {
		t.Fatalf("status %d", code)
	}
	if r := out.Body.Reasons; len(r) != 1 || r[0].Code != "demand_up" || r[0].Count != 1 || r[0].Label == "" {
		t.Errorf("reasons %+v", r)
	}
}
