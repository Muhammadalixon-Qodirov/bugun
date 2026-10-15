package httpapi

import (
	"context"
	"strconv"
	"testing"

	"bugun/internal/market"
)

func TestListingStatsAndInterest(t *testing.T) {
	a := newTestAPI(t)
	ctx := context.Background()
	seller, tok := a.signIn(900000531, "entrepreneur", 2)
	_, buyerTok := a.signIn(900000532, "entrepreneur", 2)
	_, adminTok := a.signIn(900000533, "admin", 2)
	saved, err := a.listings.Upsert(ctx, seller, market.Input{ProductID: 9, PriceSom: 7000, TradeType: "ulgurji"})
	if err != nil {
		t.Fatal(err)
	}
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := a.pool.Exec(ctx, sql, args...); err != nil {
			t.Fatal(err)
		}
	}
	// 12 visitors today, 3 of them opened the contact, 4 visitors 3 days ago.
	exec(`INSERT INTO listing_events (listing_id, kind, viewer_key) SELECT $1, 'view', int4send(g) FROM generate_series(1, 12) g`, saved.ID)
	exec(`INSERT INTO listing_events (listing_id, kind, viewer_key) SELECT $1, 'contact', int4send(g) FROM generate_series(1, 3) g`, saved.ID)
	exec(`INSERT INTO listing_events (listing_id, kind, viewer_key, day) SELECT $1, 'view', int4send(g), current_date - 3 FROM generate_series(1, 4) g`, saved.ID)
	exec(`INSERT INTO deals (listing_id, product_id, seller_id, source, status, buyer_confirmed_at, seller_confirmed_at, quantity, price_som, unit, region_id)
		SELECT $1, 9, $2, 'seller_marked', 'confirmed', now(), now(), 100, 6900 + g, 'kg', 2 FROM generate_series(1, 5) g`, saved.ID, seller.ID)
	exec(`INSERT INTO demand_events (kind, product_id, province_id, viewer_key) SELECT 'view', 9, (SELECT COALESCE(parent_id, id) FROM regions WHERE id = 2), int4send(1000 + g) FROM generate_series(1, 6) g`)
	t.Cleanup(func() {
		_, _ = a.pool.Exec(context.Background(), `DELETE FROM demand_events WHERE product_id = 9 AND viewer_key IN (SELECT int4send(1000 + g) FROM generate_series(1, 6) g)`)
	})

	path := "/v1/me/listings/" + strconv.FormatInt(saved.ID, 10) + "/stats"
	var st StatsOutput
	if code := a.call(tok, "GET", path, nil, &st.Body); code != 200 {
		t.Fatalf("stats: status %d", code)
	}
	b := st.Body
	if b.Days != 7 || len(b.Series) != 7 || b.Views != 16 || b.Contacts != 3 || b.ContactRate != 0.188 || b.DealsConfirmed != 5 || b.SoldQuantity != 500 {
		t.Errorf("stats: %+v", b)
	}
	if last := b.Series[6]; last.Views != 12 || last.Contacts != 3 {
		t.Errorf("today (last in series): %+v", last)
	}
	if b.Market != nil {
		t.Errorf("market shown with fewer than 5 other listings: %+v", b.Market)
	}
	if code := a.call(tok, "GET", path+"?days=30", nil, &st.Body); code != 200 || len(st.Body.Series) != 30 {
		t.Errorf("30 days: status %d, %d entries", code, len(st.Body.Series))
	}
	if code := a.call(buyerTok, "GET", path, nil, nil); code != 404 {
		t.Errorf("someone else's stats: status %d, want 404", code)
	}

	var mine MyListingsOutput
	if code := a.call(tok, "GET", "/v1/me/listings", nil, &mine.Body); code != 200 {
		t.Fatalf("my listings: status %d", code)
	}
	for _, l := range mine.Body.Listings {
		if l.ID == saved.ID && (l.Funnel7d == nil || l.Funnel7d.Views != 16 || l.Funnel7d.Contacts != 3 || l.Funnel7d.Deals != 5) {
			t.Errorf("funnel_7d: %+v", l.Funnel7d)
		}
	}

	var prod ProductOutput
	if code := a.call("", "GET", "/v1/products/olma?region=2", nil, &prod.Body); code != 200 {
		t.Fatalf("product: status %d", code)
	}
	in := prod.Body.Interest
	if in == nil || in.VisitorsToday == nil || *in.VisitorsToday < 6 || in.Deals7d == nil || *in.Deals7d < 5 || in.DealMedian7d == nil {
		t.Errorf("interest: %+v", in)
	}

	if code := a.call(tok, "GET", "/v1/admin/deals", nil, nil); code != 403 {
		t.Errorf("admin deals for a seller: status %d, want 403", code)
	}
	var ad AdminDealsOutput
	if code := a.call(adminTok, "GET", "/v1/admin/deals?status=confirmed", nil, &ad.Body); code != 200 || len(ad.Body.Deals) < 5 {
		t.Errorf("admin deals: status %d, %d deals", code, len(ad.Body.Deals))
	}
}

