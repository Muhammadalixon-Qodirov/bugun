package insight

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"bugun/internal/geo"
)

// Features of the boosted-tree forecast (PROGNOZ_ML.md §1): one row per
// product, trade type, viloyat (0 = country) and week, built only from what
// was known at the end of that week, so a past row never sees its future.
// The holiday calendar is the one exception by nature: it is known ahead.

// Horizons are the weeks ahead the model forecasts.
const Horizons = 4

// FeatureNames name the columns of a row, in order.
var FeatureNames = []string{
	"chg1", "chg2", "chg4", "chg8", "chg13", "level52", "vol8",
	"last_year1", "last_year2", "last_year3", "last_year4",
	"vs_country", "neighbors_chg1",
	"woy_sin", "woy_cos", "month",
	"holiday_days", "holiday_on", "holiday_kind",
	"frost_week", "frost_2weeks", "heat_week", "rain_week", "tmin_anomaly",
	"usd_chg1", "usd_chg4", "imported",
	"supply_ratio", "demand_ratio", "demand_level",
	"category", "is_country", "wholesale",
}

var featureIndex = func() map[string]int {
	m := make(map[string]int, len(FeatureNames))
	for i, n := range FeatureNames {
		m[n] = i
	}
	return m
}()

// FeatureGroups tell which driver each column belongs to, for explanations.
var FeatureGroups = map[string]string{
	"chg1": "history", "chg2": "history", "chg4": "history", "chg8": "history", "chg13": "history", "level52": "history", "vol8": "history",
	"last_year1": "season", "last_year2": "season", "last_year3": "season", "last_year4": "season",
	"vs_country": "history", "neighbors_chg1": "history",
	"woy_sin": "season", "woy_cos": "season", "month": "season",
	"holiday_days": "holiday", "holiday_on": "holiday", "holiday_kind": "holiday",
	"frost_week": "weather", "frost_2weeks": "weather", "heat_week": "weather", "rain_week": "weather", "tmin_anomaly": "weather",
	"usd_chg1": "fx", "usd_chg4": "fx", "imported": "fx",
	"supply_ratio": "demand_supply", "demand_ratio": "demand_supply", "demand_level": "demand_supply",
	"category": "history", "is_country": "history", "wholesale": "history",
}

// SeriesKey is one price line.
type SeriesKey struct {
	Product  int16
	Trade    string
	Province int16 // 0 = country
}

// MLInput is everything the features read, week by week from Start.
type MLInput struct {
	Start     time.Time // a Monday
	Weeks     int
	Prices    map[SeriesKey][]float64 // per week, NaN when unknown
	Offers    map[SeriesKey][]float64
	Demo      map[SeriesKey][]float64
	Demand    map[[2]int16][]float64 // (product, province; 0 = country)
	Frost     map[int16][]float64    // per viloyat (0 = country mean)
	Heat      map[int16][]float64
	Rain      map[int16][]float64
	Tmin      map[int16][]float64
	USD       []float64
	Holidays  []MLHoliday
	Category  map[int16]string
	Slug      map[int16]string
	Neighbors map[int16][]int16
}

// MLHoliday is a holiday and the category it moves.
type MLHoliday struct {
	Kind       float64 // 1 Ramazon, 2 Ramazon hayiti, 3 Qurbon hayiti, 4 Navroʻz, 5 Yangi yil, 6 toʻy mavsumi, 7 other
	Category   string
	Start, End time.Time
}

// Row is one training or forecasting row.
type Row struct {
	Key     SeriesKey
	Week    int // index from MLInput.Start
	X       []float64
	Y       [Horizons]float64 // log change 1..4 weeks ahead; NaN when unknown
	Demo    float64
	Current float64 // the week's price
}

var nan = math.NaN()

func holidayKind(title string) float64 {
	t := strings.ToLower(title)
	switch {
	case strings.Contains(t, "ramazon hayit"):
		return 2
	case strings.Contains(t, "ramazon"):
		return 1
	case strings.Contains(t, "qurbon"):
		return 3
	case strings.Contains(t, "navro"):
		return 4
	case strings.Contains(t, "yangi yil"):
		return 5
	case strings.Contains(t, "toʻy"), strings.Contains(t, "to'y"), strings.Contains(t, "toy mavsum"):
		return 6
	}
	return 7
}

