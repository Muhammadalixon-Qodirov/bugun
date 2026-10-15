package analytics

import (
	"math"

	"bugun/internal/ml"
)

// Automatic pricing (DINAMIK_NARX.md, spec 14.4 decision of 2026-09-25). The
// system follows the market within the seller's own range; it never leads it.

// RepriceRules are the tunable parts of a decision (config: REPRICE_*).
type RepriceRules struct {
	// Goals: how far from the market median the seller wants to sit, by goal.
	Goals map[string]float64
	// DailyStep caps the sum of one day's changes, measured from the price the
	// day started with.
	DailyStep float64
}

// DefaultRepriceRules: fast sits 3% under the median, profit 3% above, and a
// price moves at most 5% a day.
var DefaultRepriceRules = RepriceRules{
	Goals:     map[string]float64{"fast": 0.97, "balanced": 1.00, "profit": 1.03},
	DailyStep: 0.05,
}

const (
	// MinRepricePeers: with fewer other prices the market tells us too little.
	MinRepricePeers = 5
	// minChange: smaller moves are left out, so prices do not twitch.
	minChange = 0.02
)

// RepriceInput is everything one decision looks at.
type RepriceInput struct {
	Current    int64   // the listing's price now
	DayStart   int64   // its price when the Tashkent day began (for the daily cap)
	Min, Max   int64   // the seller's range
	Goal       string  // fast, balanced, profit
	Peers      int     // other live prices in the viloyat
	Median     float64 // their weighted median
	P75        float64 // their upper quartile: the ceiling for any automatic price
	MedianPrev float64 // the median a week ago; 0 = unknown
	Pressure   float64 // demand-supply index; used when PressureOK
	PressureOK bool
	// The listing's own last 7 days against the market's.
	Views, Contacts int
	MarketRate      float64 // contacts per view across the product in the viloyat; 0 = unknown
	// RFQ answers in the last 30 days: offers sent and how many buyers picked.
	RFQOffers, RFQPicked int
	// StockRatio is the listing's stock against the usual stock of sellers of
	// this product; 0 = unknown.
	StockRatio float64
	// Sales in the last 7 days: the listing's, and the average of other live
	// listings of the product in the viloyat (SalesKnown when there are any).
	Sales       int
	MarketSales float64
	SalesKnown  bool
}

// Factor is one reason behind a decision, as a signed share of the price.
type Factor struct {
	Code  string  `json:"code"` // goal, pressure, trend, response, rfq, sales, stock, cap
	Share float64 `json:"share"`
}

// Decision is the outcome: a new price, or why the price stays.
type Decision struct {
	Change  bool
	Price   int64
	Target  float64 // before caps and rounding
	Factors []Factor
	// Contrib splits the actual move into exact Shapley shares of the old
	// price (ML_REJA.md 2b): market (moving to the viloyat median), goal and
	// each factor, plus rounding. They add up to Price/Current − 1 even when
	// a cap cut the move, which Factors alone cannot show. Set on changes.
	Contrib []Factor
	Hold    string // why nothing changes: few_prices, small, capped
}

func clamp(x, lo, hi float64) float64 { return math.Max(lo, math.Min(hi, x)) }

// Reprice decides a listing's next price under the default rules.
func Reprice(in RepriceInput) Decision { return DefaultRepriceRules.Reprice(in) }

