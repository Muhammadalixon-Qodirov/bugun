package insight

import (
	"context"
	"encoding/json"
	"log/slog"
	"math"
	"sort"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"bugun/internal/ml"
	"bugun/internal/store/db"
)

// The boosted-tree forecast (PROGNOZ_ML.md §2–5): one model per horizon
// (1–4 weeks) over every series at once, trained nightly on three years of
// weeks. The last 12 target weeks pick the number of trees, then the model
// is refit on everything. Every series' latest week is forecast and
// explained with TreeSHAP, grouped into drivers, and written to forecast_ml
// for the analytics page. The model is never stored: it is relearned from
// the data every night.

// MLHistoryYears of weeks train the model.
const MLHistoryYears = 3

// mlValidWeeks: the last target weeks held out to choose the tree count.
const mlValidWeeks = 12

// mlParams trade a little accuracy for speed on half a million rows.
var mlParams = ml.GBMParams{Subsample: 0.6}

// HorizonFit is one horizon's figures.
type HorizonFit struct {
	Horizon   int     `json:"horizon"`
	Trees     int     `json:"trees"`
	TrainRows int     `json:"train_rows"`
	ValidRows int     `json:"valid_rows"`
	ValidMAPE float64 `json:"valid_mape" doc:"Mean |forecast/actual − 1| on the held-out weeks"`
}

// fitHorizon trains the horizon's model: early stopping on the last weeks,
// then a refit on all rows with that many trees.
func fitHorizon(rows []Row, h int) (*ml.GBM, HorizonFit) {
	fit := HorizonFit{Horizon: h}
	lastTarget := -1
	for _, r := range rows {
		if !math.IsNaN(r.Y[h-1]) && r.Week+h > lastTarget {
			lastTarget = r.Week + h
		}
	}
	var x, vx, all [][]float64
	var y, vy, ally []float64
	for _, r := range rows {
		t := r.Y[h-1]
		if math.IsNaN(t) {
			continue
		}
		all, ally = append(all, r.X), append(ally, t)
		if r.Week+h > lastTarget-mlValidWeeks {
			vx, vy = append(vx, r.X), append(vy, t)
		} else {
			x, y = append(x, r.X), append(y, t)
		}
	}
	fit.TrainRows, fit.ValidRows = len(x), len(vx)
	if len(all) == 0 {
		return nil, fit
	}
	trees := 200
	if len(x) > 0 && len(vx) >= 100 {
		probe := ml.TrainGBM(x, y, vx, vy, mlParams)
		trees = max(20, len(probe.Trees))
		sum := 0.0
		for i, row := range vx {
			sum += math.Abs(math.Exp(probe.Predict(row)-vy[i]) - 1)
		}
		fit.ValidMAPE = math.Round(sum/float64(len(vx))*10000) / 10000
	}
	p := mlParams
	p.Trees = trees
	m := ml.TrainGBM(all, ally, nil, nil, p)
	fit.Trees = len(m.Trees)
	return m, fit
}

// drivers splits one forecast into driver groups as shares of the base
// price that add up to exp(change) − 1. The model's expected change (what
// every series gets before its own features) counts as history.
func drivers(m *ml.GBM, x []float64, change float64) map[string]float64 {
	phi, expected := m.Explain(x)
	groups := map[string]float64{"history": expected}
	for i, v := range phi {
		groups[FeatureGroups[FeatureNames[i]]] += v
	}
	scale := 1.0
	if math.Abs(change) > 1e-9 {
		scale = (math.Exp(change) - 1) / change
	}
	out := map[string]float64{}
	for g, v := range groups {
		if s := math.Round(v*scale*10000) / 10000; s != 0 {
			out[g] = s
		}
	}
	return out
}

// TrainForecastModel is the nightly job: train, forecast every series and
// replace forecast_ml. It returns how many series were forecast.
func (s *Service) TrainForecastModel(ctx context.Context) (int, error) {
	run, err := s.q.StartMLRun(ctx, "train")
	if err != nil {
		return 0, err
	}
	n, err := s.trainForecastModel(ctx, run)
	if err != nil {
		msg := err.Error()
		_ = s.q.FinishMLRun(context.WithoutCancel(ctx), db.FinishMLRunParams{ID: run, Horizons: []byte("[]"), Importance: []byte("{}"),
			Error: pgtype.Text{String: msg, Valid: true}})
	}
	return n, err
}

