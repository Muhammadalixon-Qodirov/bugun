package rfq

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"bugun/internal/market"
	"bugun/internal/store"
	"bugun/internal/store/db"
	"bugun/internal/tz"
)

func TestSavings(t *testing.T) {
	cases := []struct {
		median   float64
		cheapest int64
		qty      float64
		want     int64
	}{
		{4200, 3900, 200, 60000},
		{4200, 4500, 200, 0}, // no saving when the offer is above the market
		{0, 3900, 200, 0},    // no market median yet
		{4200, 0, 200, 0},    // no offers yet
	}
	for _, c := range cases {
		if got := Savings(c.median, c.cheapest, c.qty); got != c.want {
			t.Errorf("Savings(%v, %v, %v) = %v, want %v", c.median, c.cheapest, c.qty, got, c.want)
		}
	}
}

// recorder is a Notifier that remembers what it was told.
type recorder struct {
	mu     sync.Mutex
	events []string
}

func (r *recorder) add(e string) { r.mu.Lock(); r.events = append(r.events, e); r.mu.Unlock() }
func (r *recorder) RFQCreated(_ context.Context, _ db.Rfq, sellers []uuid.UUID) {
	r.add("created")
}
func (r *recorder) OfferMade(context.Context, db.Rfq, db.Offer)     { r.add("offer") }
func (r *recorder) OfferChosen(context.Context, db.Rfq, db.Offer)   { r.add("chosen") }
func (r *recorder) OfferAnswered(context.Context, db.Rfq, db.Offer) { r.add("answered") }

// Integration tests run against a real database when TEST_DATABASE_URL is set.
func setup(t *testing.T) (*Service, *recorder, *pgxpool.Pool, *db.Queries) {
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
	rec := &recorder{}
	return NewService(pool, rec), rec, pool, db.New(pool)
}

func user(t *testing.T, q *db.Queries, tgID int64, role string, region int16) db.User {
	t.Helper()
	ctx := context.Background()
	id, err := q.UpsertTelegramUser(ctx, db.UpsertTelegramUserParams{TelegramID: tgID, DisplayName: "Test " + role})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = q.DeleteUser(context.Background(), id) })
	u, err := q.UpdateProfile(ctx, db.UpdateProfileParams{
		ID: id, Role: pgtype.Text{String: role, Valid: true}, RoleApproved: true,
		RegionID: pgtype.Int2{Int16: region, Valid: true}, Categories: []string{"sabzavot"},
	})
	if err != nil {
		t.Fatal(err)
	}
	return u
}

