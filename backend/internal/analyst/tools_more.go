package analyst

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"bugun/internal/insight"
	"bugun/internal/llm"
	"bugun/internal/store/db"
)

// The chat assistant's further functions (CHATBOT_REJA.md §4). Like the first
// eight they only read, run the site's own code and return where their
// numbers come from. Proposals (create_request, watch_price) change nothing:
// they come back as actions the user confirms with a button.

const (
	ToolFindProduct     = "find_product"
	ToolFindRegion      = "find_region"
	ToolAdvice          = "advice"
	ToolSeasonality     = "seasonality"
	ToolWeekdays        = "weekday_prices"
	ToolPriceReasons    = "price_reasons"
	ToolDistrictPrices  = "district_prices"
	ToolWholesaleRetail = "wholesale_retail"
	ToolArbitrage       = "arbitrage"
	ToolRegionOverview  = "region_overview"
	ToolWeeklyMovers    = "weekly_movers"
	ToolCompareProducts = "compare_products"
	ToolBasketCost      = "basket_cost"
	ToolExchangeRate    = "exchange_rate"
	ToolWeather         = "weather"
	ToolMarketEvents    = "market_events"
	ToolSellerInfo      = "seller_info"
	ToolPlatformHelp    = "platform_help"
	ToolCreateRequest   = "create_request"
	ToolWatchPrice      = "watch_price"
	// Control: they end the turn instead of fetching data.
	ToolAskUser      = "ask_user"
	ToolCannotAnswer = "cannot_answer"
)

// Action is a change the assistant proposes; the client runs it with the
// user's confirmation through the usual endpoint (POST /v1/rfqs, POST /v1/watches).
type Action struct {
	Type   string         `json:"type" enum:"create_request,watch_price"`
	Label  string         `json:"label" doc:"Button text in the answer's language"`
	Params map[string]any `json:"params" doc:"create_request: product_id, product, quantity, unit, region_id, days; watch_price: product_id, province_id, trade"`
}

// moreArgs are the further tools' arguments.
type moreArgs struct {
	Name     string       `json:"name"`
	Products []string     `json:"products"`
	Period   string       `json:"period"`
	Category string       `json:"category"`
	Items    []basketItem `json:"items"`
	Currency string       `json:"currency"`
	Seller   string       `json:"seller"`
	Topic    string       `json:"topic"`
	Days     int          `json:"days"`
	Unit     string       `json:"unit"`
	Question string       `json:"question"`
	Options  []string     `json:"options"`
	Reason   string       `json:"reason"`
}

type basketItem struct {
	Product  string  `json:"product"`
	Quantity float64 `json:"quantity"`
	Unit     string  `json:"unit"`
}

var categoryProp = map[string]any{"type": "string", "enum": []string{"sabzavot", "meva", "gosht", "don", "oziq_ovqat", "qurilish", "xizmat"}, "description": "Category (optional)"}

