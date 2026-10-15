package main

import (
	"context"
	"hash/fnv"
	"log/slog"
	"math"
	"math/rand/v2"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"bugun/internal/insight"
	"bugun/internal/tz"
)

// Demo price history (TAHLIL_REJA.md 2.2). The platform has no years of
// prices yet, so the analytics page is shown on a generated history: seasons
// by category, a steady gap between viloyatlar, yearly inflation, noise and a
// few market events. It is written straight into price_daily as synthetic
// rows with demo_share = 1, never as observations, so it cannot touch live
// medians; a viloyat-day with enough real prices keeps the real row.

// season is when a category is cheapest and how strongly prices swing.
type season struct {
	amplitude float64 // ± share around the yearly level
	peakMonth float64 // month (1–12) of the highest price
}

var seasons = map[string]season{
	"sabzavot":   {0.35, 2.5}, // winter storage and greenhouses: dear in Feb–Mar, cheap in Aug
	"meva":       {0.30, 4.5}, // dear before the new crop in spring, cheap in Sep
	"don":        {0.08, 5.5}, // before the July harvest
	"gosht":      {0.06, 3.5}, // holidays and spring
	"oziq_ovqat": {0.04, 1.5},
	"qurilish":   {0.05, 7}, // building season
	"xizmat":     {0.05, 9}, // harvest transport
}

const (
	inflation = 0.10 // a year
	noiseStep = 0.012
	noiseKeep = 0.93 // day-to-day memory of the noise
)

// demoEvent moves prices for a while; it also goes into the events log.
type demoEvent struct {
	Title      string
	Kind       string // frost, heat, export, fuel, holiday, harvest, roads
	DaysBefore int    // before the end of the history
	Days       int    // how long the effect lasts
	Effect     float64
	Categories []string // empty = all
	Products   []string // slugs; empty = the categories
	Provinces  []string // SOATO codes; empty = all
}

var demoEvents = []demoEvent{
	{Title: "Kuchli sovuq urdi, issiqxonalar zarar koʻrdi", Kind: "frost", DaysBefore: 960, Days: 28, Effect: 0.30, Categories: []string{"sabzavot"}},
	{Title: "Qozogʻistonga eksport kengaydi", Kind: "export", DaysBefore: 820, Days: 40, Effect: 0.15, Products: []string{"pomidor", "bodring", "piyoz"}},
	{Title: "Dizel yoqilgʻisi 20% qimmatladi", Kind: "fuel", DaysBefore: 620, Days: 90, Effect: 0.05},
	{Title: "Ramazon hayiti oldidan talab", Kind: "holiday", DaysBefore: 540, Days: 14, Effect: 0.12, Categories: []string{"gosht"}},
	{Title: "Kartoshka va piyozdan rekord hosil", Kind: "harvest", DaysBefore: 410, Days: 45, Effect: -0.25, Products: []string{"kartoshka", "piyoz"}},
	{Title: "Qor sababli togʻ yoʻllari yopildi", Kind: "roads", DaysBefore: 270, Days: 12, Effect: 0.18, Categories: []string{"sabzavot", "meva"}},
	{Title: "Navroʻz oldidan talab oshdi", Kind: "holiday", DaysBefore: 195, Days: 10, Effect: 0.08, Categories: []string{"sabzavot"}},
	{Title: "Jazirama (45°C): hosil tez buziladi", Kind: "heat", DaysBefore: 80, Days: 20, Effect: 0.20, Products: []string{"pomidor", "bodring", "rayhon", "kashnich", "petrushka"}},
}

// historyRange returns the days to fill: from years before the first real
// price up to today. Days with enough real prices replace generated ones.
func (s *seeder) historyRange(ctx context.Context, years int) (from, to time.Time, err error) {
	var first *time.Time
	err = s.pool.QueryRow(ctx, `
		SELECT min(observed_at) FROM price_observations
		WHERE status IN ('active', 'expired') AND source_trust(source) > 0`).Scan(&first)
	if err != nil {
		return
	}
	end := time.Now()
	if first != nil {
		end = *first
	}
	e := end.In(tz.Tashkent)
	firstReal := time.Date(e.Year(), e.Month(), e.Day(), 0, 0, 0, 0, time.UTC)
	to = insight.Day(time.Now()).Time
	from = firstReal.AddDate(-years, 0, -1)
	return
}