func (s *Service) trainForecastModel(ctx context.Context, run int64) (int, error) {
	started := time.Now()
	in, err := s.LoadMLInput(ctx, time.Now().AddDate(-MLHistoryYears, 0, 0))
	if err != nil {
		return 0, err
	}
	rows := BuildRows(in)
	// The rows to forecast: each series' latest week, if recent.
	latest := map[SeriesKey]int{}
	for i, r := range rows {
		if r.Week < in.Weeks-2 {
			continue
		}
		if j, ok := latest[r.Key]; !ok || rows[j].Week < r.Week {
			latest[r.Key] = i
		}
	}
	var fits []HorizonFit
	importance := make([]float64, len(FeatureNames))
	var out []db.InsertForecastMLParams
	for h := 1; h <= Horizons; h++ {
		if err := ctx.Err(); err != nil {
			return 0, err
		}
		m, fit := fitHorizon(rows, h)
		fits = append(fits, fit)
		if m == nil {
			continue
		}
		for i, v := range m.Importance {
			importance[i] += v / Horizons
		}
		for _, i := range latest {
			r := rows[i]
			change := m.Predict(r.X)
			d, _ := json.Marshal(drivers(m, r.X, change))
			out = append(out, db.InsertForecastMLParams{ProductID: r.Key.Product, TradeType: r.Key.Trade, ProvinceID: r.Key.Province,
				Horizon: int16(h), OriginWeek: pgtype.Date{Time: in.Start.AddDate(0, 0, 7*r.Week), Valid: true}, Base: r.Current,
				Change: change, Drivers: d, RunID: run})
		}
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)
	q := s.q.WithTx(tx)
	if err := q.DeleteForecastML(ctx); err != nil {
		return 0, err
	}
	if _, err := q.InsertForecastML(ctx, out); err != nil {
		return 0, err
	}
	hj, _ := json.Marshal(fits)
	ij, _ := json.Marshal(importanceReport(importance))
	if err := q.FinishMLRun(ctx, db.FinishMLRunParams{ID: run, Rows: int32(len(rows)), Weeks: int32(in.Weeks), Horizons: hj, Importance: ij}); err != nil {
		return 0, err
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	s.Invalidate()
	slog.InfoContext(ctx, "forecast model trained", "rows", len(rows), "series", len(latest), "took", time.Since(started).Round(time.Second))
	return len(latest), nil
}

// Importance is how much each driver group and feature mattered.
type Importance struct {
	Groups   map[string]float64 `json:"groups"`
	Features []FeatureWeight    `json:"features" doc:"Top features, most important first"`
}

type FeatureWeight struct {
	Feature string  `json:"feature"`
	Group   string  `json:"group"`
	Weight  float64 `json:"weight"`
}

func importanceReport(w []float64) Importance {
	imp := Importance{Groups: map[string]float64{}}
	for i, v := range w {
		name := FeatureNames[i]
		imp.Groups[FeatureGroups[name]] = math.Round((imp.Groups[FeatureGroups[name]]+v)*1000) / 1000
		imp.Features = append(imp.Features, FeatureWeight{Feature: name, Group: FeatureGroups[name], Weight: math.Round(v*1000) / 1000})
	}
	sort.SliceStable(imp.Features, func(i, j int) bool { return imp.Features[i].Weight > imp.Features[j].Weight })
	if len(imp.Features) > 12 {
		imp.Features = imp.Features[:12]
	}
	return imp
}

// forecastMLRows reads the stored forecast of one series (province 0 = country).
func (s *Service) forecastMLRows(ctx context.Context, product int16, trade string, province int16) ([]db.ForecastMLForRow, error) {
	rows, err := s.q.ForecastMLFor(ctx, db.ForecastMLForParams{ProductID: product, TradeType: trade, ProvinceID: province})
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return rows, err
}

// mlBacktestWeeks: the past year the model is checked on.
const mlBacktestWeeks = 52

// BacktestForecastModel checks the model on the past year without letting
// it see that year: one model per horizon is trained only on targets before
// the cut a year ago, then forecasts every week after it. The errors go to
// forecast_ml_backtest, where the analytics page compares them with the
// other methods on the same weeks. It returns how many errors were stored.
func (s *Service) BacktestForecastModel(ctx context.Context) (int, error) {
	run, err := s.q.StartMLRun(ctx, "backtest")
	if err != nil {
		return 0, err
	}
	in, err := s.LoadMLInput(ctx, time.Now().AddDate(-MLHistoryYears, 0, 0))
	if err != nil {
		return 0, err
	}
	rows := BuildRows(in)
	cut := in.Weeks - 1 - mlBacktestWeeks
	var out []db.InsertMLBacktestParams
	var fits []HorizonFit
	for h := 1; h <= Horizons; h++ {
		if err := ctx.Err(); err != nil {
			return 0, err
		}
		var before []Row
		for _, r := range rows {
			if r.Week+h <= cut {
				before = append(before, r)
			}
		}
		m, fit := fitHorizon(before, h)
		if m == nil {
			fits = append(fits, fit)
			continue
		}
		sum, n := 0.0, 0
		for _, r := range rows {
			if r.Week <= cut || math.IsNaN(r.Y[h-1]) {
				continue
			}
			e := math.Abs(math.Exp(m.Predict(r.X)-r.Y[h-1]) - 1)
			sum += e
			n++
			out = append(out, db.InsertMLBacktestParams{ProductID: r.Key.Product, TradeType: r.Key.Trade, ProvinceID: r.Key.Province,
				OriginWeek: pgtype.Date{Time: in.Start.AddDate(0, 0, 7*r.Week), Valid: true}, Horizon: int16(h), Error: e})
		}
		if n > 0 {
			fit.ValidMAPE = math.Round(sum/float64(n)*10000) / 10000
		}
		fits = append(fits, fit)
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)
	q := s.q.WithTx(tx)
	if err := q.DeleteMLBacktest(ctx); err != nil {
		return 0, err
	}
	if _, err := q.InsertMLBacktest(ctx, out); err != nil {
		return 0, err
	}
	hj, _ := json.Marshal(fits)
	if err := q.FinishMLRun(ctx, db.FinishMLRunParams{ID: run, Rows: int32(len(rows)), Weeks: int32(in.Weeks), Horizons: hj, Importance: []byte("{}")}); err != nil {
		return 0, err
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	s.Invalidate()
	return len(out), nil
}

// ModelRun is one finished training or backtest of the forecast model.
type ModelRun struct {
	Kind       string       `json:"kind" enum:"train,backtest"`
	StartedAt  time.Time    `json:"started_at"`
	FinishedAt time.Time    `json:"finished_at"`
	Rows       int          `json:"rows" doc:"Series-weeks the model learned from"`
	Weeks      int          `json:"weeks"`
	Horizons   []HorizonFit `json:"horizons" doc:"Per week ahead; for backtest, valid_mape is the error on the past year"`
	Importance *Importance  `json:"importance,omitempty" doc:"Train runs only"`
	Error      string       `json:"error,omitempty"`
}

// ModelReport is the latest train and backtest run (admin page).
func (s *Service) ModelReport(ctx context.Context) ([]ModelRun, error) {
	runs, err := s.q.LatestMLRuns(ctx)
	if err != nil {
		return nil, err
	}
	out := []ModelRun{}
	for _, r := range runs {
		m := ModelRun{Kind: r.Kind, StartedAt: r.StartedAt, FinishedAt: *r.FinishedAt, Rows: int(r.Rows), Weeks: int(r.Weeks),
			Horizons: []HorizonFit{}, Error: r.Error.String}
		_ = json.Unmarshal(r.Horizons, &m.Horizons)
		if r.Kind == "train" {
			var imp Importance
			if json.Unmarshal(r.Importance, &imp) == nil && imp.Groups != nil {
				m.Importance = &imp
			}
		}
		out = append(out, m)
	}
	return out, nil
}
