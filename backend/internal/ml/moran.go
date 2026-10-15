package ml

import "math"

// Local Moran's I (Anselin 1995) asks whether a place and its neighbours are
// alike: a dear viloyat among dear neighbours is a hot spot, a cheap one among
// cheap neighbours a cold spot, and a dear one among cheap neighbours an
// outlier. Significance comes from shuffling the other values over the
// neighbours (conditional permutation).

const moranPermutations = 999

// MoranAlpha: the pseudo p-value a spot must beat.
const MoranAlpha = 0.05

// LocalSpot is one place's result.
type LocalSpot struct {
	I     float64 // local Moran's I
	P     float64 // pseudo p-value, one-sided in the direction of I
	Kind  string  // hot, cold, high_outlier, low_outlier; empty when not significant
	Valid bool    // false for places without neighbours
}

// LocalMoran computes Local Moran's I for values x over the neighbour lists
// (indices into x, row-standardised weights).
func LocalMoran(x []float64, neighbors [][]int) []LocalSpot {
	n := len(x)
	out := make([]LocalSpot, n)
	if n < 3 {
		return out
	}
	mean := 0.0
	for _, v := range x {
		mean += v / float64(n)
	}
	z := make([]float64, n)
	m2 := 0.0
	for i, v := range x {
		z[i] = v - mean
		m2 += z[i] * z[i] / float64(n)
	}
	if m2 == 0 {
		return out
	}
	r := newRand()
	others := make([]int, 0, n-1)
	for i := 0; i < n; i++ {
		nb := neighbors[i]
		if len(nb) == 0 {
			continue
		}
		lag := 0.0
		for _, j := range nb {
			lag += z[j] / float64(len(nb))
		}
		obs := z[i] / m2 * lag
		others = others[:0]
		for j := 0; j < n; j++ {
			if j != i {
				others = append(others, j)
			}
		}
		extreme := 0
		for p := 0; p < moranPermutations; p++ {
			// A random set of len(nb) other places plays the neighbours.
			for k := 0; k < len(nb); k++ {
				s := k + r.IntN(len(others)-k)
				others[k], others[s] = others[s], others[k]
			}
			l := 0.0
			for k := 0; k < len(nb); k++ {
				l += z[others[k]] / float64(len(nb))
			}
			sim := z[i] / m2 * l
			if (obs >= 0 && sim >= obs) || (obs < 0 && sim <= obs) {
				extreme++
			}
		}
		sp := LocalSpot{I: obs, P: float64(extreme+1) / float64(moranPermutations+1), Valid: true}
		if sp.P < MoranAlpha && math.Abs(obs) > 0 {
			switch {
			case z[i] > 0 && lag > 0:
				sp.Kind = "hot"
			case z[i] < 0 && lag < 0:
				sp.Kind = "cold"
			case z[i] > 0:
				sp.Kind = "high_outlier"
			default:
				sp.Kind = "low_outlier"
			}
		}
		out[i] = sp
	}
	return out
}