// seedHistory replaces the demo history with a new one.
func (s *seeder) seedHistory(ctx context.Context, years int, products []product) error {
	from, to, err := s.historyRange(ctx, years)
	if err != nil {
		return err
	}
	provinces, err := s.q.ListProvinces(ctx)
	if err != nil {
		return err
	}
	// One transaction: a failed run leaves the old history in place.
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `DELETE FROM price_daily WHERE synthetic`); err != nil {
		return err
	}
	// Open anomaly flags belong to the old history; the jobs look again.
	if _, err := tx.Exec(ctx, `DELETE FROM price_anomalies WHERE status = 'open'`); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `CREATE TEMP TABLE history_rows (LIKE price_daily INCLUDING DEFAULTS) ON COMMIT DROP`); err != nil {
		return err
	}
	dailyFrom := to.AddDate(-1, 0, 0) // older days are kept weekly, on Mondays
	columns := []string{"product_id", "trade_type", "province_id", "day", "median", "p25", "p75", "min_price", "max_price", "offers", "demo_share", "synthetic"}

	live, err := s.liveMedians(ctx)
	if err != nil {
		return err
	}
	var n int64
	for _, p := range products {
		sn := seasons[p.Category]
		markup := p.Markup
		if markup == 0 {
			markup = categoryMarkup[p.Category]
		}
		// The history ends where the live market is now, so the two meet
		// without a jump; without enough live prices the seed price is used.
		price := p.Price
		if m, ok := live[p.ID]; ok {
			price = m
		}
		anchor := seasonal(sn, to)
		shared := newNoise(s.rng)
		var rows [][]any // one product at a time keeps memory small
		for _, pv := range provinces {
			soato := strconv.FormatInt(pv.Soato.Int64, 10)
			offset := provinceOffset(p.Slug, soato)
			own := newNoise(s.rng)
			for day := from; !day.After(to); day = day.AddDate(0, 0, 1) {
				shock := shared.next() + own.next()
				if day.Before(dailyFrom) && day.Weekday() != time.Monday {
					continue
				}
				years := day.Sub(to).Hours() / 24 / 365
				level := price * math.Pow(1+inflation, years) * seasonal(sn, day) / anchor
				level *= (1 + offset) * (1 + shock) * (1 + eventEffect(p, soato, day, to))
				for _, trade := range []string{"ulgurji", "chakana"} {
					m := level
					if trade == "chakana" {
						m *= 1 + markup
					}
					spread := 0.06 + 0.04*s.rng.Float64()
					rows = append(rows, []any{
						p.ID, trade, pv.ID, pgtype.Date{Time: day, Valid: true},
						round(m), round(m * (1 - spread)), round(m * (1 + spread)),
						int64(round(m * (1 - 2*spread))), int64(round(m * (1 + 2.2*spread))), int32(3 + s.rng.IntN(13)), float32(1), true,
					})
				}
			}
		}
		if _, err := tx.CopyFrom(ctx, pgx.Identifier{"history_rows"}, columns, pgx.CopyFromRows(rows)); err != nil {
			return err
		}
	}
	if err := s.seedEvents(ctx, tx, to, products); err != nil {
		return err
	}
	// Days that already have enough real prices keep them.
	tag, err := tx.Exec(ctx, `INSERT INTO price_daily SELECT * FROM history_rows ON CONFLICT DO NOTHING`)
	if err != nil {
		return err
	}
	n = tag.RowsAffected()
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	slog.Info("demo history", "rows", n, "from", from.Format(time.DateOnly), "to", to.Format(time.DateOnly))
	return nil
}

