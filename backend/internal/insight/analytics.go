package insight

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"bugun/internal/analytics"
	"bugun/internal/market"
	"bugun/internal/store/db"
)

// Params pick what to analyse.
type Params struct {
	ProductID int16
	Province  int16 // 0 = whole country
	Trade     string
	Period    string  // 30d, 90d, 1y, 3y
	Transport float64 // so'm per unit per 100 km, for arbitrage
}

// Periods and how their series are bucketed.
var periods = map[string]struct {
	days   int
	bucket string
}{
	"30d": {30, "day"},
	"90d": {90, "day"},
	"1y":  {365, "day"},
	"3y":  {3 * 365, "week"},
}

func errUnknownPeriod(p string) error { return fmt.Errorf("unknown period %q", p) }

// ValidPeriod reports whether p is a known period.
func ValidPeriod(p string) bool { _, ok := periods[p]; return ok }

type Scope struct {
	ProvinceID   int16  `json:"province_id,omitempty" doc:"Omitted for the whole country"`
	ProvinceName string `json:"province_name,omitempty"`
	Trade        string `json:"trade"`
	Period       string `json:"period"`
	From         string `json:"from"`
	To           string `json:"to"`
	Bucket       string `json:"bucket" enum:"day,week" doc:"One series point per day or per week (from Monday)"`
}

type Quality struct {
	OffersNow         int32            `json:"offers_now" doc:"Live prices behind the median now"`
	BySource          map[string]int32 `json:"by_source" doc:"Prices of the last 30 days by source: seller, seller_auto, rfq_offer, deal, ..."`
	ConfirmedDeals30d int32            `json:"confirmed_deals_30d"`
	DemoShare         float64          `json:"demo_share" doc:"Share of the shown history (series) that is demo data: demo sellers or generated synthetic history, 0–1; 0 = all live. Show a Demo mark above 0"`
	HistoryDemoShare  float64          `json:"history_demo_share" doc:"The same share over the ~26 months behind changes, trend, seasonality, weekdays and the forecast"`
	LastPriceAt       *time.Time       `json:"last_price_at,omitempty"`
}

type NowStats struct {
	Median int64 `json:"median"`
	P25    int64 `json:"p25"`
	P75    int64 `json:"p75"`
	Min    int64 `json:"min"`
	Max    int64 `json:"max"`
	Offers int32 `json:"offers"`
}

type SeriesPoint struct {
	Date      string  `json:"date"`
	Median    int64   `json:"median"`
	P25       int64   `json:"p25"`
	P75       int64   `json:"p75"`
	Min       int64   `json:"min"`
	Max       int64   `json:"max"`
	Offers    int32   `json:"offers"`
	DemoShare float64 `json:"demo_share" doc:"Share of the prices behind this that are demo data (demo sellers or generated synthetic history), 0–1; 0 = all live. Show a Demo mark above 0"`
}

type Record struct {
	Price        int64  `json:"price"`
	Date         string `json:"date"`
	ProvinceID   int16  `json:"province_id"`
	ProvinceName string `json:"province_name"`
}

type YearRecords struct {
	Year int32   `json:"year"`
	Max  *Record `json:"max,omitempty"`
	Min  *Record `json:"min,omitempty"`
}

type Records struct {
	Max    *Record       `json:"max,omitempty" doc:"Dearest day in the period (days with at least 3 prices)"`
	Min    *Record       `json:"min,omitempty"`
	ByYear []YearRecords `json:"by_year"`
}