func TestSellerRatingAndModeration(t *testing.T) {
	a := newTestAPI(t)
	ctx := context.Background()
	seller, _ := a.signIn(900000551, "entrepreneur", 2)
	buyer, _ := a.signIn(900000552, "entrepreneur", 2)
	_, adminTok := a.signIn(900000553, "admin", 2)
	saved, err := a.listings.Upsert(ctx, seller, market.Input{ProductID: 9, PriceSom: 7000, TradeType: "ulgurji"})
	if err != nil {
		t.Fatal(err)
	}
	var held int64
	for i, r := range []struct {
		stars   int
		comment string
		status  string
	}{{5, "Zo'r", "published"}, {4, "", "published"}, {1, "Aldadi", "held"}} {
		var dealID int64
		if err := a.pool.QueryRow(ctx, `
			INSERT INTO deals (listing_id, product_id, seller_id, buyer_id, source, status, buyer_confirmed_at, seller_confirmed_at, unit, region_id, day)
			VALUES ($1, 9, $2, $3, 'contact', 'confirmed', now(), now(), 'kg', 2, current_date - $4::int) RETURNING id`,
			saved.ID, seller.ID, buyer.ID, i).Scan(&dealID); err != nil {
			t.Fatal(err)
		}
		if err := a.pool.QueryRow(ctx, `INSERT INTO ratings (deal_id, seller_id, buyer_id, stars, comment, status) VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`,
			dealID, seller.ID, buyer.ID, r.stars, r.comment, r.status).Scan(&held); err != nil {
			t.Fatal(err)
		}
	}
	var page ListingPageOutput
	if code := a.call("", "GET", "/v1/listings/"+strconv.FormatInt(saved.ID, 10), nil, &page.Body); code != 200 {
		t.Fatalf("listing page: status %d", code)
	}
	r := page.Body.Seller.Rating
	if r == nil || r.Count != 3 || r.Average != 3.3 || len(r.Comments) != 1 || r.Comments[0].Comment != "Zo'r" {
		t.Errorf("rating: %+v, want 3 ratings, 3.3, only the shown comment", r)
	}

	var list HeldRatingsOutput
	if code := a.call(adminTok, "GET", "/v1/admin/ratings/held", nil, &list.Body); code != 200 || len(list.Body.Ratings) == 0 {
		t.Fatalf("held ratings: status %d, %+v", code, list.Body.Ratings)
	}
	path := "/v1/admin/ratings/" + strconv.FormatInt(held, 10) + "/decision"
	if code := a.call(adminTok, "POST", path, map[string]any{"publish": false}, nil); code != 204 {
		t.Errorf("hide: status %d", code)
	}
	if code := a.call(adminTok, "POST", path, map[string]any{"publish": true}, nil); code != 404 {
		t.Errorf("deciding twice: status %d, want 404", code)
	}
}
