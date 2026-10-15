package insight

import (
	"math"
	"sort"
	"time"

	"bugun/internal/ml"
)

// Forecast (TAHLIL_REJA.md 3.4, ML_REJA.md 1–2): several methods forecast the
// next 4 weeks and each is checked on the same weeks of the past year; the
// most accurate one is shown, with how much better it did than simply
// carrying last week forward, an interval from its own past errors and the
// share of the change each driver brings (exact Shapley values).

// ForecastRules are the forecast's parameters (config: FORECAST_*).
type ForecastRules struct {
	Weeks            int     // weeks ahead
	TrendDamping     float64 // each week ahead counts the trend this much less
	MaxTrendEffect   float64 // the trend never moves a forecast more than this
	MinBacktests     int     // weekly origins needed for an honest interval
	IntervalQuantile float64 // share of past errors inside low–high
}

// DefaultForecastRules: 4 weeks ahead, trend damped by 0.8 a week and capped
// at 15%, at least 20 past weeks tested, an 80% interval.
var DefaultForecastRules = ForecastRules{Weeks: 4, TrendDamping: 0.8, MaxTrendEffect: 0.15, MinBacktests: 20, IntervalQuantile: 0.8}

// Forecast methods.
const (
	MethodNaive       = "naive"        // last week's price stays
	MethodLastYear    = "last_year"    // last week's price moves as it did a year ago
	MethodSeasonTrend = "season_trend" // seasonal index and a damped trend
	MethodML          = "ml"           // boosted trees over holidays, weather, the dollar and demand (PROGNOZ_ML.md)
)

type ForecastWeek struct {
	Week    string   `json:"week" doc:"First day of the forecast week"`
	Median  int64    `json:"median"`
	Low     int64    `json:"low" doc:"80% of past errors were inside low–high"`
	High    int64    `json:"high"`
	Drivers []Driver `json:"drivers" doc:"What moves the forecast away from last week's price; the shares add up to median/base − 1"`
}

// Driver is one reason behind a forecast week, as a share of last week's price.
type Driver struct {
	Code  string  `json:"code" enum:"season,trend,last_year,history,holiday,weather,fx,demand_supply"`
	Share float64 `json:"share" doc:"0.05 = this driver adds 5% of last week's price"`
}

// MethodScore is how one method did on the past year.
type MethodScore struct {
	Method string  `json:"method" enum:"naive,last_year,season_trend,ml"`
	MAPE   float64 `json:"mape" doc:"Mean absolute error over the 4 weeks ahead, 0.08 = 8%"`
}

type Forecast struct {
	Method  string         `json:"method" enum:"naive,last_year,season_trend,ml" doc:"The method with the smallest past error, used for weeks; ml weeks start on Mondays"`
	Base    int64          `json:"base" doc:"Last week's mean price, where every forecast starts"`
	Skill   float64        `json:"skill" doc:"How much smaller the chosen method's past error is than naive's: 0.3 = 30% more accurate; 0 when naive is best"`
	Methods []MethodScore  `json:"methods" doc:"Every method checked on the same past weeks, best first"`
	Weeks   []ForecastWeek `json:"weeks"`
	MAPE    []float64      `json:"mape" doc:"The chosen method's mean absolute error per week ahead over the past year, 0.08 = 8%"`
	Tested  int            `json:"tested" doc:"How many past weeks the methods were checked on"`
	Note    string         `json:"note"`
	// Set by the analytics page from the history the forecast was built on.
	DemoShare float64 `json:"demo_share" doc:"Share of the history behind the forecast that is demo data (demo sellers or generated synthetic history), 0–1; 0 = all live. Show a Demo mark above 0"`
}

// seasonAt interpolates the monthly index to a day (each month's value sits
// at its middle).
func seasonAt(s *Seasonality, day time.Time) float64 {
	m := float64(day.Month()-1) + float64(day.Day()-1)/31 - 0.5 // months since mid-January
	lo := int(math.Floor(m))
	frac := m - float64(lo)
	a := s.Months[(lo%12+12)%12]
	b := s.Months[((lo+1)%12+12)%12]
	return a + (b-a)*frac
}

// parts is one forecast as last week's price times multiplicative drivers.
type parts struct {
	base    float64
	factors []Driver // Share holds the multiplier − 1
}

func (p parts) value() float64 {
	f := p.base
	for _, d := range p.factors {
		f *= 1 + d.Share
	}
	return f
}

