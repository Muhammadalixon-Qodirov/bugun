package market

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
)

func TestMarkSold(t *testing.T) {
	svc, u, pool := testSeller(t)
	ctx := context.Background()
	stock := 500.0
	saved, err := svc.Upsert(ctx, u, Input{ProductID: 12, PriceSom: 3000, TradeType: "ulgurji", Stock: &stock})
	if err != nil {
		t.Fatal(err)
	}

	stranger := u
	stranger.ID = uuid.New()
	if _, err := svc.MarkSold(ctx, stranger, saved.ID, 10, nil); !errors.Is(err, ErrNotFound) {
		t.Errorf("another seller: %v, want not found", err)
	}
	var ve *ValidationError
	if _, err := svc.MarkSold(ctx, u, saved.ID, 600, nil); !errors.As(err, &ve) || ve.Code != "more_than_stock" {
		t.Errorf("more than the stock: %v, want more_than_stock", err)
	}
	if _, err := svc.MarkSold(ctx, u, saved.ID, 0, nil); !errors.As(err, &ve) || ve.Field != "quantity" {
		t.Errorf("zero: %v, want a quantity error", err)
	}

	price := int64(2900)
	sold, err := svc.MarkSold(ctx, u, saved.ID, 200, &price)
	if err != nil || sold.Matched || sold.SoldOut || sold.Deal.Source != "seller_marked" || sold.Deal.PriceSom.Int64 != 2900 {
		t.Fatalf("first sale: %+v %v", sold, err)
	}
	if left := Float(sold.Listing.Stock); left == nil || *left != 300 || sold.Listing.Status != StatusActive {
		t.Errorf("after 200 of 500: stock %v, status %s", left, sold.Listing.Status)
	}

	// A buyer's report on the listing is confirmed; the buyer's price wins.
	if _, err := pool.Exec(ctx, `
		INSERT INTO deals (listing_id, product_id, seller_id, source, status, buyer_confirmed_at, price_som, unit, region_id)
		VALUES ($1, 12, $2, 'contact', 'reported', now(), 2800, 'kg', 2)`, saved.ID, u.ID); err != nil {
		t.Fatal(err)
	}
	sold, err = svc.MarkSold(ctx, u, saved.ID, 300, &price)
	if err != nil || !sold.Matched || sold.Deal.Status != "confirmed" || sold.Deal.PriceSom.Int64 != 2800 {
		t.Errorf("confirming a buyer's report: %+v %v", sold.Deal, err)
	}
	if !sold.SoldOut || sold.Listing.Status != StatusHidden || sold.Listing.Stock.Valid {
		t.Errorf("last 300: sold out %v, status %s, stock %v; want hidden with no stock", sold.SoldOut, sold.Listing.Status, sold.Listing.Stock)
	}
	// The confirmed deal's price joins the median at full trust; the seller's
	// own unconfirmed mark does not.
	dealPrice := func(dealID int64) (status string, price int64) {
		t.Helper()
		if err := pool.QueryRow(ctx, `SELECT status, price_som FROM price_observations WHERE deal_id = $1 AND source = 'deal' AND status <> 'expired'`, dealID).
			Scan(&status, &price); err != nil {
			return "", 0
		}
		return status, price
	}
	if st, p := dealPrice(sold.Deal.ID); st != "active" || p != 2800 {
		t.Errorf("confirmed deal in the median: %q %d, want active 2 800", st, p)
	}
	var marked int64
	if err := pool.QueryRow(ctx, `SELECT id FROM deals WHERE listing_id = $1 AND source = 'seller_marked'`, saved.ID).Scan(&marked); err != nil {
		t.Fatal(err)
	}
	if st, _ := dealPrice(marked); st != "" {
		t.Errorf("an unconfirmed deal reached the median: %q", st)
	}
	// A typo (10 × the listing price) is held, not counted.
	var typo int64
	if err := pool.QueryRow(ctx, `
		INSERT INTO deals (listing_id, product_id, seller_id, source, status, buyer_confirmed_at, seller_confirmed_at, price_som, unit, region_id)
		VALUES ($1, 12, $2, 'contact', 'confirmed', now(), now(), 30000, 'kg', 2) RETURNING id`, saved.ID, u.ID).Scan(&typo); err != nil {
		t.Fatal(err)
	}
	if err := SyncDealPrice(ctx, svc.q, typo); err != nil {
		t.Fatal(err)
	}
	if st, _ := dealPrice(typo); st != "review" {
		t.Errorf("typo deal price: %q, want review", st)
	}

	// Without a stock the listing stays as it is.
	other, err := svc.Upsert(ctx, u, Input{ProductID: 13, PriceSom: 3000, TradeType: "ulgurji"})
	if err != nil {
		t.Fatal(err)
	}
	if sold, err := svc.MarkSold(ctx, u, other.ID, 1000, nil); err != nil || sold.SoldOut || sold.Listing.Status != StatusActive {
		t.Errorf("no stock given: %+v %v", sold, err)
	}
}

// A hidden listing's price leaves other sellers' market position and peers.
func TestHiddenListingLeavesPeers(t *testing.T) {
	svc, u, _ := testSeller(t)
	ctx := context.Background()
	saved, err := svc.Upsert(ctx, u, Input{ProductID: 14, PriceSom: 3000, TradeType: "ulgurji"})
	if err != nil {
		t.Fatal(err)
	}
	province, err := ProvinceOf(ctx, svc.q, u.RegionID.Int16)
	if err != nil {
		t.Fatal(err)
	}
	count := func() int {
		prices, _, err := peerPrices(ctx, svc.q, province, 14, "ulgurji", 0)
		if err != nil {
			t.Fatal(err)
		}
		n := 0
		for _, p := range prices {
			if p == 3000 {
				n++
			}
		}
		return n
	}
	before := count()
	if err := svc.SetVisible(ctx, u, saved.ID, false); err != nil {
		t.Fatal(err)
	}
	if after := count(); after != before-1 {
		t.Errorf("peers at 3 000: %d before hiding, %d after; want one fewer", before, after)
	}
}
