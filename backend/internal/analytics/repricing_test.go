package analytics

import (
	"math"
	"testing"
)

// base is the DINAMIK_NARX.md example: tomatoes at 6 000, range 5 500–7 000.
func base() RepriceInput {
	return RepriceInput{
		Current: 6000, DayStart: 6000, Min: 5500, Max: 7000, Goal: "balanced",
		Peers: 12, Median: 6100, P75: 6500,
	}
}

func TestRepriceDocExample(t *testing.T) {
	in := base()
	in.Pressure, in.PressureOK = 1.4, true // demand up: +4%
	in.MedianPrev = 5980                   // market +2%: +1%
	d := Reprice(in)
	// 6 100 × 1.05 ≈ 6 405, but the day's step stops it at 6 300.
	if !d.Change || d.Price != 6300 {
		t.Fatalf("decision %+v, want 6 300", d)
	}
	if codes(d)["pressure"] == 0 || codes(d)["trend"] == 0 || codes(d)["cap"] >= 0 {
		t.Errorf("factors %+v, want pressure, trend and a downward cap", d.Factors)
	}
	// The capped +5% is shared fairly: pressure brought more than the trend,
	// the market median a little, and together they are exactly the move.
	c, sum := map[string]float64{}, 0.0
	for _, f := range d.Contrib {
		c[f.Code] = f.Share
		sum += f.Share
	}
	if math.Abs(sum-(6300.0/6000-1)) > 0.002 {
		t.Errorf("contributions %+v add up to %.4f, want 0.05", d.Contrib, sum)
	}
	if !(c["pressure"] > c["trend"] && c["trend"] > 0 && c["market"] > 0) || c["cap"] != 0 {
		t.Errorf("contributions %+v", d.Contrib)
	}
}

func TestRepriceRules(t *testing.T) {
	in := base()
	in.Pressure, in.PressureOK = 1.4, true
	in.MedianPrev = 5980
	// A wider daily step lets the doc example reach its target (≈6 405 → 6 400).
	wide := DefaultRepriceRules
	wide.DailyStep = 0.10
	if d := wide.Reprice(in); !d.Change || d.Price != 6400 {
		t.Errorf("10%% step: %+v, want 6 400", d)
	}
	// A seller aiming for profit sits higher than the defaults put them.
	rich := RepriceRules{Goals: map[string]float64{"balanced": 1.05}, DailyStep: 0.2}
	if d := rich.Reprice(base()); !d.Change || d.Price <= 6100 {
		t.Errorf("goal factor 1.05: %+v, want above the median", d)
	}
}

func TestRepriceNeverBelowSellerFloor(t *testing.T) {
	in := base()
	in.Median = 5200
	in.Pressure, in.PressureOK = 0.6, true
	in.StockRatio = 3
	d := Reprice(in)
	if !d.Change || d.Price != 5700 {
		// 5 200 is below the floor 5 500, but the day allows only −5%: 5 700.
		t.Fatalf("decision %+v, want 5 700 (daily step)", d)
	}
	// Next day from 5 700: the market wants 4 900, the floor stops it at 5 500.
	in.Current, in.DayStart = 5700, 5700
	if d := Reprice(in); !d.Change || d.Price != 5500 {
		t.Errorf("decision %+v, want the floor 5 500", d)
	}
}

func TestRepriceNeverAboveMarketUpperQuartile(t *testing.T) {
	in := base()
	in.Max = 9000
	in.Goal = "profit"
	in.Pressure, in.PressureOK = 3, true
	in.MedianPrev = 5000
	d := Reprice(in)
	if d.Price > 6300 || d.Price > int64(in.P75) {
		t.Errorf("price %d above the daily step or P75 %v", d.Price, in.P75)
	}
	in.Current, in.DayStart = 6450, 6450
	if d := Reprice(in); d.Change && d.Price > 6500 {
		t.Errorf("price %d above P75 6 500", d.Price)
	}
}

func TestRepriceHolds(t *testing.T) {
	in := base()
	in.Peers = 4
	if d := Reprice(in); d.Change || d.Hold != "few_prices" {
		t.Errorf("few peers: %+v", d)
	}
	in = base()
	in.Median = 6050 // within 2%: leave it
	if d := Reprice(in); d.Change || d.Hold != "small" {
		t.Errorf("small move: %+v", d)
	}
	in = base()
	in.DayStart = 6000
	in.Current = 6300 // already moved +5% today
	in.Pressure, in.PressureOK = 2, true
	if d := Reprice(in); d.Change && d.Price > 6300 {
		t.Errorf("second rise in a day: %+v", d)
	}
}

func TestRepriceListingSignals(t *testing.T) {
	in := base()
	in.Median = 6000
	in.Views, in.Contacts, in.MarketRate = 100, 1, 0.1 // 1% vs 10%
	in.RFQOffers, in.RFQPicked = 4, 0
	d := Reprice(in)
	// 6 000 × 0.96 = 5 760, rounded to 5 800.
	if !d.Change || d.Price != 5800 {
		t.Errorf("decision %+v, want 5 800", d)
	}
	if codes(d)["response"] >= 0 || codes(d)["rfq"] >= 0 {
		t.Errorf("factors %+v, want response and rfq", d.Factors)
	}
}

func TestRepriceSales(t *testing.T) {
	in := base()
	in.Median = 6000
	in.Current, in.DayStart = 5900, 5900
	in.Sales, in.MarketSales, in.SalesKnown = 4, 1.5, true // over twice the usual
	if d := Reprice(in); !d.Change || d.Price != 6100 || codes(d)["sales"] <= 0 {
		t.Errorf("fast sales: %+v, want +2%% (6 120 → 6 100)", d)
	}
	in.Current, in.DayStart = 6100, 6100
	in.Sales, in.Contacts = 0, 6 // calls, no sales
	if d := Reprice(in); !d.Change || d.Price != 5900 || codes(d)["sales"] >= 0 {
		t.Errorf("no sales: %+v, want −2%% (5 880 → 5 900)", d)
	}
	in.Contacts = 2 // too few calls to say
	if d := Reprice(in); codes(d)["sales"] != 0 {
		t.Errorf("few contacts moved the price: %+v", d)
	}
}

func TestRoundingStep(t *testing.T) {
	for p, want := range map[float64]float64{900: 50, 6300: 100, 25000: 500, 112000: 1000, 3_800_000: 10_000} {
		if got := roundingStep(p); got != want {
			t.Errorf("roundingStep(%v) = %v, want %v", p, got, want)
		}
	}
}

func codes(d Decision) map[string]float64 {
	m := map[string]float64{}
	for _, f := range d.Factors {
		m[f.Code] += f.Share
	}
	return m
}
