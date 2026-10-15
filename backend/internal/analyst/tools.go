package analyst

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"bugun/internal/analytics"
	"bugun/internal/ask"
	"bugun/internal/insight"
	"bugun/internal/llm"
	"bugun/internal/market"
	"bugun/internal/store/db"
	"bugun/internal/tz"
)

// The functions the model may call (spec AI-5). Each runs the same
// deterministic code as the site and returns its numbers with where they
// come from; the model never computes a number itself.

// Tool names. The spec's names: narx_statistikasi, narx_dinamikasi,
// eng_arzon_takliflar, bosim_indeksi, hududlarni_solishtir; the last three
// (forecast, anomalies, top demand) are the site's other analytics.
const (
	ToolPriceStats     = "price_stats"
	ToolPriceTrend     = "price_trend"
	ToolCheapestOffers = "cheapest_offers"
	ToolPressure       = "pressure_index"
	ToolCompareRegions = "compare_regions"
	ToolForecast       = "forecast"
	ToolAnomalies      = "anomalies"
	ToolTopDemand      = "top_demand"
)

// ToolNames lists every tool, for routing evaluation.
var ToolNames = []string{ToolPriceStats, ToolPriceTrend, ToolCheapestOffers, ToolPressure, ToolCompareRegions, ToolForecast, ToolAnomalies, ToolTopDemand}

// Table is the rows behind an answer.
type Table struct {
	Columns []string `json:"columns" doc:"Stable column keys (median, observations, region, …) to translate"`
	Rows    [][]any  `json:"rows"`
}

// Source is one function call behind an answer.
type Source struct {
	Tool         string         `json:"tool" enum:"price_stats,price_trend,cheapest_offers,pressure_index,compare_regions,forecast,anomalies,top_demand,find_product,find_region,advice,seasonality,weekday_prices,price_reasons,district_prices,wholesale_retail,arbitrage,region_overview,weekly_movers,compare_products,basket_cost,exchange_rate,weather,market_events,seller_info,platform_help,create_request,watch_price"`
	Params       map[string]any `json:"params" doc:"The arguments as resolved: product slug, region, trade, dates"`
	DataFrom     string         `json:"data_from,omitempty" doc:"First date of the data used, YYYY-MM-DD"`
	DataTo       string         `json:"data_to,omitempty" doc:"Last date of the data used"`
	Observations int            `json:"observations" doc:"Prices (or offers, weeks, days) behind the numbers"`
	DemoShare    float64        `json:"demo_share" doc:"Share of the data that is demo (demo sellers or generated history), 0–1; show a Demo mark above 0"`
	Enough       bool           `json:"enough" doc:"false: fewer than 3 observations (or no history); the answer says so and does not guess"`
	Error        string         `json:"error,omitempty" enum:"unknown_product,unknown_region,bad_arguments" doc:"The call could not run"`
}

// result is what a tool gives: data for the model, the table, the source and
// a templated summary for answers without a model.
type result struct {
	data    map[string]any
	table   *Table
	source  Source
	summary func(lang string) string
	// What the call was about, for the question log.
	productID int16
	region    *ask.Region
	// action is a change proposed for the user to confirm.
	action *Action
}

// args are every tool's arguments; each tool reads the ones it has.
type args struct {
	Product  string   `json:"product"`
	Region   string   `json:"region"`
	Regions  []string `json:"regions"`
	Trade    string   `json:"trade"`
	DateFrom string   `json:"date_from"`
	DateTo   string   `json:"date_to"`
	Weeks    int      `json:"weeks"`
	Quantity float64  `json:"quantity"`
}

var errUnknownProduct = errors.New("unknown_product")

func strProp(desc string) map[string]any {
	return map[string]any{"type": "string", "description": desc}
}

var (
	productProp = strProp("Product as the user wrote it or its catalog name, any language (e.g. piyoz, помидор)")
	regionProp  = strProp("Viloyat, city or tuman name as written (e.g. Samarqand, Chust); omit for the whole country")
	tradeProp   = map[string]any{"type": "string", "enum": []string{"ulgurji", "chakana"}, "description": "ulgurji (wholesale, default) or chakana (retail)"}
)

func schema(required []string, props map[string]any) map[string]any {
	return map[string]any{"type": "object", "properties": props, "required": required, "additionalProperties": false}
}

