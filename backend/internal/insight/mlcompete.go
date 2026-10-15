package insight

import (
	"context"
	"encoding/json"
	"math"
	"sort"
	"time"

	"bugun/internal/store/db"
)

// The boosted-tree model joins the forecast competition (PROGNOZ_ML.md §3):
// its backtest errors are per week (Monday to Sunday), so every method is
// scored again at the end of exactly those weeks, and the one with the
// smallest error over the weeks all could forecast is shown.

const mlNote = "Taxmin, kafolat emas: bayram, ob-havo, dollar kursi, talab va taklifni hisobga oladigan model (gradient boosting). Oʻtgan yilgi sinovda boshqa usullardan aniqroq chiqdi; oraliq oʻsha xatolardan olingan."

// withML adds the model to a.Forecast when its forecast and backtest exist.
func (s *Service) withML(ctx context.Context, a *Analytics, p Params, points []Point) error {
	rows, err := s.forecastMLRows(ctx, p.ProductID, p.Trade, p.Province)
	if err != nil || len(rows) != Horizons {
		return err
	}
	bt, err := s.q.MLBacktestFor(ctx, db.MLBacktestForParams{ProductID: p.ProductID, TradeType: p.Trade, ProvinceID: p.Province})
	if err != nil {
		return err
	}
	mlErrs := map[time.Time][]float64{}
	for _, r := range bt {
		o := r.OriginWeek.Time.AddDate(0, 0, 6) // the Sunday that ends the week
		if mlErrs[o] == nil {
			mlErrs[o] = make([]float64, Horizons)
		}
		mlErrs[o][r.Horizon-1] = r.Error
	}
	fr := s.forecastRules
	if fr.Weeks != Horizons {
		return nil // the model forecasts exactly 4 weeks
	}
	errs := map[string]map[time.Time][]float64{MethodML: {}}
	for _, m := range forecastMethods {
		errs[m] = map[time.Time][]float64{}
	}
	var common []time.Time
	for o, e := range mlErrs {
		full := true
		for _, v := range e {
			full = full && v > 0
		}
		if !full {
			continue
		}
		var past []Point
		for _, pt := range points {
			if !pt.Day.After(o) {
				past = append(past, pt)
			}
		}
		ne, ok := fr.errorsAt(MethodNaive, past, points, a.Seasonality, o)
		if !ok {
			continue
		}
		errs[MethodML][o], errs[MethodNaive][o] = e, ne
		for _, m := range []string{MethodLastYear, MethodSeasonTrend} {
			if me, ok := fr.errorsAt(m, past, points, a.Seasonality, o); ok {
				errs[m][o] = me
			}
		}
		common = append(common, o)
	}
	if len(common) < fr.MinBacktests {
		return nil
	}
	sort.Slice(common, func(i, j int) bool { return common[i].Before(common[j]) })
	scores := map[string]float64{}
	var methods []MethodScore
	for _, m := range append([]string{MethodML}, forecastMethods...) {
		sum, n := 0.0, 0
		for _, o := range common {
			if e, ok := errs[m][o]; ok {
				for _, v := range e {
					sum += v
					n++
				}
			}
		}
		if n < fr.MinBacktests*Horizons { // too few weeks to compare
			continue
		}
		scores[m] = sum / float64(n)
		methods = append(methods, MethodScore{Method: m, MAPE: round3(scores[m])})
	}
	sort.SliceStable(methods, func(i, j int) bool { return scores[methods[i].Method] < scores[methods[j].Method] })
	skill := func(m string) float64 {
		if naive := scores[MethodNaive]; naive > 0 && m != MethodNaive {
			return round3(1 - scores[m]/naive)
		}
		return 0
	}
	if methods[0].Method != MethodML {
		// Another method is better: keep its forecast, with the fair scores.
		if a.Forecast != nil {
			a.Forecast.Methods = methods
			a.Forecast.Skill = skill(a.Forecast.Method)
		}
		return nil
	}
	fc := Forecast{Method: MethodML, Methods: methods, Skill: skill(MethodML), Tested: len(common), Note: mlNote,
		MAPE: make([]float64, Horizons), Weeks: []ForecastWeek{}, Base: roundPrice(rows[0].Base), DemoShare: a.Quality.HistoryDemoShare}
	for h := 1; h <= Horizons; h++ {
		e := make([]float64, 0, len(common))
		for _, o := range common {
			e = append(e, errs[MethodML][o][h-1])
		}
		mean := 0.0
		for _, v := range e {
			mean += v / float64(len(e))
		}
		fc.MAPE[h-1] = round3(mean)
		sort.Float64s(e)
		q := e[int(math.Min(float64(len(e)-1), math.Ceil(fr.IntervalQuantile*float64(len(e)))-1))]
		r := rows[h-1]
		f := r.Base * math.Exp(r.Change)
		var d map[string]float64
		_ = json.Unmarshal(r.Drivers, &d)
		week := ForecastWeek{Week: r.OriginWeek.Time.AddDate(0, 0, 7*h).Format(time.DateOnly), Median: roundPrice(f),
			Low: roundPrice(f * (1 - q)), High: roundPrice(f * (1 + q)), Drivers: []Driver{}}
		for code, share := range d {
			week.Drivers = append(week.Drivers, Driver{Code: code, Share: share})
		}
		sort.Slice(week.Drivers, func(i, j int) bool { return math.Abs(week.Drivers[i].Share) > math.Abs(week.Drivers[j].Share) })
		fc.Weeks = append(fc.Weeks, week)
	}
	a.Forecast = &fc
	return nil
}
