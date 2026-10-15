package insight

import (
	"context"
	"math"
	"sort"
	"time"

	"bugun/internal/ml"
	"bugun/internal/store/db"
)

// Daily price anomalies (ML_REJA.md 4, HUDUD's ml/anomaly.py layers 1–2):
// a viloyat-day whose median is far from the same series' last four weeks
// (temporal), or that jumped against its own last week while the other
// viloyatlar did not (cross_section). Both use the median and MAD on log
// prices, which an outlier cannot drag towards itself. Flagged days leave
// the charts and every statistic through the price_daily_clean view until an
// admin dismisses them. A move that lasts is a new level, not an error: a
// day with three of the four days after it, or before it, at about the same
// price is not flagged. Flags on the last four days are provisional, as the
// days after them are not in yet: marked, but kept in the charts, so real
// news is never hidden. A jump near a holiday or a recorded market event of
// the product is explained, so it is not flagged either.

const (
	// AnomalyHourDays and AnomalyNightDays: the hourly run checks the last
	// week, the nightly one the last year.
	AnomalyHourDays  = 7
	AnomalyNightDays = 365
	temporalRefDays  = 28 // the series' own past a day is compared with
	minTemporalRef   = 10 // days of that past needed
	crossRefDays     = 7  // a day's move is measured against its last week
	minCrossRef      = 4
	lastingDays      = 3 // this many of the next 4 days near the price make it a new level
	lastingBand      = 0.1
	eventMargin      = 3  // days around an event that it explains
	maxExplainDays   = 14 // a longer event (a season) does not explain a lone day
)

// AnomalyRules are the thresholds of the check (config: ANOMALY_*).
type AnomalyRules struct {
	Z            float64 // robust z beyond which a day is suspicious
	MinMove      float64 // temporal: and at least this far from the usual price
	MinCrossMove float64 // cross_section: the move must differ from the others' by this much
	MinProvinces int     // cross_section: viloyatlar with a price that day needed to compare
}

// DefaultAnomalyRules: z beyond 3.5, a 25% move against the series' past, a
// 30% move against the other viloyatlar, at least 5 of them.
var DefaultAnomalyRules = AnomalyRules{Z: 3.5, MinMove: 0.25, MinCrossMove: 0.3, MinProvinces: 5}

// Anomaly is one suspicious viloyat-day.
type Anomaly struct {
	ProductID  int16
	Trade      string
	ProvinceID int16
	Day        time.Time
	Layer      string  // temporal, cross_section
	Value      float64 // the day's median
	Reference  float64 // what was expected
	Z          float64
	Settled    bool // four later days were seen: the flag is final
}

type seriesKey struct {
	product  int16
	trade    string
	province int16
}

// lasting reports whether the price at day i is a level rather than a lone
// point: at least 3 of the 4 days after it, or of the 4 days before it, are
// within 10% of it. An error stands alone; a real move lasts.
func lasting(days []time.Time, vals []float64, i int) bool {
	after, before := 0, 0
	for j := i + 1; j < len(days) && days[j].Sub(days[i]) <= 4*24*time.Hour; j++ {
		if math.Abs(vals[j]/vals[i]-1) <= lastingBand {
			after++
		}
	}
	for j := i - 1; j >= 0 && days[i].Sub(days[j]) <= 4*24*time.Hour; j-- {
		if math.Abs(vals[j]/vals[i]-1) <= lastingBand {
			before++
		}
	}
	return after >= lastingDays || before >= lastingDays
}

// DetectAnomalies checks the days from `from` on; rows must hold at least
// four weeks before it and be sorted by product, trade, province and day.
// The last day in rows counts as today. explained, when set, tells the
// viloyat-days an event accounts for; they are never flagged.
func DetectAnomalies(rows []db.DailyForAnomaliesRow, from time.Time, explained func(product, province int16, day time.Time) bool) []Anomaly {
	return DefaultAnomalyRules.Detect(rows, from, explained)
}

