package insight

import (
	"math"
	"testing"
	"time"
)

func synthInput(weeks int, bump float64) MLInput {
	start := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC) // a Monday
	in := MLInput{Start: start, Weeks: weeks, Prices: map[SeriesKey][]float64{}, Offers: map[SeriesKey][]float64{}, Demo: map[SeriesKey][]float64{},
		Demand: map[[2]int16][]float64{}, Frost: map[int16][]float64{}, Heat: map[int16][]float64{}, Rain: map[int16][]float64{}, Tmin: map[int16][]float64{},
		Category: map[int16]string{1: "sabzavot"}, Slug: map[int16]string{1: "pomidor"}, Neighbors: map[int16][]int16{101: {102}, 102: {101, 103}, 103: {102}},
		Holidays: []MLHoliday{{Kind: 5, Category: "sabzavot", Start: start.AddDate(0, 0, 7*60+10), End: start.AddDate(0, 0, 7*60+12)}}}
	series := func(f func(t int) float64) []float64 {
		xs := make([]float64, weeks)
		for t := range xs {
			xs[t] = f(t)
		}
		return xs
	}
	for _, prov := range []int16{0, 101, 102, 103} {
		k := SeriesKey{1, "chakana", prov}
		in.Prices[k] = series(func(t int) float64 {
			v := 5000 * (1 + 0.2*math.Sin(2*math.Pi*float64(t)/52)) * (1 + 0.01*float64(prov%7))
			if t > 80 {
				v *= bump
			}
			return v
		})
		in.Offers[k] = series(func(t int) float64 { return 20 + float64(t%5) })
		in.Demo[k] = series(func(int) float64 { return 1 })
		in.Demand[[2]int16{1, prov}] = series(func(t int) float64 { return float64(t % 7) })
		in.Frost[prov] = series(func(t int) float64 { return float64((t + int(prov)) % 4) })
		in.Heat[prov] = series(func(int) float64 { return 0 })
		in.Rain[prov] = series(func(t int) float64 { return float64(t % 3) })
		in.Tmin[prov] = series(func(t int) float64 { return 10 * math.Sin(2*math.Pi*float64(t)/52) })
	}
	in.USD = series(func(t int) float64 { return 12500 + 10*float64(t) })
	return in
}

// A row never sees the weeks after it: changing everything after week 80
// leaves the features of weeks up to 80 as they were.
func TestFeaturesNoLeak(t *testing.T) {
	a := BuildRows(synthInput(120, 1))
	b := BuildRows(synthInput(120, 1.5))
	later := synthInput(120, 1)
	for k := range later.Prices {
		for w := 81; w < 120; w++ {
			later.Prices[k][w] *= 3
			later.Offers[k][w] = 0
		}
	}
	for p := range later.Frost {
		for w := 81; w < 120; w++ {
			later.Frost[p][w], later.Tmin[p][w] = 7, -20
		}
	}
	for w := 81; w < 120; w++ {
		later.USD[w] = 99999
	}
	c := BuildRows(later)
	if len(a) != len(b) || len(a) != len(c) {
		t.Fatalf("rows %d %d %d", len(a), len(b), len(c))
	}
	for i := range a {
		if a[i].Week > 80 {
			continue
		}
		for f := range a[i].X {
			x, y, z := a[i].X[f], b[i].X[f], c[i].X[f]
			same := (math.IsNaN(x) && math.IsNaN(y) && math.IsNaN(z)) || (x == y && x == z)
			if !same {
				t.Fatalf("week %d %v: %s = %v / %v / %v changed with later data", a[i].Week, a[i].Key, FeatureNames[f], x, y, z)
			}
		}
	}
}

func TestFeatureValues(t *testing.T) {
	in := synthInput(120, 1)
	rows := BuildRows(in)
	var r Row
	for _, x := range rows {
		if x.Key == (SeriesKey{1, "chakana", 102}) && x.Week == 59 {
			r = x
		}
	}
	p := in.Prices[r.Key]
	get := func(n string) float64 { return r.X[featureIndex[n]] }
	if got, want := get("chg1"), math.Log(p[59]/p[58]); math.Abs(got-want) > 1e-12 {
		t.Errorf("chg1 %v, want %v", got, want)
	}
	if got, want := r.Y[1], math.Log(p[61]/p[59]); math.Abs(got-want) > 1e-12 {
		t.Errorf("2-week target %v, want %v", got, want)
	}
	if d := get("holiday_days"); d < 1 || d > 20 {
		t.Errorf("holiday in 60 weeks + 10 days, seen from week 59: %v days", d)
	}
	if get("is_country") != 0 || get("wholesale") != 0 || get("category") != 1 || math.IsNaN(get("neighbors_chg1")) || math.IsNaN(get("vs_country")) {
		t.Errorf("row %v", r.X)
	}
	if math.IsNaN(get("last_year1")) || math.IsNaN(get("tmin_anomaly")) || math.IsNaN(get("level52")) {
		t.Errorf("a year of history is there: %v", r.X)
	}
	if len(FeatureNames) != len(FeatureGroups) {
		t.Errorf("%d names, %d groups", len(FeatureNames), len(FeatureGroups))
	}
}

// A horizon model trains on synthetic weeks, and the drivers of a forecast
// add up to its change.
func TestFitHorizonAndDrivers(t *testing.T) {
	in := synthInput(160, 1)
	rows := BuildRows(in)
	m, fit := fitHorizon(rows, 2)
	if m == nil || fit.Trees < 20 || fit.TrainRows == 0 || fit.ValidRows == 0 {
		t.Fatalf("fit %+v", fit)
	}
	r := rows[len(rows)-10]
	change := m.Predict(r.X)
	d := drivers(m, r.X, change)
	sum := 0.0
	for _, v := range d {
		sum += v
	}
	if math.Abs(sum-(math.Exp(change)-1)) > 0.001 {
		t.Errorf("drivers %v add up to %.4f, want %.4f", d, sum, math.Exp(change)-1)
	}
	for g := range d {
		if g != "history" && g != "season" && g != "holiday" && g != "weather" && g != "fx" && g != "demand_supply" {
			t.Errorf("unknown driver group %q", g)
		}
	}
}
