package analytics

import "math"

// Reason says why a price was held for review; empty means the price passed.
type Reason string

const (
	ReasonNone Reason = ""
	// ReasonJumpFromPrevious: the seller's new price is MaxJump times higher or
	// lower than their last one, which is almost always an extra or missing zero.
	ReasonJumpFromPrevious Reason = "jump_from_previous"
	// ReasonFarFromMarket: far from what other sellers in the viloyat ask (spec §10.2).
	ReasonFarFromMarket Reason = "far_from_market"
	// ReasonFarFromReference: too few sellers to compare with, and the price is
	// more than ReferenceBand away from the official benchmark (UzEx, narxtahlil).
	ReasonFarFromReference Reason = "far_from_reference"
)

const (
	MaxJump       = 5.0
	ReferenceBand = 0.5
)

// Check is everything a new price is compared against. Zero values mean
// "unknown" and skip that comparison.
type Check struct {
	Peers     []float64 // other sellers' live prices in the same viloyat
	Previous  float64   // this seller's last price for the same listing
	Reference float64   // official benchmark price for the product
}

// Screen decides whether price should be held for review. The seller's own
// history is checked first because a typo is the most common cause; then other
// sellers when there are enough of them; otherwise the official benchmark;
// and with three or four sellers and no benchmark, their median (IsOutlier).
func Screen(c Check, price float64) Reason {
	if price <= 0 {
		return ReasonNone
	}
	if c.Previous > 0 {
		if r := price / c.Previous; r >= MaxJump || r <= 1/MaxJump {
			return ReasonJumpFromPrevious
		}
	}
	switch {
	case len(c.Peers) >= OutlierMinSample:
		if IsOutlier(c.Peers, price) {
			return ReasonFarFromMarket
		}
	case c.Reference > 0:
		if math.Abs(price-c.Reference)/c.Reference > ReferenceBand {
			return ReasonFarFromReference
		}
	case IsOutlier(c.Peers, price):
		return ReasonFarFromMarket
	}
	return ReasonNone
}
