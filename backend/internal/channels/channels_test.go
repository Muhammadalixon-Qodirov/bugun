package channels

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"bugun/internal/pricing"
	"bugun/internal/store"
	"bugun/internal/store/db"
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
	prices, err := pricing.NewService(ctx, pool)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM price_observations WHERE tg_chat_id < 0`)
		_, _ = pool.Exec(context.Background(), `DELETE FROM telegram_channels WHERE chat_id < 0`)
	})
	return NewService(pool, prices), pool, db.New(pool)
}

func user(t *testing.T, q *db.Queries, tgID int64, role string, region int16) db.User {
	t.Helper()
	ctx := context.Background()
	id, err := q.UpsertTelegramUser(ctx, db.UpsertTelegramUserParams{TelegramID: tgID, DisplayName: "Test " + role})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = q.DeleteUser(context.Background(), id) })
	p := db.UpdateProfileParams{ID: id, Role: pgtype.Text{String: role, Valid: true}, RoleApproved: true, Categories: []string{"sabzavot"}}
	if region != 0 {
		p.RegionID = pgtype.Int2{Int16: region, Valid: true}
	}
	u, err := q.UpdateProfile(ctx, p)
	if err != nil {
		t.Fatal(err)
	}
	if err := q.SetBotEnabled(ctx, db.SetBotEnabledParams{ID: id, BotEnabled: true}); err != nil {
		t.Fatal(err)
	}
	u.BotEnabled = true
	return u
}

func TestChannelPosts(t *testing.T) {
	svc, pool, q := setup(t)
	ctx := context.Background()
	owner := user(t, q, 900000801, "entrepreneur", 2)
	count := func(sql string, args ...any) (n int) {
		t.Helper()
		if err := pool.QueryRow(ctx, sql, args...).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}

	// A chat nobody connected is not read.
	if _, ok, err := svc.Ingest(ctx, Post{ChatID: -1001, MessageID: 1, Text: "pomidor 6000"}); ok || err != nil {
		t.Fatalf("unconnected chat read: %v %v", ok, err)
	}
	ch, err := svc.Connected(ctx, Chat{ID: -1001, Kind: KindChannel, Title: "Test kanal", Username: "test_kanal"}, owner.TelegramID, &owner)
	if err != nil || ch.Status != "active" || !ch.ProvinceID.Valid || ch.ProvinceID.Int16 == 2 {
		t.Fatalf("connected: %+v %v (tuman 2 gives its viloyat)", ch, err)
	}

	res, ok, err := svc.Ingest(ctx, Post{ChatID: -1001, MessageID: 10, Text: "🍅 Pomidor 6000\nkartoshka 3500 so'm\nnimadir yozuv"})
	if err != nil || !ok || len(res.Taken) != 2 || res.PostsSeen != 1 {
		t.Fatalf("post: %+v %v %v", res, ok, err)
	}
	if n := count(`SELECT count(*) FROM price_observations WHERE tg_chat_id = -1001 AND tg_message_id = 10 AND source = 'telegram_channel'
		AND status = 'active' AND source_url = 'https://t.me/test_kanal/10' AND seller_id = $1`, owner.ID); n != 2 {
		t.Errorf("%d channel prices stored, want 2 with the post link and the owner", n)
	}
	// An edit replaces the post's prices.
	if _, _, err := svc.Ingest(ctx, Post{ChatID: -1001, MessageID: 10, Text: "pomidor 6200", Edited: true}); err != nil {
		t.Fatal(err)
	}
	if n := count(`SELECT count(*) FROM price_observations WHERE tg_chat_id = -1001 AND tg_message_id = 10 AND status = 'active'`); n != 1 {
		t.Errorf("after the edit %d live prices, want 1", n)
	}
	if n := count(`SELECT count(*) FROM listings WHERE seller_id = $1`, owner.ID); n != 0 {
		t.Errorf("listings updated without the owner asking: %d", n)
	}

	// With "update my listings" on, the post also updates the owner's listing.
	if _, err := svc.SetUpdateListings(ctx, owner, ch.ID, true); err != nil {
		t.Fatal(err)
	}
	res, _, err = svc.Ingest(ctx, Post{ChatID: -1001, MessageID: 11, Text: "piyoz 2500"})
	if err != nil || res.Listings != 1 || count(`SELECT count(*) FROM listings WHERE seller_id = $1 AND price_som = 2500`, owner.ID) != 1 {
		t.Errorf("listing update: %+v %v", res, err)
	}

	// A price twice the viloyat median is held for review.
	for _, p := range []int{3000, 3100, 3200} {
		if _, err := pool.Exec(ctx, `INSERT INTO price_observations (product_id, price_som, unit, trade_type, region_id, source, valid_until, raw_text, tg_chat_id)
			VALUES (7, $1, 'kg', 'ulgurji', $2, 'seller', now() + interval '1 day', 'channel-test', -1)`, p, ch.ProvinceID.Int16); err != nil {
			t.Fatal(err)
		}
	}
	name := ""
	if err := pool.QueryRow(ctx, `SELECT name FROM products WHERE id = 7`).Scan(&name); err != nil {
		t.Fatal(err)
	}
	res, _, err = svc.Ingest(ctx, Post{ChatID: -1001, MessageID: 12, Text: name + " ulgurji 9000"})
	if err != nil || len(res.Taken) != 1 || res.Held != 1 {
		t.Errorf("far price: %+v %v", res, err)
	}
	if svc.Removed(ctx, -1001) != nil {
		t.Fatal("remove")
	}
	if _, ok, _ := svc.Ingest(ctx, Post{ChatID: -1001, MessageID: 13, Text: "pomidor 6000"}); ok {
		t.Error("a removed channel is still read")
	}
}

func TestPendingGroupClaimedLater(t *testing.T) {
	svc, pool, q := setup(t)
	ctx := context.Background()
	// Added by someone without a profile yet: pending, nothing read.
	ch, err := svc.Connected(ctx, Chat{ID: -1002, Kind: KindSupergroup, Title: "Savdo guruhi"}, 900000811, nil)
	if err != nil || ch.Status != "pending" {
		t.Fatalf("pending: %+v %v", ch, err)
	}
	if _, ok, _ := svc.Ingest(ctx, Post{ChatID: -1002, MessageID: 1, Text: "pomidor 6000", SenderTg: 900000812}); ok {
		t.Error("pending group read")
	}
	// They sign up, see it in their list and pick a viloyat.
	adder := user(t, q, 900000811, "entrepreneur", 0)
	mine, err := svc.Mine(ctx, adder)
	if err != nil || len(mine) != 1 {
		t.Fatalf("mine: %+v %v", mine, err)
	}
	stranger := user(t, q, 900000813, "entrepreneur", 0)
	if _, err := svc.SetProvince(ctx, stranger, ch.ID, 1); !errors.Is(err, ErrNotFound) {
		t.Errorf("stranger claimed the group: %v", err)
	}
	if ch, err = svc.SetProvince(ctx, adder, ch.ID, 1); err != nil || ch.Status != "active" {
		t.Fatalf("claim: %+v %v", ch, err)
	}
	// A seller's message in the group is theirs; someone unknown gives an anonymous price.
	seller := user(t, q, 900000812, "entrepreneur", 2)
	for i, sender := range []int64{seller.TelegramID, 900000899} {
		if res, ok, err := svc.Ingest(ctx, Post{ChatID: -1002, MessageID: int64(20 + i), Text: "sabzi 3000", SenderTg: sender}); err != nil || !ok || len(res.Taken) != 1 {
			t.Fatalf("group message %d: %+v %v %v", i, res, ok, err)
		}
	}
	var mine2, anon int
	if err := pool.QueryRow(ctx, `SELECT count(*) FILTER (WHERE seller_id = $1), count(*) FILTER (WHERE seller_id IS NULL)
		FROM price_observations WHERE tg_chat_id = -1002`, seller.ID).Scan(&mine2, &anon); err != nil || mine2 != 1 || anon != 1 {
		t.Errorf("group prices: seller %d, anonymous %d, %v", mine2, anon, err)
	}
	if chat, err := svc.Disconnect(ctx, adder, ch.ID); err != nil || chat != -1002 {
		t.Errorf("disconnect: %d %v", chat, err)
	}
}

type fakeModel struct {
	cands []pricing.Candidate
	calls int
}

func (f *fakeModel) Extract(context.Context, string, string) ([]pricing.Candidate, error) {
	f.calls++
	return f.cands, nil
}

// A post the rules cannot read goes to the model; only candidates written in
// the post become prices, marked with the model's confidence.
func TestModelReadsHardPosts(t *testing.T) {
	svc, pool, q := setup(t)
	ctx := context.Background()
	owner := user(t, q, 900000821, "entrepreneur", 2)
	if _, err := svc.Connected(ctx, Chat{ID: -1003, Kind: KindChannel, Title: "Qiyin kanal"}, owner.TelegramID, &owner); err != nil {
		t.Fatal(err)
	}
	model := &fakeModel{cands: []pricing.Candidate{
		{Product: "pomidor", Price: 7, Trade: "chakana"},      // "7 lik" in the post: 7 000
		{Product: "kartoshka", Price: 4200, Trade: "chakana"}, // not in the post
	}}
	svc.prices.SetExtractor(model)
	res, ok, err := svc.Ingest(ctx, Post{ChatID: -1003, MessageID: 1, Text: "Bugun qizil narsalarimiz 7 lik, kelinglar!"})
	if err != nil || !ok || !res.ByModel || len(res.Taken) != 1 || res.Taken[0].PriceSom != 7000 {
		t.Fatalf("model post: %+v %v %v", res, ok, err)
	}
	var conf float32
	if err := pool.QueryRow(ctx, `SELECT ai_confidence FROM price_observations WHERE tg_chat_id = -1003`).Scan(&conf); err != nil || conf != 0.5 {
		t.Fatalf("confidence %v %v", conf, err)
	}
	// Posts the rules read, or without any number, never reach the model.
	if _, _, err := svc.Ingest(ctx, Post{ChatID: -1003, MessageID: 2, Text: "pomidor 6500"}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.Ingest(ctx, Post{ChatID: -1003, MessageID: 3, Text: "Salom, bugun bozor yopiq"}); err != nil {
		t.Fatal(err)
	}
	if model.calls != 1 {
		t.Errorf("model called %d times, want 1", model.calls)
	}
}
