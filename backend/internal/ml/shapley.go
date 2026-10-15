package ml

// MaxShapleyPlayers keeps the exact sum small: 2^12 coalitions.
const MaxShapleyPlayers = 12

// Shapley returns each player's exact Shapley value for the game v, where
// v(mask) is the outcome with the players whose bits are set in mask. With a
// handful of players every coalition is counted, so there is no sampling
// error and the values add up exactly to v(all) - v(none). It panics above
// MaxShapleyPlayers.
func Shapley(n int, v func(mask uint) float64) []float64 {
	if n > MaxShapleyPlayers {
		panic("ml.Shapley: too many players")
	}
	full := uint(1)<<n - 1
	vals := make([]float64, full+1)
	for m := uint(0); m <= full; m++ {
		vals[m] = v(m)
	}
	// weight[s] = s! (n-s-1)! / n!
	weight := make([]float64, n)
	for s := 0; s < n; s++ {
		w := 1.0
		for k := 1; k <= s; k++ {
			w *= float64(k)
		}
		for k := 1; k <= n-s-1; k++ {
			w *= float64(k)
		}
		for k := 1; k <= n; k++ {
			w /= float64(k)
		}
		weight[s] = w
	}
	phi := make([]float64, n)
	for m := uint(0); m <= full; m++ {
		size := popcount(m)
		for i := 0; i < n; i++ {
			bit := uint(1) << i
			if m&bit != 0 {
				continue
			}
			phi[i] += weight[size] * (vals[m|bit] - vals[m])
		}
	}
	return phi
}

func popcount(m uint) int {
	n := 0
	for ; m != 0; m &= m - 1 {
		n++
	}
	return n
}
