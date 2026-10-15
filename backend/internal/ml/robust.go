// Package ml holds the small statistical and machine-learning methods behind
// the analytics (ML_REJA.md): robust z-scores, exact Shapley values,
// Isolation Forest, K-Means with silhouette and Local Moran's I. They are
// plain functions over slices, retrained from the data on every run and
// deterministic: every random choice draws from Seed.
package ml

import (
	"math"
	"math/rand/v2"
	"sort"
)

// Seed makes every random method repeatable: the same data always gives the
// same answer.
const Seed = 42

func newRand() *rand.Rand { return rand.New(rand.NewPCG(Seed, Seed)) }

// Median of xs; 0 for none. xs is not changed.
func Median(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	s := append([]float64(nil), xs...)
	sort.Float64s(s)
	n := len(s)
	if n%2 == 1 {
		return s[n/2]
	}
	return (s[n/2-1] + s[n/2]) / 2
}

// MAD is the median absolute deviation from the median.
func MAD(xs []float64) float64 {
	m := Median(xs)
	d := make([]float64, len(xs))
	for i, x := range xs {
		d[i] = math.Abs(x - m)
	}
	return Median(d)
}

// madScale turns a MAD into a standard deviation for normal data.
const madScale = 1.4826

// RobustZ is how many robust standard deviations x lies from the reference
// values. Unlike mean and standard deviation, the median and MAD are not
// pulled towards the outlier itself: in [4 5 5 6 100] the mean-based z of 100
// is 2.0, the robust one 64. ok is false with fewer than 5 values or no
// spread at all.
func RobustZ(x float64, ref []float64) (z float64, ok bool) {
	if len(ref) < 5 {
		return 0, false
	}
	mad := MAD(ref)
	if mad == 0 {
		return 0, false
	}
	return (x - Median(ref)) / (madScale * mad), true
}

// Standardize scales every column of rows to median 0 and robust spread 1, so
// no feature dominates a distance just by its unit. Columns without spread
// become 0.
func Standardize(rows [][]float64) [][]float64 {
	if len(rows) == 0 {
		return nil
	}
	d := len(rows[0])
	out := make([][]float64, len(rows))
	for i := range out {
		out[i] = make([]float64, d)
	}
	col := make([]float64, len(rows))
	for j := 0; j < d; j++ {
		for i, r := range rows {
			col[i] = r[j]
		}
		m, s := Median(col), madScale*MAD(col)
		if s == 0 {
			// Fall back to the mean deviation for mostly constant columns.
			for _, x := range col {
				s += math.Abs(x-m) / float64(len(col))
			}
		}
		for i := range rows {
			if s > 0 {
				out[i][j] = (rows[i][j] - m) / s
			}
		}
	}
	return out
}