// Reprice decides a listing's next price.
func (r RepriceRules) Reprice(in RepriceInput) Decision {
	if in.Peers < MinRepricePeers || in.Median <= 0 || in.Current <= 0 {
		return Decision{Hold: "few_prices"}
	}
	d := Decision{}
	g, ok := r.Goals[in.Goal]
	if !ok {
		g = 1
	}
	if g != 1 {
		d.Factors = append(d.Factors, Factor{"goal", g - 1})
	}

	var shares []Factor // the factors that move the target, without goal and cap
	add := func(code string, share float64) {
		if share != 0 {
			d.Factors = append(d.Factors, Factor{code, share})
			shares = append(shares, Factor{code, share})
		}
	}
	sum := 0.0
	if in.PressureOK {
		s := clamp(0.10*(in.Pressure-1), -0.05, 0.05)
		add("pressure", s)
		sum += s
	}
	if in.MedianPrev > 0 {
		s := clamp(0.5*(in.Median/in.MedianPrev-1), -0.03, 0.03)
		add("trend", s)
		sum += s
	}
	if in.Views >= 20 && in.MarketRate > 0 && float64(in.Contacts)/float64(in.Views) < 0.5*in.MarketRate {
		add("response", -0.02) // many look, few call: the price may be too high
		sum -= 0.02
	}
	if in.RFQOffers >= 3 && in.RFQPicked == 0 {
		add("rfq", -0.02) // buyers keep choosing someone else
		sum -= 0.02
	}
	switch {
	case in.Sales >= 2 && in.SalesKnown && float64(in.Sales) >= 2*in.MarketSales:
		add("sales", 0.02) // selling twice as fast as others
		sum += 0.02
	case in.Sales == 0 && in.Contacts >= 5:
		add("sales", -0.02) // people call but nobody buys
		sum -= 0.02
	}
	switch {
	case in.StockRatio >= 2:
		add("stock", -0.02)
		sum -= 0.02
	case in.StockRatio > 0 && in.StockRatio <= 0.5:
		add("stock", 0.01)
		sum += 0.01
	}
	d.Target = in.Median * g * (1 + sum)

	// Caps: the seller's range, the market's upper quartile, the daily step.
	lo, hi := float64(in.Min), float64(in.Max)
	if in.P75 > 0 && in.P75 < hi {
		hi = math.Max(in.P75, lo)
	}
	start := float64(in.DayStart)
	if start <= 0 {
		start = float64(in.Current)
	}
	lo = math.Max(lo, start*(1-r.DailyStep))
	hi = math.Min(hi, start*(1+r.DailyStep))
	if lo > hi { // the day's room is outside the seller's range: stay put
		d.Hold = "capped"
		return d
	}
	price := clamp(d.Target, lo, hi)
	if price != d.Target {
		d.Factors = append(d.Factors, Factor{"cap", price/d.Target - 1})
	}
	// Round like a seller would, then keep the rounded price inside the caps.
	step := roundingStep(price)
	rounded := math.Round(price/step) * step
	if rounded > hi {
		rounded = math.Floor(hi/step) * step
	}
	if rounded < lo {
		rounded = math.Ceil(lo/step) * step
	}
	if rounded < lo || rounded > hi {
		rounded = price // the room is narrower than one step
	}
	cur := float64(in.Current)
	if math.Abs(rounded-cur)/cur < minChange {
		d.Hold = "small"
		return d
	}
	d.Change, d.Price = true, int64(rounded)
	d.Contrib = contributions(cur, in.Median, g, shares, lo, hi, rounded)
	return d
}

// contributions shares the move from cur to the capped price among the
// players by exact Shapley values: without "market" the target starts from
// the current price instead of the median, without a factor its share is
// left out, and the caps always apply.
func contributions(cur, median, goal float64, shares []Factor, lo, hi, rounded float64) []Factor {
	codes := []string{"market"}
	if goal != 1 {
		codes = append(codes, "goal")
	}
	first := len(codes)
	for _, f := range shares {
		codes = append(codes, f.Code)
	}
	price := func(mask uint) float64 {
		base, g, sum := cur, 1.0, 0.0
		if mask&1 != 0 {
			base = median
		}
		if first == 2 && mask&2 != 0 {
			g = goal
		}
		for i, f := range shares {
			if mask&(1<<(first+i)) != 0 {
				sum += f.Share
			}
		}
		return clamp(base*g*(1+sum), lo, hi)
	}
	phi := ml.Shapley(len(codes), price)
	out := make([]Factor, 0, len(codes)+1)
	for i, c := range codes {
		if v := phi[i] / cur; math.Abs(v) >= 0.0005 {
			out = append(out, Factor{c, v})
		}
	}
	if v := (rounded - price(uint(1)<<len(codes)-1)) / cur; math.Abs(v) >= 0.0005 {
		out = append(out, Factor{"rounding", v})
	}
	return out
}

// roundingStep is how bazaar prices are quoted: 50, 100, 500, 1 000 or 10 000.
func roundingStep(p float64) float64 {
	switch {
	case p >= 1_000_000:
		return 10_000
	case p >= 100_000:
		return 1000
	case p >= 20_000:
		return 500
	case p >= 5000:
		return 100
	}
	return 50
}
