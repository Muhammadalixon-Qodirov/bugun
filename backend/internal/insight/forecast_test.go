package insight

import (
	"math"
	"testing"
	"time"
)

// A price that follows a known season is forecast within a few percent, and
// the interval contains the truth.
func TestForecastOf(t *testing.T) {
	end := time.Date(2026, 9, 25, 0, 0, 0, 0, time.UTC)
	season := func(d time.Time) float64 {
		return 1 + 0.3*math.Cos(2*math.Pi*(float64(d.Month())+float64(d.Day())/31-2.5)/12)
	}
	var ps []Point
	var ms []MonthPoint
	monthly := map[time.Time][]float64{}
	for i := 3 * 365; i >= 0; i-- {
		d := end.AddDate(0, 0, -i)
		v := 5000 * season(d)
		ps = append(ps, Point{Day: d, Median: v})
		m := time.Date(d.Year(), d.Month(), 1, 0, 0, 0, 0, time.UTC)
		monthly[m] = append(monthly[m], v)
	}
	for m, vs := range monthly {
		sum := 0.0
		for _, v := range vs {
			sum += v
		}
		ms = append(ms, MonthPoint{Month: m, Median: sum / float64(len(vs))})
	}
	s, ok := SeasonalIndex(ms)
	if !ok {
		t.Fatal("no season")
	}
	f, ok := ForecastOf(ps, &s, end)
	if !ok || len(f.Weeks) != 4 || f.Tested < 40 || f.MAPE[0] > 0.05 {
		t.Fatalf("forecast %+v %v", f, ok)
	}
	// The rules set the horizon and how many past weeks are needed.
	if f2, ok := (ForecastRules{Weeks: 2, TrendDamping: 0.8, MaxTrendEffect: 0.15, MinBacktests: 20, IntervalQuantile: 0.8}).Forecast(ps, &s, end); !ok || len(f2.Weeks) != 2 || len(f2.MAPE) != 2 {
		t.Errorf("2-week forecast %+v %v", f2, ok)
	}
	strict := DefaultForecastRules
	strict.MinBacktests = f.Tested + 1
	if _, ok := strict.Forecast(ps, &s, end); ok {
		t.Error("forecast with fewer past weeks than MinBacktests")
	}
	truth := 5000 * season(end.AddDate(0, 0, 25))
	w := f.Weeks[3]
	if float64(w.Low) > truth*1.01 || float64(w.High) < truth*0.99 {
		t.Errorf("week 4: %+v, truth %.0f", w, truth)
	}
	if _, ok := ForecastOf(ps[len(ps)-100:], &s, end); ok {
		t.Error("forecast judged on 100 days of history")
	}
	if f, ok := ForecastOf(ps, nil, end); !ok || f.Method == MethodSeasonTrend {
		t.Errorf("without a season the other methods still forecast: %+v %v", f.Method, ok)
	}
	// A clean seasonal price: carrying last week forward is clearly worse.
	if f.Method == MethodNaive || f.Skill < 0.3 || len(f.Methods) != 3 || f.Methods[0].Method != f.Method {
		t.Errorf("method %s skill %.2f methods %+v", f.Method, f.Skill, f.Methods)
	}
	// Drivers add up to the change from last week's price.
	for _, w := range f.Weeks {
		sum := 0.0
		for _, d := range w.Drivers {
			sum += d.Share
		}
		if got := float64(f.Base) * (1 + sum); math.Abs(got/float64(w.Median)-1) > 0.02 {
			t.Errorf("week %s: base %d × (1+%.3f) = %.0f, median %d", w.Week, f.Base, sum, got, w.Median)
		}
	}
}

// On a random walk nothing beats carrying last week forward by much, and
// the result says so honestly.
func TestForecastRandomWalk(t *testing.T) {
	end := time.Date(2026, 9, 25, 0, 0, 0, 0, time.UTC)
	var ps []Point
	v := 5000.0
	x := uint64(7)
	for i := 800; i >= 0; i-- {
		x = x*6364136223846793005 + 1442695040888963407
		v *= 1 + (float64(x>>40)/float64(1<<24)-0.5)*0.02
		ps = append(ps, Point{Day: end.AddDate(0, 0, -i), Median: v})
	}
	f, ok := ForecastOf(ps, nil, end)
	if !ok || f.Skill > 0.2 {
		t.Fatalf("random walk: %+v %v", f.Methods, ok)
	}
}
