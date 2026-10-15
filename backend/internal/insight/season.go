package insight

import (
	"fmt"
	"math"
	"time"
)

// MonthPoint is one month's price.
type MonthPoint struct {
	Month  time.Time // first day of the month
	Median float64
}

// Seasonality is how each calendar month usually compares with the year.
type Seasonality struct {
	Months        []float64 `json:"months" doc:"12 values, January first: 1.12 = usually 12% above the yearly level"`
	CheapestMonth int       `json:"cheapest_month" doc:"1–12"`
	DearestMonth  int       `json:"dearest_month" doc:"1–12"`
	Samples       int       `json:"samples" doc:"Months the index is built from"`
}

// minSeasonSamples: at least a year of months that have a full year around them.
const minSeasonSamples = 12

// SeasonalIndex divides each month by the centred 12-month moving average
// around it (classical decomposition: inflation and trend drop out), then
// averages by calendar month and scales the 12 values to a mean of 1. ok is
// false without enough history or with a month never seen.
func SeasonalIndex(ms []MonthPoint) (Seasonality, bool) {
	byMonth := map[time.Time]float64{}
	for _, m := range ms {
		byMonth[m.Month] = m.Median
	}
	var sums [12]float64
	var counts [12]int
	samples := 0
	for _, m := range ms {
		// 2×12 centred moving average: months -6..+6, the two ends at half weight.
		total, weight, full := 0.0, 0.0, true
		for k := -6; k <= 6; k++ {
			v, ok := byMonth[m.Month.AddDate(0, k, 0)]
			if !ok {
				full = false
				break
			}
			w := 1.0
			if k == -6 || k == 6 {
				w = 0.5
			}
			total += v * w
			weight += w
		}
		if !full || total == 0 {
			continue
		}
		i := int(m.Month.Month()) - 1
		sums[i] += m.Median / (total / weight)
		counts[i]++
		samples++
	}
	if samples < minSeasonSamples {
		return Seasonality{}, false
	}
	s := Seasonality{Months: make([]float64, 12), Samples: samples}
	mean := 0.0
	for i := range sums {
		if counts[i] == 0 {
			return Seasonality{}, false
		}
		s.Months[i] = sums[i] / float64(counts[i])
		mean += s.Months[i] / 12
	}
	lo, hi := 0, 0
	for i := range s.Months {
		s.Months[i] = math.Round(s.Months[i]/mean*1000) / 1000
		if s.Months[i] < s.Months[lo] {
			lo = i
		}
		if s.Months[i] > s.Months[hi] {
			hi = i
		}
	}
	s.CheapestMonth, s.DearestMonth = lo+1, hi+1
	return s, true
}

// Advice is a plain answer to "buy now or wait?" and, for sellers, "sell now
// or hold?", with the reasons behind it.
type Advice struct {
	Buyer   string       `json:"buyer" enum:"buy_now,wait,neutral"`
	Seller  string       `json:"seller" enum:"sell_now,hold,neutral"`
	Reasons []string     `json:"reasons" doc:"Plain Uzbek sentences with the numbers behind the verdict"`
	Items   []AdviceItem `json:"items" doc:"The same reasons as codes with their numbers, in the same order, for other languages"`
}

// AdviceItem is one reason behind the advice, for clients that word it themselves.
type AdviceItem struct {
	Code      string  `json:"code" enum:"trend_up,trend_down,season_dearer,season_cheaper,surplus,shortage,below_year,above_year,holiday_soon,holiday_now"`
	Share     float64 `json:"share,omitempty" doc:"Size of the change as a fraction (0.12 = 12%): weekly trend, month-to-month season change, or level against the year"`
	FromMonth int     `json:"from_month,omitempty" doc:"season_*: this month, 1–12"`
	Month     int     `json:"month,omitempty" doc:"season_*: next month, 1–12"`
	Title     string  `json:"title,omitempty" doc:"holiday_*: the holiday's name as stored (Uzbek)"`
	Days      int     `json:"days,omitempty" doc:"holiday_soon: days until it starts"`
	Until     string  `json:"until,omitempty" doc:"holiday_now: last day, YYYY-MM-DD"`
}

// AdviceInput is what the verdict looks at; zero values mean unknown.
type AdviceInput struct {
	Trend       *Trend
	Season      *Seasonality
	Today       time.Time
	Zone        string   // pressure zone: surplus, balanced, shortage
	LevelVsYear *float64 // price now against its 1-year average
	Holidays    []Holiday
}