// moreToolDefs are the further functions offered to the model.
func moreToolDefs() map[string]llm.Tool {
	def := func(name, desc string, params map[string]any) llm.Tool {
		return llm.Tool{Type: "function", Function: llm.ToolFunction{Name: name, Description: desc, Parameters: params}}
	}
	p := map[string]any{"product": productProp, "region": regionProp, "trade": tradeProp}
	return map[string]llm.Tool{
		ToolFindProduct: def(ToolFindProduct, "Look a product word up in the catalog; several candidates when it is ambiguous (e.g. qalampir). Use before asking the user which one.",
			schema([]string{"name"}, map[string]any{"name": strProp("The word as written")})),
		ToolFindRegion: def(ToolFindRegion, "Look a place name up: viloyat, city or tuman, and its viloyat.",
			schema([]string{"name"}, map[string]any{"name": strProp("The place as written")})),
		ToolAdvice: def(ToolAdvice, "Buy now or wait (buyer), sell now or hold (seller), with the reasons: trend, season, demand, level against the year, holidays.",
			schema([]string{"product"}, p)),
		ToolSeasonality: def(ToolSeasonality, "How a product's price usually moves through the year: cheapest and dearest month and the monthly index.",
			schema([]string{"product"}, p)),
		ToolWeekdays: def(ToolWeekdays, "Which day of the week a product is usually cheapest and dearest.",
			schema([]string{"product"}, p)),
		ToolPriceReasons: def(ToolPriceReasons, "Why a price moved: weeks with sharp changes and the events around them, and the reasons sellers gave.",
			schema([]string{"product"}, p)),
		ToolDistrictPrices: def(ToolDistrictPrices, "Prices of a product by tuman and bazaar inside one viloyat.",
			schema([]string{"product", "region"}, p)),
		ToolWholesaleRetail: def(ToolWholesaleRetail, "How much dearer retail is than wholesale for a product.",
			schema([]string{"product"}, map[string]any{"product": productProp, "region": regionProp})),
		ToolArbitrage: def(ToolArbitrage, "Where buying and selling a product pays after the road cost, between viloyatlar.",
			schema([]string{"product"}, p)),
		ToolRegionOverview: def(ToolRegionOverview, "One viloyat's market: sellers, listings, bazaars, food price index, shortages, surpluses, similar viloyatlar.",
			schema([]string{"region"}, map[string]any{"region": regionProp, "trade": tradeProp})),
		ToolWeeklyMovers: def(ToolWeeklyMovers, "Products whose country price rose or fell most this week.",
			schema(nil, map[string]any{"category": categoryProp, "trade": tradeProp})),
		ToolCompareProducts: def(ToolCompareProducts, "Compare 2 to 4 products over a period: price now and change.",
			schema([]string{"products"}, map[string]any{"products": map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Products (2–4)"},
				"region": regionProp, "trade": tradeProp, "period": map[string]any{"type": "string", "enum": []string{"30d", "90d", "1y"}, "description": "Default 90d"}})),
		ToolBasketCost: def(ToolBasketCost, "Cost of a shopping list or a recipe's ingredients: each item's price and cost, and the total. Quantities are the user's, or typical amounts you propose and say are estimates.",
			schema([]string{"items"}, map[string]any{"region": regionProp, "trade": map[string]any{"type": "string", "enum": []string{"ulgurji", "chakana"}, "description": "chakana (retail, default here) or ulgurji"},
				"items": map[string]any{"type": "array", "description": "Up to 15 items", "items": map[string]any{"type": "object", "additionalProperties": false,
					"required": []string{"product", "quantity"},
					"properties": map[string]any{"product": productProp, "quantity": map[string]any{"type": "number", "description": "Amount"},
						"unit": strProp("kg, gramm, dona, litr, qop… (default: the product's unit)")}}}})),
		ToolExchangeRate: def(ToolExchangeRate, "Central Bank so'm exchange rate and its change over 30 days.",
			schema(nil, map[string]any{"currency": map[string]any{"type": "string", "enum": []string{"USD", "RUB", "KZT"}, "description": "Default USD"}})),
		ToolWeather: def(ToolWeather, "Weather of the last 7 days in a viloyat (or the country) and frost or heat days of the last 2 weeks, which move vegetable and fruit prices.",
			schema(nil, map[string]any{"region": regionProp})),
		ToolMarketEvents: def(ToolMarketEvents, "Recent and coming market events and holidays that touch a product or everything.",
			schema(nil, map[string]any{"product": productProp, "region": regionProp})),
		ToolSellerInfo: def(ToolSellerInfo, "A seller's public page: where, live listings, rating.",
			schema([]string{"seller"}, map[string]any{"seller": strProp("The seller's name as written")})),
		ToolPlatformHelp: def(ToolPlatformHelp, "How to use Bugun: posting prices, automatic pricing, requests (RFQ), price alerts, Telegram channels, account.",
			schema([]string{"topic"}, map[string]any{"topic": map[string]any{"type": "string", "enum": helpTopics, "description": "Topic"}})),
		ToolCreateRequest: def(ToolCreateRequest, "Propose a buyer request (RFQ) to sellers; the user confirms it with a button. Use only when the user asks to buy or to get offers.",
			schema([]string{"product", "quantity"}, map[string]any{"product": productProp, "region": regionProp,
				"quantity": map[string]any{"type": "number", "description": "Amount wanted"}, "unit": strProp("Unit (default: the product's)"),
				"days": map[string]any{"type": "integer", "minimum": 1, "maximum": 7, "description": "How long it stays open (default 3)"}})),
		ToolWatchPrice: def(ToolWatchPrice, "Propose a price alert (message when it gets cheaper); the user confirms it with a button.",
			schema([]string{"product"}, p)),
		ToolAskUser: def(ToolAskUser, "Ask the user one short question when the product, place or amount is unclear and a default would mislead. Give up to 5 options when you can. Never put prices or other numbers in it.",
			schema([]string{"question"}, map[string]any{"question": strProp("The question, in the answer's language"),
				"options": map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Short options for buttons (optional)"}})),
		ToolCannotAnswer: def(ToolCannotAnswer, "Say you cannot answer: the question is not about markets and prices, or no function has the data. The admins see these questions.",
			schema([]string{"reason"}, map[string]any{"reason": map[string]any{"type": "string", "enum": []string{"off_topic", "no_data", "no_tool"}},
				"topic": strProp("What the question was about, in a few words")})),
	}
}

func (s *Service) runMore(ctx context.Context, name, raw string, lang string) (result, error) {
	var a args
	var m moreArgs
	if strings.TrimSpace(raw) != "" {
		if json.Unmarshal([]byte(raw), &a) != nil || json.Unmarshal([]byte(raw), &m) != nil {
			return failed(name, "bad_arguments"), nil
		}
	}
	switch name {
	case ToolFindProduct:
		return s.findProduct(ctx, m.Name)
	case ToolFindRegion:
		return s.findRegion(ctx, m.Name), nil
	case ToolWeeklyMovers:
		return s.weeklyMovers(ctx, tradeOf(a.Trade), m.Category)
	case ToolBasketCost:
		trade := "chakana"
		if strings.EqualFold(a.Trade, "ulgurji") {
			trade = "ulgurji"
		}
		return s.basketCost(ctx, m.Items, a.Region, trade)
	case ToolExchangeRate:
		return s.exchangeRate(ctx, m.Currency)
	case ToolWeather:
		return s.weather(ctx, a.Region)
	case ToolSellerInfo:
		return s.sellerInfo(ctx, m.Seller)
	case ToolPlatformHelp:
		return platformHelp(m.Topic, lang), nil
	case ToolCompareProducts:
		return s.compareProducts(ctx, m.Products, a.Region, tradeOf(a.Trade), m.Period)
	case ToolRegionOverview:
		return s.regionOverview(ctx, a.Region, tradeOf(a.Trade))
	case ToolMarketEvents:
		return s.marketEvents(ctx, a.Product, a.Region)
	}
	// The rest are about one product.
	sc, err := s.scopeOf(ctx, a, true)
	if errors.Is(err, errUnknownProduct) {
		return failed(name, "unknown_product"), nil
	}
	if err != nil {
		return result{}, err
	}
	var res result
	switch name {
	case ToolAdvice:
		res, err = s.advice(ctx, sc)
	case ToolSeasonality:
		res, err = s.seasonality(ctx, sc)
	case ToolWeekdays:
		res, err = s.weekdays(ctx, sc)
	case ToolPriceReasons:
		res, err = s.priceReasons(ctx, sc)
	case ToolDistrictPrices:
		if sc.province == 0 {
			return failed(name, "unknown_region"), nil
		}
		res, err = s.districtPrices(ctx, sc)
	case ToolWholesaleRetail:
		res, err = s.wholesaleRetail(ctx, sc)
	case ToolArbitrage:
		res, err = s.arbitrage(ctx, sc)
	case ToolCreateRequest:
		res, err = s.createRequest(ctx, sc, a.Quantity, m.Unit, m.Days, lang)
	case ToolWatchPrice:
		res = watchPrice(sc, lang)
	default:
		return failed(name, "bad_arguments"), nil
	}
	res.productID, res.region = sc.product.ID, sc.region
	return res, err
}

// simple builds a result whose summary lists the data's lines.
func simple(tool string, params map[string]any, data map[string]any, enough bool, lines func(lang string) []string) result {
	return result{data: data, source: Source{Tool: tool, Params: params, Enough: enough}, summary: func(lang string) string {
		return strings.Join(lines(lang), "\n")
	}}
}

func (s *Service) findProduct(ctx context.Context, name string) (result, error) {
	params := map[string]any{"name": name}
	var matches []map[string]any
	seen := map[int16]bool{}
	add := func(id int16) error {
		if seen[id] {
			return nil
		}
		p, err := s.q.GetProduct(ctx, id)
		if err != nil {
			return err
		}
		seen[id] = true
		matches = append(matches, map[string]any{"slug": p.Slug, "name": p.Name, "unit": p.BaseUnit, "category": p.Category})
		return nil
	}
	if d, problem := s.prices.ParseDemand(name); (problem == nil || problem.Code != "no_product") && d.ProductID != 0 {
		if err := add(d.ProductID); err != nil {
			return result{}, err
		}
	}
	if s.matcher != nil && strings.TrimSpace(name) != "" {
		_, cands, err := s.matcher.MatchProduct(ctx, name)
		if err != nil {
			return result{}, err
		}
		for _, c := range cands {
			if len(matches) >= 5 {
				break
			}
			if err := add(c.ProductID); err != nil {
				return result{}, err
			}
		}
	}
	data := map[string]any{"matches": matches, "enough": len(matches) > 0}
	if len(matches) > 1 {
		data["note"] = "several products match; ask the user which one"
	}
	return simple(ToolFindProduct, params, data, len(matches) > 0, func(lang string) []string {
		if len(matches) == 0 {
			return []string{tr(lang, "no_product")}
		}
		names := make([]string, len(matches))
		for i, m := range matches {
			names[i] = m["name"].(string)
		}
		return []string{tr(lang, "found_products", strings.Join(names, ", "))}
	}), nil
}

func (s *Service) findRegion(_ context.Context, name string) result {
	params := map[string]any{"name": name}
	r := s.ask.FindRegion(name)
	if r == nil {
		return failed(ToolFindRegion, "unknown_region")
	}
	data := map[string]any{"name": r.Name, "kind": r.Kind, "enough": true}
	return simple(ToolFindRegion, params, data, true, func(lang string) []string { return []string{r.Name} })
}

func (s *Service) advice(ctx context.Context, sc scope) (result, error) {
	a, err := s.analytics(ctx, sc)
	if err != nil {
		return result{}, err
	}
	p, area := sc.product, s.provinceName(ctx, sc.province)
	adv := a.Advice
	data := map[string]any{"product": p.Name, "trade": sc.trade, "area": place("uz", area), "buyer": adv.Buyer, "seller": adv.Seller,
		"reasons": adv.Reasons, "enough": len(adv.Reasons) > 0}
	res := simple(ToolAdvice, sc.params(), data, len(adv.Reasons) > 0, func(lang string) []string {
		lines := []string{tr(lang, "advice", p.Name, place(lang, area), tr(lang, "adv_"+adv.Buyer))}
		for _, r := range adv.Reasons {
			lines = append(lines, "• "+r)
		}
		return lines
	})
	res.source.DemoShare = a.Quality.DemoShare
	return res, nil
}

var monthKeys = []string{"jan", "feb", "mar", "apr", "may", "jun", "jul", "aug", "sep", "oct", "nov", "dec"}

func (s *Service) seasonality(ctx context.Context, sc scope) (result, error) {
	a, err := s.analytics(ctx, sc)
	if err != nil {
		return result{}, err
	}
	p, area := sc.product, s.provinceName(ctx, sc.province)
	sn := a.Seasonality
	if sn == nil {
		data := map[string]any{"product": p.Name, "area": place("uz", area), "enough": false}
		return simple(ToolSeasonality, sc.params(), data, false, func(lang string) []string {
			return []string{tr(lang, "season_none", p.Name, place(lang, area))}
		}), nil
	}
	months := map[string]float64{}
	for i, v := range sn.Months {
		months[monthKeys[i]] = v
	}
	data := map[string]any{"product": p.Name, "area": place("uz", area), "cheapest_month": sn.CheapestMonth, "dearest_month": sn.DearestMonth,
		"index_by_month": months, "note": "1.12 = usually 12% above the yearly level", "enough": true,
		"cheapest_pct_below_year": math.Round((1 - sn.Months[sn.CheapestMonth-1]) * 100), "dearest_pct_above_year": math.Round((sn.Months[sn.DearestMonth-1] - 1) * 100)}
	res := simple(ToolSeasonality, sc.params(), data, true, func(lang string) []string {
		return []string{tr(lang, "season", p.Name, place(lang, area), monthName(lang, sn.CheapestMonth), pct(1-sn.Months[sn.CheapestMonth-1]),
			monthName(lang, sn.DearestMonth), pct(sn.Months[sn.DearestMonth-1]-1))}
	})
	res.source.DemoShare = a.Quality.HistoryDemoShare
	return res, nil
}

func (s *Service) weekdays(ctx context.Context, sc scope) (result, error) {
	a, err := s.analytics(ctx, sc)
	if err != nil {
		return result{}, err
	}
	p, area := sc.product, s.provinceName(ctx, sc.province)
	w := a.Weekdays
	if w == nil || w.Spread < 0.01 {
		data := map[string]any{"product": p.Name, "area": place("uz", area), "enough": w != nil, "pattern": false}
		return simple(ToolWeekdays, sc.params(), data, w != nil, func(lang string) []string {
			return []string{tr(lang, "weekdays_none", p.Name, place(lang, area))}
		}), nil
	}
	data := map[string]any{"product": p.Name, "area": place("uz", area), "cheapest_day": w.CheapestDay, "dearest_day": w.DearestDay,
		"spread_pct": math.Round(w.Spread * 100), "weeks": w.WeeksCounted, "enough": true, "note": "day 1 = Monday"}
	return simple(ToolWeekdays, sc.params(), data, true, func(lang string) []string {
		return []string{tr(lang, "weekdays", p.Name, place(lang, area), dayName(lang, w.CheapestDay), dayName(lang, w.DearestDay), pct(w.Spread))}
	}), nil
}

func (s *Service) priceReasons(ctx context.Context, sc scope) (result, error) {
	a, err := s.analytics(ctx, sc)
	if err != nil {
		return result{}, err
	}
	p, area := sc.product, s.provinceName(ctx, sc.province)
	jumps := []map[string]any{}
	for _, j := range a.Jumps {
		ev := []string{}
		for _, e := range j.Events {
			ev = append(ev, e.Title)
		}
		jumps = append(jumps, map[string]any{"week": j.Week, "change_pct": math.Round(j.Change * 100), "events": ev})
	}
	reasons := []map[string]any{}
	for _, r := range a.Reasons {
		reasons = append(reasons, map[string]any{"reason": r.Label, "direction": r.Direction, "sellers": r.Count})
	}
	enough := len(jumps) > 0 || len(reasons) > 0 || a.Trend != nil
	data := map[string]any{"product": p.Name, "area": place("uz", area), "sharp_weeks": jumps, "seller_reasons": reasons, "enough": enough}
	if a.Trend != nil {
		data["weekly_change_pct"] = math.Round(a.Trend.Weekly * 100)
	}
	res := simple(ToolPriceReasons, sc.params(), data, enough, func(lang string) []string {
		lines := []string{tr(lang, "reasons_head", p.Name, place(lang, area))}
		for _, j := range a.Jumps {
			line := "• " + j.Week + ": " + signedPct(j.Change)
			for _, e := range j.Events {
				line += " — " + e.Title
			}
			lines = append(lines, line)
		}
		for _, r := range a.Reasons {
			lines = append(lines, "• "+tr(lang, "seller_reason", r.Label, r.Count))
		}
		if len(lines) == 1 {
			lines = append(lines, tr(lang, "reasons_none"))
		}
		return lines
	})
	res.source.DemoShare = a.Quality.DemoShare
	return res, nil
}

func areaRows(list []insight.AreaPrice) ([]map[string]any, [][]any) {
	rows, table := []map[string]any{}, [][]any{}
	for _, x := range list {
		rows = append(rows, map[string]any{"name": x.Name, "median": x.Median, "prices": x.Offers})
		table = append(table, []any{x.Name, x.Median, x.Offers})
	}
	return rows, table
}

func (s *Service) districtPrices(ctx context.Context, sc scope) (result, error) {
	a, err := s.analytics(ctx, sc)
	if err != nil {
		return result{}, err
	}
	p, area := sc.product, s.provinceName(ctx, sc.province)
	districts, dt := areaRows(a.Districts)
	markets, mt := areaRows(a.Markets)
	enough := len(districts) > 0 || len(markets) > 0
	data := map[string]any{"product": p.Name, "unit": p.BaseUnit, "trade": sc.trade, "area": area, "districts": districts, "bazaars": markets, "enough": enough}
	res := simple(ToolDistrictPrices, sc.params(), data, enough, func(lang string) []string {
		if !enough {
			return []string{tr(lang, "districts_none", p.Name, area)}
		}
		lines := []string{tr(lang, "districts", p.Name, area)}
		for _, x := range append(append([]insight.AreaPrice{}, a.Districts...), a.Markets...) {
			lines = append(lines, "• "+tr(lang, "area", x.Name, som(float64(x.Median)), x.Offers))
		}
		return lines
	})
	res.table = &Table{Columns: []string{"area", "median", "observations"}, Rows: append(dt, mt...)}
	return res, nil
}

func (s *Service) wholesaleRetail(ctx context.Context, sc scope) (result, error) {
	a, err := s.analytics(ctx, sc)
	if err != nil {
		return result{}, err
	}
	p, area := sc.product, s.provinceName(ctx, sc.province)
	m := a.RetailMarkup
	data := map[string]any{"product": p.Name, "area": place("uz", area), "enough": m != nil}
	if m != nil {
		data["retail_over_wholesale_pct"] = math.Round(*m * 100)
	}
	return simple(ToolWholesaleRetail, sc.params(), data, m != nil, func(lang string) []string {
		if m == nil {
			return []string{tr(lang, "markup_none", p.Name, place(lang, area))}
		}
		return []string{tr(lang, "markup", p.Name, place(lang, area), pct(*m))}
	}), nil
}

func (s *Service) arbitrage(ctx context.Context, sc scope) (result, error) {
	a, err := s.analytics(ctx, sc)
	if err != nil {
		return result{}, err
	}
	p := sc.product
	list := a.Arbitrage
	if len(list) > 3 {
		list = list[:3]
	}
	pairs := []map[string]any{}
	for _, x := range list {
		pairs = append(pairs, map[string]any{"buy_in": x.Buy, "buy_price": x.BuyPrice, "sell_in": x.Sell, "sell_price": x.SellPrice,
			"road_km": x.DistanceKm, "road_cost": x.Transport, "net_per_unit": x.Net})
	}
	data := map[string]any{"product": p.Name, "unit": p.BaseUnit, "trade": sc.trade, "pairs": pairs, "road_rate_per_100km": a.Transport, "enough": len(pairs) > 0}
	return simple(ToolArbitrage, sc.params(), data, len(pairs) > 0, func(lang string) []string {
		if len(list) == 0 {
			return []string{tr(lang, "arbitrage_none", p.Name)}
		}
		lines := []string{tr(lang, "arbitrage", p.Name)}
		for _, x := range list {
			lines = append(lines, "• "+tr(lang, "arbitrage_pair", x.Buy, som(float64(x.BuyPrice)), x.Sell, som(float64(x.SellPrice)), som(float64(x.Net))))
		}
		return lines
	}), nil
}

func (s *Service) regionOverview(ctx context.Context, region, trade string) (result, error) {
	r := s.ask.FindRegion(region)
	if r == nil {
		return failed(ToolRegionOverview, "unknown_region"), nil
	}
	o, err := s.insight.RegionOverview(ctx, r.Province, trade)
	if err != nil {
		return result{}, err
	}
	names := func(list []insight.RegionProduct) []string {
		out := []string{}
		for i, x := range list {
			if i == 5 {
				break
			}
			out = append(out, x.Name)
		}
		return out
	}
	similar := []string{}
	for _, x := range o.Similar {
		similar = append(similar, x.Province.Name)
	}
	data := map[string]any{"region": o.ProvinceName, "trade": trade, "sellers": o.Passport.Sellers, "listings": o.Passport.Listings,
		"bazaars": o.Passport.Markets, "products_priced": len(o.Products), "shortages": names(o.Shortages), "surpluses": names(o.Surpluses),
		"similar_regions": similar, "demo_share_pct": math.Round(o.DemoShare * 100), "enough": len(o.Products) > 0}
	if n := len(o.Index); n > 1 {
		data["food_index_now"] = math.Round(o.Index[n-1].Region*10) / 10
		data["food_index_note"] = "first week of the year = 100"
	}
	res := simple(ToolRegionOverview, map[string]any{"region": o.ProvinceName, "trade": trade}, data, len(o.Products) > 0, func(lang string) []string {
		lines := []string{tr(lang, "region", o.ProvinceName, o.Passport.Sellers, o.Passport.Listings, o.Passport.Markets)}
		if len(o.Shortages) > 0 {
			lines = append(lines, tr(lang, "region_short", strings.Join(names(o.Shortages), ", ")))
		}
		if len(o.Surpluses) > 0 {
			lines = append(lines, tr(lang, "region_surplus", strings.Join(names(o.Surpluses), ", ")))
		}
		return lines
	})
	res.source.DemoShare = o.DemoShare
	return res, nil
}

func (s *Service) weeklyMovers(ctx context.Context, trade, category string) (result, error) {
	h, err := s.insight.Heatmap(ctx, trade, category)
	if err != nil {
		return result{}, err
	}
	pick := func(list []insight.Mover) ([]map[string]any, []string) {
		out, lines := []map[string]any{}, []string{}
		for i, m := range list {
			if i == 5 {
				break
			}
			out = append(out, map[string]any{"product": m.Product.Name, "price": m.Country, "week_change_pct": math.Round(m.Week * 100)})
			lines = append(lines, m.Product.Name+" "+signedPct(m.Week))
		}
		return out, lines
	}
	up, upLines := pick(h.Up)
	down, downLines := pick(h.Down)
	params := map[string]any{"trade": trade}
	if category != "" {
		params["category"] = category
	}
	enough := len(up)+len(down) > 0
	data := map[string]any{"trade": trade, "rose": up, "fell": down, "enough": enough, "demo_share_pct": math.Round(h.DemoShare * 100)}
	res := simple(ToolWeeklyMovers, params, data, enough, func(lang string) []string {
		if !enough {
			return []string{tr(lang, "movers_none")}
		}
		return []string{tr(lang, "movers_up", strings.Join(upLines, ", ")), tr(lang, "movers_down", strings.Join(downLines, ", "))}
	})
	res.source.DemoShare = h.DemoShare
	return res, nil
}

func (s *Service) compareProducts(ctx context.Context, products []string, region, trade, period string) (result, error) {
	if len(products) < 2 || len(products) > 4 {
		return failed(ToolCompareProducts, "bad_arguments"), nil
	}
	if period != "30d" && period != "1y" {
		period = "90d"
	}
	var province int16
	if r := s.ask.FindRegion(region); r != nil && region != "" {
		province = r.Province
	}
	specs := []insight.LineSpec{}
	for _, name := range products {
		p, err := s.resolveProduct(ctx, name)
		if errors.Is(err, errUnknownProduct) {
			return failed(ToolCompareProducts, "unknown_product"), nil
		}
		if err != nil {
			return result{}, err
		}
		specs = append(specs, insight.LineSpec{ProductID: p.ID, Province: province})
	}
	c, err := s.insight.Compare(ctx, specs, trade, period)
	if err != nil {
		return result{}, err
	}
	lines := []map[string]any{}
	for _, l := range c.Lines {
		m := map[string]any{"product": l.Product.Name, "unit": l.Product.Unit}
		if l.Now != nil {
			m["price_now"] = *l.Now
		}
		if l.Change != nil {
			m["change_pct"] = math.Round(*l.Change * 100)
		}
		lines = append(lines, m)
	}
	params := map[string]any{"products": products, "trade": trade, "period": period}
	data := map[string]any{"period": period, "trade": trade, "lines": lines, "enough": len(lines) > 0, "demo_share_pct": math.Round(c.DemoShare * 100)}
	res := simple(ToolCompareProducts, params, data, len(lines) > 0, func(lang string) []string {
		out := []string{tr(lang, "compare_head", period)}
		for _, l := range c.Lines {
			line := "• " + l.Product.Name
			if l.Now != nil {
				line += ": " + som(float64(*l.Now)) + " " + tr(lang, "som")
			}
			if l.Change != nil {
				line += " (" + signedPct(*l.Change) + ")"
			}
			out = append(out, line)
		}
		return out
	})
	res.source.DemoShare = c.DemoShare
	return res, nil
}

const maxBasketItems = 15

// basketCost prices a shopping list at the live medians (widened like the
// price answers when a place has too few prices).
func (s *Service) basketCost(ctx context.Context, items []basketItem, region, trade string) (result, error) {
	if len(items) == 0 || len(items) > maxBasketItems {
		return failed(ToolBasketCost, "bad_arguments"), nil
	}
	lines := []map[string]any{}
	missing := []string{}
	table := &Table{Columns: []string{"product", "quantity", "unit", "price", "cost", "area"}, Rows: [][]any{}}
	total, priced := 0.0, 0
	for _, it := range items {
		if it.Quantity <= 0 {
			missing = append(missing, it.Product)
			continue
		}
		p, err := s.resolveProduct(ctx, it.Product)
		if errors.Is(err, errUnknownProduct) {
			missing = append(missing, it.Product)
			continue
		}
		if err != nil {
			return result{}, err
		}
		qty, err := s.prices.ToBase(p.ID, it.Unit, it.Quantity)
		if err != nil {
			missing = append(missing, it.Product+" ("+it.Unit+")")
			continue
		}
		sc := scope{product: p, trade: trade}
		if region != "" {
			if r := s.ask.FindRegion(region); r != nil {
				sc.region, sc.province = r, r.Province
			}
		}
		st, err := s.ask.Stats(ctx, p.ID, trade, sc.region)
		if err != nil {
			return result{}, err
		}
		if !st.Enough {
			missing = append(missing, p.Name)
			continue
		}
		cost := math.Round(st.Median * qty)
		total += cost
		priced++
		lines = append(lines, map[string]any{"product": p.Name, "quantity": qty, "unit": p.BaseUnit, "price": math.Round(st.Median),
			"cost": cost, "area": place("uz", st.Scope), "prices": st.Offers})
		table.Rows = append(table.Rows, []any{p.Name, qty, p.BaseUnit, math.Round(st.Median), cost, place("uz", st.Scope)})
	}
	params := map[string]any{"items": items, "trade": trade}
	if region != "" {
		params["region"] = region
	}
	data := map[string]any{"trade": trade, "lines": lines, "total": total, "not_priced": missing, "enough": priced > 0,
		"note": "prices are live medians; items in not_priced are left out of the total"}
	res := simple(ToolBasketCost, params, data, priced > 0, func(lang string) []string {
		out := []string{tr(lang, "basket_head", trades(lang, trade))}
		for _, l := range lines {
			out = append(out, "• "+tr(lang, "basket_line", l["product"], trimNum(l["quantity"].(float64)), l["unit"], som(l["price"].(float64)), som(l["cost"].(float64))))
		}
		out = append(out, tr(lang, "basket_total", som(total)))
		if len(missing) > 0 {
			out = append(out, tr(lang, "basket_missing", strings.Join(missing, ", ")))
		}
		return out
	})
	res.table = table
	res.source.Observations = priced
	return res, nil
}

func (s *Service) exchangeRate(ctx context.Context, ccy string) (result, error) {
	ccy = strings.ToUpper(strings.TrimSpace(ccy))
	if ccy != "RUB" && ccy != "KZT" {
		ccy = "USD"
	}
	today := insight.Day(time.Now()).Time
	rows, err := s.q.FxSeries(ctx, db.FxSeriesParams{Bucket: "day", Ccy: ccy, FromDay: pgtype.Date{Time: today.AddDate(0, 0, -30), Valid: true},
		ToDay: pgtype.Date{Time: today, Valid: true}})
	if err != nil {
		return result{}, err
	}
	params := map[string]any{"currency": ccy}
	if len(rows) == 0 {
		data := map[string]any{"currency": ccy, "enough": false}
		return simple(ToolExchangeRate, params, data, false, func(lang string) []string { return []string{tr(lang, "fx_none")} }), nil
	}
	first, last := rows[0], rows[len(rows)-1]
	change := last.Rate/first.Rate - 1
	data := map[string]any{"currency": ccy, "rate": math.Round(last.Rate*100) / 100, "date": last.Day.Time.Format(time.DateOnly),
		"rate_30_days_ago": math.Round(first.Rate*100) / 100, "change_pct": math.Round(change*1000) / 10, "source": "Central Bank of Uzbekistan", "enough": true}
	res := simple(ToolExchangeRate, params, data, true, func(lang string) []string {
		return []string{tr(lang, "fx", ccy, fmt.Sprintf("%.2f", last.Rate), last.Day.Time.Format(time.DateOnly), signedPct(change))}
	})
	res.source.DataFrom, res.source.DataTo, res.source.Observations = first.Day.Time.Format(time.DateOnly), last.Day.Time.Format(time.DateOnly), len(rows)
	return res, nil
}

func (s *Service) weather(ctx context.Context, region string) (result, error) {
	var prov pgtype.Int2
	area := ""
	if region != "" {
		r := s.ask.FindRegion(region)
		if r == nil {
			return failed(ToolWeather, "unknown_region"), nil
		}
		prov, area = pgtype.Int2{Int16: r.Province, Valid: true}, s.provinceName(ctx, r.Province)
	}
	today := insight.Day(time.Now()).Time
	days, err := s.q.WeatherSeries(ctx, db.WeatherSeriesParams{Bucket: "day", FromDay: pgtype.Date{Time: today.AddDate(0, 0, -6), Valid: true},
		ToDay: pgtype.Date{Time: today, Valid: true}, ProvinceID: prov})
	if err != nil {
		return result{}, err
	}
	ext, err := s.q.WeatherExtremes(ctx, db.WeatherExtremesParams{FromDay: pgtype.Date{Time: today.AddDate(0, 0, -13), Valid: true},
		ToDay: pgtype.Date{Time: today, Valid: true}, ProvinceID: prov})
	if err != nil {
		return result{}, err
	}
	list := []map[string]any{}
	for _, d := range days {
		list = append(list, map[string]any{"date": d.Day.Time.Format(time.DateOnly), "min_c": math.Round(d.Tmin), "max_c": math.Round(d.Tmax), "rain_mm": math.Round(d.Precip*10) / 10})
	}
	extremes := []map[string]any{}
	for _, e := range ext {
		extremes = append(extremes, map[string]any{"date": e.Day.Time.Format(time.DateOnly), "region": e.ProvinceName, "kind": e.Kind, "temp_c": math.Round(e.Temp)})
	}
	params := map[string]any{}
	if area != "" {
		params["region"] = area
	}
	data := map[string]any{"area": place("uz", area), "days": list, "frost_or_heat": extremes, "enough": len(list) > 0, "source": "Open-Meteo"}
	return simple(ToolWeather, params, data, len(list) > 0, func(lang string) []string {
		if len(days) == 0 {
			return []string{tr(lang, "weather_none")}
		}
		d := days[len(days)-1]
		out := []string{tr(lang, "weather", place(lang, area), d.Day.Time.Format(time.DateOnly), trimNum(math.Round(d.Tmin)), trimNum(math.Round(d.Tmax)))}
		if len(ext) > 0 {
			out = append(out, tr(lang, "weather_extremes", len(ext)))
		}
		return out
	}), nil
}

func (s *Service) marketEvents(ctx context.Context, product, region string) (result, error) {
	today := insight.Day(time.Now()).Time
	params := map[string]any{}
	arg := db.EventsForParams{FromDay: pgtype.Date{Time: today.AddDate(0, 0, -45), Valid: true}, ToDay: pgtype.Date{Time: today.AddDate(0, 0, 30), Valid: true}}
	if product != "" {
		p, err := s.resolveProduct(ctx, product)
		if errors.Is(err, errUnknownProduct) {
			return failed(ToolMarketEvents, "unknown_product"), nil
		}
		if err != nil {
			return result{}, err
		}
		arg.ProductID, arg.Category = p.ID, p.Category
		params["product"] = p.Slug
	}
	if region != "" {
		if r := s.ask.FindRegion(region); r != nil {
			arg.ProvinceID = pgtype.Int2{Int16: r.Province, Valid: true}
			params["region"] = r.Name
		}
	}
	rows, err := s.q.EventsFor(ctx, arg)
	if err != nil {
		return result{}, err
	}
	list := []map[string]any{}
	for i, e := range rows {
		if i == 10 {
			break
		}
		m := map[string]any{"title": e.Title, "kind": e.Kind, "starts": e.StartsOn.Time.Format(time.DateOnly)}
		if e.EndsOn.Valid {
			m["ends"] = e.EndsOn.Time.Format(time.DateOnly)
		}
		if e.ProvinceName != "" {
			m["region"] = e.ProvinceName
		}
		list = append(list, m)
	}
	data := map[string]any{"events": list, "window": "45 days back, 30 days ahead", "enough": true}
	return simple(ToolMarketEvents, params, data, true, func(lang string) []string {
		if len(list) == 0 {
			return []string{tr(lang, "events_none")}
		}
		out := []string{tr(lang, "events_head")}
		for _, e := range list {
			out = append(out, "• "+e["starts"].(string)+": "+e["title"].(string))
		}
		return out
	}), nil
}

func (s *Service) sellerInfo(ctx context.Context, name string) (result, error) {
	name = strings.TrimSpace(name)
	if len([]rune(name)) < 2 {
		return failed(ToolSellerInfo, "bad_arguments"), nil
	}
	found, err := s.q.SellersByName(ctx, name)
	if err != nil {
		return result{}, err
	}
	params := map[string]any{"seller": name}
	if len(found) == 0 {
		data := map[string]any{"enough": false}
		return simple(ToolSellerInfo, params, data, false, func(lang string) []string { return []string{tr(lang, "seller_none", name)} }), nil
	}
	sellers := []map[string]any{}
	for _, f := range found {
		u, err := s.q.PublicSeller(ctx, f.ID)
		if err != nil {
			continue
		}
		m := map[string]any{"name": u.Name, "region": u.RegionName, "bazaar": u.MarketName, "live_listings": u.Listings, "products": u.Products,
			"page": "/sotuvchi/" + u.ID.String()}
		if r, err := s.q.SellerRating(ctx, u.ID); err == nil && r.Ratings >= 3 {
			m["rating"], m["ratings"] = math.Round(r.Average*10)/10, r.Ratings
		}
		sellers = append(sellers, m)
	}
	data := map[string]any{"sellers": sellers, "enough": len(sellers) > 0}
	return simple(ToolSellerInfo, params, data, len(sellers) > 0, func(lang string) []string {
		out := []string{}
		for _, m := range sellers {
			out = append(out, tr(lang, "seller", m["name"], m["region"], m["live_listings"]))
		}
		return out
	}), nil
}

func (s *Service) createRequest(ctx context.Context, sc scope, quantity float64, unit string, days int, lang string) (result, error) {
	if quantity <= 0 {
		return failed(ToolCreateRequest, "bad_arguments"), nil
	}
	qty, err := s.prices.ToBase(sc.product.ID, unit, quantity)
	if err != nil {
		return failed(ToolCreateRequest, "bad_arguments"), nil
	}
	if days < 1 || days > 7 {
		days = 3
	}
	p := sc.product
	act := Action{Type: ToolCreateRequest, Label: tr(lang, "act_request", p.Name, trimNum(qty), p.BaseUnit),
		Params: map[string]any{"product_id": p.ID, "product": p.Slug, "quantity": qty, "unit": p.BaseUnit, "days": days}}
	if sc.region != nil {
		act.Params["region_id"] = sc.region.ID
	}
	params := sc.params()
	params["quantity"], params["days"] = qty, days
	data := map[string]any{"proposed": true, "product": p.Name, "quantity": qty, "unit": p.BaseUnit, "days": days,
		"note": "nothing is sent until the user presses the button", "enough": true}
	res := simple(ToolCreateRequest, params, data, true, func(lang string) []string { return []string{tr(lang, "act_confirm")} })
	res.action = &act
	return res, nil
}

func watchPrice(sc scope, lang string) result {
	p := sc.product
	act := Action{Type: ToolWatchPrice, Label: tr(lang, "act_watch", p.Name), Params: map[string]any{"product_id": p.ID, "province_id": sc.province, "trade": sc.trade}}
	data := map[string]any{"proposed": true, "product": p.Name, "note": "nothing is saved until the user presses the button", "enough": true}
	res := simple(ToolWatchPrice, sc.params(), data, true, func(lang string) []string { return []string{tr(lang, "act_confirm")} })
	res.action = &act
	return res
}

func monthName(lang string, m int) string {
	if m < 1 || m > 12 {
		return ""
	}
	return tr(lang, "m_"+monthKeys[m-1])
}

var dayKeys = []string{"mon", "tue", "wed", "thu", "fri", "sat", "sun"}

func dayName(lang string, d int) string {
	if d < 1 || d > 7 {
		return ""
	}
	return tr(lang, "d_"+dayKeys[d-1])
}
