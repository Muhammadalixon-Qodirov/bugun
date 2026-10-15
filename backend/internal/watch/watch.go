// Package watch lets a user follow a product and be told when it gets
// cheaper (QIZIQISH_VA_SAVDO.md step 8).
package watch

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"bugun/internal/analytics"
	"bugun/internal/store/db"
	"bugun/internal/tz"
)

const (
	// MaxPerUser caps how many watches one user keeps.
	MaxPerUser = 20
	// Drop is how far the median must fall from its recent peak, and how far
	// under the median a new price must be when no limit was set.
	Drop = 0.05
)

var (
	ErrTooMany  = errors.New("too many watches")
	ErrNotFound = errors.New("watch not found")
)

type Service struct {
	pool *pgxpool.Pool
	q    *db.Queries
}

func NewService(pool *pgxpool.Pool) *Service { return &Service{pool: pool, q: db.New(pool)} }

// Add follows a product in a viloyat (0 = whole country). Adding it again
// only changes the price limit. The limit per user holds under concurrency
// because the user row is locked.
func (s *Service) Add(ctx context.Context, u db.User, productID, province int16, trade string, maxPrice *int64) (db.Watch, error) {
	return s.add(ctx, u, productID, province, trade, maxPrice, false)
}

// Follow is Add without a limit of its own: a watch that already exists keeps
// the limit set on the site.
func (s *Service) Follow(ctx context.Context, u db.User, productID, province int16, trade string) (db.Watch, error) {
	return s.add(ctx, u, productID, province, trade, nil, true)
}

func (s *Service) add(ctx context.Context, u db.User, productID, province int16, trade string, maxPrice *int64, keepLimit bool) (db.Watch, error) {
	var w db.Watch
	err := s.inTx(ctx, func(q *db.Queries) error {
		if err := q.LockUser(ctx, u.ID); err != nil {
			return err
		}
		n, err := q.CountWatches(ctx, u.ID)
		if err != nil {
			return err
		}
		prov := pgtype.Int2{Int16: province, Valid: province != 0}
		existing, err := q.ListWatches(ctx, u.ID)
		if err != nil {
			return err
		}
		known := false
		limit := pgtype.Int8{}
		for _, e := range existing {
			if e.ProductID == productID && e.TradeType == trade && e.ProvinceID == prov {
				known = true
				if keepLimit {
					limit = e.MaxPrice
				}
			}
		}
		if !known && n >= MaxPerUser {
			return ErrTooMany
		}
		m, err := q.WatchMedian(ctx, db.WatchMedianParams{ProductID: productID, TradeType: trade, ProvinceID: prov})
		if err != nil {
			return err
		}
		peak := pgtype.Float8{}
		if m.Offers >= analytics.MinObservations {
			peak = pgtype.Float8{Float64: m.Median, Valid: true}
		}
		if maxPrice != nil {
			limit = pgtype.Int8{Int64: *maxPrice, Valid: true}
		}
		w, err = q.UpsertWatch(ctx, db.UpsertWatchParams{
			UserID: u.ID, ProductID: productID, ProvinceID: prov, TradeType: trade, MaxPrice: limit, PeakMedian: peak,
		})
		return err
	})
	return w, err
}

func (s *Service) Remove(ctx context.Context, u db.User, id int64) error {
	n, err := s.q.DeleteWatch(ctx, db.DeleteWatchParams{ID: id, UserID: u.ID})
	if err == nil && n == 0 {
		return ErrNotFound
	}
	return err
}

func (s *Service) List(ctx context.Context, u db.User) ([]db.ListWatchesRow, error) {
	return s.q.ListWatches(ctx, u.ID)
}

// Alert is one message to send.
type Alert struct {
	Watch     db.WatchesToCheckRow
	Cheaper   bool    // a new price at or under the limit
	Price     int64   // that price
	Region    string  // where that listing is
	ListingID int64   // that listing
	Dropped   bool    // the median fell Drop or more from its peak
	Median    float64 // the median now
	Peak      float64 // the peak it fell from
}

// Check looks at every watch and returns what to tell. A watch alerts at most
// once per Tashkent day; its peak median follows the market up, so a fall is
// measured from the recent high.
func (s *Service) Check(ctx context.Context, now time.Time) ([]Alert, error) {
	watches, err := s.q.WatchesToCheck(ctx)
	if err != nil {
		return nil, err
	}
	today := now.In(tz.Tashkent).Format(time.DateOnly)
	var alerts []Alert
	for _, w := range watches {
		m, err := s.q.WatchMedian(ctx, db.WatchMedianParams{ProductID: w.ProductID, TradeType: w.TradeType, ProvinceID: w.ProvinceID})
		if err != nil {
			return alerts, err
		}
		enough := m.Offers >= analytics.MinObservations
		a := Alert{Watch: w, Median: m.Median}
		peak := w.PeakMedian

		limit := int64(0)
		switch {
		case w.MaxPrice.Valid:
			limit = w.MaxPrice.Int64
		case enough:
			limit = int64(m.Median * (1 - Drop))
		}
		if limit > 0 {
			c, err := s.q.CheapestNewListing(ctx, db.CheapestNewListingParams{
				ProductID: w.ProductID, TradeType: w.TradeType, Since: w.CheckedAt, MaxPrice: limit,
				UserID: w.UserID, ProvinceID: w.ProvinceID,
			})
			switch {
			case err == nil:
				a.Cheaper, a.Price, a.Region, a.ListingID = true, c.PriceSom, c.RegionName, c.ID
			case !errors.Is(err, pgx.ErrNoRows):
				return alerts, err
			}
		}
		if enough && peak.Valid && m.Median <= peak.Float64*(1-Drop) {
			a.Dropped, a.Peak = true, peak.Float64
		}

		alerted := w.NotifiedAt != nil && w.NotifiedAt.In(tz.Tashkent).Format(time.DateOnly) == today
		notify := (a.Cheaper || a.Dropped) && !alerted
		switch {
		case notify && enough:
			peak = pgtype.Float8{Float64: m.Median, Valid: true} // measure the next fall from here
		case enough && (!peak.Valid || m.Median > peak.Float64):
			peak = pgtype.Float8{Float64: m.Median, Valid: true}
		}
		if err := s.q.MarkWatchChecked(ctx, db.MarkWatchCheckedParams{ID: w.ID, PeakMedian: peak, Notified: notify}); err != nil {
			return alerts, err
		}
		if notify {
			alerts = append(alerts, a)
		}
	}
	return alerts, nil
}

func (s *Service) inTx(ctx context.Context, fn func(q *db.Queries) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := fn(s.q.WithTx(tx)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
