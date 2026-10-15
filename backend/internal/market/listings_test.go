package market

import (
	"context"
	"math"
	"math/rand/v2"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"bugun/internal/analytics"
	"bugun/internal/store"
	"bugun/internal/store/db"
)

// Runs against a real database when TEST_DATABASE_URL is set, e.g.
// postgres://localhost:5432/bugun_test?sslmode=disable.
func testSeller(t *testing.T) (*Service, db.User, *pgxpool.Pool) {
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
	id, err := q.UpsertTelegramUser(ctx, db.UpsertTelegramUserParams{TelegramID: 900000002, DisplayName: "Test Sotuvchi"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = q.DeleteUser(context.Background(), id) })
	u, err := q.UpdateProfile(ctx, db.UpdateProfileParams{
		ID: id, Role: pgtype.Text{String: "entrepreneur", Valid: true}, RoleApproved: true,
		RegionID: pgtype.Int2{Int16: 2, Valid: true}, Categories: []string{"sabzavot"},
	})
	if err != nil {
		t.Fatal(err)
	}
	return NewService(pool), u, pool
}

func TestRefreshDueRenewsOnlyLiveListingsNearExpiry(t *testing.T) {
	svc, u, pool := testSeller(t)
	ctx := context.Background()
	now := time.Now()

	plan := []struct {
		product int16
		expires time.Time
		hidden  bool
		renew   bool
	}{
		{1, now.Add(-time.Hour), false, true},     // expired
		{2, now.Add(24 * time.Hour), false, true}, // due tomorrow
		{3, now.Add(6 * 24 * time.Hour), false, false},
		{4, now.Add(time.Hour), true, false}, // hidden: the seller chose to keep it off
	}
	ids := make([]int64, len(plan))
	for i, p := range plan {
		saved, err := svc.Upsert(ctx, u, Input{ProductID: p.product, PriceSom: 5000, TradeType: "ulgurji"})
		if err != nil {
			t.Fatal(err)
		}
		ids[i] = saved.ID
		if _, err := pool.Exec(ctx, `UPDATE listings SET valid_until = $2 WHERE id = $1`, saved.ID, p.expires); err != nil {
			t.Fatal(err)
		}
		if p.hidden {
			if err := svc.SetVisible(ctx, u, saved.ID, false); err != nil {
				t.Fatal(err)
			}
		}
	}

	n, until, err := svc.RefreshDue(ctx, u)
	if err != nil || n != 2 {
		t.Fatalf("RefreshDue = %d, %v; want 2 renewed", n, err)
	}
	for i, p := range plan {
		var got time.Time
		if err := pool.QueryRow(ctx, `SELECT valid_until FROM listings WHERE id = $1`, ids[i]).Scan(&got); err != nil {
			t.Fatal(err)
		}
		renewed := got.Sub(until).Abs() < time.Second
		if renewed != p.renew {
			t.Errorf("listing for product %d: renewed = %v, want %v", p.product, renewed, p.renew)
		}
	}
	// The renewed prices count towards the median again.
	var live int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM price_observations WHERE listing_id = ANY($1) AND status = 'active' AND valid_until > now()`, ids[:2]).Scan(&live); err != nil {
		t.Fatal(err)
	}
	if live != 2 {
		t.Errorf("live observations for renewed listings = %d, want 2", live)
	}
}

func TestVisibilityCannotSkipReview(t *testing.T) {
	svc, u, pool := testSeller(t)
	ctx := context.Background()
	status := func(id int64) string {
		var s string
		if err := pool.QueryRow(ctx, `SELECT status FROM listings WHERE id = $1`, id).Scan(&s); err != nil {
			t.Fatal(err)
		}
		return s
	}

	// An ordinary listing hides and shows again.
	live, err := svc.Upsert(ctx, u, Input{ProductID: 1, PriceSom: 5000, TradeType: "ulgurji"})
	if err != nil {
		t.Fatal(err)
	}
	for _, step := range []struct {
		visible bool
		want    string
	}{{false, StatusHidden}, {true, StatusActive}} {
		if err := svc.SetVisible(ctx, u, live.ID, step.visible); err != nil {
			t.Fatal(err)
		}
		if got := status(live.ID); got != step.want {
			t.Errorf("visible=%v: status %s, want %s", step.visible, got, step.want)
		}
	}

	// Held for review or rejected: hiding and showing change nothing.
	for _, held := range []string{StatusReview, "rejected"} {
		saved, err := svc.Upsert(ctx, u, Input{ProductID: 2, PriceSom: 5000, TradeType: "ulgurji"})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, `UPDATE listings SET status = $2 WHERE id = $1`, saved.ID, held); err != nil {
			t.Fatal(err)
		}
		for _, visible := range []bool{true, false, true} {
			if err := svc.SetVisible(ctx, u, saved.ID, visible); err != nil {
				t.Fatal(err)
			}
			if got := status(saved.ID); got != held {
				t.Errorf("%s listing after visible=%v: status %s", held, visible, got)
			}
		}
		if _, err := pool.Exec(ctx, `DELETE FROM listings WHERE id = $1`, saved.ID); err != nil {
			t.Fatal(err)
		}
	}
}

func TestReviewApprovesOrRejectsHeldListings(t *testing.T) {
	svc, u, pool := testSeller(t)
	ctx := context.Background()

	held := func(product int16) int64 {
		saved, err := svc.Upsert(ctx, u, Input{ProductID: product, PriceSom: 5000, TradeType: "ulgurji"})
		if err != nil {
			t.Fatal(err)
		}
		// Hold it as the outlier check would.
		if _, err := pool.Exec(ctx, `UPDATE listings SET status = 'review' WHERE id = $1`, saved.ID); err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, `UPDATE price_observations SET status = 'review' WHERE listing_id = $1 AND status = 'active'`, saved.ID); err != nil {
			t.Fatal(err)
		}
		return saved.ID
	}
	state := func(id int64) (listing, observation, note string) {
		err := pool.QueryRow(ctx, `
			SELECT l.status, COALESCE(l.review_note, ''),
			       COALESCE((SELECT o.status FROM price_observations o WHERE o.listing_id = l.id ORDER BY o.id DESC LIMIT 1), '')
			FROM listings l WHERE l.id = $1`, id).Scan(&listing, &note, &observation)
		if err != nil {
			t.Fatal(err)
		}
		return
	}

	ok, bad := held(1), held(2)
	if err := svc.Review(ctx, ok, true, ""); err != nil {
		t.Fatal(err)
	}
	if err := svc.Review(ctx, bad, false, "Narx juda past, rasmni tekshiring"); err != nil {
		t.Fatal(err)
	}
	if l, o, _ := state(ok); l != StatusActive || o != "active" {
		t.Errorf("approved: listing %s, observation %s; want active, active", l, o)
	}
	if l, o, n := state(bad); l != StatusRejected || o != "rejected" || n == "" {
		t.Errorf("rejected: listing %s, observation %s, note %q; want rejected with a note", l, o, n)
	}
	// A second admin acting on the same listing gets a clear "already done".
	if err := svc.Review(ctx, ok, false, ""); err != ErrNotFound {
		t.Errorf("second decision: %v, want ErrNotFound", err)
	}
}

func TestHeldPricesCarryTheReason(t *testing.T) {
	svc, u, pool := testSeller(t)
	ctx := context.Background()
	reason := func(id int64) (status, why string) {
		if err := pool.QueryRow(ctx, `SELECT status, COALESCE(review_reason, '') FROM listings WHERE id = $1`, id).
			Scan(&status, &why); err != nil {
			t.Fatal(err)
		}
		return
	}

	// A normal price, then the same listing with an extra zero from the bot.
	first, err := svc.Upsert(ctx, u, Input{ProductID: 3, PriceSom: 5000, TradeType: "ulgurji"})
	if err != nil {
		t.Fatal(err)
	}
	if s, w := reason(first.ID); s != StatusActive || w != "" {
		t.Fatalf("first price: %s %q, want active without reason", s, w)
	}
	typo, err := svc.Upsert(ctx, u, Input{ProductID: 3, PriceSom: 50000, TradeType: "ulgurji"})
	if err != nil {
		t.Fatal(err)
	}
	if s, w := reason(typo.ID); s != StatusReview || w != "jump_from_previous" {
		t.Errorf("extra zero: %s %q, want review jump_from_previous", s, w)
	}
	fixed, err := svc.Upsert(ctx, u, Input{ProductID: 3, PriceSom: 5200, TradeType: "ulgurji"})
	if err != nil {
		t.Fatal(err)
	}
	if s, w := reason(fixed.ID); s != StatusActive || w != "" {
		t.Errorf("corrected price: %s %q, want active without reason", s, w)
	}

	// No other sellers: an official benchmark decides.
	if _, err := pool.Exec(ctx, `
		INSERT INTO price_observations (product_id, price_som, unit, trade_type, region_id, source, valid_until)
		VALUES (4, 4000, 'kg', 'ulgurji', 1, 'uzex', now() + interval '7 days')`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM price_observations WHERE product_id = 4 AND source = 'uzex'`)
	})
	far, err := svc.Upsert(ctx, u, Input{ProductID: 4, PriceSom: 9000, TradeType: "ulgurji"})
	if err != nil {
		t.Fatal(err)
	}
	if s, w := reason(far.ID); s != StatusReview || w != "far_from_reference" {
		t.Errorf("far from benchmark: %s %q, want review far_from_reference", s, w)
	}
}

