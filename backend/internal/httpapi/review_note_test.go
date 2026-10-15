package httpapi

import (
	"context"
	"strconv"
	"testing"

	"bugun/internal/market"
)

// An admin's rejection note reaches the seller, in the list and the single listing.
func TestSellerSeesRejectionNote(t *testing.T) {
	a := newTestAPI(t)
	ctx := context.Background()
	seller, tok := a.signIn(900000541, "entrepreneur", 2)
	_, adminTok := a.signIn(900000542, "admin", 2)
	saved, err := a.listings.Upsert(ctx, seller, market.Input{ProductID: 9, PriceSom: 7000, TradeType: "ulgurji"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.pool.Exec(ctx, `UPDATE listings SET status = 'review' WHERE id = $1`, saved.ID); err != nil {
		t.Fatal(err)
	}
	id := strconv.FormatInt(saved.ID, 10)
	const note = "Narxda nol ortiqcha"
	if code := a.call(adminTok, "POST", "/v1/admin/listings/"+id+"/decision", map[string]any{"approve": false, "note": note}, nil); code != 204 {
		t.Fatalf("reject: status %d", code)
	}

	var list MyListingsOutput
	if code := a.call(tok, "GET", "/v1/me/listings", nil, &list.Body); code != 200 {
		t.Fatalf("list: status %d", code)
	}
	found := false
	for _, l := range list.Body.Listings {
		if l.ID == saved.ID {
			found = true
			if l.Status != market.StatusRejected || l.ReviewNote != note {
				t.Errorf("list: status %s, note %q", l.Status, l.ReviewNote)
			}
		}
	}
	if !found {
		t.Fatal("rejected listing missing from the list")
	}
	var one ListingOutput
	if code := a.call(tok, "GET", "/v1/me/listings/"+id, nil, &one.Body); code != 200 || one.Body.Listing.ReviewNote != note {
		t.Errorf("single: status %d, note %q", code, one.Body.Listing.ReviewNote)
	}
}