// drivers splits the change from base into each factor's exact Shapley share.
func (p parts) drivers() []Driver {
	out := []Driver{}
	if len(p.factors) == 0 || p.base <= 0 {
		return out
	}
	phi := ml.Shapley(len(p.factors), func(mask uint) float64 {
		f := 1.0
		for i, d := range p.factors {
			if mask&(1<<i) != 0 {
				f *= 1 + d.Share
			}
		}
		return f
	})
	for i, d := range p.factors {
		if v := round3(phi[i]); v != 0 {
			out = append(out, Driver{Code: d.Code, Share: v})
		}
	}
	return out
}

// predict forecasts the mean of the week starting `ahead` weeks after end,
// from the data up to end, with one method.
func (fr ForecastRules) predict(method string, ps []Point, s *Seasonality, end time.Time, ahead int) (parts, bool) {
	base, ok := windowMean(ps, end, 7)
	if !ok {
		return parts{}, false
	}
	p := parts{base: base}
	switch method {
	case MethodNaive:
	case MethodLastYear:
		then, ok1 := windowMean(ps, end.AddDate(0, 0, -364), 7)
		later, ok2 := windowMean(ps, end.AddDate(0, 0, 7*ahead-364), 7)
		if !ok1 || !ok2 || then <= 0 {
			return parts{}, false
		}
		p.factors = append(p.factors, Driver{Code: "last_year", Share: later/then - 1})
	case MethodSeasonTrend:
		if s == nil {
			return parts{}, false
		}
		target := end.AddDate(0, 0, 7*ahead-3) // middle of the target week
		now := end.AddDate(0, 0, -3)           // middle of the base week
		p.factors = append(p.factors, Driver{Code: "season", Share: seasonAt(s, target)/seasonAt(s, now) - 1})
		if t, ok := TrendOf(ps, end); ok {
			effect, weight := 0.0, 1.0
			for k := 1; k <= ahead; k++ {
				weight *= fr.TrendDamping
				effect += t.Weekly * weight
			}
			p.factors = append(p.factors, Driver{Code: "trend", Share: math.Max(-fr.MaxTrendEffect, math.Min(fr.MaxTrendEffect, effect))})
		}
	default:
		return parts{}, false
	}
	return p, true
}

var forecastMethods = []string{MethodNaive, MethodLastYear, MethodSeasonTrend}

// ForecastOf forecasts the next weeks after end (4 under the default rules). Every method is checked on
// every week of the past year where all of them could forecast; a method
// that could not forecast often enough drops out (naive never does). ok is
// false without enough past weeks to judge.
func ForecastOf(ps []Point, s *Seasonality, end time.Time) (Forecast, bool) {
	return DefaultForecastRules.Forecast(ps, s, end)
}

// errorsAt is a method's error 1..Weeks weeks after origin, forecast from
// past (the points up to origin) and checked against ps; ok only when every
// week could be checked.
func (fr ForecastRules) errorsAt(method string, past, ps []Point, s *Seasonality, origin time.Time) ([]float64, bool) {
	var e []float64
	for h := 1; h <= fr.Weeks; h++ {
		f, ok := fr.predict(method, past, s, origin, h)
		actual, ok2 := windowMean(ps, origin.AddDate(0, 0, 7*h), 7)
		if !ok || !ok2 || actual <= 0 {
			return nil, false
		}
		e = append(e, math.Abs(f.value()/actual-1))
	}
	return e, true
}