// Detect is DetectAnomalies under these rules.
func (ar AnomalyRules) Detect(rows []db.DailyForAnomaliesRow, from time.Time, explained func(product, province int16, day time.Time) bool) []Anomaly {
	if explained == nil {
		explained = func(int16, int16, time.Time) bool { return false }
	}
	type series struct {
		days []time.Time
		vals []float64
	}
	all := map[seriesKey]*series{}
	var keys []seriesKey
	for _, r := range rows {
		if r.Median <= 0 {
			continue
		}
		k := seriesKey{r.ProductID, r.TradeType, r.ProvinceID}
		s := all[k]
		if s == nil {
			s = &series{}
			all[k] = s
			keys = append(keys, k)
		}
		s.days = append(s.days, r.Day.Time)
		s.vals = append(s.vals, r.Median)
	}

	var last time.Time
	for _, r := range rows {
		if r.Day.Time.After(last) {
			last = r.Day.Time
		}
	}
	settled := func(d time.Time) bool { return !d.AddDate(0, 0, 4).After(last) }

	var out []Anomaly
	// Layer 1: each series against its own recent past.
	type move struct {
		key   seriesKey
		i     int
		c     float64 // log change against the last week
		value float64
		ref   float64
	}
	type dayKey struct {
		product int16
		trade   string
		day     time.Time
	}
	moves := map[dayKey][]move{}
	for _, k := range keys {
		s := all[k]
		for i, d := range s.days {
			var ref, week []float64
			for j := i - 1; j >= 0 && d.Sub(s.days[j]) <= temporalRefDays*24*time.Hour; j-- {
				ref = append(ref, math.Log(s.vals[j]))
				if d.Sub(s.days[j]) <= crossRefDays*24*time.Hour {
					week = append(week, math.Log(s.vals[j]))
				}
			}
			if d.Before(from) {
				continue
			}
			x := math.Log(s.vals[i])
			if len(week) >= minCrossRef {
				dk := dayKey{k.product, k.trade, d}
				moves[dk] = append(moves[dk], move{k, i, x - ml.Median(week), s.vals[i], math.Exp(ml.Median(week))})
			}
			if len(ref) < minTemporalRef {
				continue
			}
			z, ok := ml.RobustZ(x, ref)
			usual := math.Exp(ml.Median(ref))
			if !ok || math.Abs(z) <= ar.Z || math.Abs(s.vals[i]/usual-1) < ar.MinMove || lasting(s.days, s.vals, i) || explained(k.product, k.province, d) {
				continue
			}
			out = append(out, Anomaly{ProductID: k.product, Trade: k.trade, ProvinceID: k.province, Day: d,
				Layer: "temporal", Value: s.vals[i], Reference: usual, Z: round3(z), Settled: settled(d)})
		}
	}
	// Layer 2: a viloyat that jumped when the others did not.
	groups := make([]dayKey, 0, len(moves))
	for g := range moves {
		groups = append(groups, g)
	}
	sort.Slice(groups, func(i, j int) bool {
		a, b := groups[i], groups[j]
		if a.product != b.product {
			return a.product < b.product
		}
		if a.trade != b.trade {
			return a.trade < b.trade
		}
		return a.day.Before(b.day)
	})
	for _, g := range groups {
		ms := moves[g]
		if len(ms) < ar.MinProvinces {
			continue
		}
		for n, m := range ms {
			others := make([]float64, 0, len(ms)-1)
			for o, x := range ms {
				if o != n {
					others = append(others, x.c)
				}
			}
			z, ok := ml.RobustZ(m.c, others)
			if !ok || math.Abs(z) <= ar.Z || math.Abs(m.c-ml.Median(others)) < math.Log(1+ar.MinCrossMove) {
				continue
			}
			s := all[m.key]
			if lasting(s.days, s.vals, m.i) || explained(m.key.product, m.key.province, s.days[m.i]) {
				continue
			}
			out = append(out, Anomaly{ProductID: m.key.product, Trade: m.key.trade, ProvinceID: m.key.province, Day: s.days[m.i],
				Layer: "cross_section", Value: m.value, Reference: m.ref * math.Exp(ml.Median(others)), Z: round3(z), Settled: settled(s.days[m.i])})
		}
	}
	return out
}

// RefreshAnomalies re-checks the last `days` days, one product at a time:
// open flags that no longer hold go, new ones come; flags an admin decided
// on stay as they are.
func (s *Service) RefreshAnomalies(ctx context.Context, days int) (int, error) {
	today := Day(time.Now()).Time
	from := today.AddDate(0, 0, -days)
	since := date(from.AddDate(0, 0, -temporalRefDays))
	products, err := s.q.ProductsWithDaily(ctx, since)
	if err != nil {
		return 0, err
	}
	events, err := s.q.EventsSince(ctx, since)
	if err != nil {
		return 0, err
	}
	var found []Anomaly
	for _, p := range products {
		rows, err := s.q.DailyForAnomalies(ctx, db.DailyForAnomaliesParams{ProductID: p.ID, FromDay: since})
		if err != nil {
			return 0, err
		}
		found = append(found, s.anomalyRules.Detect(rows, from, explainedBy(events, p.Category))...)
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)
	q := s.q.WithTx(tx)
	if err := q.DeleteOpenAnomalies(ctx, date(from)); err != nil {
		return 0, err
	}
	for _, a := range found {
		if err := q.InsertAnomaly(ctx, db.InsertAnomalyParams{
			ProductID: a.ProductID, TradeType: a.Trade, ProvinceID: a.ProvinceID, Day: date(a.Day),
			Layer: a.Layer, Value: a.Value, Reference: a.Reference, Z: a.Z, Settled: a.Settled,
		}); err != nil {
			return 0, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	s.Invalidate() // charts change with the flags
	return len(found), nil
}

// explainedBy says whether a short event of the product (or its category,
// or every product) in its viloyat (or the whole country) was on within
// three days of the day. Seasons such as the wedding season last for months
// and would excuse every error in them, so they do not count.
func explainedBy(events []db.EventsSinceRow, category string) func(product, province int16, day time.Time) bool {
	return func(product, province int16, day time.Time) bool {
		for _, e := range events {
			if e.EndsOn.Time.Sub(e.StartsOn.Time) > maxExplainDays*24*time.Hour {
				continue
			}
			if (e.ProductID.Valid && e.ProductID.Int16 != product) || (e.Category.Valid && e.Category.String != category) ||
				(e.ProvinceID.Valid && e.ProvinceID.Int16 != province) {
				continue
			}
			if !day.Before(e.StartsOn.Time.AddDate(0, 0, -eventMargin)) && !day.After(e.EndsOn.Time.AddDate(0, 0, eventMargin)) {
				return true
			}
		}
		return false
	}
}

// Invalidate drops every cached page, e.g. after an admin decided on a flag.
func (s *Service) Invalidate() {
	s.mu.Lock()
	s.cache, s.pages = nil, nil
	s.mu.Unlock()
}