// Analytics is the product analytics page (TAHLIL_REJA.md §3).
type Analytics struct {
	Scope   Scope         `json:"scope"`
	Quality Quality       `json:"quality"`
	Now     *NowStats     `json:"now,omitempty" doc:"Omitted with fewer than 3 live prices"`
	Changes Changes       `json:"changes"`
	Trend   *Trend        `json:"trend,omitempty" doc:"Omitted with fewer than 7 days of the last 28"`
	Series  []SeriesPoint `json:"series"`
	Records Records       `json:"records"`
	Regional
	Seasonality *Seasonality  `json:"seasonality,omitempty" doc:"Omitted without about two years of history"`
	Pressure    *PressureNow  `json:"pressure,omitempty" doc:"Demand against supply now (spec 10.3); omitted without two earlier weeks of data"`
	Advice      Advice        `json:"advice"`
	Jumps       []Jump        `json:"jumps" doc:"Weeks that moved more than 15%, with events around them"`
	Weekdays    *Weekdays     `json:"weekdays,omitempty" doc:"Omitted without 8 full weeks in the last 12"`
	Supply      Supply        `json:"supply"`
	Fx          *Fx           `json:"fx,omitempty" doc:"Central Bank rates over the period; omitted before they are loaded"`
	Weather     *Weather      `json:"weather,omitempty" doc:"Open-Meteo weather over the period; omitted before it is loaded"`
	Reasons     []ReasonCount `json:"reasons" doc:"Why sellers changed this product's price in the last 30 days, as they said in the bot"`
	Forecast    *Forecast     `json:"forecast,omitempty" doc:"Next 4 weeks from the method that did best on the past year, with its accuracy and drivers; omitted without about a year of history"`
	Events      []EventRef    `json:"events" doc:"Events in the period, for marking the chart"`
	Anomalies   []AnomalyRef  `json:"anomalies" doc:"Suspicious viloyat-days in the period (ML_REJA.md 4); settled ones are left out of the series and every statistic"`
}

// AnomalyRef is a suspicious viloyat-day to mark on the chart.
type AnomalyRef struct {
	Date         string  `json:"date"`
	ProvinceID   int16   `json:"province_id"`
	ProvinceName string  `json:"province_name"`
	Layer        string  `json:"layer" enum:"temporal,cross_section" doc:"temporal: far from the viloyat's own last 4 weeks; cross_section: jumped while the other viloyatlar did not"`
	Value        int64   `json:"value" doc:"The day's median"`
	Reference    int64   `json:"reference" doc:"What was expected"`
	Excluded     bool    `json:"excluded" doc:"Left out of the chart and statistics; false while provisional (the last 4 days) so real news is never hidden"`
	Diff         float64 `json:"diff" doc:"value against reference, 0.4 = 40% above"`
	DemoShare    float64 `json:"demo_share" doc:"Share of the flagged day's prices that are demo data (demo sellers or generated synthetic history), 0–1"`
}

// Supply is who sells now.
type Supply struct {
	Listings      int32    `json:"listings" doc:"Live listings"`
	Sellers       int32    `json:"sellers"`
	SellersChange *float64 `json:"sellers_change,omitempty" doc:"Against sellers with a live price a week ago"`
	Stock         *float64 `json:"stock,omitempty" doc:"Total stock the sellers gave, in the product's unit; omitted when none gave one"`
	Stocked       int32    `json:"stocked" doc:"How many listings gave a stock"`
}

// PressureNow is the demand–supply index now.
type PressureNow struct {
	Value float64 `json:"value" doc:"1 is a usual week; above 1 demand outruns supply"`
	Zone  string  `json:"zone" enum:"surplus,balanced,shortage"`
}

type cached struct {
	at time.Time
	a  *Analytics
}

// cacheFor keeps a page for a while: the history changes hourly at most.
const cacheFor = 10 * time.Minute

// Analytics builds the page for p.
func (s *Service) Analytics(ctx context.Context, p Params) (*Analytics, error) {
	s.mu.Lock()
	if c, ok := s.cache[p]; ok && time.Since(c.at) < cacheFor {
		s.mu.Unlock()
		return c.a, nil
	}
	s.mu.Unlock()

	a, err := s.build(ctx, p)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	if s.cache == nil {
		s.cache = map[Params]cached{}
	}
	s.cache[p] = cached{time.Now(), a}
	s.mu.Unlock()
	return a, nil
}