// toolDefs are the functions offered to the model.
func toolDefs() []llm.Tool {
	def := func(name, desc string, params map[string]any) llm.Tool {
		return llm.Tool{Type: "function", Function: llm.ToolFunction{Name: name, Description: desc, Parameters: params}}
	}
	return []llm.Tool{
		def(ToolPriceStats, "Market price of a product: median, p25, p75, min, max, number of prices, date range. Live prices now, or a past period with date_from/date_to.",
			schema([]string{"product"}, map[string]any{"product": productProp, "region": regionProp, "trade": tradeProp,
				"date_from": strProp("Period start YYYY-MM-DD (optional)"), "date_to": strProp("Period end YYYY-MM-DD (optional)")})),
		def(ToolPriceTrend, "Weekly median prices of a product for the last weeks, with the change from the first to the last week.",
			schema([]string{"product"}, map[string]any{"product": productProp, "region": regionProp, "trade": tradeProp,
				"weeks": map[string]any{"type": "integer", "minimum": 2, "maximum": 52, "description": "How many weeks (default 8)"}})),
		def(ToolCheapestOffers, "Live seller offers of a product, cheapest first; with quantity only sellers who can sell that amount.",
			schema([]string{"product"}, map[string]any{"product": productProp, "region": regionProp, "trade": tradeProp,
				"quantity": map[string]any{"type": "number", "description": "Amount wanted in the product's unit (optional)"}})),
		def(ToolPressure, "Demand against supply for a product this week: demand, sellers, the index (1 = normal week), zone and the price trend.",
			schema([]string{"product"}, map[string]any{"product": productProp, "region": regionProp, "trade": tradeProp})),
		def(ToolCompareRegions, "Median live price of a product by viloyat, cheapest first; optionally only the named regions.",
			schema([]string{"product"}, map[string]any{"product": productProp, "trade": tradeProp,
				"regions": map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Regions to compare (optional)"}})),
		def(ToolForecast, "Price forecast for the next 4 weeks, from the method that did best on the past year, with its range.",
			schema([]string{"product"}, map[string]any{"product": productProp, "region": regionProp, "trade": tradeProp})),
		def(ToolAnomalies, "Suspicious days in a product's daily prices over the last 90 days (flagged by the anomaly check).",
			schema([]string{"product"}, map[string]any{"product": productProp, "region": regionProp, "trade": tradeProp})),
		def(ToolTopDemand, "Products people look for most this week, in a viloyat or the whole country.",
			schema(nil, map[string]any{"region": regionProp})),
	}
}

// scope is a tool call's product, place and trade, resolved.
type scope struct {
	product  db.GetProductRow
	region   *ask.Region
	province int16 // viloyat of region; 0 = country
	trade    string
}

func (sc scope) params() map[string]any {
	p := map[string]any{"trade": sc.trade}
	if sc.product.ID != 0 {
		p["product"] = sc.product.Slug
	}
	if sc.region != nil {
		p["region"] = sc.region.Name
	}
	return p
}

func (sc scope) placeName() string {
	if sc.region != nil {
		return sc.region.Name
	}
	return ""
}

func tradeOf(t string) string {
	if strings.EqualFold(strings.TrimSpace(t), "chakana") || strings.Contains(strings.ToLower(t), "retail") {
		return "chakana"
	}
	return "ulgurji"
}

// resolveProduct finds the catalog product a text means: the parser's
// dictionary, then the catalog matcher, then the slug itself.
func (s *Service) resolveProduct(ctx context.Context, text string) (db.GetProductRow, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return db.GetProductRow{}, errUnknownProduct
	}
	if d, problem := s.prices.ParseDemand(text); problem == nil || problem.Code != "no_product" {
		if d.ProductID != 0 {
			return s.q.GetProduct(ctx, d.ProductID)
		}
	}
	if s.matcher != nil {
		best, _, err := s.matcher.MatchProduct(ctx, text)
		if err != nil {
			return db.GetProductRow{}, err
		}
		if best != nil {
			return s.q.GetProduct(ctx, best.ProductID)
		}
	}
	p, err := s.q.GetProductBySlug(ctx, strings.ToLower(text))
	if errors.Is(err, pgx.ErrNoRows) {
		return db.GetProductRow{}, errUnknownProduct
	}
	if err != nil {
		return db.GetProductRow{}, err
	}
	return db.GetProductRow{ID: p.ID, Slug: p.Slug, Name: p.Name, Category: p.Category, BaseUnit: p.BaseUnit}, nil
}

func (s *Service) scopeOf(ctx context.Context, a args, needProduct bool) (scope, error) {
	sc := scope{trade: tradeOf(a.Trade)}
	if needProduct {
		p, err := s.resolveProduct(ctx, a.Product)
		if err != nil {
			return sc, err
		}
		sc.product = p
	}
	if a.Region != "" {
		if r := s.ask.FindRegion(a.Region); r != nil {
			sc.region, sc.province = r, r.Province
		}
	}
	return sc, nil
}

// run executes one tool call. A call that cannot run (unknown product, bad
// arguments) gives a result with an error for the model to see.
func (s *Service) run(ctx context.Context, name, raw string) (result, error) {
	var a args
	if strings.TrimSpace(raw) != "" {
		if err := json.Unmarshal([]byte(raw), &a); err != nil {
			return failed(name, "bad_arguments"), nil
		}
	}
	sc, err := s.scopeOf(ctx, a, name != ToolTopDemand)
	if errors.Is(err, errUnknownProduct) {
		return failed(name, "unknown_product"), nil
	}
	if err != nil {
		return result{}, err
	}
	var res result
	switch name {
	case ToolPriceStats:
		if a.DateFrom != "" || a.DateTo != "" {
			res, err = s.priceStatsRange(ctx, sc, a)
		} else {
			res, err = s.priceStats(ctx, sc)
		}
	case ToolPriceTrend:
		res, err = s.priceTrend(ctx, sc, a.Weeks)
	case ToolCheapestOffers:
		res, err = s.cheapestOffers(ctx, sc, a.Quantity)
	case ToolCompareRegions:
		res, err = s.compareRegions(ctx, sc, a.Regions)
	case ToolPressure, ToolForecast, ToolAnomalies, ToolTopDemand:
		return s.runScoped(ctx, name, sc)
	default:
		return failed(name, "bad_arguments"), nil
	}
	res.productID, res.region = sc.product.ID, sc.region
	return res, err
}

func failed(tool, code string) result {
	return result{data: map[string]any{"error": code}, source: Source{Tool: tool, Params: map[string]any{}, Error: code},
		summary: func(lang string) string { return tr(lang, "no_product") }}
}

func dateStr(t time.Time) string { return t.In(tz.Tashkent).Format(time.DateOnly) }

func round2(x float64) float64 { return math.Round(x*100) / 100 }

// priceStats is the live price now (narx_statistikasi without dates).
func (s *Service) priceStats(ctx context.Context, sc scope) (result, error) {
	st, err := s.ask.Stats(ctx, sc.product.ID, sc.trade, sc.region)
	if err != nil {
		return result{}, err
	}
	return s.statsResult(ctx, sc, st)
}

func (s *Service) statsResult(ctx context.Context, sc scope, st ask.Stats) (result, error) {
	var scopeID pgtype.Int2
	switch {
	case st.Scope == "":
	case sc.region != nil && st.Scope == sc.region.Name:
		scopeID = pgtype.Int2{Int16: sc.region.ID, Valid: true}
	case sc.region != nil:
		scopeID = pgtype.Int2{Int16: sc.region.Province, Valid: true}
	}
	demo, err := s.q.AnalystLiveDemoShare(ctx, db.AnalystLiveDemoShareParams{ProductID: sc.product.ID, TradeType: sc.trade, RegionID: scopeID})
	if err != nil {
		return result{}, err
	}
	p, unit := sc.product, sc.product.BaseUnit
	data := map[string]any{"product": p.Name, "unit": unit, "trade": sc.trade, "area": place("uz", st.Scope),
		"observations": st.Offers, "enough": st.Enough, "demo_share_pct": math.Round(demo * 100)}
	src := Source{Tool: ToolPriceStats, Params: sc.params(), Observations: int(st.Offers), DemoShare: round2(demo), Enough: st.Enough}
	res := result{data: data, source: src}
	widened := sc.region != nil && st.Scope != sc.region.Name
	if widened {
		data["note"] = "few prices in " + sc.region.Name + ", a wider area was used"
	}
	if !st.Enough {
		res.summary = func(lang string) string {
			return tr(lang, "insufficient", p.Name+" ("+place(lang, st.Scope)+")", st.Offers)
		}
		return res, nil
	}
	data["median"], data["p25"], data["p75"] = math.Round(st.Median), math.Round(st.P25), math.Round(st.P75)
	data["min"], data["max"] = st.Min, st.Max
	data["from"], data["to"] = dateStr(st.From), dateStr(st.To)
	res.source.DataFrom, res.source.DataTo = dateStr(st.From), dateStr(st.To)
	res.table = &Table{Columns: []string{"area", "median", "p25", "p75", "min", "max", "observations"},
		Rows: [][]any{{place("uz", st.Scope), math.Round(st.Median), math.Round(st.P25), math.Round(st.P75), st.Min, st.Max, st.Offers}}}
	res.summary = func(lang string) string {
		text := tr(lang, "stats", p.Name, trades(lang, sc.trade), place(lang, st.Scope), som(st.Median), unit, som(st.P25), som(st.P75),
			som(float64(st.Min)), som(float64(st.Max)), st.Offers, dateStr(st.From), dateStr(st.To))
		if widened {
			text = tr(lang, "widened", sc.region.Name) + " " + text
		}
		return withDemo(lang, text, demo)
	}
	return res, nil
}

func withDemo(lang, text string, demo float64) string {
	if demo > 0 {
		return text + " " + tr(lang, "demo", pct(demo))
	}
	return text
}

// maxRangeDays bounds a past period.
const maxRangeDays = 3 * 366

// priceStatsRange is narx_statistikasi over a past period, from the daily
// price history (the same rows as the charts).
func (s *Service) priceStatsRange(ctx context.Context, sc scope, a args) (result, error) {
	today := insight.Day(time.Now()).Time
	from, to := today.AddDate(0, 0, -29), today
	var err error
	if a.DateFrom != "" {
		if from, err = time.Parse(time.DateOnly, a.DateFrom); err != nil {
			return failed(ToolPriceStats, "bad_arguments"), nil
		}
	}
	if a.DateTo != "" {
		if to, err = time.Parse(time.DateOnly, a.DateTo); err != nil {
			return failed(ToolPriceStats, "bad_arguments"), nil
		}
	}
	if to.After(today) {
		to = today
	}
	if from.After(to) || to.Sub(from) > maxRangeDays*24*time.Hour {
		return failed(ToolPriceStats, "bad_arguments"), nil
	}
	rows, err := s.q.PriceSeries(ctx, db.PriceSeriesParams{
		Bucket: "day", ProductID: sc.product.ID, TradeType: sc.trade, ProvinceID: pgtype.Int2{Int16: sc.province, Valid: sc.province != 0},
		FromDay: pgtype.Date{Time: from, Valid: true}, ToDay: pgtype.Date{Time: to, Valid: true},
	})
	if err != nil {
		return result{}, err
	}
	var sum, offers, demo float64
	var lo, hi int64
	for i, r := range rows {
		sum += r.Median * float64(r.Offers)
		demo += r.DemoShare * float64(r.Offers)
		offers += float64(r.Offers)
		if i == 0 || r.MinPrice < lo {
			lo = r.MinPrice
		}
		if r.MaxPrice > hi {
			hi = r.MaxPrice
		}
	}
	params := sc.params()
	params["date_from"], params["date_to"] = from.Format(time.DateOnly), to.Format(time.DateOnly)
	area := ""
	if sc.province != 0 {
		if r, err := s.q.GetRegion(ctx, sc.province); err == nil {
			area = r.Name
		}
	}
	n := int(offers)
	enough := n >= analytics.MinObservations
	src := Source{Tool: ToolPriceStats, Params: params, DataFrom: params["date_from"].(string), DataTo: params["date_to"].(string), Observations: n, Enough: enough}
	p := sc.product
	data := map[string]any{"product": p.Name, "unit": p.BaseUnit, "trade": sc.trade, "area": place("uz", area),
		"from": src.DataFrom, "to": src.DataTo, "observations": n, "enough": enough}
	res := result{data: data, source: src}
	if !enough {
		res.summary = func(lang string) string { return tr(lang, "insufficient", p.Name+" ("+place(lang, area)+")", n) }
		return res, nil
	}
	median := sum / offers
	share := demo / offers
	res.source.DemoShare = round2(share)
	data["median"], data["min"], data["max"], data["demo_share_pct"] = math.Round(median), lo, hi, math.Round(share*100)
	res.table = &Table{Columns: []string{"area", "from", "to", "median", "min", "max", "observations"},
		Rows: [][]any{{place("uz", area), src.DataFrom, src.DataTo, math.Round(median), lo, hi, n}}}
	res.summary = func(lang string) string {
		return withDemo(lang, tr(lang, "stats_range", p.Name, trades(lang, sc.trade), place(lang, area), src.DataFrom, src.DataTo,
			som(median), p.BaseUnit, som(float64(lo)), som(float64(hi)), n), share)
	}
	return res, nil
}

// priceTrend is narx_dinamikasi: weekly medians.
func (s *Service) priceTrend(ctx context.Context, sc scope, weeks int) (result, error) {
	if weeks <= 0 {
		weeks = 8
	}
	weeks = min(max(weeks, 2), 52)
	today := insight.Day(time.Now()).Time
	from := today.AddDate(0, 0, -7*weeks+1)
	rows, err := s.q.PriceSeries(ctx, db.PriceSeriesParams{
		Bucket: "week", ProductID: sc.product.ID, TradeType: sc.trade, ProvinceID: pgtype.Int2{Int16: sc.province, Valid: sc.province != 0},
		FromDay: pgtype.Date{Time: from, Valid: true}, ToDay: pgtype.Date{Time: today, Valid: true},
	})
	if err != nil {
		return result{}, err
	}
	params := sc.params()
	params["weeks"] = weeks
	area := s.provinceName(ctx, sc.province)
	p := sc.product
	points := []map[string]any{}
	table := &Table{Columns: []string{"week", "median", "observations", "demo_share"}, Rows: [][]any{}}
	var offers, demo float64
	for _, r := range rows {
		wk := r.Day.Time.Format(time.DateOnly)
		points = append(points, map[string]any{"week": wk, "median": math.Round(r.Median), "observations": r.Offers})
		table.Rows = append(table.Rows, []any{wk, math.Round(r.Median), r.Offers, round2(r.DemoShare)})
		offers += float64(r.Offers)
		demo += r.DemoShare * float64(r.Offers)
	}
	enough := len(rows) >= 2 && offers >= analytics.MinObservations
	src := Source{Tool: ToolPriceTrend, Params: params, Observations: int(offers), Enough: enough}
	if len(rows) > 0 {
		src.DataFrom, src.DataTo = rows[0].Day.Time.Format(time.DateOnly), rows[len(rows)-1].Day.Time.Format(time.DateOnly)
	}
	share := 0.0
	if offers > 0 {
		share = demo / offers
	}
	src.DemoShare = round2(share)
	data := map[string]any{"product": p.Name, "unit": p.BaseUnit, "trade": sc.trade, "area": place("uz", area), "weeks": points, "enough": enough,
		"demo_share_pct": math.Round(share * 100)}
	res := result{data: data, source: src, table: table}
	if !enough {
		res.summary = func(lang string) string {
			return tr(lang, "insufficient", p.Name+" ("+place(lang, area)+")", int(offers))
		}
		return res, nil
	}
	first, last := rows[0].Median, rows[len(rows)-1].Median
	change := last/first - 1
	data["change_pct"] = math.Round(change * 100)
	dir := "flat"
	if change > 0.02 {
		dir = "up"
	} else if change < -0.02 {
		dir = "down"
	}
	data["direction"] = dir
	res.summary = func(lang string) string {
		return withDemo(lang, tr(lang, "trend", p.Name, trades(lang, sc.trade), place(lang, area), len(rows), som(first), som(last),
			tr(lang, dir)+", "+signedPct(change)), share)
	}
	return res, nil
}

func signedPct(x float64) string {
	if x > 0 {
		return "+" + pct(x)
	}
	return pct(x)
}

func (s *Service) provinceName(ctx context.Context, province int16) string {
	if province == 0 {
		return ""
	}
	if r, err := s.q.GetRegion(ctx, province); err == nil {
		return r.Name
	}
	return ""
}

const maxOffers = 5

// cheapestOffers is eng_arzon_takliflar.
func (s *Service) cheapestOffers(ctx context.Context, sc scope, quantity float64) (result, error) {
	params := sc.params()
	var qty pgtype.Numeric
	if quantity > 0 {
		params["quantity"] = quantity
		if err := qty.Scan(fmt.Sprintf("%g", quantity)); err != nil {
			return failed(ToolCheapestOffers, "bad_arguments"), nil
		}
	}
	var region pgtype.Int2
	if sc.region != nil {
		region = pgtype.Int2{Int16: sc.region.ID, Valid: true}
	}
	rows, err := s.q.AnalystOffers(ctx, db.AnalystOffersParams{ProductID: sc.product.ID, TradeType: sc.trade, RegionID: region, Quantity: qty, MaxRows: maxOffers})
	if err != nil {
		return result{}, err
	}
	p := sc.product
	offers := []map[string]any{}
	table := &Table{Columns: []string{"seller", "place", "price", "unit", "min_order", "stock", "updated"}, Rows: [][]any{}}
	var demo float64
	var newest, oldest time.Time
	for i, r := range rows {
		where := r.RegionName
		if r.MarketName != "" {
			where = r.MarketName + ", " + r.RegionName
		}
		o := map[string]any{"seller": r.SellerName, "place": where, "price": r.PriceSom, "unit": r.Unit, "updated": dateStr(r.UpdatedAt)}
		row := []any{r.SellerName, where, r.PriceSom, r.Unit, nil, nil, dateStr(r.UpdatedAt)}
		if v := market.Float(r.MinOrder); v != nil {
			o["min_order"], row[4] = *v, *v
		}
		if v := market.Float(r.Stock); v != nil {
			o["stock"], row[5] = *v, *v
		}
		offers = append(offers, o)
		table.Rows = append(table.Rows, row)
		if r.IsDemo {
			demo++
		}
		if i == 0 || r.UpdatedAt.After(newest) {
			newest = r.UpdatedAt
		}
		if i == 0 || r.UpdatedAt.Before(oldest) {
			oldest = r.UpdatedAt
		}
	}
	share := 0.0
	if len(rows) > 0 {
		share = demo / float64(len(rows))
	}
	src := Source{Tool: ToolCheapestOffers, Params: params, Observations: len(rows), DemoShare: round2(share), Enough: len(rows) > 0}
	if len(rows) > 0 {
		src.DataFrom, src.DataTo = dateStr(oldest), dateStr(newest)
	}
	area := sc.placeName()
	data := map[string]any{"product": p.Name, "trade": sc.trade, "area": place("uz", area), "offers": offers, "enough": len(rows) > 0}
	res := result{data: data, table: table, source: src}
	res.summary = func(lang string) string {
		if len(rows) == 0 {
			return tr(lang, "offers_none", p.Name, trades(lang, sc.trade), place(lang, area))
		}
		lines := []string{tr(lang, "offers", p.Name, trades(lang, sc.trade), place(lang, area))}
		for _, o := range offers {
			lines = append(lines, "• "+tr(lang, "offer", som(float64(o["price"].(int64))), o["unit"], o["seller"], o["place"]))
		}
		return withDemo(lang, strings.Join(lines, "\n"), share)
	}
	return res, nil
}

// pressure is bosim_indeksi: demand, supply, the index and the trend.
func (s *Service) pressure(ctx context.Context, sc scope) (result, error) {
	demand, supply, err := market.PressureWindows(ctx, s.q, sc.product.ID, sc.province, sc.trade)
	if err != nil {
		return result{}, err
	}
	mean := func(xs []float64) float64 {
		sum, n := 0.0, 0
		for _, x := range xs {
			if x > 0 {
				sum += x
				n++
			}
		}
		if n == 0 {
			return 0
		}
		return math.Round(sum/float64(n)*10) / 10
	}
	value, zone, ok := analytics.Pressure(demand[0], demand[1:], supply[0], supply[1:])
	p := sc.product
	area := s.provinceName(ctx, sc.province)
	today := insight.Day(time.Now()).Time
	data := map[string]any{"product": p.Name, "trade": sc.trade, "area": place("uz", area), "demand_now": demand[0], "demand_usual": mean(demand[1:]),
		"sellers_now": supply[0], "sellers_usual": mean(supply[1:]), "enough": ok}
	src := Source{Tool: ToolPressure, Params: sc.params(), DataFrom: today.AddDate(0, 0, -34).Format(time.DateOnly), DataTo: today.Format(time.DateOnly),
		Observations: int(supply[0]), Enough: ok}
	table := &Table{Columns: []string{"demand_now", "demand_usual", "sellers_now", "sellers_usual", "index", "zone"},
		Rows: [][]any{{demand[0], mean(demand[1:]), supply[0], mean(supply[1:]), nil, nil}}}
	if ok {
		value = round2(value)
		data["index"], data["zone"] = value, zone
		table.Rows[0][4], table.Rows[0][5] = value, zone
	}
	if a, err := s.insight.Analytics(ctx, insight.Params{ProductID: p.ID, Province: sc.province, Trade: sc.trade, Period: "30d", Transport: insight.DefaultTransport}); err == nil && a.Trend != nil {
		data["trend"] = a.Trend.Direction
		data["weekly_change_pct"] = math.Round(a.Trend.Weekly * 100)
		src.DemoShare = a.Quality.DemoShare
	}
	res := result{data: data, source: src, table: table}
	res.summary = func(lang string) string {
		if !ok {
			return tr(lang, "pressure_none", p.Name, trades(lang, sc.trade), place(lang, area), trimNum(demand[0]), int(supply[0]))
		}
		return tr(lang, "pressure", p.Name, trades(lang, sc.trade), place(lang, area), trimNum(value), tr(lang, "zone_"+zone),
			trimNum(demand[0]), trimNum(mean(demand[1:])), int(supply[0]), trimNum(mean(supply[1:])))
	}
	return res, nil
}

func trimNum(x float64) string {
	if x == math.Trunc(x) {
		return fmt.Sprintf("%.0f", x)
	}
	return fmt.Sprintf("%.1f", x)
}

// compareRegions is hududlarni_solishtir: live medians by viloyat.
func (s *Service) compareRegions(ctx context.Context, sc scope, names []string) (result, error) {
	var areas []ask.Area
	var err error
	if len(names) == 0 && sc.region != nil && sc.region.Kind != "viloyat" {
		areas, err = s.ask.Areas(ctx, sc.product.ID, sc.trade, sc.region) // tumans of the named place's viloyat
	} else {
		areas, err = s.ask.Areas(ctx, sc.product.ID, sc.trade, nil)
	}
	if err != nil {
		return result{}, err
	}
	params := sc.params()
	if len(names) > 0 {
		want := map[string]bool{}
		var resolved []string
		for _, n := range names {
			if r := s.ask.FindRegion(n); r != nil {
				pv := r.Province
				if reg, err := s.q.GetRegion(ctx, pv); err == nil {
					want[reg.Name] = true
					resolved = append(resolved, reg.Name)
				}
			}
		}
		params["regions"] = resolved
		kept := areas[:0]
		for _, a := range areas {
			if want[a.Name] {
				kept = append(kept, a)
			}
		}
		areas = kept
	}
	sort.SliceStable(areas, func(i, j int) bool { return areas[i].Median < areas[j].Median })
	demo, err := s.q.AnalystLiveDemoShare(ctx, db.AnalystLiveDemoShareParams{ProductID: sc.product.ID, TradeType: sc.trade})
	if err != nil {
		return result{}, err
	}
	p := sc.product
	rows := []map[string]any{}
	table := &Table{Columns: []string{"region", "median", "observations"}, Rows: [][]any{}}
	total := 0
	for _, a := range areas {
		rows = append(rows, map[string]any{"region": a.Name, "median": math.Round(a.Median), "observations": a.Offers})
		table.Rows = append(table.Rows, []any{a.Name, math.Round(a.Median), a.Offers})
		total += int(a.Offers)
	}
	src := Source{Tool: ToolCompareRegions, Params: params, Observations: total, DemoShare: round2(demo), Enough: len(areas) > 0}
	today := insight.Day(time.Now()).Time
	src.DataTo = today.Format(time.DateOnly)
	data := map[string]any{"product": p.Name, "unit": p.BaseUnit, "trade": sc.trade, "areas": rows, "enough": len(areas) > 0,
		"note": "each area has at least 3 live prices", "demo_share_pct": math.Round(demo * 100)}
	res := result{data: data, table: table, source: src}
	res.summary = func(lang string) string {
		if len(areas) == 0 {
			return tr(lang, "areas_none", p.Name, trades(lang, sc.trade))
		}
		lines := []string{tr(lang, "areas", p.Name, trades(lang, sc.trade))}
		for _, a := range areas {
			lines = append(lines, "• "+tr(lang, "area", a.Name, som(a.Median), a.Offers))
		}
		return withDemo(lang, strings.Join(lines, "\n"), demo)
	}
	return res, nil
}

func (s *Service) analytics(ctx context.Context, sc scope) (*insight.Analytics, error) {
	return s.insight.Analytics(ctx, insight.Params{ProductID: sc.product.ID, Province: sc.province, Trade: sc.trade, Period: "90d", Transport: insight.DefaultTransport})
}

// forecast is the next weeks' forecast of the analytics page.
func (s *Service) forecast(ctx context.Context, sc scope) (result, error) {
	a, err := s.analytics(ctx, sc)
	if err != nil {
		return result{}, err
	}
	p := sc.product
	area := s.provinceName(ctx, sc.province)
	src := Source{Tool: ToolForecast, Params: sc.params(), DataFrom: a.Scope.From, DataTo: a.Scope.To, DemoShare: a.Quality.HistoryDemoShare}
	data := map[string]any{"product": p.Name, "unit": p.BaseUnit, "trade": sc.trade, "area": place("uz", area)}
	res := result{data: data, source: src}
	f := a.Forecast
	if f == nil {
		data["enough"] = false
		res.summary = func(lang string) string {
			return tr(lang, "forecast_none", p.Name, trades(lang, sc.trade), place(lang, area))
		}
		return res, nil
	}
	res.source.Enough, res.source.Observations, res.source.DemoShare = true, f.Tested, f.DemoShare
	weeks := []map[string]any{}
	table := &Table{Columns: []string{"week", "median", "low", "high"}, Rows: [][]any{}}
	for _, w := range f.Weeks {
		weeks = append(weeks, map[string]any{"week": w.Week, "median": w.Median, "low": w.Low, "high": w.High})
		table.Rows = append(table.Rows, []any{w.Week, w.Median, w.Low, w.High})
	}
	data["enough"], data["method"], data["last_week_price"], data["weeks"] = true, f.Method, f.Base, weeks
	if len(f.MAPE) > 0 {
		data["error_week1_pct"] = math.Round(f.MAPE[0] * 100)
	}
	data["demo_share_pct"] = math.Round(f.DemoShare * 100)
	res.table = table
	res.summary = func(lang string) string {
		lines := []string{tr(lang, "forecast", p.Name, trades(lang, sc.trade), place(lang, area), f.Method)}
		for _, w := range f.Weeks {
			lines = append(lines, "• "+tr(lang, "forecast_week", w.Week, som(float64(w.Median)), som(float64(w.Low)), som(float64(w.High))))
		}
		return withDemo(lang, strings.Join(lines, "\n"), f.DemoShare)
	}
	return res, nil
}

const maxAnomalies = 10

// anomalies are the flagged days of the last 90 days.
func (s *Service) anomalies(ctx context.Context, sc scope) (result, error) {
	a, err := s.analytics(ctx, sc)
	if err != nil {
		return result{}, err
	}
	p := sc.product
	area := s.provinceName(ctx, sc.province)
	list := append([]insight.AnomalyRef(nil), a.Anomalies...)
	sort.SliceStable(list, func(i, j int) bool { return list[i].Date > list[j].Date })
	if len(list) > maxAnomalies {
		list = list[:maxAnomalies]
	}
	days := []map[string]any{}
	table := &Table{Columns: []string{"date", "region", "value", "expected", "layer"}, Rows: [][]any{}}
	for _, x := range list {
		days = append(days, map[string]any{"date": x.Date, "region": x.ProvinceName, "value": x.Value, "expected": x.Reference, "layer": x.Layer})
		table.Rows = append(table.Rows, []any{x.Date, x.ProvinceName, x.Value, x.Reference, x.Layer})
	}
	src := Source{Tool: ToolAnomalies, Params: sc.params(), DataFrom: a.Scope.From, DataTo: a.Scope.To, Observations: len(a.Series),
		DemoShare: a.Quality.DemoShare, Enough: len(a.Series) > 0}
	data := map[string]any{"product": p.Name, "trade": sc.trade, "area": place("uz", area), "days": days, "enough": len(a.Series) > 0,
		"period_from": a.Scope.From, "period_to": a.Scope.To}
	res := result{data: data, table: table, source: src}
	res.summary = func(lang string) string {
		if len(list) == 0 {
			return tr(lang, "anomalies_none", p.Name, trades(lang, sc.trade), place(lang, area))
		}
		lines := []string{tr(lang, "anomalies", p.Name, trades(lang, sc.trade), place(lang, area))}
		for _, x := range list {
			lines = append(lines, "• "+tr(lang, "anomaly", x.Date, x.ProvinceName, som(float64(x.Value)), som(float64(x.Reference))))
		}
		return strings.Join(lines, "\n")
	}
	return res, nil
}

const maxDemand = 5

// topDemand is what people look for most this week.
func (s *Service) topDemand(ctx context.Context, sc scope) (result, error) {
	d, err := s.insight.TopDemand(ctx, sc.province)
	if err != nil {
		return result{}, err
	}
	area := s.provinceName(ctx, sc.province)
	top := d.Top
	if len(top) > maxDemand {
		top = top[:maxDemand]
	}
	items := []map[string]any{}
	table := &Table{Columns: []string{"product", "score", "usual", "median"}, Rows: [][]any{}}
	for _, it := range top {
		m := map[string]any{"product": it.Product.Name, "score": math.Round(it.Score), "usual": math.Round(it.Usual)}
		row := []any{it.Product.Name, math.Round(it.Score), math.Round(it.Usual), nil}
		if it.Median != nil {
			m["median"], row[3] = *it.Median, *it.Median
		}
		items = append(items, m)
		table.Rows = append(table.Rows, row)
	}
	today := insight.Day(time.Now()).Time
	params := map[string]any{}
	if sc.region != nil {
		params["region"] = sc.region.Name
	}
	src := Source{Tool: ToolTopDemand, Params: params, DataFrom: today.AddDate(0, 0, -6).Format(time.DateOnly), DataTo: today.Format(time.DateOnly),
		Observations: len(top), Enough: len(top) > 0}
	data := map[string]any{"area": place("uz", area), "top": items, "enough": len(top) > 0}
	res := result{data: data, table: table, source: src}
	res.summary = func(lang string) string {
		if len(top) == 0 {
			return tr(lang, "demand_none", place(lang, area))
		}
		lines := []string{tr(lang, "demand", place(lang, area))}
		for _, it := range top {
			lines = append(lines, "• "+tr(lang, "demand_item", it.Product.Name, trimNum(math.Round(it.Score))))
		}
		return strings.Join(lines, "\n")
	}
	return res, nil
}
