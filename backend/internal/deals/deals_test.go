package deals

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"bugun/internal/market"
	"bugun/internal/store"
	"bugun/internal/store/db"
	"bugun/internal/tz"
)

func setup(t *testing.T) (*Service, *pgxpool.Pool, *db.Queries) {
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
	return NewService(pool), pool, db.New(pool)
}

func user(t *testing.T, q *db.Queries, tgID int64, role string) db.User {
	t.Helper()
	ctx := context.Background()
	id, err := q.UpsertTelegramUser(ctx, db.UpsertTelegramUserParams{TelegramID: tgID, DisplayName: "Test " + role})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = q.DeleteUser(context.Background(), id) })
	u, err := q.UpdateProfile(ctx, db.UpdateProfileParams{
		ID: id, Role: pgtype.Text{String: role, Valid: true}, RoleApproved: true,
		RegionID: pgtype.Int2{Int16: 2, Valid: true}, Categories: []string{"sabzavot"},
	})
	if err != nil {
		t.Fatal(err)
	}
	return u
}

func at(clock string) time.Time {
	t, err := time.ParseInLocation("2006-01-02 15:04", "2026-09-25 "+clock, tz.Tashkent)
	if err != nil {
		panic(err)
	}
	return t
}

func TestInQuietHours(t *testing.T) {
	for clock, quiet := range map[string]bool{"08:59": true, "09:00": false, "19:59": false, "20:00": true, "23:30": true} {
		if got := InQuietHours(at(clock)); got != quiet {
			t.Errorf("InQuietHours(%s) = %v, want %v", clock, got, quiet)
		}
	}
	if InQuietHours(at("12:00").UTC()) {
		t.Error("the server clock's zone must not matter")
	}
}

