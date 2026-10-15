package ml

import (
	"math"
	"testing"
)

// condExpect is a tree's output when only the features in mask are known:
// a split on an unknown feature follows both children by their rows.
func (t gbmTree) condExpect(x []float64, mask uint) float64 {
	var rec func(i int) float64
	rec = func(i int) float64 {
		n := t[i]
		if n.Feature < 0 {
			return n.Value
		}
		if mask&(1<<n.Feature) != 0 {
			v := x[n.Feature]
			if (v != v && n.MissingLeft) || (v == v && v < n.Threshold) {
				return rec(n.Left)
			}
			return rec(n.Right)
		}
		return (t[n.Left].Cover*rec(n.Left) + t[n.Right].Cover*rec(n.Right)) / n.Cover
	}
	return rec(0)
}

// TreeSHAP equals the brute-force Shapley values of the same expectation,
// and the values add up to the prediction.
func TestTreeSHAPExact(t *testing.T) {
	x, y := pricesLike(4000, 11, 0.01)
	m := TrainGBM(x, y, nil, nil, GBMParams{Trees: 25, Depth: 4, MinLeaf: 20})
	rows := [][]float64{{6, 0.03, 1, 0.2, -0.5}, {0, -0.02, 0, 0.9, 1.2}, {5, math.NaN(), 0, 0.5, 0}}
	for _, row := range rows {
		phi, expected := m.Explain(row)
		sum := expected
		for _, v := range phi {
			sum += v
		}
		if math.Abs(sum-m.Predict(row)) > 1e-9 {
			t.Errorf("sum %.10f, prediction %.10f", sum, m.Predict(row))
		}
		brute := Shapley(m.Features, func(mask uint) float64 {
			out := m.Base
			for _, tr := range m.Trees {
				out += tr.condExpect(row, mask)
			}
			return out
		})
		for f := range phi {
			if math.Abs(phi[f]-brute[f]) > 1e-9 {
				t.Errorf("feature %d: TreeSHAP %.10f, brute force %.10f", f, phi[f], brute[f])
			}
		}
	}
	// A hard frost row: frost is the largest contribution.
	phi, _ := m.Explain([]float64{6, 0, 0, 0.5, 0})
	for f := 1; f < len(phi); f++ {
		if math.Abs(phi[f]) > phi[0] {
			t.Errorf("feature %d (%.4f) outweighs frost (%.4f)", f, phi[f], phi[0])
		}
	}
}
