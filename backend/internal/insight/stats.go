package insight

import (
	"math"
	"time"
)

// Point is one value of a daily series.
type Point struct {
	Day    time.Time
	Median float64
}

// windowMean is the mean median over the days (end-days, end]. ok is false
// when the window has no data.
func windowMean(ps []Point, end time.Time, days int) (float64, bool) {
	start := end.AddDate(0, 0, -days)
	sum, n := 0.0, 0
	for _, p := range ps {
		if p.Day.After(start) && !p.Day.After(end) {
			sum += p.Median
			n++
		}
	}
	if n == 0 {
		return 0, false
	}
	return sum / float64(n), true
}

// Changes compares the last days with earlier ones: d1 is the last day against
// the day before, the others compare the last week's mean with the week the
// same distance back. A missing comparison is nil.
type Changes struct {
	D1  *float64 `json:"d1" doc:"Last day against the day before"`
	D7  *float64 `json:"d7" doc:"Last 7 days against the 7 before"`
	D30 *float64 `json:"d30" doc:"Last 7 days against a month ago"`
	Y1  *float64 `json:"y1" doc:"Last 7 days against a year ago"`
}

func ChangesOf(ps []Point, end time.Time) Changes {
	var c Changes
	rel := func(now, then float64) *float64 {
		if then <= 0 {
			return nil
		}
		v := math.Round((now/then-1)*1000) / 1000
		return &v
	}
	if a, ok := windowMean(ps, end, 1); ok {
		if b, ok := windowMean(ps, end.AddDate(0, 0, -1), 1); ok {
			c.D1 = rel(a, b)
		}
	}
	now, ok := windowMean(ps, end, 7)
	if !ok {
		return c
	}
	for _, x := range []struct {
		back int
		dst  **float64
	}{{7, &c.D7}, {30, &c.D30}, {365, &c.Y1}} {
		if then, ok := windowMean(ps, end.AddDate(0, 0, -x.back), 7); ok {
			*x.dst = rel(now, then)
		}
	}
	return c
}

// Trend is the direction of the last 28 days.
type Trend struct {
	Direction string  `json:"direction" enum:"up,down,flat"`
	Weekly    float64 `json:"weekly" doc:"Change per week from a straight-line fit, e.g. 0.021 = +2.1%"`
}

// flatBand: slower than 1% a week is "flat".
const flatBand = 0.01

// TrendOf fits a straight line to the last 28 days (at least 7 points) and
// expresses its slope per week relative to the mean. ok is false without
// enough data.
func TrendOf(ps []Point, end time.Time) (Trend, bool) {
	start := end.AddDate(0, 0, -28)
	var xs, ys []float64
	for _, p := range ps {
		if p.Day.After(start) && !p.Day.After(end) {
			xs = append(xs, p.Day.Sub(start).Hours()/24)
			ys = append(ys, p.Median)
		}
	}
	if len(xs) < 7 {
		return Trend{}, false
	}
	var mx, my float64
	for i := range xs {
		mx += xs[i]
		my += ys[i]
	}
	mx /= float64(len(xs))
	my /= float64(len(ys))
	var sxy, sxx float64
	for i := range xs {
		sxy += (xs[i] - mx) * (ys[i] - my)
		sxx += (xs[i] - mx) * (xs[i] - mx)
	}
	if sxx == 0 || my == 0 {
		return Trend{}, false
	}
	weekly := sxy / sxx * 7 / my
	t := Trend{Direction: "flat", Weekly: math.Round(weekly*1000) / 1000}
	switch {
	case weekly > flatBand:
		t.Direction = "up"
	case weekly < -flatBand:
		t.Direction = "down"
	}
	return t, true
}

// Weekdays is how each day of the week usually compares with its week.
type Weekdays struct {
	Days         []float64 `json:"days" doc:"7 values, Monday first: 0.98 = usually 2% under the week's mean"`
	CheapestDay  int       `json:"cheapest_day" doc:"1 = Monday … 7 = Sunday"`
	DearestDay   int       `json:"dearest_day"`
	Spread       float64   `json:"spread" doc:"Dearest over cheapest day minus 1; under 0.01 there is no real pattern"`
	WeeksCounted int       `json:"weeks"`
}

// minWeekdayWeeks: each weekday needs this many weeks to be compared.
const minWeekdayWeeks = 8

// WeekdaysOf compares each day of the last 12 weeks with the mean of its
// week (Monday to Sunday) and averages by weekday.
func WeekdaysOf(ps []Point, end time.Time) (Weekdays, bool) {
	start := end.AddDate(0, 0, -84)
	byWeek := map[time.Time][]Point{}
	for _, p := range ps {
		if p.Day.After(start) && !p.Day.After(end) {
			w := p.Day.AddDate(0, 0, -((int(p.Day.Weekday()) + 6) % 7))
			byWeek[w] = append(byWeek[w], p)
		}
	}
	var sums [7]float64
	var counts [7]int
	for _, days := range byWeek {
		if len(days) < 7 {
			continue // a whole week only
		}
		mean := 0.0
		for _, d := range days {
			mean += d.Median / float64(len(days))
		}
		if mean == 0 {
			continue
		}
		for _, d := range days {
			i := (int(d.Day.Weekday()) + 6) % 7
			sums[i] += d.Median / mean
			counts[i]++
		}
	}
	w := Weekdays{Days: make([]float64, 7), WeeksCounted: counts[0]}
	lo, hi := 0, 0
	for i := range sums {
		if counts[i] < minWeekdayWeeks {
			return Weekdays{}, false
		}
		w.Days[i] = math.Round(sums[i]/float64(counts[i])*1000) / 1000
		if w.Days[i] < w.Days[lo] {
			lo = i
		}
		if w.Days[i] > w.Days[hi] {
			hi = i
		}
	}
	w.CheapestDay, w.DearestDay = lo+1, hi+1
	w.Spread = math.Round((w.Days[hi]/w.Days[lo]-1)*1000) / 1000
	return w, true
}

// Correlation is Pearson's r of two equally long series; ok is false with
// fewer than minCorrelationPoints or no variation.
func Correlation(xs, ys []float64) (float64, bool) {
	if len(xs) != len(ys) || len(xs) < minCorrelationPoints {
		return 0, false
	}
	var mx, my float64
	for i := range xs {
		mx += xs[i]
		my += ys[i]
	}
	mx /= float64(len(xs))
	my /= float64(len(ys))
	var sxy, sxx, syy float64
	for i := range xs {
		sxy += (xs[i] - mx) * (ys[i] - my)
		sxx += (xs[i] - mx) * (xs[i] - mx)
		syy += (ys[i] - my) * (ys[i] - my)
	}
	if sxx == 0 || syy == 0 {
		return 0, false
	}
	return math.Round(sxy/math.Sqrt(sxx*syy)*100) / 100, true
}

const minCorrelationPoints = 20