func (s *Service) build(ctx context.Context, p Params) (*Analytics, error) {
	per, ok := periods[p.Period]
	if !ok {
		return nil, errUnknownPeriod(p.Period)
	}
	today := Day(time.Now()).Time
	from := today.AddDate(0, 0, -per.days+1)
	prov := pgtype.Int2{Int16: p.Province, Valid: p.Province != 0}
	a := &Analytics{Scope: Scope{Trade: p.Trade, Period: p.Period, From: from.Format(time.DateOnly), To: today.Format(time.DateOnly), Bucket: per.bucket}}
	if p.Province != 0 {
		r, err := s.q.GetRegion(ctx, p.Province)
		if err != nil {
			return nil, err
		}
		a.Scope.ProvinceID, a.Scope.ProvinceName = r.ID, r.Name
	}

	// Changes, trend and forecast always look at the last ~26 months, day by
	// day: the forecast is checked on last year's weeks with a year before each.
	daily, err := s.q.PriceSeries(ctx, db.PriceSeriesParams{
		Bucket: "day", ProductID: p.ProductID, TradeType: p.Trade, ProvinceID: prov,
		FromDay: date(today.AddDate(0, 0, -800)), ToDay: date(today),
	})
	if err != nil {
		return nil, err
	}
	points := make([]Point, len(daily))
	var histDemo, histOffers float64
	for i, r := range daily {
		points[i] = Point{Day: r.Day.Time, Median: r.Median}
		histDemo += r.DemoShare * float64(r.Offers)
		histOffers += float64(r.Offers)
	}
	if histOffers > 0 {
		a.Quality.HistoryDemoShare = math.Round(histDemo/histOffers*100) / 100
	}
	a.Changes = ChangesOf(points, today)
	if t, ok := TrendOf(points, today); ok {
		a.Trend = &t
	}

	rows := daily
	if per.bucket != "day" || per.days > 400 {
		if rows, err = s.q.PriceSeries(ctx, db.PriceSeriesParams{
			Bucket: per.bucket, ProductID: p.ProductID, TradeType: p.Trade, ProvinceID: prov,
			FromDay: date(from), ToDay: date(today),
		}); err != nil {
			return nil, err
		}
	}
	a.Series = []SeriesPoint{}
	var shown []Point
	var demoSum, offerSum float64
	for _, r := range rows {
		if r.Day.Time.Before(from) {
			continue
		}
		a.Series = append(a.Series, SeriesPoint{
			Date: r.Day.Time.Format(time.DateOnly), Median: rnd(r.Median), P25: rnd(r.P25), P75: rnd(r.P75),
			Min: r.MinPrice, Max: r.MaxPrice, Offers: r.Offers, DemoShare: math.Round(r.DemoShare*100) / 100,
		})
		shown = append(shown, Point{Day: r.Day.Time, Median: r.Median})
		demoSum += r.DemoShare * float64(r.Offers)
		offerSum += float64(r.Offers)
	}
	if offerSum > 0 {
		a.Quality.DemoShare = math.Round(demoSum/offerSum*100) / 100
	}

	weeks := shown
	if per.bucket == "day" {
		weeks = weekly(shown)
	}
	if err := s.events(ctx, a, p, prov, weeks, from, today); err != nil {
		return nil, err
	}
	if err := s.records(ctx, a, p, prov, from, today); err != nil {
		return nil, err
	}
	if err := s.anomalies(ctx, a, p, prov, from, today); err != nil {
		return nil, err
	}
	if err := s.live(ctx, a, p, prov); err != nil {
		return nil, err
	}
	if err := s.regional(ctx, a, p, prov); err != nil {
		return nil, err
	}
	if err := s.outlook(ctx, a, p, prov, points, today); err != nil {
		return nil, err
	}
	if f, ok := s.forecastRules.Forecast(points, a.Seasonality, today); ok {
		f.DemoShare = a.Quality.HistoryDemoShare
		a.Forecast = &f
	}
	if err := s.withML(ctx, a, p, points); err != nil {
		return nil, err
	}
	if w, ok := WeekdaysOf(points, today); ok {
		a.Weekdays = &w
	}
	if err := s.supply(ctx, a, p, prov); err != nil {
		return nil, err
	}
	prices := make(map[string]float64, len(a.Series))
	for _, sp := range a.Series {
		prices[sp.Date] = float64(sp.Median)
	}
	if err := s.fx(ctx, a, per.bucket, from, today, prices); err != nil {
		return nil, err
	}
	if err := s.weather(ctx, a, prov, per.bucket, from, today); err != nil {
		return nil, err
	}
	if err := s.reasons(ctx, a, p, prov); err != nil {
		return nil, err
	}
	return a, nil
}