var categoryCode = map[string]float64{"sabzavot": 1, "meva": 2, "gosht": 3, "don": 4, "oziq_ovqat": 5, "qurilish": 6, "xizmat": 7}

// importedProducts follow the dollar more than the harvest.
var importedProducts = map[string]bool{"banan": true, "apelsin": true, "mandarin": true, "kivi": true, "limon": true, "choy": true,
	"shakar": true, "kungaboqar-yogi": true, "grechka": true}

func logChange(a, b float64) float64 {
	if a > 0 && b > 0 {
		return math.Log(a / b)
	}
	return nan
}

func meanOf(xs []float64) (float64, int) {
	sum, n := 0.0, 0
	for _, x := range xs {
		if !math.IsNaN(x) {
			sum += x
			n++
		}
	}
	if n == 0 {
		return nan, 0
	}
	return sum / float64(n), n
}

func at(xs []float64, i int) float64 {
	if i < 0 || i >= len(xs) {
		return nan
	}
	return xs[i]
}

// BuildRows makes a row for every week of every series with a price.
// Targets are filled where the later weeks are known.
func BuildRows(in MLInput) []Row {
	keys := make([]SeriesKey, 0, len(in.Prices))
	for k := range in.Prices {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		a, b := keys[i], keys[j]
		if a.Product != b.Product {
			return a.Product < b.Product
		}
		if a.Trade != b.Trade {
			return a.Trade < b.Trade
		}
		return a.Province < b.Province
	})
	var rows []Row
	for _, k := range keys {
		p := in.Prices[k]
		for t := 0; t < in.Weeks; t++ {
			if math.IsNaN(p[t]) {
				continue
			}
			r := Row{Key: k, Week: t, X: in.features(k, t), Current: p[t], Demo: at(in.Demo[k], t)}
			for h := 1; h <= Horizons; h++ {
				r.Y[h-1] = logChange(at(p, t+h), p[t])
			}
			rows = append(rows, r)
		}
	}
	return rows
}

