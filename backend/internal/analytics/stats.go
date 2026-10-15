// Package analytics holds the deterministic statistics behind every number the
// platform shows (spec §10). Nothing here calls an AI model.
package analytics

import (
	"math"
	"sort"
)

// MinObservations is the smallest sample for which a median is shown (spec §10.7).
const MinObservations = 3

// Median returns the middle value (mean of the two middle values for even n).
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

// Percentile returns the p-th percentile (0..100) with linear interpolation.
func Percentile(xs []float64, p float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	s := append([]float64(nil), xs...)
	sort.Float64s(s)
	pos := p / 100 * float64(len(s)-1)
	lo := int(math.Floor(pos))
	hi := int(math.Ceil(pos))
	return s[lo] + (s[hi]-s[lo])*(pos-float64(lo))
}

// MAD is the median absolute deviation from the median.
func MAD(xs []float64) float64 {
	m := Median(xs)
	dev := make([]float64, len(xs))
	for i, x := range xs {
		dev[i] = math.Abs(x - m)
	}
	return Median(dev)
}

// OutlierMinSample is the sample size below which the robust z-score is not used.
const OutlierMinSample = 5

// RobustZ is the modified z-score of x against sample xs (Iglewicz & Hoaglin).
// MAD is floored at 2% of the median: when most sellers quote the same price
// the raw MAD is zero and every small difference would look extreme.
func RobustZ(xs []float64, x float64) float64 {
	med := Median(xs)
	mad := math.Max(MAD(xs), 0.02*math.Abs(med))
	if mad == 0 {
		return 0
	}
	return 0.6745 * (x - med) / mad
}

// IsOutlier reports whether price x should be held for review (spec §10.2):
// |z| > 3.5 against at least five other prices. With three or four prices it
// falls back to a ±50% band around their median; with fewer it never flags.
func IsOutlier(others []float64, x float64) bool {
	switch {
	case len(others) >= OutlierMinSample:
		return math.Abs(RobustZ(others, x)) > 3.5
	case len(others) >= MinObservations:
		m := Median(others)
		return m > 0 && math.Abs(x-m)/m > 0.5
	}
	return false
}
