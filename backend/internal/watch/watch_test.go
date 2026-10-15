package watch

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"bugun/internal/market"
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
	if err := q.SetBotEnabled(ctx, db.SetBotEnabledParams{ID: id, BotEnabled: true}); err != nil {
		t.Fatal(err)
	}
	return u
}

func TestWatchLimit(t *testing.T) {
	svc, _, q := setup(t)
	ctx := context.Background()
	u := user(t, q, 900000701, "entrepreneur")
	for p := int16(1); p <= MaxPerUser; p++ {
		if _, err := svc.Add(ctx, u, p, 0, "ulgurji", nil); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := svc.Add(ctx, u, MaxPerUser+1, 0, "ulgurji", nil); !errors.Is(err, ErrTooMany) {
		t.Errorf("watch over the limit: %v, want ErrTooMany", err)
	}
	limit := int64(3000)
	if w, err := svc.Add(ctx, u, 1, 0, "ulgurji", &limit); err != nil || w.MaxPrice.Int64 != 3000 {
		t.Errorf("changing an existing watch at the limit: %+v %v", w, err)
	}
	stranger := u
	stranger.ID = user(t, q, 900000702, "entrepreneur").ID
	list, _ := svc.List(ctx, u)
	if err := svc.Remove(ctx, stranger, list[0].ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("removing someone else's watch: %v", err)
	}
	if err := svc.Remove(ctx, u, list[0].ID); err != nil {
		t.Errorf("remove: %v", err)
	}
}

func TestFollowKeepsTheSiteLimit(t *testing.T) {
	svc, _, q := setup(t)
	ctx := context.Background()
	u := user(t, q, 900000703, "entrepreneur")
	limit := int64(4500)
	if _, err := svc.Add(ctx, u, 3, 0, "ulgurji", &limit); err != nil {
		t.Fatal(err)
	}
	if w, err := svc.Follow(ctx, u, 3, 0, "ulgurji"); err != nil || !w.MaxPrice.Valid || w.MaxPrice.Int64 != limit {
		t.Errorf("following again from the bot: limit %+v, err %v; want %d kept", w.MaxPrice, err, limit)
	}
	if w, err := svc.Follow(ctx, u, 4, 0, "ulgurji"); err != nil || w.MaxPrice.Valid {
		t.Errorf("new watch from the bot: limit %+v, err %v; want none", w.MaxPrice, err)
	}
	if w, err := svc.Add(ctx, u, 3, 0, "ulgurji", nil); err != nil || w.MaxPrice.Valid {
		t.Errorf("clearing the limit on the site: %+v, err %v", w.MaxPrice, err)
	}
}

func TestWatchAlerts(t *testing.T) {
	svc, pool, q := setup(t)
	ctx := context.Background()
	buyer := user(t, q, 900000711, "entrepreneur")
	seller := user(t, q, 900000712, "entrepreneur")
	province, err := market.ProvinceOf(ctx, q, 2)
	if err != nil {
		t.Fatal(err)
	}
	// Three prices around 6 000 so the median is known.
	for _, p := range []int{5900, 6000, 6100} {
		if _, err := pool.Exec(ctx, `
			INSERT INTO price_observations (product_id, price_som, unit, trade_type, region_id, source, valid_until, raw_text)
			VALUES (16, $1, 'kg', 'ulgurji', 2, 'seller', now() + interval '7 days', 'watch-test')`, p); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM price_observations WHERE raw_text = 'watch-test'`)
	})
	limit := int64(5000)
	w, err := svc.Add(ctx, buyer, 16, province, "ulgurji", &limit)
	if err != nil || !w.PeakMedian.Valid || w.PeakMedian.Float64 != 6000 {
		t.Fatalf("add: %+v %v, want the median 6 000 as the starting peak", w, err)
	}
	mine := func(alerts []Alert) *Alert {
		for i := range alerts {
			if alerts[i].Watch.ID == w.ID {
				return &alerts[i]
			}
		}
		return nil
	}
	now := time.Now()

	// Nothing new yet.
	if alerts, err := svc.Check(ctx, now); err != nil || mine(alerts) != nil {
		t.Fatalf("no news: %+v %v", mine(alerts), err)
	}
	// A seller posts 4 800, under the limit.
	if _, err := market.NewService(pool).Upsert(ctx, seller, market.Input{ProductID: 16, PriceSom: 4800, TradeType: "ulgurji"}); err != nil {
		t.Fatal(err)
	}
	alerts, err := svc.Check(ctx, now)
	a := mine(alerts)
	if err != nil || a == nil || !a.Cheaper || a.Price != 4800 {
		t.Fatalf("cheaper price: %+v %v", a, err)
	}
	// Once a day: a second check stays quiet.
	if alerts, err := svc.Check(ctx, now); err != nil || mine(alerts) != nil {
		t.Errorf("second alert the same day: %+v %v", mine(alerts), err)
	}

	// The median falls more than 5% from its peak (next day).
	if _, err := pool.Exec(ctx, `UPDATE watches SET peak_median = 7000, notified_at = now() - interval '2 days' WHERE id = $1`, w.ID); err != nil {
		t.Fatal(err)
	}
	alerts, err = svc.Check(ctx, now)
	a = mine(alerts)
	if err != nil || a == nil || !a.Dropped || a.Peak != 7000 || a.Median >= 7000*(1-Drop) {
		t.Errorf("median drop: %+v %v", a, err)
	}
	var peak float64
	if err := pool.QueryRow(ctx, `SELECT peak_median FROM watches WHERE id = $1`, w.ID).Scan(&peak); err != nil || peak == 7000 {
		t.Errorf("peak after the alert: %v %v, want the current median", peak, err)
	}
}