// The SQL weighted_quantile and analytics.WeightedQuantile must agree, and
// price_weight must follow spec §10.1.
func TestWeightedMedianMatchesGo(t *testing.T) {
	_, _, pool := testSeller(t)
	ctx := context.Background()

	var w float64
	if err := pool.QueryRow(ctx, `SELECT price_weight('seller', now() - interval '1 day')`).Scan(&w); err != nil {
		t.Fatal(err)
	}
	if want := 0.8 * math.Exp(-0.2); math.Abs(w-want) > 1e-9 {
		t.Errorf("price_weight(seller, 1 day) = %v, want %v", w, want)
	}
	for _, src := range []string{"uzex", "narxtahlil"} {
		if err := pool.QueryRow(ctx, `SELECT price_weight($1, now())`, src).Scan(&w); err != nil || w != 0 {
			t.Errorf("price_weight(%s) = %v, %v; benchmarks must weigh 0", src, w, err)
		}
	}

	rng := rand.New(rand.NewPCG(1, 2))
	for round := 0; round < 50; round++ {
		n := 1 + rng.IntN(12)
		prices, weights := make([]float64, n), make([]float64, n)
		for i := range prices {
			prices[i] = float64(1000 + rng.IntN(20)*100)
			weights[i] = rng.Float64()
		}
		for _, q := range []float64{0.25, 0.5, 0.75} {
			var got float64
			if err := pool.QueryRow(ctx, `SELECT weighted_quantile($1::float8[], $2::float8[], $3)`, prices, weights, q).Scan(&got); err != nil {
				t.Fatal(err)
			}
			if want := analytics.WeightedQuantile(prices, weights, q); got != want {
				t.Fatalf("q=%v prices=%v weights=%v: SQL %v, Go %v", q, prices, weights, got, want)
			}
		}
	}
}

// "Prices unchanged" renews every live listing for a week, not only those near expiry.
func TestRenewAllExtendsEveryLiveListing(t *testing.T) {
	svc, u, pool := testSeller(t)
	ctx := context.Background()
	var ids []int64
	for _, p := range []int16{7, 8} {
		saved, err := svc.Upsert(ctx, u, Input{ProductID: p, PriceSom: 5000, TradeType: "ulgurji"})
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, saved.ID)
	}
	// One listing still has five days left, the other expired yesterday.
	if _, err := pool.Exec(ctx, `UPDATE listings SET valid_until = now() + interval '5 days' WHERE id = $1`, ids[0]); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE listings SET valid_until = now() - interval '1 day' WHERE id = $1`, ids[1]); err != nil {
		t.Fatal(err)
	}
	n, until, err := svc.RenewAll(ctx, u)
	if err != nil || n != 2 {
		t.Fatalf("RenewAll = %d, %v; want 2", n, err)
	}
	var fresh int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM listings WHERE id = ANY($1) AND valid_until = $2`, ids, until).Scan(&fresh); err != nil || fresh != 2 {
		t.Errorf("%d listings run to %v, want 2 (%v)", fresh, until, err)
	}
}
