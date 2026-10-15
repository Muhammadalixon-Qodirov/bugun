package analytics

import "sort"

// WeightedQuantile is the lowest value at which the running weight reaches q
// of the total (a weighted lower quantile). Nil weights count every value
// equally. It returns 0 when nothing has weight. It mirrors the SQL function
// weighted_quantile (migration 00012); weights come from SQL price_weight,
// the source's trust times freshness (spec §10.1).
func WeightedQuantile(values, weights []float64, q float64) float64 {
	type pair struct{ v, w float64 }
	ps := make([]pair, 0, len(values))
	total := 0.0
	for i, v := range values {
		w := 1.0
		if weights != nil {
			w = weights[i]
		}
		if w > 0 {
			ps = append(ps, pair{v, w})
			total += w
		}
	}
	if total == 0 {
		return 0
	}
	sort.Slice(ps, func(i, j int) bool { return ps[i].v < ps[j].v })
	cum := 0.0
	for _, p := range ps {
		cum += p.w
		if cum >= q*total*(1-1e-9) {
			return p.v
		}
	}
	return ps[len(ps)-1].v
}