// features is the row of series k at week t.
func (in MLInput) features(k SeriesKey, t int) []float64 {
	p := in.Prices[k]
	x := make([]float64, len(FeatureNames))
	for i := range x {
		x[i] = nan
	}
	set := func(name string, v float64) { x[featureIndex[name]] = v }
	cur := p[t]
	for _, lag := range []int{1, 2, 4, 8, 13} {
		set(fmt.Sprintf("chg%d", lag), logChange(cur, at(p, t-lag)))
	}
	if t >= 25 {
		if m, n := meanOf(p[max(0, t-51) : t+1]); n >= 26 {
			set("level52", logChange(cur, m))
		}
	}
	var chg []float64
	for i := t - 7; i <= t; i++ {
		if v := logChange(at(p, i), at(p, i-1)); !math.IsNaN(v) {
			chg = append(chg, v)
		}
	}
	if len(chg) >= 4 {
		m, _ := meanOf(chg)
		ss := 0.0
		for _, v := range chg {
			ss += (v - m) * (v - m)
		}
		set("vol8", math.Sqrt(ss/float64(len(chg))))
	}
	for h := 1; h <= Horizons; h++ {
		set(fmt.Sprintf("last_year%d", h), logChange(at(p, t-52+h), at(p, t-52)))
	}
	if k.Province != 0 {
		if c, ok := in.Prices[SeriesKey{k.Product, k.Trade, 0}]; ok {
			set("vs_country", logChange(cur, at(c, t)))
		}
		var ns []float64
		for _, nb := range in.Neighbors[k.Province] {
			if q, ok := in.Prices[SeriesKey{k.Product, k.Trade, nb}]; ok {
				ns = append(ns, logChange(at(q, t), at(q, t-1)))
			}
		}
		if m, n := meanOf(ns); n > 0 {
			set("neighbors_chg1", m)
		}
	}
	week := in.Start.AddDate(0, 0, 7*t)
	_, woy := week.ISOWeek()
	set("woy_sin", math.Sin(2*math.Pi*float64(woy)/52))
	set("woy_cos", math.Cos(2*math.Pi*float64(woy)/52))
	set("month", float64(week.Month()))

	cat := in.Category[k.Product]
	days, on, kind := 60.0, 0.0, 0.0
	end := week.AddDate(0, 0, 6)
	for _, h := range in.Holidays {
		if h.Category != cat {
			continue
		}
		if !h.Start.After(end) && !h.End.Before(week) {
			on, kind = 1, h.Kind
		}
		if d := h.Start.Sub(end).Hours() / 24; d > 0 && d < days {
			days = d
			if on == 0 {
				kind = h.Kind
			}
		}
	}
	set("holiday_days", days)
	set("holiday_on", on)
	set("holiday_kind", kind)

	prov := k.Province
	set("frost_week", at(in.Frost[prov], t))
	if a, b := at(in.Frost[prov], t), at(in.Frost[prov], t-1); !math.IsNaN(a) && !math.IsNaN(b) {
		set("frost_2weeks", a+b)
	}
	set("heat_week", at(in.Heat[prov], t))
	set("rain_week", at(in.Rain[prov], t))
	if tm := at(in.Tmin[prov], t); !math.IsNaN(tm) {
		// Against the same week of earlier years only.
		var past []float64
		for y := t - 52; y >= 0; y -= 52 {
			past = append(past, at(in.Tmin[prov], y))
		}
		if m, n := meanOf(past); n > 0 {
			set("tmin_anomaly", tm-m)
		}
	}
	set("usd_chg1", logChange(at(in.USD, t), at(in.USD, t-1)))
	set("usd_chg4", logChange(at(in.USD, t), at(in.USD, t-4)))
	if importedProducts[in.Slug[k.Product]] {
		set("imported", 1)
	} else {
		set("imported", 0)
	}
	ratio := func(xs []float64) float64 {
		now := at(xs, t)
		prev, n := meanOf([]float64{at(xs, t-1), at(xs, t-2), at(xs, t-3), at(xs, t-4)})
		if math.IsNaN(now) || n < 2 || prev <= 0 {
			return nan
		}
		return now / prev
	}
	set("supply_ratio", ratio(in.Offers[k]))
	d := in.Demand[[2]int16{k.Product, k.Province}]
	set("demand_ratio", ratio(d))
	if v := at(d, t); !math.IsNaN(v) {
		set("demand_level", math.Log1p(v))
	}
	set("category", categoryCode[cat])
	if k.Province == 0 {
		set("is_country", 1)
	} else {
		set("is_country", 0)
	}
	if k.Trade == "ulgurji" {
		set("wholesale", 1)
	} else {
		set("wholesale", 0)
	}
	return x
}

// monday is the Monday of t's week.
func monday(t time.Time) time.Time {
	t = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
	return t.AddDate(0, 0, -((int(t.Weekday()) + 6) % 7))
}

