// Package insight builds the product analytics page (TAHLIL_REJA.md) from the
// daily price history and live market data.
package insight

import (
	"context"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"bugun/internal/store/db"
	"bugun/internal/tz"
)

type Service struct {
	pool          *pgxpool.Pool
	q             *db.Queries
	anomalyRules  AnomalyRules
	forecastRules ForecastRules
	clusterRules  ClusterRules

	mu    sync.Mutex
	cache map[Params]cached
	pages map[string]page // other cached pages by key
}

type page struct {
	at time.Time
	v  any
}

func (s *Service) cached(key string) (any, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.pages[key]
	if !ok || time.Since(p.at) >= cacheFor {
		return nil, false
	}
	return p.v, true
}

func (s *Service) store(key string, v any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.pages == nil {
		s.pages = map[string]page{}
	}
	s.pages[key] = page{time.Now(), v}
}

// Option changes a Service's defaults.
type Option func(*Service)

// WithAnomalyRules sets the thresholds of the anomaly check.
func WithAnomalyRules(r AnomalyRules) Option { return func(s *Service) { s.anomalyRules = r } }

// WithForecastRules sets the forecast's parameters.
func WithForecastRules(r ForecastRules) Option { return func(s *Service) { s.forecastRules = r } }

// WithClusterRules sets the parameters of the viloyat grouping.
func WithClusterRules(r ClusterRules) Option { return func(s *Service) { s.clusterRules = r } }

func NewService(pool *pgxpool.Pool, opts ...Option) *Service {
	s := &Service{pool: pool, q: db.New(pool), anomalyRules: DefaultAnomalyRules,
		forecastRules: DefaultForecastRules, clusterRules: DefaultClusterRules}
	for _, o := range opts {
		o(s)
	}
	return s
}

// Day is a Tashkent calendar day as a date value.
func Day(t time.Time) pgtype.Date {
	t = t.In(tz.Tashkent)
	return pgtype.Date{Time: time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC), Valid: true}
}

// RebuildDaily recomputes the daily history for the Tashkent days from..to in
// one transaction, so readers never see a half-built range. It returns how
// many rows were written.
func (s *Service) RebuildDaily(ctx context.Context, from, to time.Time) (int, error) {
	f, t := Day(from), Day(to)
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)
	q := s.q.WithTx(tx)
	if err := q.DeletePriceDaily(ctx, db.DeletePriceDailyParams{FromDay: f, ToDay: t}); err != nil {
		return 0, err
	}
	n, err := q.BuildPriceDaily(ctx, db.BuildPriceDailyParams{FromDay: f, ToDay: t})
	if err != nil {
		return 0, err
	}
	return int(n), tx.Commit(ctx)
}

// RefreshRecent is the scheduler's job: yesterday and today, so late prices
// and today's changes are in.
func (s *Service) RefreshRecent(ctx context.Context) (int, error) {
	now := time.Now()
	return s.RebuildDaily(ctx, now.AddDate(0, 0, -1), now)
}
