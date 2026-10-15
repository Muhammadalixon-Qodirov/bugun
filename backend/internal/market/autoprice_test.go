package market

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"bugun/internal/store/db"
)

func TestRepriceFollowsTheMarketWithinTheDailyStep(t *testing.T) {
	svc, u, pool := testSeller(t)
	ctx := context.Background()

	// Six other sellers in the viloyat at 6 000–6 500.
	for _, p := range []int{6000, 6100, 6200, 6200, 6300, 6500} {
		if _, err := pool.Exec(ctx, `
			INSERT INTO price_observations (product_id, price_som, unit, trade_type, region_id, source, valid_until, raw_text)
			VALUES (9, $1, 'kg', 'ulgurji', $2, 'seller', now() + interval '7 days', 'autoprice-test')`, p, u.RegionID.Int16); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM price_observations WHERE raw_text = 'autoprice-test'`)
	})

	saved, err := svc.Upsert(ctx, u, Input{ProductID: 9, PriceSom: 5600, TradeType: "ulgurji"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SetPricing(ctx, u, saved.ID, Pricing{Mode: PricingAuto, PriceMin: i64(5000), PriceMax: i64(7000)}); err != nil {
		t.Fatal(err)
	}
	// Set by hand long ago, so the 24-hour pause is over.
	if _, err := pool.Exec(ctx, `UPDATE listings SET manual_price_at = now() - interval '2 days' WHERE id = $1`, saved.ID); err != nil {
		t.Fatal(err)
	}
	find := func(changes []AutoChange) *AutoChange {
		for i := range changes {
			if changes[i].Listing.ID == saved.ID {
				return &changes[i]
			}
		}
		return nil
	}

	// The market is near 6 200, but one day moves at most 5%: 5 880, quoted as 5 800.
	changes, err := svc.Reprice(ctx)
	if err != nil {
		t.Fatal(err)
	}
	c := find(changes)
	if c == nil || c.Change.OldPrice != 5600 || c.Change.NewPrice != 5800 || c.Listing.PriceSom != 5800 {
		t.Fatalf("first run: %+v, want 5 600 → 5 800", c)
	}
	var source string
	var live int
	if err := pool.QueryRow(ctx, `
		SELECT min(source), count(*) FROM price_observations
		WHERE listing_id = $1 AND status = 'active' AND price_som = 5800`, saved.ID).Scan(&source, &live); err != nil || live != 1 || source != SourceSellerAuto {
		t.Errorf("observation: %d live, source %q, %v; want one seller_auto", live, source, err)
	}

	// Later the same day the day's room is used up: no second step.
	changes, err = svc.Reprice(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if c := find(changes); c != nil {
		t.Errorf("second run the same day moved the price again: %+v", c.Change)
	}

	// Undo: refused for another seller or after a newer price, then the old
	// price comes back as the seller's own.
	stranger := u
	stranger.ID = uuid.New()
	if _, _, err := svc.RevertAutoPrice(ctx, stranger, c.Change.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("another seller's undo: %v, want not found", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE listings SET price_som = 5900 WHERE id = $1`, saved.ID); err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.RevertAutoPrice(ctx, u, c.Change.ID); !errors.Is(err, ErrStale) {
		t.Errorf("undo after a newer price: %v, want stale", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE listings SET price_som = 5800 WHERE id = $1`, saved.ID); err != nil {
		t.Fatal(err)
	}
	l, ch, err := svc.RevertAutoPrice(ctx, u, c.Change.ID)
	if err != nil || l.PriceSom != 5600 || ch.Status != "reverted" || l.ManualPriceAt == nil {
		t.Fatalf("undo: %d %s %v %v, want 5 600 reverted with a manual time", l.PriceSom, ch.Status, l.ManualPriceAt, err)
	}
	if err := pool.QueryRow(ctx, `
		SELECT min(source), count(*) FROM price_observations
		WHERE listing_id = $1 AND status = 'active'`, saved.ID).Scan(&source, &live); err != nil || live != 1 || source != SourceSeller {
		t.Errorf("after undo: %d live, source %q, %v; want one seller price", live, source, err)
	}
	if _, _, err := svc.RevertAutoPrice(ctx, u, c.Change.ID); !errors.Is(err, ErrReverted) {
		t.Errorf("second undo: %v, want already reverted", err)
	}

	// A price set by hand pauses automatic pricing for 24 hours.
	if _, err := pool.Exec(ctx, `DELETE FROM price_changes WHERE listing_id = $1`, saved.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE listings SET manual_price_at = now() - interval '2 days' WHERE id = $1`, saved.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Upsert(ctx, u, Input{ProductID: 9, PriceSom: 5700, TradeType: "ulgurji"}); err != nil {
		t.Fatal(err)
	}
	changes, err = svc.Reprice(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if c := find(changes); c != nil {
		t.Errorf("repriced right after a manual price: %+v", c.Change)
	}

	// Manual mode is never touched.
	if _, err := pool.Exec(ctx, `UPDATE listings SET manual_price_at = NULL WHERE id = $1`, saved.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SetPricing(ctx, u, saved.ID, Pricing{Mode: PricingManual}); err != nil {
		t.Fatal(err)
	}
	changes, err = svc.Reprice(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if c := find(changes); c != nil {
		t.Errorf("repriced a manual listing: %+v", c.Change)
	}
}

// This week's unique views and contact reveals of the listing are compared
// with other listings of the product in the viloyat.
func TestRepriceInputResponse(t *testing.T) {
	svc, u, pool := testSeller(t)
	ctx := context.Background()
	q := svc.q
	otherID, err := q.UpsertTelegramUser(ctx, db.UpsertTelegramUserParams{TelegramID: 900000003, DisplayName: "Test Sotuvchi 2"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = q.DeleteUser(context.Background(), otherID) })
	other, err := q.UpdateProfile(ctx, db.UpdateProfileParams{
		ID: otherID, Role: pgtype.Text{String: "entrepreneur", Valid: true}, RoleApproved: true,
		RegionID: u.RegionID, Categories: []string{"sabzavot"},
	})
	if err != nil {
		t.Fatal(err)
	}
	mine, err := svc.Upsert(ctx, u, Input{ProductID: 11, PriceSom: 3000, TradeType: "ulgurji"})
	if err != nil {
		t.Fatal(err)
	}
	theirs, err := svc.Upsert(ctx, other, Input{ProductID: 11, PriceSom: 3000, TradeType: "ulgurji"})
	if err != nil {
		t.Fatal(err)
	}
	// Unique visitors this week; an old visit is left out.
	for _, e := range []struct {
		id      int64
		kind    string
		n       int
		daysAgo int
	}{{mine.ID, "view", 40, 1}, {mine.ID, "contact", 1, 1}, {mine.ID, "view", 500, 10}, {theirs.ID, "view", 200, 2}, {theirs.ID, "contact", 20, 2}} {
		if _, err := pool.Exec(ctx, `
			INSERT INTO listing_events (listing_id, kind, viewer_key, day, created_at)
			SELECT $1, $2, int4send(g), current_date - $4::int, now() - make_interval(days => $4::int)
			FROM generate_series(1, $3) g`, e.id, e.kind, e.n, e.daysAgo); err != nil {
			t.Fatal(err)
		}
	}
	// Sales this week: three here, one at the other seller, one old one here.
	for _, d := range []struct {
		id, ago int64
		seller  any
	}{{mine.ID, 1, u.ID}, {mine.ID, 2, u.ID}, {mine.ID, 3, u.ID}, {mine.ID, 20, u.ID}, {theirs.ID, 1, other.ID}} {
		if _, err := pool.Exec(ctx, `
			INSERT INTO deals (listing_id, product_id, seller_id, source, status, seller_confirmed_at, unit, region_id, created_at)
			VALUES ($1, 11, $2, 'seller_marked', 'reported', now(), 'kg', 2, now() - make_interval(days => $3::int))`, d.id, d.seller, d.ago); err != nil {
			t.Fatal(err)
		}
	}
	l, err := q.GetListing(ctx, mine.ID)
	if err != nil {
		t.Fatal(err)
	}
	// repriceInput stops early without peers; give it five other prices.
	for range 5 {
		if _, err := pool.Exec(ctx, `
			INSERT INTO price_observations (product_id, price_som, unit, trade_type, region_id, source, valid_until, raw_text)
			VALUES (11, 3000, 'kg', 'ulgurji', $1, 'seller', now() + interval '7 days', 'autoprice-test')`, u.RegionID.Int16); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM price_observations WHERE raw_text = 'autoprice-test'`)
	})
	in, err := repriceInput(ctx, q, l, DefaultAutoPriceRules.MinMarketViews)
	if err != nil {
		t.Fatal(err)
	}
	if in.Views != 40 || in.Contacts != 1 || in.MarketRate != 0.1 {
		t.Errorf("views %d, contacts %d, market rate %v; want 40, 1, 0.1", in.Views, in.Contacts, in.MarketRate)
	}
	if in.Sales != 3 || !in.SalesKnown || in.MarketSales != 1 {
		t.Errorf("sales %d, market %v (%v); want 3 against 1 per other listing", in.Sales, in.MarketSales, in.SalesKnown)
	}
}