// Holiday is a holiday that touches the product, on now or coming soon.
type Holiday struct {
	Title      string
	Start, End time.Time
}

func dateUzShort(t time.Time) string { return fmt.Sprintf("%d-%s", t.Day(), monthNames[t.Month()-1]) }

var monthNames = []string{"yanvar", "fevral", "mart", "aprel", "may", "iyun", "iyul", "avgust", "sentabr", "oktabr", "noyabr", "dekabr"}

// AdviceOf weighs rising or falling prices, the coming month's season, the
// demand pressure and how high prices are against the year. Each points to
// buying now (prices go up or are low) or waiting (they go down or are high).
func AdviceOf(in AdviceInput) Advice {
	score := 0.0 // > 0: buy now
	a := Advice{Reasons: []string{}, Items: []AdviceItem{}}
	add := func(text string, item AdviceItem) {
		a.Reasons = append(a.Reasons, text)
		a.Items = append(a.Items, item)
	}
	pct := func(x float64) string { return fmt.Sprintf("%.0f%%", math.Abs(x)*100) }
	if t := in.Trend; t != nil {
		switch t.Direction {
		case "up":
			score++
			add(fmt.Sprintf("Oxirgi 4 haftada narx oshmoqda: haftasiga taxminan %s.", pct(t.Weekly)), AdviceItem{Code: "trend_up", Share: round3(math.Abs(t.Weekly))})
		case "down":
			score--
			add(fmt.Sprintf("Oxirgi 4 haftada narx tushmoqda: haftasiga taxminan %s.", pct(t.Weekly)), AdviceItem{Code: "trend_down", Share: round3(math.Abs(t.Weekly))})
		}
	}
	if s := in.Season; s != nil {
		this := int(in.Today.Month()) - 1
		next := (this + 1) % 12
		change := s.Months[next]/s.Months[this] - 1
		switch {
		case change > 0.03:
			score++
			add(fmt.Sprintf("Odatda %sda narx %sdagidan %s qimmat boʻladi.", monthNames[next], monthNames[this], pct(change)),
				AdviceItem{Code: "season_dearer", Share: round3(math.Abs(change)), FromMonth: this + 1, Month: next + 1})
		case change < -0.03:
			score--
			add(fmt.Sprintf("Odatda %sda narx %sdagidan %s arzon boʻladi.", monthNames[next], monthNames[this], pct(change)),
				AdviceItem{Code: "season_cheaper", Share: round3(math.Abs(change)), FromMonth: this + 1, Month: next + 1})
		}
	}
	switch in.Zone {
	case "surplus":
		score++
		add("Hozir taklif odatdagidan koʻp: xarid uchun qulay payt.", AdviceItem{Code: "surplus"})
	case "shortage":
		add("Hozir talab odatdagidan yuqori: narx yana oshishi mumkin.", AdviceItem{Code: "shortage"})
	}
	if l := in.LevelVsYear; l != nil {
		switch {
		case *l < -0.05:
			score++
			add(fmt.Sprintf("Narx yillik oʻrtachadan %s past.", pct(*l)), AdviceItem{Code: "below_year", Share: round3(math.Abs(*l))})
		case *l > 0.10:
			score--
			add(fmt.Sprintf("Narx yillik oʻrtachadan %s yuqori.", pct(*l)), AdviceItem{Code: "above_year", Share: round3(math.Abs(*l))})
		}
	}
	seen := map[string]bool{}
	for _, h := range in.Holidays {
		if seen[h.Title] {
			continue
		}
		seen[h.Title] = true
		if days := int(h.Start.Sub(in.Today).Hours() / 24); days > 0 {
			score++
			add(fmt.Sprintf("%sga %d kun qoldi: odatda bu mahsulotga talab va narx oshadi.", h.Title, days),
				AdviceItem{Code: "holiday_soon", Title: h.Title, Days: days})
		} else {
			add(fmt.Sprintf("Hozir: %s (%s gacha), talab odatdagidan yuqori.", h.Title, dateUzShort(h.End)),
				AdviceItem{Code: "holiday_now", Title: h.Title, Until: h.End.Format(time.DateOnly)})
		}
	}
	switch {
	case score >= 1:
		a.Buyer, a.Seller = "buy_now", "hold"
	case score <= -1:
		a.Buyer, a.Seller = "wait", "sell_now"
	default:
		a.Buyer, a.Seller = "neutral", "neutral"
	}
	return a
}

func round3(x float64) float64 { return math.Round(x*1000) / 1000 }
