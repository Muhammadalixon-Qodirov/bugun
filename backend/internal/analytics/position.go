package analytics

import "math"

// Position is where a seller's price stands among other sellers' prices for
// the same product (spec §10.4): "12 sellers, you are 9th".
type Position struct {
	Enough  bool    // false when there are fewer than MinObservations other prices
	Rank    int     // 1 = cheapest; equal prices share a rank
	Total   int     // other sellers plus this one
	Median  float64 // weighted median of the other sellers' prices (spec §10.1)
	DiffPct int     // price vs. that median, in percent
}

// PositionOf ranks price among peers (the other sellers' prices); weights are
// the peers' median weights, nil for equal weights.
func PositionOf(peers, weights []float64, price float64) Position {
	p := Position{Total: len(peers) + 1}
	if len(peers) < MinObservations {
		return p
	}
	cheaper := 0
	for _, x := range peers {
		if x < price {
			cheaper++
		}
	}
	p.Enough = true
	p.Rank = cheaper + 1
	p.Median = WeightedQuantile(peers, weights, 0.5)
	if p.Median > 0 {
		p.DiffPct = int(math.Round(100 * (price - p.Median) / p.Median))
	}
	return p
}
