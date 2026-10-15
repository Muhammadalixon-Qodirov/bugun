package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/google/uuid"

	"bugun/internal/market"
)

func TestPublicSellerPage(t *testing.T) {
	a := newTestAPI(t)
	ctx := context.Background()
	seller, _ := a.signIn(900000611, "entrepreneur", 2)
	staff, _ := a.signIn(900000612, "market_admin", 2)
	if _, err := a.pool.Exec(ctx, `UPDATE users SET telegram_username = 'secret_handle', public_name = 'Vodiy Test' WHERE id = $1`, seller.ID); err != nil {
		t.Fatal(err)
	}
	live, err := a.listings.Upsert(ctx, seller, market.Input{ProductID: 9, PriceSom: 7000, TradeType: "ulgurji"})
	if err != nil {
		t.Fatal(err)
	}
	hidden, err := a.listings.Upsert(ctx, seller, market.Input{ProductID: 10, PriceSom: 5000, TradeType: "chakana"})
	if err != nil {
		t.Fatal(err)
	}
	if err := a.listings.SetVisible(ctx, seller, hidden.ID, false); err != nil {
		t.Fatal(err)
	}

	path := "/v1/sellers/" + seller.ID.String()
	var page SellerPageOutput
	if code := a.call("", "GET", path, nil, &page.Body); code != 200 {
		t.Fatalf("seller page: status %d", code)
	}
	p := page.Body.Seller
	if p.ID != seller.ID.String() || p.Name != "Vodiy Test" || p.Listings != 1 || p.Products != 1 || p.LastActive == nil {
		t.Errorf("profile: %+v", p)
	}
	if len(p.Categories) != 1 || p.RegionName == "" || p.Rating != nil {
		t.Errorf("categories/region/rating: %+v", p)
	}
	if len(page.Body.Listings) != 1 || page.Body.Listings[0].ID != live.ID || page.Body.Listings[0].SellerID != seller.ID.String() {
		t.Errorf("listings: %+v", page.Body.Listings)
	}

	// The raw answer must not carry private contact details.
	res, err := http.Get(a.srv.URL + path)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var raw map[string]json.RawMessage
	if err := json.NewDecoder(res.Body).Decode(&raw); err != nil {
		t.Fatal(err)
	}
	body := strings.ToLower(string(raw["seller"]))
	for _, private := range []string{"secret_handle", "\"phone\":", "telegram_id", "telegram_username", "900000611"} {
		if strings.Contains(body, private) {
			t.Errorf("seller page leaks %q: %s", private, body)
		}
	}

	var listing ListingPageOutput
	if code := a.call("", "GET", "/v1/listings/"+strconv.FormatInt(live.ID, 10), nil, &listing.Body); code != 200 || listing.Body.Seller.ID != seller.ID.String() {
		t.Errorf("listing page seller id: status %d, %q", code, listing.Body.Seller.ID)
	}

	for _, id := range []string{staff.ID.String(), uuid.NewString(), "not-a-uuid"} {
		if code := a.call("", "GET", "/v1/sellers/"+id, nil, nil); code != 404 {
			t.Errorf("seller %s: status %d, want 404", id, code)
		}
	}
}
