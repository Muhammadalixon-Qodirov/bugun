package insight

import (
	"math"
	"strings"
	"testing"
	"time"
)

// Three years of a price that is dearest in February and cheapest in August,
// on top of 10% inflation: the index finds the season, not the inflation.
func TestSeasonalIndex(t *testing.T) {
	var ms []MonthPoint
	start := time.Date(2023, 9, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 37; i++ {
		m := start.AddDate(0, i, 0)
		season := 1 + 0.3*math.Cos(2*math.Pi*(float64(m.Month())-2)/12)
		ms = append(ms, MonthPoint{Month: m, Median: 5000 * math.Pow(1.1, float64(i)/12) * season})
	}
	s, ok := SeasonalIndex(ms)
	if !ok || s.DearestMonth != 2 || s.CheapestMonth != 8 || s.Months[1] < 1.2 || s.Months[7] > 0.8 {
		t.Fatalf("seasonality %+v %v", s, ok)
	}
	if _, ok := SeasonalIndex(ms[:14]); ok {
		t.Error("an index from 14 months")
	}
}

func TestAdviceOf(t *testing.T) {
	season := &Seasonality{Months: []float64{1.2, 1.3, 1.2, 1.1, 1, 0.9, 0.8, 0.7, 0.8, 0.9, 1, 1.1}}
	up := &Trend{Direction: "up", Weekly: 0.03}
	low := -0.08
	// September, prices rising, October usually dearer, prices below the year: buy now.
	a := AdviceOf(AdviceInput{Trend: up, Season: season, Today: time.Date(2026, 9, 25, 0, 0, 0, 0, time.UTC), Zone: "surplus", LevelVsYear: &low})
	if a.Buyer != "buy_now" || a.Seller != "hold" || len(a.Reasons) != 4 || !strings.Contains(a.Reasons[1], "oktabrda") {
		t.Errorf("rising into autumn: %+v", a)
	}
	// The same reasons as codes, in order, with their numbers.
	want := []AdviceItem{
		{Code: "trend_up", Share: 0.03},
		{Code: "season_dearer", Share: 0.125, FromMonth: 9, Month: 10},
		{Code: "surplus"},
		{Code: "below_year", Share: 0.08},
	}
	if len(a.Items) != len(want) {
		t.Fatalf("items %+v", a.Items)
	}
	for i, w := range want {
		if a.Items[i] != w {
			t.Errorf("item %d = %+v, want %+v", i, a.Items[i], w)
		}
	}
	// March, prices falling, April usually cheaper, prices high: wait (sellers sell now).
	high := 0.15
	a = AdviceOf(AdviceInput{Trend: &Trend{Direction: "down", Weekly: -0.02}, Season: season, Today: time.Date(2026, 3, 10, 0, 0, 0, 0, time.UTC), LevelVsYear: &high})
	if a.Buyer != "wait" || a.Seller != "sell_now" {
		t.Errorf("falling into spring: %+v", a)
	}
	if a := AdviceOf(AdviceInput{Today: time.Now()}); a.Buyer != "neutral" || len(a.Reasons) != 0 || a.Items == nil || len(a.Items) != 0 {
		t.Errorf("nothing known: %+v", a)
	}
}

func TestAdviceHolidays(t *testing.T) {
	today := time.Date(2026, 9, 26, 0, 0, 0, 0, time.UTC)
	a := AdviceOf(AdviceInput{Today: today, Holidays: []Holiday{
		{Title: "Toʻy mavsumi", Start: time.Date(2026, 9, 15, 0, 0, 0, 0, time.UTC), End: time.Date(2026, 11, 30, 0, 0, 0, 0, time.UTC)},
		{Title: "Qurbon hayiti", Start: today.AddDate(0, 0, 12), End: today.AddDate(0, 0, 19)},
		{Title: "Qurbon hayiti", Start: today.AddDate(0, 0, 12), End: today.AddDate(0, 0, 19)}, // another category's row
	}})
	if a.Buyer != "buy_now" || len(a.Reasons) != 2 ||
		!strings.Contains(a.Reasons[0], "Hozir: Toʻy mavsumi (30-noyabr gacha)") || !strings.Contains(a.Reasons[1], "Qurbon hayitiga 12 kun qoldi") {
		t.Errorf("advice %+v", a)
	}
	if len(a.Items) != 2 || a.Items[0] != (AdviceItem{Code: "holiday_now", Title: "Toʻy mavsumi", Until: "2026-11-30"}) ||
		a.Items[1] != (AdviceItem{Code: "holiday_soon", Title: "Qurbon hayiti", Days: 12}) {
		t.Errorf("holiday items %+v", a.Items)
	}
}