// LoadMLInput reads the weeks from `from` to the current week.
func (s *Service) LoadMLInput(ctx context.Context, from time.Time) (MLInput, error) {
	start := monday(from)
	now := monday(Day(time.Now()).Time)
	in := MLInput{Start: start, Weeks: int(now.Sub(start).Hours()/(24*7)) + 1,
		Prices: map[SeriesKey][]float64{}, Offers: map[SeriesKey][]float64{}, Demo: map[SeriesKey][]float64{},
		Demand: map[[2]int16][]float64{}, Frost: map[int16][]float64{}, Heat: map[int16][]float64{}, Rain: map[int16][]float64{},
		Tmin: map[int16][]float64{}, Category: map[int16]string{}, Slug: map[int16]string{}, Neighbors: map[int16][]int16{}}
	week := func(d pgtype.Date) int { return int(monday(d.Time).Sub(start).Hours() / (24 * 7)) }
	blank := func() []float64 {
		xs := make([]float64, in.Weeks)
		for i := range xs {
			xs[i] = nan
		}
		return xs
	}
	fromDay := pgtype.Date{Time: start, Valid: true}
	prices, err := s.q.MLWeeklyPrices(ctx, fromDay)
	if err != nil {
		return in, err
	}
	// Country series: offers-weighted mean of the viloyatlar.
	type acc struct{ sum, w, demo float64 }
	country := map[SeriesKey][]acc{}
	for _, r := range prices {
		w := week(r.Week)
		if w < 0 || w >= in.Weeks {
			continue
		}
		k := SeriesKey{r.ProductID, r.TradeType, r.ProvinceID}
		if in.Prices[k] == nil {
			in.Prices[k], in.Offers[k], in.Demo[k] = blank(), blank(), blank()
		}
		in.Prices[k][w], in.Offers[k][w], in.Demo[k][w] = r.Price, r.Offers, r.DemoShare
		ck := SeriesKey{r.ProductID, r.TradeType, 0}
		if country[ck] == nil {
			country[ck] = make([]acc, in.Weeks)
		}
		a := &country[ck][w]
		a.sum += r.Price * r.Offers
		a.w += r.Offers
		a.demo += r.DemoShare * r.Offers
	}
	for k, weeks := range country {
		in.Prices[k], in.Offers[k], in.Demo[k] = blank(), blank(), blank()
		for w, a := range weeks {
			if a.w > 0 {
				in.Prices[k][w], in.Offers[k][w], in.Demo[k][w] = a.sum/a.w, a.w, a.demo/a.w
			}
		}
	}
	demand, err := s.q.MLDemandWeekly(ctx, fromDay)
	if err != nil {
		return in, err
	}
	for _, r := range demand {
		w := week(r.Week)
		if w < 0 || w >= in.Weeks {
			continue
		}
		for _, prov := range []int16{r.ProvinceID, 0} {
			k := [2]int16{r.ProductID, prov}
			if in.Demand[k] == nil {
				in.Demand[k] = make([]float64, in.Weeks) // no signal means zero demand
			}
			in.Demand[k][w] += r.N
			if r.ProvinceID == 0 {
				break
			}
		}
	}
	weather, err := s.q.MLWeatherWeekly(ctx, fromDay)
	if err != nil {
		return in, err
	}
	type wsum struct{ frost, heat, rain, tmin, n float64 }
	countryWx := make([]wsum, in.Weeks)
	for _, r := range weather {
		w := week(r.Week)
		if w < 0 || w >= in.Weeks {
			continue
		}
		for _, m := range []map[int16][]float64{in.Frost, in.Heat, in.Rain, in.Tmin} {
			if m[r.ProvinceID] == nil {
				m[r.ProvinceID] = blank()
			}
		}
		in.Frost[r.ProvinceID][w], in.Heat[r.ProvinceID][w], in.Rain[r.ProvinceID][w], in.Tmin[r.ProvinceID][w] = r.FrostDays, r.HeatDays, r.Precip, r.Tmin
		c := &countryWx[w]
		c.frost, c.heat, c.rain, c.tmin, c.n = c.frost+r.FrostDays, c.heat+r.HeatDays, c.rain+r.Precip, c.tmin+r.Tmin, c.n+1
	}
	in.Frost[0], in.Heat[0], in.Rain[0], in.Tmin[0] = blank(), blank(), blank(), blank()
	for w, c := range countryWx {
		if c.n > 0 {
			in.Frost[0][w], in.Heat[0][w], in.Rain[0][w], in.Tmin[0][w] = c.frost/c.n, c.heat/c.n, c.rain/c.n, c.tmin/c.n
		}
	}
	fx, err := s.q.MLFxWeekly(ctx, fromDay)
	if err != nil {
		return in, err
	}
	in.USD = blank()
	for _, r := range fx {
		if w := week(r.Week); w >= 0 && w < in.Weeks {
			in.USD[w] = r.Rate
		}
	}
	hs, err := s.q.MLHolidays(ctx)
	if err != nil {
		return in, err
	}
	for _, h := range hs {
		in.Holidays = append(in.Holidays, MLHoliday{Kind: holidayKind(h.Title), Category: h.Category.String, Start: h.StartsOn.Time, End: h.EndsOn.Time})
	}
	products, err := s.q.ListProducts(ctx)
	if err != nil {
		return in, err
	}
	for _, p := range products {
		in.Category[p.ID], in.Slug[p.ID] = p.Category, p.Slug
	}
	provinces, err := s.q.ListProvinces(ctx)
	if err != nil {
		return in, err
	}
	nb, err := geo.Neighbors()
	if err != nil {
		return in, err
	}
	bySoato := map[int64]int16{}
	for _, p := range provinces {
		bySoato[p.Soato.Int64] = p.ID
	}
	for _, p := range provinces {
		for _, s := range nb[p.Soato.Int64] {
			if id, ok := bySoato[s]; ok {
				in.Neighbors[p.ID] = append(in.Neighbors[p.ID], id)
			}
		}
	}
	return in, nil
}
