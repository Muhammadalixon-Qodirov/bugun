package httpapi

import (
	"context"
	"strconv"
	"testing"
	"time"

	"bugun/internal/market"
)

// TestPriceChangeEndpoints: a seller sees an automatic change with its reasons
// and undoes it once; another seller cannot.
func TestPriceChangeEndpoints(t *testing.T) {
	a := newTestAPI(t)
	ctx := context.Background()
	pool, listings := a.pool, a.listings
	call := func(token, method, path string, out any) int { return a.call(token, method, path, nil, out) }

	seller, tok := a.signIn(900000501, "entrepreneur", 2)
	_, otherTok := a.signIn(900000502, "entrepreneur", 2)
	saved, err := listings.Upsert(ctx, seller, market.Input{ProductID: 9, PriceSom: 6300, TradeType: "ulgurji"})
	if err != nil {
		t.Fatal(err)
	}
	lo, hi := int64(5500), int64(7000)
	if _, err := listings.SetPricing(ctx, seller, saved.ID, market.Pricing{Mode: market.PricingAuto, PriceMin: &lo, PriceMax: &hi}); err != nil {
		t.Fatal(err)
	}
	// As if the job had moved 6 000 to 6 300.
	var changeID int64
	if err := pool.QueryRow(ctx, `
		INSERT INTO price_changes (listing_id, old_price, new_price, reasons)
		VALUES ($1, 6000, 6300, '{"median": 6100.4, "peers": 12, "factors": [{"code": "pressure", "share": 0.04}, {"code": "cap", "share": -0.0163}]}')
		RETURNING id`, saved.ID).Scan(&changeID); err != nil {
		t.Fatal(err)
	}
	history := "/v1/me/listings/" + strconv.FormatInt(saved.ID, 10) + "/price-changes"
	revert := "/v1/me/price-changes/" + strconv.FormatInt(changeID, 10) + "/revert"

	var list PriceChangesOutput
	if code := call(tok, "GET", history, &list.Body); code != 200 || len(list.Body.Changes) != 1 {
		t.Fatalf("history: status %d, %+v", code, list.Body)
	}
	c := list.Body.Changes[0]
	if !c.CanRevert || c.Median != 6100 || c.Peers != 12 || len(c.Factors) != 2 || c.Factors[1].Share != -0.016 {
		t.Errorf("change: %+v", c)
	}
	if code := call(otherTok, "GET", history, nil); code != 404 {
		t.Errorf("another seller's history: status %d, want 404", code)
	}
	if code := call(otherTok, "POST", revert, nil); code != 404 {
		t.Errorf("another seller's undo: status %d, want 404", code)
	}

	var out ListingOutput
	if code := call(tok, "POST", revert, &out.Body); code != 200 {
		t.Fatalf("undo: status %d", code)
	}
	if l := out.Body.Listing; l.PriceSom != 6000 || l.Pricing.PausedUntil == nil || time.Until(*l.Pricing.PausedUntil) < 23*time.Hour {
		t.Errorf("after undo: price %d, paused until %v", l.PriceSom, l.Pricing.PausedUntil)
	}
	if code := call(tok, "POST", revert, nil); code != 409 {
		t.Errorf("second undo: status %d, want 409", code)
	}
	if code := call(tok, "GET", history, &list.Body); code != 200 || list.Body.Changes[0].Status != "reverted" || list.Body.Changes[0].CanRevert {
		t.Errorf("history after undo: %+v", list.Body.Changes)
	}
}