// liveMedians is the nationwide wholesale median per product from live
// prices, for products with at least 3 of them.
func (s *seeder) liveMedians(ctx context.Context) (map[int16]float64, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT o.product_id, weighted_quantile(array_agg(o.price_som::float8 ORDER BY o.id),
		                                       array_agg(source_trust(o.source) ORDER BY o.id), 0.5)
		FROM price_observations o
		WHERE o.status = 'active' AND o.valid_until > now() AND o.trade_type = 'ulgurji'
		GROUP BY o.product_id
		HAVING count(*) >= 3`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int16]float64{}
	for rows.Next() {
		var id int16
		var m float64
		if err := rows.Scan(&id, &m); err != nil {
			return nil, err
		}
		out[id] = m
	}
	return out, rows.Err()
}

// seedEvents writes the demo events into the events log (is_demo), one row
// per product or category they touch.
func (s *seeder) seedEvents(ctx context.Context, tx pgx.Tx, end time.Time, products []product) error {
	if _, err := tx.Exec(ctx, `DELETE FROM market_events WHERE is_demo`); err != nil {
		return err
	}
	ids := map[string]int16{}
	for _, p := range products {
		ids[p.Slug] = p.ID
	}
	insert := func(e demoEvent, product *int16, category *string) error {
		start := end.AddDate(0, 0, -e.DaysBefore)
		_, err := tx.Exec(ctx, `
			INSERT INTO market_events (title, kind, starts_on, ends_on, product_id, category, note, is_demo)
			VALUES ($1, $2, $3, $4, $5, $6, 'Demo voqea: demo narx tarixini tushuntirish uchun', true)`,
			e.Title, e.Kind, pgtype.Date{Time: start, Valid: true}, pgtype.Date{Time: start.AddDate(0, 0, e.Days), Valid: true}, product, category)
		return err
	}
	for _, e := range demoEvents {
		switch {
		case len(e.Products) > 0:
			for _, slug := range e.Products {
				if id, ok := ids[slug]; ok {
					if err := insert(e, &id, nil); err != nil {
						return err
					}
				}
			}
		case len(e.Categories) > 0:
			for _, c := range e.Categories {
				if err := insert(e, nil, &c); err != nil {
					return err
				}
			}
		default:
			if err := insert(e, nil, nil); err != nil {
				return err
			}
		}
	}
	return nil
}

// seasonal is the season factor on a day: 1 ± amplitude around the peak month.
func seasonal(sn season, day time.Time) float64 {
	month := float64(day.Month()) + float64(day.Day()-1)/31
	return 1 + sn.amplitude*math.Cos(2*math.Pi*(month-sn.peakMonth)/12)
}

// provinceOffset is a steady gap of a viloyat for a product, within ±10%;
// the capital pays more.
func provinceOffset(slug, soato string) float64 {
	h := fnv.New32a()
	h.Write([]byte(slug + "|" + soato))
	off := float64(h.Sum32()%2001)/10000 - 0.1
	if soato == "1726" { // Toshkent shahri
		off += 0.08
	}
	return off
}

// eventEffect ramps an event's effect up over a few days and lets it fade.
func eventEffect(p product, soato string, day, end time.Time) float64 {
	total := 0.0
	for _, e := range demoEvents {
		if !e.applies(p, soato) {
			continue
		}
		start := end.AddDate(0, 0, -e.DaysBefore)
		d := day.Sub(start).Hours() / 24
		if d < 0 || d > float64(e.Days) {
			continue
		}
		rise := math.Min(1, d/3)
		fade := 1 - math.Max(0, d-float64(e.Days)/2)/(float64(e.Days)/2)
		total += e.Effect * rise * fade
	}
	return total
}

func (e demoEvent) applies(p product, soato string) bool {
	if len(e.Provinces) > 0 && !contains(e.Provinces, soato) {
		return false
	}
	switch {
	case len(e.Products) > 0:
		return contains(e.Products, p.Slug)
	case len(e.Categories) > 0:
		return contains(e.Categories, p.Category)
	}
	return true
}

func contains(xs []string, x string) bool {
	for _, y := range xs {
		if y == x {
			return true
		}
	}
	return false
}

// noise is a mean-reverting random walk (AR(1)).
type noise struct {
	r *rand.Rand
	x float64
}

func newNoise(r *rand.Rand) *noise { return &noise{r: r} }

func (n *noise) next() float64 {
	n.x = noiseKeep*n.x + noiseStep*n.r.NormFloat64()
	return n.x
}

func round(x float64) float64 { return float64(roundPrice(x)) }