func (s *Service) records(ctx context.Context, a *Analytics, p Params, prov pgtype.Int2, from, to time.Time) error {
	rows, err := s.q.PriceRecords(ctx, db.PriceRecordsParams{
		ProductID: p.ProductID, TradeType: p.Trade, FromDay: date(from), ToDay: date(to), ProvinceID: prov,
	})
	if err != nil {
		return err
	}
	a.Records.ByYear = []YearRecords{}
	for _, r := range rows {
		rec := &Record{Price: rnd(r.Median), Date: r.Day.Time.Format(time.DateOnly), ProvinceID: r.ProvinceID, ProvinceName: r.ProvinceName}
		if n := len(a.Records.ByYear); n == 0 || a.Records.ByYear[n-1].Year != r.Year {
			a.Records.ByYear = append(a.Records.ByYear, YearRecords{Year: r.Year})
		}
		y := &a.Records.ByYear[len(a.Records.ByYear)-1]
		if r.IsMax {
			y.Max = rec
			if a.Records.Max == nil || rec.Price > a.Records.Max.Price {
				a.Records.Max = rec
			}
		} else {
			y.Min = rec
			if a.Records.Min == nil || rec.Price < a.Records.Min.Price {
				a.Records.Min = rec
			}
		}
	}
	return nil
}

func (s *Service) anomalies(ctx context.Context, a *Analytics, p Params, prov pgtype.Int2, from, to time.Time) error {
	rows, err := s.q.AnomaliesFor(ctx, db.AnomaliesForParams{ProductID: p.ProductID, TradeType: p.Trade, FromDay: date(from), ToDay: date(to), ProvinceID: prov})
	if err != nil {
		return err
	}
	a.Anomalies = make([]AnomalyRef, len(rows))
	for i, r := range rows {
		a.Anomalies[i] = AnomalyRef{
			Date: r.Day.Time.Format(time.DateOnly), ProvinceID: r.ProvinceID, ProvinceName: r.ProvinceName, Layer: r.Layer,
			Value: rnd(r.Value), Reference: rnd(r.Reference), Excluded: r.Status == "confirmed" || r.Settled,
			Diff: round3(r.Value/r.Reference - 1), DemoShare: math.Round(r.DemoShare*100) / 100,
		}
	}
	return nil
}

