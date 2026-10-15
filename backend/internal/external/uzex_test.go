package external

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"bugun/internal/store/db"
)

// testdata/uzex_quotes.json is the real answer of uzex.uz/Quote/GetQuotes on
// 26.09.2026 (pretty-printed).
func uzexFixture(t *testing.T) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/uzex_quotes.json")
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestParseUzExQuotes(t *testing.T) {
	quotes, err := ParseUzExQuotes(uzexFixture(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(quotes) != 27 {
		t.Errorf("%d quotes, want 27", len(quotes))
	}
	byName := map[string]float64{}
	for _, q := range quotes {
		byName[q.Name] = q.Price
	}
	if p := byName["Bug‘doy 3 sinf"]; p != 3222925.54 {
		t.Errorf("wheat = %v", p)
	}

	m := MatchUzEx(quotes)
	got := map[string]int64{}
	for _, x := range m {
		got[x.Slug] = int64(x.BasePrice + 0.5)
	}
	// Per kg: the exchange quotes per tonne.
	want := map[string]int64{"bugdoy": 3223, "armatura": 8627, "sement": 574}
	if len(got) != len(want) {
		t.Errorf("matched %v, want %v", got, want)
	}
	for slug, p := range want {
		if got[slug] != p {
			t.Errorf("%s = %d so'm/kg, want %d", slug, got[slug], p)
		}
	}

	for _, bad := range []string{"", "<html>", "{}", "[]", `[{"name":"x","summa":"n/a"}]`} {
		if _, err := ParseUzExQuotes([]byte(bad)); err == nil {
			t.Errorf("%q parsed", bad)
		}
	}
}

func TestNormalizeQuoteName(t *testing.T) {
	// The exchange mixes Cyrillic look-alikes and several apostrophes into names.
	if a, b := normalizeQuoteName("Bug‘doy  uni 1 nаv"), normalizeQuoteName("bug'doy uni 1 nav"); a != b {
		t.Errorf("%q != %q", a, b)
	}
	for in, want := range map[string]float64{"3 222 925.54": 3222925.54, "1 500": 1500, "12,5": 12.5, "1,500,000": 1500000, "7 620.00": 7620} {
		if got, err := parseSom(in); err != nil || got != want {
			t.Errorf("parseSom(%q) = %v, %v; want %v", in, got, err, want)
		}
	}
	for _, bad := range []string{"", "-5", "0", "abc"} {
		if _, err := parseSom(bad); err == nil {
			t.Errorf("parseSom(%q) accepted", bad)
		}
	}
}

func TestUzExSync(t *testing.T) {
	q, pool := testQueries(t)
	ctx := context.Background()
	if _, err := pool.Exec(ctx, `DELETE FROM reference_prices WHERE source = 'uzex'`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM reference_prices WHERE source = 'uzex'`) })

	body := uzexFixture(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	defer srv.Close()
	// 23:30 UTC is already the next day in Tashkent.
	u := &UzEx{Q: q, Client: srv.Client(), URL: srv.URL, Now: func() time.Time { return time.Date(2026, 9, 25, 23, 30, 0, 0, time.UTC) }}
	for range 2 { // the same day twice: updated, not duplicated
		if n, err := u.Sync(ctx); err != nil || n != 3 {
			t.Fatalf("Sync = %d, %v", n, err)
		}
	}
	var rows int
	var day time.Time
	if err := pool.QueryRow(ctx, `SELECT count(*), max(quoted_on) FROM reference_prices WHERE source = 'uzex'`).Scan(&rows, &day); err != nil {
		t.Fatal(err)
	}
	if rows != 3 || day.Format(time.DateOnly) != "2026-09-26" {
		t.Errorf("%d rows on %s, want 3 on 2026-09-26", rows, day.Format(time.DateOnly))
	}

	wheat, err := q.GetProductBySlug(ctx, "bugdoy")
	if err != nil {
		t.Fatal(err)
	}
	latest, err := q.LatestReferencePrices(ctx, db.LatestReferencePricesParams{ProductID: wheat.ID, TradeType: "ulgurji"})
	if err != nil || len(latest) != 1 || latest[0].PriceSom != 3223 || latest[0].QuoteUnit != "tonna" || latest[0].QuoteSom != 3222925.54 {
		t.Errorf("latest = %+v, %v", latest, err)
	}
	fresh, err := q.ReferenceFreshness(ctx)
	if err != nil || len(fresh) != 1 || fresh[0].Source != "uzex" {
		t.Errorf("freshness = %+v, %v", fresh, err)
	}

	// A list without the expected items fails loudly.
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`[{"name":"Dizel yoqilg‘isi","summa":"16 971 576.05"}]`))
	}))
	defer bad.Close()
	if _, err := (&UzEx{Q: q, Client: bad.Client(), URL: bad.URL}).Sync(ctx); err == nil {
		t.Error("a list with no catalog items synced")
	}
	down := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { http.Error(w, "x", 500) }))
	defer down.Close()
	if _, err := (&UzEx{Q: q, Client: down.Client(), URL: down.URL}).Sync(ctx); err == nil {
		t.Error("a failed request synced")
	}
}
