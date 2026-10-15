package pricing

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"

	"bugun/internal/store"
	"bugun/internal/store/db"
)

// Integration tests run against a real database when TEST_DATABASE_URL is set,
// e.g. postgres://localhost:5432/bugun_test?sslmode=disable.
var testQueries *db.Queries

func testService(t *testing.T) (*Service, db.User) {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	pool, err := store.Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err := store.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	q := db.New(pool)
	testQueries = q
	id, err := q.UpsertTelegramUser(ctx, db.UpsertTelegramUserParams{TelegramID: 900000001, DisplayName: "Test Sotuvchi"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = q.DeleteUser(context.Background(), id) })
	u, err := q.UpdateProfile(ctx, db.UpdateProfileParams{
		ID: id, Role: pgtype.Text{String: "entrepreneur", Valid: true}, RoleApproved: true,
		RegionID: pgtype.Int2{Int16: 2, Valid: true}, MarketID: pgtype.Int2{Int16: 1, Valid: true},
		Categories: []string{"sabzavot"},
	})
	if err != nil {
		t.Fatal(err)
	}
	svc, err := NewService(ctx, pool)
	if err != nil {
		t.Fatal(err)
	}
	return svc, u
}

func TestConfirmIsSingleUse(t *testing.T) {
	svc, u := testService(t)
	ctx := context.Background()
	d, err := svc.CreateDraft(ctx, u, "kartoshka 4500, 2 tonna", ViaWeb)
	if err != nil || d.ID == 0 {
		t.Fatalf("CreateDraft = %+v, %v", d, err)
	}

	// Two concurrent taps on "To'g'ri": exactly one succeeds.
	var wg sync.WaitGroup
	results := make([]error, 2)
	for i := range results {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, results[i] = svc.Confirm(ctx, u, d.ID)
		}(i)
	}
	wg.Wait()
	ok, gone := 0, 0
	for _, err := range results {
		switch {
		case err == nil:
			ok++
		case errors.Is(err, ErrDraftGone):
			gone++
		default:
			t.Fatalf("unexpected error: %v", err)
		}
	}
	if ok != 1 || gone != 1 {
		t.Fatalf("ok=%d gone=%d, want 1 and 1", ok, gone)
	}
	listings, err := testQueries.ListSellerListings(ctx, u.ID)
	if err != nil || len(listings) != 1 || listings[0].PriceSom != 4500 {
		t.Fatalf("listings = %+v, %v", listings, err)
	}
}

func TestNewPriceReplacesOld(t *testing.T) {
	svc, u := testService(t)
	ctx := context.Background()
	for i, text := range []string{"piyoz 3800", "piyoz 3600"} {
		d, err := svc.CreateDraft(ctx, u, text, ViaWeb)
		if err != nil {
			t.Fatal(err)
		}
		saved, err := svc.Confirm(ctx, u, d.ID)
		if err != nil {
			t.Fatal(err)
		}
		// The second save knows the price it replaced (for "Nega o'zgardi?").
		if want := []int64{0, 3800}[i]; saved[0].PrevPrice != want {
			t.Errorf("save %d: prev price %d, want %d", i+1, saved[0].PrevPrice, want)
		}
	}
	listings, err := testQueries.ListSellerListings(ctx, u.ID)
	if err != nil || len(listings) != 1 || listings[0].PriceSom != 3600 {
		t.Fatalf("listings = %+v, %v", listings, err)
	}
	// History keeps both prices; only the latest is active.
	var active, total int
	if err := svc.pool.QueryRow(ctx,
		`SELECT count(*) FILTER (WHERE status = 'active'), count(*) FROM price_observations WHERE listing_id = $1`,
		listings[0].ID).Scan(&active, &total); err != nil {
		t.Fatal(err)
	}
	if active != 1 || total != 2 {
		t.Fatalf("observations active=%d total=%d, want 1 and 2", active, total)
	}
}

// Only entrepreneurs with a region post prices: not market staff, and not
// an entrepreneur who has not set a region yet.
func TestOnlyEntrepreneursWithRegionSubmit(t *testing.T) {
	svc, u := testService(t)
	staff := u
	staff.Role = pgtype.Text{String: "market_admin", Valid: true}
	if _, err := svc.CreateDraft(context.Background(), staff, "kartoshka 4500", ViaWeb); !errors.Is(err, ErrNotSeller) {
		t.Errorf("market staff: err = %v, want ErrNotSeller", err)
	}
	noRegion := u
	noRegion.RegionID = pgtype.Int2{}
	if _, err := svc.CreateDraft(context.Background(), noRegion, "kartoshka 4500", ViaWeb); !errors.Is(err, ErrNotSeller) {
		t.Errorf("entrepreneur without a region: err = %v, want ErrNotSeller", err)
	}
}

func TestDeletedSellerLeavesMedian(t *testing.T) {
	svc, u := testService(t)
	ctx := context.Background()
	d, err := svc.CreateDraft(ctx, u, "sabzi 3000", ViaWeb)
	if err != nil {
		t.Fatal(err)
	}
	saved, err := svc.Confirm(ctx, u, d.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := testQueries.DeleteUser(ctx, u.ID); err != nil {
		t.Fatal(err)
	}
	var active int
	if err := svc.pool.QueryRow(ctx,
		`SELECT count(*) FROM price_observations WHERE status = 'active' AND raw_text = 'sabzi 3000' AND observed_at > now() - interval '1 minute'`,
	).Scan(&active); err != nil {
		t.Fatal(err)
	}
	if active != 0 || len(saved) != 1 {
		t.Fatalf("active observations after account deletion = %d, want 0", active)
	}
}

// Spec §8.2: after confirming, the seller sees "12 sellers, you are 9th".
func TestConfirmReturnsMarketPosition(t *testing.T) {
	svc, u := testService(t)
	ctx := context.Background()

	// Eleven other sellers in the same viloyat quote onions (product 2).
	for i, price := range []int64{4000, 4100, 4200, 4200, 4300, 4400, 4500, 4600, 4700, 4800, 4900} {
		id, err := testQueries.UpsertTelegramUser(ctx, db.UpsertTelegramUserParams{TelegramID: int64(900000100 + i), DisplayName: "Test Raqib"})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = testQueries.DeleteUser(context.Background(), id) })
		peer, err := testQueries.UpdateProfile(ctx, db.UpdateProfileParams{
			ID: id, Role: pgtype.Text{String: "entrepreneur", Valid: true}, RoleApproved: true,
			RegionID: pgtype.Int2{Int16: 3, Valid: true}, Categories: []string{"sabzavot"},
		})
		if err != nil {
			t.Fatal(err)
		}
		d, err := svc.CreateDraft(ctx, peer, "piyoz "+FormatSom(price), ViaWeb)
		if err != nil || d.ID == 0 {
			t.Fatalf("peer draft: %v %+v", err, d)
		}
		if _, err := svc.Confirm(ctx, peer, d.ID); err != nil {
			t.Fatal(err)
		}
	}

	d, err := svc.CreateDraft(ctx, u, "piyoz 4650", ViaWeb)
	if err != nil || d.ID == 0 {
		t.Fatalf("draft: %v %+v", err, d)
	}
	saved, err := svc.Confirm(ctx, u, d.ID)
	if err != nil {
		t.Fatal(err)
	}
	p := saved[0].Position
	if p == nil {
		t.Fatalf("no position; status %s", saved[0].Status)
	}
	if p.Rank != 9 || p.Total != 12 || p.Median != 4400 || p.DiffPercent != 6 {
		t.Errorf("position = %+v, want rank 9 of 12, median 4400, +6%%", *p)
	}
}