func (s *Service) live(ctx context.Context, a *Analytics, p Params, prov pgtype.Int2) error {
	st, err := s.q.LiveStats(ctx, db.LiveStatsParams{ProductID: p.ProductID, TradeType: p.Trade, ProvinceID: prov})
	if err != nil {
		return err
	}
	a.Quality.OffersNow = st.Offers
	if st.Offers > 0 {
		t := st.LastPriceAt
		a.Quality.LastPriceAt = &t
	}
	if st.Offers >= analytics.MinObservations {
		a.Now = &NowStats{Median: rnd(st.Median), P25: rnd(st.P25), P75: rnd(st.P75), Min: st.MinPrice, Max: st.MaxPrice, Offers: st.Offers}
	}
	mix, err := s.q.SourceMix(ctx, db.SourceMixParams{ProductID: p.ProductID, TradeType: p.Trade, ProvinceID: prov})
	if err != nil {
		return err
	}
	a.Quality.BySource = map[string]int32{}
	for _, m := range mix {
		a.Quality.BySource[m.Source] = m.N
	}
	a.Quality.ConfirmedDeals30d, err = s.q.ConfirmedDeals30d(ctx, db.ConfirmedDeals30dParams{ProductID: p.ProductID, ProvinceID: prov})
	return err
}

func date(t time.Time) pgtype.Date { return pgtype.Date{Time: t, Valid: true} }

func rnd(x float64) int64 { return int64(math.Round(x)) }

// holidayHorizon: holidays this many days ahead enter the advice.
const holidayHorizon = 21

// outlook adds the season, the demand pressure and the buy/sell advice.
func (s *Service) outlook(ctx context.Context, a *Analytics, p Params, prov pgtype.Int2, daily []Point, today time.Time) error {
	months, err := s.q.PriceSeries(ctx, db.PriceSeriesParams{
		Bucket: "month", ProductID: p.ProductID, TradeType: p.Trade, ProvinceID: prov,
		FromDay: date(today.AddDate(-4, 0, 0)), ToDay: date(today),
	})
	if err != nil {
		return err
	}
	ms := make([]MonthPoint, len(months))
	for i, m := range months {
		ms[i] = MonthPoint{Month: m.Day.Time, Median: m.Median}
	}
	if sn, ok := SeasonalIndex(ms); ok {
		a.Seasonality = &sn
	}

	demand, supply, err := market.PressureWindows(ctx, s.q, p.ProductID, p.Province, p.Trade)
	if err != nil {
		return err
	}
	if v, zone, ok := analytics.Pressure(demand[0], demand[1:], supply[0], supply[1:]); ok {
		a.Pressure = &PressureNow{Value: math.Round(v*100) / 100, Zone: zone}
	}

	in := AdviceInput{Trend: a.Trend, Season: a.Seasonality, Today: today}
	if a.Pressure != nil {
		in.Zone = a.Pressure.Zone
	}
	if now, ok := windowMean(daily, today, 7); ok {
		if year, ok := windowMean(daily, today, 365); ok {
			in.LevelVsYear = diff(now, year)
		}
	}
	prod, err := s.q.GetProduct(ctx, p.ProductID)
	if err != nil {
		return err
	}
	hs, err := s.q.HolidaysAround(ctx, db.HolidaysAroundParams{Category: prod.Category, Today: date(today), Days: holidayHorizon})
	if err != nil {
		return err
	}
	for _, h := range hs {
		in.Holidays = append(in.Holidays, Holiday{Title: h.Title, Start: h.StartsOn.Time, End: h.EndsOn.Time})
	}
	a.Advice = AdviceOf(in)
	return nil
}

func (s *Service) supply(ctx context.Context, a *Analytics, p Params, prov pgtype.Int2) error {
	now, err := s.q.SupplyNow(ctx, db.SupplyNowParams{ProductID: p.ProductID, TradeType: p.Trade, ProvinceID: prov})
	if err != nil {
		return err
	}
	ago, err := s.q.SellersWeekAgo(ctx, db.SellersWeekAgoParams{ProductID: p.ProductID, TradeType: p.Trade, ProvinceID: prov})
	if err != nil {
		return err
	}
	a.Supply = Supply{Listings: now.Listings, Sellers: now.Sellers, SellersChange: diff(float64(now.Sellers), float64(ago)), Stocked: now.Stocked}
	if now.Stocked > 0 {
		st := math.Round(now.Stock*10) / 10
		a.Supply.Stock = &st
	}
	return nil
}