func TestRFQFlow(t *testing.T) {
	svc, rec, pool, q := setup(t)
	ctx := context.Background()

	buyer := user(t, q, 900000301, "entrepreneur", 2)  // Namangan shahri
	seller := user(t, q, 900000302, "entrepreneur", 3) // Chortoq, same viloyat
	outsider := user(t, q, 900000303, "entrepreneur", 3)
	stranger := user(t, q, 900000304, "entrepreneur", 2)
	if _, err := market.NewService(pool).Upsert(ctx, seller, market.Input{ProductID: 1, PriceSom: 3300, TradeType: "ulgurji"}); err != nil {
		t.Fatal(err)
	}

	// Validation and roles: market staff may neither ask nor answer.
	staff := user(t, q, 900000305, "market_admin", 2)
	if _, err := svc.Create(ctx, staff, Input{ProductID: 1, Quantity: 200}); !errors.Is(err, ErrNotBuyer) {
		t.Errorf("market staff creating an RFQ: %v, want ErrNotBuyer", err)
	}
	var ve *ValidationError
	if _, err := svc.Create(ctx, buyer, Input{ProductID: 1, Quantity: 200, Days: 9}); !errors.As(err, &ve) || ve.Field != "days" {
		t.Errorf("9 days: %v, want a days validation error", err)
	}

	created, err := svc.Create(ctx, buyer, Input{ProductID: 1, Quantity: 200})
	if err != nil {
		t.Fatal(err)
	}
	if created.Sellers != 1 {
		t.Errorf("RFQ reached %d sellers, want 1 (only the one with a live listing)", created.Sellers)
	}
	id := created.RFQ.ID

	// The buyer may keep at most Limits().MaxOpen requests open.
	for i := 1; i < svc.Limits().MaxOpen; i++ {
		if _, err := svc.Create(ctx, buyer, Input{ProductID: 2, Quantity: 10}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := svc.Create(ctx, buyer, Input{ProductID: 3, Quantity: 10}); !errors.Is(err, ErrTooManyOpen) {
		t.Errorf("request over the limit: %v, want ErrTooManyOpen", err)
	}

	// Only sellers with a live listing of the product in the viloyat may answer.
	if _, err := svc.Offer(ctx, outsider, id, 3200, nil); !errors.Is(err, ErrNotEligible) {
		t.Errorf("seller without a listing: %v, want ErrNotEligible", err)
	}
	if _, err := svc.Offer(ctx, staff, id, 3200, nil); !errors.Is(err, ErrNotSeller) {
		t.Errorf("market staff answering: %v, want ErrNotSeller", err)
	}
	offer, err := svc.Offer(ctx, seller, id, 3200, nil)
	if err != nil {
		t.Fatal(err)
	}
	var source, status string
	if err := pool.QueryRow(ctx, `SELECT source, status FROM price_observations WHERE offer_id = $1 AND status <> 'expired'`, offer.ID).
		Scan(&source, &status); err != nil || source != "rfq_offer" || status != "active" {
		t.Errorf("offer observation: %s %s %v, want an active rfq_offer price", source, status, err)
	}

	// Contacts stay hidden until both sides agree.
	if _, err := svc.Parties(ctx, buyer, offer.ID); !errors.Is(err, ErrBadState) {
		t.Errorf("contacts before agreement: %v, want ErrBadState", err)
	}
	if _, err := svc.Choose(ctx, stranger, offer.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("another buyer choosing: %v, want ErrNotFound", err)
	}
	if _, err := svc.Choose(ctx, buyer, offer.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Offer(ctx, seller, id, 3000, nil); !errors.Is(err, ErrOfferLocked) {
		t.Errorf("changing a picked offer: %v, want ErrOfferLocked", err)
	}
	if _, err := svc.Answer(ctx, seller, offer.ID, true); err != nil {
		t.Fatal(err)
	}
	// Agreement is a confirmed deal at the offer price; recording it again adds nothing.
	if err := q.RecordRFQDeal(ctx, offer.ID); err != nil {
		t.Fatal(err)
	}
	var deals int
	var price int64
	var qty float64
	var linked bool
	if err := pool.QueryRow(ctx, `
		SELECT count(*), max(price_som), max(quantity)::float8, bool_and(listing_id IS NOT NULL)
		FROM deals WHERE offer_id = $1 AND source = 'rfq' AND status = 'confirmed' AND buyer_id = $2`, offer.ID, buyer.ID).
		Scan(&deals, &price, &qty, &linked); err != nil || deals != 1 || price != 3200 || qty != 200 || !linked {
		t.Errorf("deal: %d rows, price %d, quantity %v, listing linked %v, %v; want one at 3 200 for 200", deals, price, qty, linked, err)
	}
	// Its price is already in the median as the offer; it is not added twice.
	var dealPrices int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM price_observations o JOIN deals d ON d.id = o.deal_id WHERE d.offer_id = $1`, offer.ID).Scan(&dealPrices); err != nil || dealPrices != 0 {
		t.Errorf("RFQ deal price counted again: %d %v", dealPrices, err)
	}
	// The database refuses a deal with oneself and a confirmation only one side gave.
	for name, sql := range map[string]string{
		"with oneself": `INSERT INTO deals (product_id, seller_id, buyer_id, source, unit, region_id) VALUES (1, $1, $1, 'contact', 'kg', 2)`,
		"one-sided":    `INSERT INTO deals (product_id, seller_id, source, status, seller_confirmed_at, unit, region_id) VALUES (1, $1, 'seller_marked', 'confirmed', now(), 'kg', 2)`,
	} {
		if _, err := pool.Exec(ctx, sql, seller.ID); err == nil {
			t.Errorf("deal %s was accepted", name)
		}
	}
	for _, u := range []db.User{buyer, seller} {
		if _, err := svc.Parties(ctx, u, offer.ID); err != nil {
			t.Errorf("contacts for %s after agreement: %v", u.Role.String, err)
		}
	}
	if _, err := svc.Parties(ctx, stranger, offer.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("contacts for a stranger: %v, want ErrNotFound", err)
	}

	want := []string{"created", "offer", "chosen", "answered"}
	if len(rec.events) != len(want) {
		t.Fatalf("notifications %v, want %v", rec.events, want)
	}
	for i := range want {
		if rec.events[i] != want[i] {
			t.Errorf("notification %d = %s, want %s", i, rec.events[i], want[i])
		}
	}
}

// An entrepreneur both sells and buys, but never deals with themselves: their
// own request does not reach them, they cannot answer it, and it is not in
// their inbox. They can still answer someone else's request.
func TestEntrepreneurOnBothSides(t *testing.T) {
	svc, rec, pool, q := setup(t)
	ctx := context.Background()
	a := user(t, q, 900000351, "entrepreneur", 2)
	b := user(t, q, 900000352, "entrepreneur", 3)
	listings := market.NewService(pool)
	for _, u := range []db.User{a, b} {
		if _, err := listings.Upsert(ctx, u, market.Input{ProductID: 6, PriceSom: 4100, TradeType: "ulgurji"}); err != nil {
			t.Fatal(err)
		}
	}

	own, err := svc.Create(ctx, a, Input{ProductID: 6, Quantity: 100})
	if err != nil {
		t.Fatal(err)
	}
	if own.Sellers != 1 {
		t.Errorf("own RFQ reached %d sellers, want 1 (the other entrepreneur, not the buyer)", own.Sellers)
	}
	recipients, err := q.RFQRecipients(ctx, db.RFQRecipientsParams{ProductID: 6, ProvinceID: 1, BuyerID: a.ID})
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range recipients {
		if r.ID == a.ID {
			t.Error("the buyer is among the recipients of their own RFQ")
		}
	}
	if _, err := svc.Offer(ctx, a, own.RFQ.ID, 4000, nil); !errors.Is(err, ErrNotFound) {
		t.Errorf("offer on one's own RFQ: %v, want ErrNotFound", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO offers (rfq_id, seller_id, price_som, status) VALUES ($1, $2, 4000, 'sent')`,
		own.RFQ.ID, a.ID); err == nil {
		t.Error("the database accepted an offer on one's own RFQ")
	}
	inbox, err := q.SellerInbox(ctx, a.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range inbox {
		if r.ID == own.RFQ.ID {
			t.Error("own RFQ is in the inbox")
		}
	}

	// The same person answers another entrepreneur's request.
	other, err := svc.Create(ctx, b, Input{ProductID: 6, Quantity: 40})
	if err != nil {
		t.Fatal(err)
	}
	o, err := svc.Offer(ctx, a, other.RFQ.ID, 3900, nil)
	if err != nil {
		t.Fatalf("entrepreneur answering another's RFQ: %v", err)
	}
	if _, err := svc.Choose(ctx, a, o.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("the seller picking their own offer: %v, want ErrNotFound", err)
	}
	if _, err := svc.Choose(ctx, b, o.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Answer(ctx, b, o.ID, true); !errors.Is(err, ErrNotFound) {
		t.Errorf("the buyer confirming for the seller: %v, want ErrNotFound", err)
	}
	if _, err := svc.Answer(ctx, a, o.ID, true); err != nil {
		t.Fatal(err)
	}
	p, err := svc.Parties(ctx, a, o.ID)
	if err != nil || p.BuyerID != b.ID || p.SellerID != a.ID {
		t.Errorf("parties: %+v %v, want buyer b and seller a", p, err)
	}
	if len(rec.events) == 0 || rec.events[0] != "created" {
		t.Errorf("notifications %v, want the RFQ announced first", rec.events)
	}
}

func TestDeclineAndCancel(t *testing.T) {
	svc, _, pool, q := setup(t)
	ctx := context.Background()
	buyer := user(t, q, 900000311, "entrepreneur", 2)
	seller := user(t, q, 900000312, "entrepreneur", 2)
	if _, err := market.NewService(pool).Upsert(ctx, seller, market.Input{ProductID: 2, PriceSom: 1900, TradeType: "ulgurji"}); err != nil {
		t.Fatal(err)
	}
	created, err := svc.Create(ctx, buyer, Input{ProductID: 2, Quantity: 500})
	if err != nil {
		t.Fatal(err)
	}
	o, err := svc.Offer(ctx, seller, created.RFQ.ID, 0, nil)
	if err != nil || o.Status != StatusDeclined || o.PriceSom.Valid {
		t.Fatalf("Menda yo'q: %+v %v, want declined without price", o, err)
	}
	// "Menda yo'q" leaves no price in the market history.
	var n int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM price_observations WHERE offer_id = $1`, o.ID).Scan(&n); err != nil || n != 0 {
		t.Errorf("observations for a declined offer: %d %v, want 0", n, err)
	}
	// The seller may still change their mind before the deadline.
	if o, err = svc.Offer(ctx, seller, created.RFQ.ID, 1850, nil); err != nil || o.Status != StatusSent {
		t.Errorf("offer after declining: %+v %v", o, err)
	}
	if err := svc.Cancel(ctx, buyer, created.RFQ.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Offer(ctx, seller, created.RFQ.ID, 1800, nil); !errors.Is(err, ErrClosed) {
		t.Errorf("offer on a cancelled RFQ: %v, want ErrClosed", err)
	}
	if err := svc.Cancel(ctx, buyer, created.RFQ.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("second cancel: %v, want ErrNotFound", err)
	}
}

// Spec §14.1 / CLAUDE §4.4: two requests at the same moment must not slip past
// the open-RFQ limit.
func TestOpenLimitHoldsUnderConcurrency(t *testing.T) {
	svc, _, _, q := setup(t)
	ctx := context.Background()
	buyer := user(t, q, 900000321, "entrepreneur", 2)
	for i := 1; i < svc.Limits().MaxOpen; i++ {
		if _, err := svc.Create(ctx, buyer, Input{ProductID: 1, Quantity: 10}); err != nil {
			t.Fatal(err)
		}
	}
	var wg sync.WaitGroup
	var mu sync.Mutex
	ok, limited := 0, 0
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := svc.Create(ctx, buyer, Input{ProductID: 1, Quantity: 10})
			mu.Lock()
			defer mu.Unlock()
			switch {
			case err == nil:
				ok++
			case errors.Is(err, ErrTooManyOpen):
				limited++
			default:
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if ok != 1 || limited != 4 {
		t.Errorf("concurrent creates: %d succeeded, %d limited; want 1 and 4", ok, limited)
	}
}

// A deleted offer's price must leave the median at once (like listings, 00008).
func TestDeletedOfferLeavesMedian(t *testing.T) {
	svc, _, pool, q := setup(t)
	ctx := context.Background()
	buyer := user(t, q, 900000331, "entrepreneur", 2)
	seller := user(t, q, 900000332, "entrepreneur", 2)
	if _, err := market.NewService(pool).Upsert(ctx, seller, market.Input{ProductID: 4, PriceSom: 6000, TradeType: "ulgurji"}); err != nil {
		t.Fatal(err)
	}
	created, err := svc.Create(ctx, buyer, Input{ProductID: 4, Quantity: 100})
	if err != nil {
		t.Fatal(err)
	}
	o, err := svc.Offer(ctx, seller, created.RFQ.ID, 5900, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := q.DeleteUser(ctx, buyer.ID); err != nil { // cascades to the RFQ and its offers
		t.Fatal(err)
	}
	var live int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM price_observations WHERE source = 'rfq_offer' AND status = 'active'
		AND (offer_id = $1 OR offer_id IS NULL)`, o.ID).Scan(&live); err != nil {
		t.Fatal(err)
	}
	if live != 0 {
		t.Errorf("%d live offer prices after the RFQ was deleted, want 0", live)
	}
}

// A picked offer stays in the seller's inbox after the deadline, so the seller
// can still confirm it.
func TestInboxKeepsPickedOffersAfterDeadline(t *testing.T) {
	svc, _, pool, q := setup(t)
	ctx := context.Background()
	buyer := user(t, q, 900000341, "entrepreneur", 2)
	seller := user(t, q, 900000342, "entrepreneur", 2)
	if _, err := market.NewService(pool).Upsert(ctx, seller, market.Input{ProductID: 5, PriceSom: 2500, TradeType: "ulgurji"}); err != nil {
		t.Fatal(err)
	}
	created, err := svc.Create(ctx, buyer, Input{ProductID: 5, Quantity: 50})
	if err != nil {
		t.Fatal(err)
	}
	o, err := svc.Offer(ctx, seller, created.RFQ.ID, 2400, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Choose(ctx, buyer, o.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE rfqs SET deadline = now() - interval '1 hour' WHERE id = $1`, created.RFQ.ID); err != nil {
		t.Fatal(err)
	}
	rows, err := q.SellerInbox(ctx, seller.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].OfferStatus.String != StatusChosen {
		t.Fatalf("inbox after deadline: %+v, want the picked offer", rows)
	}
	if _, err := svc.Answer(ctx, seller, o.ID, true); err != nil {
		t.Errorf("confirming after the deadline: %v", err)
	}
}

func TestRFQDemandKeyUsesSiteHash(t *testing.T) {
	svc, _, pool, q := setup(t)
	ctx := context.Background()
	svc.SetDemandKey(func(v string) []byte { return []byte("site|" + v) })
	buyer := user(t, q, 900000741, "entrepreneur", 2)
	if _, err := svc.Create(ctx, buyer, Input{ProductID: 4, Quantity: 50}); err != nil {
		t.Fatal(err)
	}
	day := time.Now().In(tz.Tashkent).Format(time.DateOnly)
	want := []byte("site|" + day + "|u:" + buyer.ID.String())
	var n int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM demand_events WHERE kind = 'rfq' AND viewer_key = $1`, want).Scan(&n); err != nil || n != 1 {
		t.Fatalf("rfq signals under the site key: %d, err %v", n, err)
	}
}

func TestConfiguredLimits(t *testing.T) {
	_, _, pool, q := setup(t)
	svc := NewService(pool, nil, WithLimits(Limits{MaxOpen: 1, DefaultDays: 2, MinDays: 2, MaxDays: 4}))
	ctx := context.Background()
	buyer := user(t, q, 900000391, "entrepreneur", 2)

	var ve *ValidationError
	for _, days := range []int{1, 5} {
		if _, err := svc.Create(ctx, buyer, Input{ProductID: 1, Quantity: 10, Days: days}); !errors.As(err, &ve) || ve.Field != "days" {
			t.Errorf("%d days: %v, want a days validation error", days, err)
		} else if ve.Params["min"] != 2 || ve.Params["max"] != 4 {
			t.Errorf("%d days: params %v, want min 2 max 4", days, ve.Params)
		}
	}
	created, err := svc.Create(ctx, buyer, Input{ProductID: 1, Quantity: 10})
	if err != nil {
		t.Fatal(err)
	}
	if d := time.Until(created.RFQ.Deadline); d < 47*time.Hour || d > 49*time.Hour {
		t.Errorf("default deadline in %s, want about 2 days", d)
	}
	if _, err := svc.Create(ctx, buyer, Input{ProductID: 2, Quantity: 10}); !errors.Is(err, ErrTooManyOpen) {
		t.Errorf("second open request with MaxOpen 1: %v, want ErrTooManyOpen", err)
	}
}
