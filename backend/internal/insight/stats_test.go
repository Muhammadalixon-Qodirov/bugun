package insight

import (
	"testing"
	"time"
)

func series(days int, f func(i int) float64) ([]Point, time.Time) {
	end := time.Date(2026, 9, 25, 0, 0, 0, 0, time.UTC)
	var ps []Point
	for i := days - 1; i >= 0; i-- {
		ps = append(ps, Point{Day: end.AddDate(0, 0, -i), Median: f(days - 1 - i)})
	}
	return ps, end
}

func TestChangesOf(t *testing.T) {
	// Flat 5 000 for a year, then 6 000 for the last 7 days.
	ps, end := series(400, func(i int) float64 {
		if i >= 393 {
			return 6000
		}
		return 5000
	})
	c := ChangesOf(ps, end)
	if c.D1 == nil || *c.D1 != 0 || c.D7 == nil || *c.D7 != 0.2 || c.D30 == nil || *c.D30 != 0.2 || c.Y1 == nil || *c.Y1 != 0.2 {
		t.Errorf("changes %+v", c)
	}
	short, end := series(10, func(int) float64 { return 5000 })
	if c := ChangesOf(short, end); c.D30 != nil || c.Y1 != nil || c.D7 == nil {
		t.Errorf("short history: %+v, want d7 only", c)
	}
}

func TestTrendOf(t *testing.T) {
	up, end := series(28, func(i int) float64 { return 5000 + 50*float64(i) })         // +350 a week
	if tr, ok := TrendOf(up, end); !ok || tr.Direction != "up" || tr.Weekly != 0.062 { // 350 a week over a mean of 5 675
		t.Errorf("rising: %+v %v", tr, ok)
	}
	flat, end := series(28, func(i int) float64 { return 5000 + float64(i%2) })
	if tr, _ := TrendOf(flat, end); tr.Direction != "flat" {
		t.Errorf("flat: %+v", tr)
	}
	few, end := series(5, func(int) float64 { return 5000 })
	if _, ok := TrendOf(few, end); ok {
		t.Error("trend from 5 points")
	}
}

func TestWeekdaysOf(t *testing.T) {
	// 12 full weeks where Saturday is 10% dearer than the other days.
	end := time.Date(2026, 9, 27, 0, 0, 0, 0, time.UTC) // a Sunday
	var ps []Point
	for i := 0; i < 84; i++ {
		d := end.AddDate(0, 0, -i)
		v := 1000.0
		if d.Weekday() == time.Saturday {
			v = 1100
		}
		ps = append(ps, Point{Day: d, Median: v})
	}
	w, ok := WeekdaysOf(ps, end)
	if !ok || w.DearestDay != 6 || w.Spread != 0.1 || w.WeeksCounted < 8 {
		t.Errorf("weekdays %+v %v", w, ok)
	}
	if _, ok := WeekdaysOf(ps[:30], end); ok {
		t.Error("a pattern from 4 weeks")
	}
}