// Forecast is ForecastOf under these rules: fr.Weeks weeks ahead.
func (fr ForecastRules) Forecast(ps []Point, s *Seasonality, end time.Time) (Forecast, bool) {
	forecastWeeks := fr.Weeks
	// errs[method][origin] = errors for weeks 1..4 ahead, when all four exist.
	errs := map[string]map[time.Time][]float64{}
	for _, m := range forecastMethods {
		errs[m] = map[time.Time][]float64{}
	}
	for origin := end.AddDate(0, 0, -365); !origin.After(end.AddDate(0, 0, -7*forecastWeeks)); origin = origin.AddDate(0, 0, 7) {
		var past []Point
		for _, p := range ps {
			if !p.Day.After(origin) {
				past = append(past, p)
			}
		}
		for _, m := range forecastMethods {
			if e, ok := fr.errorsAt(m, past, ps, s, origin); ok {
				errs[m][origin] = e
			}
		}
	}

	// Compare on the weeks every remaining method could forecast.
	methods := append([]string(nil), forecastMethods...)
	var common []time.Time
	for {
		common = common[:0]
		for o := range errs[MethodNaive] {
			all := true
			for _, m := range methods {
				if _, ok := errs[m][o]; !ok {
					all = false
					break
				}
			}
			if all {
				common = append(common, o)
			}
		}
		if len(common) >= fr.MinBacktests || len(methods) == 1 {
			break
		}
		// Drop the method that covered the fewest weeks.
		worst := -1
		for i, m := range methods {
			if m != MethodNaive && (worst < 0 || len(errs[m]) < len(errs[methods[worst]])) {
				worst = i
			}
		}
		methods = append(methods[:worst], methods[worst+1:]...)
	}
	if len(common) < fr.MinBacktests {
		return Forecast{}, false
	}
	sort.Slice(common, func(i, j int) bool { return common[i].Before(common[j]) })

	fc := Forecast{Tested: len(common), Weeks: []ForecastWeek{}, MAPE: make([]float64, forecastWeeks)}
	scores := map[string]float64{}
	for _, m := range methods {
		sum := 0.0
		for _, o := range common {
			for _, e := range errs[m][o] {
				sum += e
			}
		}
		scores[m] = sum / float64(len(common)*forecastWeeks)
		fc.Methods = append(fc.Methods, MethodScore{Method: m, MAPE: round3(scores[m])})
	}
	sort.SliceStable(fc.Methods, func(i, j int) bool { return scores[fc.Methods[i].Method] < scores[fc.Methods[j].Method] })
	// The best method that can forecast all four weeks from today.
	for _, m := range fc.Methods {
		ok := true
		for h := 1; h <= forecastWeeks && ok; h++ {
			_, ok = fr.predict(m.Method, ps, s, end, h)
		}
		if ok {
			fc.Method = m.Method
			break
		}
	}
	if fc.Method == "" {
		return Forecast{}, false
	}
	if naive := scores[MethodNaive]; naive > 0 && fc.Method != MethodNaive {
		fc.Skill = round3(1 - scores[fc.Method]/naive)
	}
	fc.Note = forecastNotes[fc.Method]

	for h := 1; h <= forecastWeeks; h++ {
		e := make([]float64, 0, len(common))
		for _, o := range common {
			e = append(e, errs[fc.Method][o][h-1])
		}
		mean := 0.0
		for _, x := range e {
			mean += x / float64(len(e))
		}
		fc.MAPE[h-1] = round3(mean)
		sort.Float64s(e)
		q := e[int(math.Min(float64(len(e)-1), math.Ceil(fr.IntervalQuantile*float64(len(e)))-1))]
		p, ok := fr.predict(fc.Method, ps, s, end, h)
		if !ok {
			return Forecast{}, false
		}
		fc.Base = roundPrice(p.base)
		f := p.value()
		start := end.AddDate(0, 0, 7*(h-1)+1)
		fc.Weeks = append(fc.Weeks, ForecastWeek{Week: start.Format(time.DateOnly), Median: roundPrice(f),
			Low: roundPrice(f * (1 - q)), High: roundPrice(f * (1 + q)), Drivers: p.drivers()})
	}
	return fc, true
}

var forecastNotes = map[string]string{
	MethodNaive:       "Taxmin, kafolat emas: oʻtgan yil sinovida narxni oʻzgarmas deb olish boshqa usullardan aniqroq chiqdi. Oraliq oʻtgan yildagi xatolardan olingan.",
	MethodLastYear:    "Taxmin, kafolat emas: narx oʻtgan yilning shu haftalarida qanday oʻzgargan boʻlsa, shunday oʻzgaradi deb olindi. Oraliq oʻtgan yildagi xatolardan olingan.",
	MethodSeasonTrend: "Taxmin, kafolat emas: mavsum va soʻnggi trend boʻyicha. Oraliq oʻtgan yildagi xatolardan olingan; mavsum indeksi butun tarixdan tuzilgani uchun oʻtmishdagi aniqlik biroz yaxshi koʻrinishi mumkin.",
}

// roundPrice keeps a forecast from looking more exact than it is.
func roundPrice(x float64) int64 {
	step := 10.0
	switch {
	case x >= 100_000:
		step = 1000
	case x >= 10_000:
		step = 100
	case x >= 1000:
		step = 50
	}
	return int64(math.Round(x/step) * step)
}
