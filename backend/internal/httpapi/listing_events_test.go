package httpapi

import (
	"context"
	"strconv"
	"testing"

	"bugun/internal/market"
)

// A listing's views and contacts count each visitor once a day, never the
// seller themselves.
func TestListingEventsCountVisitorsOnce(t *testing.T) {
	a := newTestAPI(t)
	ctx := context.Background()
	seller, sellerTok := a.signIn(900000511, "entrepreneur", 2)
	_, buyerTok := a.signIn(900000512, "entrepreneur", 2)
	if _, err := a.pool.Exec(ctx, `UPDATE users SET contact_telegram = true, telegram_username = 'test_sotuvchi' WHERE id = $1`, seller.ID); err != nil {
		t.Fatal(err)
	}
	saved, err := a.listings.Upsert(ctx, seller, market.Input{ProductID: 9, PriceSom: 6000, TradeType: "ulgurji"})
	if err != nil {
		t.Fatal(err)
	}
	page := "/v1/listings/" + strconv.FormatInt(saved.ID, 10)
	for _, tok := range []string{buyerTok, buyerTok, sellerTok} {
		if code := a.call(tok, "GET", page, nil, nil); code != 200 {
			t.Fatalf("listing page: status %d", code)
		}
	}
	for _, tok := range []string{buyerTok, buyerTok} {
		if code := a.call(tok, "POST", page+"/contact", nil, nil); code != 200 {
			t.Fatalf("reveal contact: status %d", code)
		}
	}
	count := func(kind string) (n int) {
		if err := a.pool.QueryRow(ctx, `SELECT count(*) FROM listing_events WHERE listing_id = $1 AND kind = $2`, saved.ID, kind).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	if v, c := count("view"), count("contact"); v != 1 || c != 1 {
		t.Errorf("events: %d views, %d contacts; want 1 and 1", v, c)
	}
	// The reveal schedules one follow-up question for the buyer, a day later.
	var prompts int
	var hours float64
	if err := a.pool.QueryRow(ctx, `SELECT count(*), COALESCE(max(extract(epoch FROM due_at - now()) / 3600), 0) FROM deal_prompts WHERE listing_id = $1`, saved.ID).
		Scan(&prompts, &hours); err != nil || prompts != 1 || hours < 23 || hours > 25 {
		t.Errorf("follow-ups: %d, due in %.1f h, %v; want 1 in 24 h", prompts, hours, err)
	}
	var views int
	if err := a.pool.QueryRow(ctx, `SELECT views FROM listings WHERE id = $1`, saved.ID).Scan(&views); err != nil || views != 2 {
		t.Errorf("view counter %d, want 2 (the seller's own visit left out)", views)
	}
}

func TestMarkSoldEndpoint(t *testing.T) {
	a := newTestAPI(t)
	ctx := context.Background()
	seller, tok := a.signIn(900000521, "entrepreneur", 2)
	_, otherTok := a.signIn(900000522, "entrepreneur", 2)
	stock := 100.0
	saved, err := a.listings.Upsert(ctx, seller, market.Input{ProductID: 9, PriceSom: 6000, TradeType: "ulgurji", Stock: &stock})
	if err != nil {
		t.Fatal(err)
	}
	path := "/v1/listings/" + strconv.FormatInt(saved.ID, 10) + "/sold"
	if code := a.call(otherTok, "POST", path, map[string]any{"quantity": 10}, nil); code != 404 {
		t.Errorf("another seller: status %d, want 404", code)
	}
	if code := a.call(tok, "POST", path, map[string]any{"quantity": 150}, nil); code != 422 {
		t.Errorf("more than the stock: status %d, want 422", code)
	}
	var out SoldOutput
	if code := a.call(tok, "POST", path, map[string]any{"quantity": 100, "price_som": 5900}, &out.Body); code != 200 {
		t.Fatalf("sold: status %d", code)
	}
	if out.Body.DealID == 0 || out.Body.Confirmed || !out.Body.SoldOut || out.Body.Listing.Status != market.StatusHidden {
		t.Errorf("sold: %+v", out.Body)
	}
}