func TestFollowUpAfterContact(t *testing.T) {
	svc, pool, q := setup(t)
	ctx := context.Background()
	seller := user(t, q, 900000601, "entrepreneur")
	buyer := user(t, q, 900000602, "entrepreneur")
	listings := market.NewService(pool)
	var ids []int64
	for _, product := range []int16{1, 2, 3, 4, 5} {
		saved, err := listings.Upsert(ctx, seller, market.Input{ProductID: product, PriceSom: 4000, TradeType: "ulgurji"})
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, saved.ID)
	}
	prompts := func(listing int64) (n int) {
		if err := pool.QueryRow(ctx, `SELECT count(*) FROM deal_prompts WHERE listing_id = $1`, listing).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}

	// One question per buyer and listing a week; never to the seller themselves.
	for range 2 {
		if err := svc.ScheduleAfterContact(ctx, buyer.ID, ids[0]); err != nil {
			t.Fatal(err)
		}
	}
	if err := svc.ScheduleAfterContact(ctx, seller.ID, ids[0]); err != nil {
		t.Fatal(err)
	}
	if n := prompts(ids[0]); n != 1 {
		t.Errorf("%d questions after two reveals and the seller's own, want 1", n)
	}
	for _, id := range ids[1:] {
		if err := svc.ScheduleAfterContact(ctx, buyer.ID, id); err != nil {
			t.Fatal(err)
		}
	}

	// Nothing is due for a day, and nothing goes out at night.
	if got, err := svc.TakeDue(ctx, at("12:00")); err != nil || len(got) != 0 {
		t.Fatalf("due right away: %v %v", got, err)
	}
	if _, err := pool.Exec(ctx, `UPDATE deal_prompts SET due_at = now() - interval '1 hour' WHERE buyer_id = $1`, buyer.ID); err != nil {
		t.Fatal(err)
	}
	if got, err := svc.TakeDue(ctx, at("22:00")); err != nil || len(got) != 0 {
		t.Fatalf("sent at night: %v %v", got, err)
	}
	// Five are due, but a buyer gets at most three a day; a second run sends none.
	first, err := svc.TakeDue(ctx, at("12:00"))
	if err != nil || len(first) != MaxPerDay {
		t.Fatalf("first run: %v %v, want %d", first, err, MaxPerDay)
	}
	if again, err := svc.TakeDue(ctx, at("12:15")); err != nil || len(again) != 0 {
		t.Errorf("second run the same day: %v %v, want none", again, err)
	}

	// Answers: only the buyer may answer; "yes" records a deal once.
	if _, err := svc.Answer(ctx, seller, first[0], AnswerYes); !errors.Is(err, ErrNotFound) {
		t.Errorf("seller answering the buyer's question: %v, want not found", err)
	}
	deal, err := svc.Answer(ctx, buyer, first[0], AnswerYes)
	if err != nil || deal == nil || deal.Status != "reported" || deal.Source != "contact" || deal.BuyerConfirmedAt == nil {
		t.Fatalf("yes: %+v %v, want a reported contact deal", deal, err)
	}
	if _, err := svc.Answer(ctx, buyer, first[0], AnswerNo); !errors.Is(err, ErrAnswered) {
		t.Errorf("answering twice: %v, want already answered", err)
	}

	// "Still talking" asks once more in 48 hours; the second time it is not offered.
	if _, err := svc.Answer(ctx, buyer, first[1], AnswerTalking); err != nil {
		t.Fatal(err)
	}
	var sent bool
	var hours float64
	if err := pool.QueryRow(ctx, `SELECT sent_at IS NOT NULL, extract(epoch FROM due_at - now()) / 3600 FROM deal_prompts WHERE id = $1`, first[1]).
		Scan(&sent, &hours); err != nil || sent || hours < 47 || hours > 49 {
		t.Errorf("after 'talking': sent %v, due in %.1f h, %v; want unsent, due in 48 h", sent, hours, err)
	}
	if _, err := pool.Exec(ctx, `UPDATE deal_prompts SET asks = 2 WHERE id = $1`, first[1]); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Answer(ctx, buyer, first[1], AnswerTalking); !errors.Is(err, ErrAnswered) {
		t.Errorf("'talking' a second time: %v, want refused", err)
	}
	if _, err := svc.Answer(ctx, buyer, first[2], AnswerNo); err != nil {
		t.Errorf("no: %v", err)
	}

	// Amount and price: checked; the seller fills only what is empty, the
	// buyer's figures win; nobody else may.
	stranger := user(t, q, 900000603, "entrepreneur")
	qty, price := 200.0, int64(3900)
	if _, err := svc.SetDetails(ctx, stranger, deal.ID, &qty, &price); !errors.Is(err, ErrNotFound) {
		t.Errorf("stranger setting details: %v, want not found", err)
	}
	bad := int64(10)
	if _, err := svc.SetDetails(ctx, buyer, deal.ID, &qty, &bad); !errors.Is(err, ErrInvalid) {
		t.Errorf("price 10: %v, want invalid", err)
	}
	sellerQty, sellerPrice := 300.0, int64(4100)
	if d, err := svc.SetDetails(ctx, seller, deal.ID, &sellerQty, &sellerPrice); err != nil || d.PriceSom.Int64 != 4100 {
		t.Errorf("seller filling empty details: %+v %v", d, err)
	}
	d, err := svc.SetDetails(ctx, buyer, deal.ID, &qty, &price)
	if err != nil || d.PriceSom.Int64 != 3900 || market.Float(d.Quantity) == nil || *market.Float(d.Quantity) != 200 {
		t.Errorf("buyer's details: %+v %v, want 200 at 3 900", d, err)
	}
	if d, err := svc.SetDetails(ctx, seller, deal.ID, &sellerQty, &sellerPrice); err != nil || d.PriceSom.Int64 != 3900 {
		t.Errorf("seller overwrote the buyer's price: %+v %v", d, err)
	}

	// A question never sent within 3 days expires.
	if _, err := pool.Exec(ctx, `UPDATE deal_prompts SET due_at = now() - interval '4 days' WHERE sent_at IS NULL AND buyer_id = $1`, buyer.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.TakeDue(ctx, at("12:30")); err != nil {
		t.Fatal(err)
	}
	var expired int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM deal_prompts WHERE buyer_id = $1 AND answer = 'expired'`, buyer.ID).Scan(&expired); err != nil || expired != 3 {
		t.Errorf("%d expired, want 3 (two never sent, one still talking) %v", expired, err)
	}
}

// Both orders of answers end in one confirmed deal.
func TestSellerAndBuyerConfirm(t *testing.T) {
	svc, pool, q := setup(t)
	ctx := context.Background()
	seller := user(t, q, 900000611, "entrepreneur")
	buyer := user(t, q, 900000612, "entrepreneur")
	other := user(t, q, 900000613, "entrepreneur")
	listings := market.NewService(pool)
	var ids []int64
	for _, product := range []int16{6, 7} {
		saved, err := listings.Upsert(ctx, seller, market.Input{ProductID: product, PriceSom: 4000, TradeType: "ulgurji"})
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, saved.ID)
	}
	ask := func(listing int64) int64 {
		t.Helper()
		if err := svc.ScheduleAfterContact(ctx, buyer.ID, listing); err != nil {
			t.Fatal(err)
		}
		var id int64
		if err := pool.QueryRow(ctx, `UPDATE deal_prompts SET sent_at = now(), asks = 1 WHERE listing_id = $1 AND buyer_id = $2 RETURNING id`, listing, buyer.ID).Scan(&id); err != nil {
			t.Fatal(err)
		}
		return id
	}
	count := func(listing int64, status string) (n int) {
		if err := pool.QueryRow(ctx, `SELECT count(*) FROM deals WHERE listing_id = $1 AND status = $2`, listing, status).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}

	if _, _, err := svc.SellerSold(ctx, other, ids[0]); !errors.Is(err, ErrNotFound) {
		t.Errorf("another seller marking the listing sold: %v, want not found", err)
	}

	// Buyer first, then seller.
	if _, err := svc.Answer(ctx, buyer, ask(ids[0]), AnswerYes); err != nil {
		t.Fatal(err)
	}
	d, matched, err := svc.SellerSold(ctx, seller, ids[0])
	if err != nil || !matched || d.Status != "confirmed" || !d.BuyerID.Valid {
		t.Errorf("buyer then seller: %+v matched %v %v", d, matched, err)
	}

	// Seller first, then buyer.
	d, matched, err = svc.SellerSold(ctx, seller, ids[1])
	if err != nil || matched || d.Status != "reported" || d.BuyerID.Valid {
		t.Errorf("seller alone: %+v matched %v %v", d, matched, err)
	}
	got, err := svc.Answer(ctx, buyer, ask(ids[1]), AnswerYes)
	if err != nil || got.ID != d.ID || got.Status != "confirmed" || !got.BuyerID.Valid || got.BuyerID.Bytes != buyer.ID {
		t.Errorf("seller then buyer: %+v %v, want deal %d confirmed", got, err, d.ID)
	}
	for _, id := range ids {
		if c, r := count(id, "confirmed"), count(id, "reported"); c != 1 || r != 0 {
			t.Errorf("listing %d: %d confirmed, %d reported; want 1 and 0", id, c, r)
		}
	}
}

func TestRatings(t *testing.T) {
	svc, pool, q := setup(t)
	ctx := context.Background()
	seller := user(t, q, 900000621, "entrepreneur")
	buyer := user(t, q, 900000622, "entrepreneur")
	if err := q.SetBotEnabled(ctx, db.SetBotEnabledParams{ID: buyer.ID, BotEnabled: true}); err != nil {
		t.Fatal(err)
	}
	deal := func(status string, daysAgo int) int64 {
		t.Helper()
		var id int64
		if err := pool.QueryRow(ctx, `
			INSERT INTO deals (product_id, seller_id, buyer_id, source, status, buyer_confirmed_at, seller_confirmed_at, unit, region_id)
			VALUES (1, $1, $2, 'contact', $3, now() - make_interval(days => $4::int),
			        CASE WHEN $3 = 'confirmed' THEN now() - make_interval(days => $4::int) END, 'kg', 2) RETURNING id`,
			seller.ID, buyer.ID, status, daysAgo).Scan(&id); err != nil {
			t.Fatal(err)
		}
		return id
	}
	old, fresh, open := deal("confirmed", 4), deal("confirmed", 1), deal("reported", 4)

	// Only the deal confirmed 3+ days ago is asked about, and only once.
	claimed, err := svc.TakeToRate(ctx)
	if err != nil {
		t.Fatal(err)
	}
	has := func(ids []int64, id int64) bool {
		for _, x := range ids {
			if x == id {
				return true
			}
		}
		return false
	}
	if !has(claimed, old) || has(claimed, fresh) || has(claimed, open) {
		t.Errorf("claimed %v: want %d, not %d or %d", claimed, old, fresh, open)
	}
	if again, _ := svc.TakeToRate(ctx); has(again, old) {
		t.Error("asked twice about the same deal")
	}

	if _, err := svc.Rate(ctx, seller, old, 5); !errors.Is(err, ErrNotRatable) {
		t.Errorf("seller rating their own deal: %v", err)
	}
	if _, err := svc.Rate(ctx, buyer, open, 5); !errors.Is(err, ErrNotRatable) {
		t.Errorf("rating an unconfirmed deal: %v", err)
	}
	if _, err := svc.Rate(ctx, buyer, old, 6); !errors.Is(err, ErrInvalid) {
		t.Errorf("6 stars: %v", err)
	}
	if _, err := svc.Comment(ctx, buyer, old, "yaxshi"); !errors.Is(err, ErrNotRatable) {
		t.Errorf("comment before stars: %v", err)
	}
	if r, err := svc.Rate(ctx, buyer, old, 2); err != nil || r.Stars != 2 || r.Status != "published" {
		t.Fatalf("2 stars: %+v %v", r, err)
	}
	if r, err := svc.Comment(ctx, buyer, old, "  Tarozi kam tortdi  "); err != nil || r.Status != "held" || r.Comment != "Tarozi kam tortdi" {
		t.Errorf("low rating comment: %+v %v, want held", r, err)
	}
	if r, err := svc.Rate(ctx, buyer, old, 4); err != nil || r.Status != "published" {
		t.Errorf("changed to 4 stars: %+v %v, want published", r, err)
	}
	if _, err := svc.Comment(ctx, buyer, old, strings.Repeat("a", MaxComment+1)); !errors.Is(err, ErrInvalid) {
		t.Errorf("long comment: %v", err)
	}
}
