package analytics

import (
	"math"
	"testing"
)

func TestMedian(t *testing.T) {
	cases := []struct {
		in   []float64
		want float64
	}{
		{nil, 0},
		{[]float64{5}, 5},
		{[]float64{3, 1, 2}, 2},
		{[]float64{4, 1, 3, 2}, 2.5},
	}
	for _, c := range cases {
		if got := Median(c.in); got != c.want {
			t.Errorf("Median(%v) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestMedianDoesNotReorderInput(t *testing.T) {
	in := []float64{3, 1, 2}
	Median(in)
	if in[0] != 3 || in[1] != 1 || in[2] != 2 {
		t.Fatalf("input was modified: %v", in)
	}
}

func TestPercentile(t *testing.T) {
	xs := []float64{10, 20, 30, 40, 50}
	for p, want := range map[float64]float64{0: 10, 25: 20, 50: 30, 75: 40, 100: 50, 10: 14} {
		if got := Percentile(xs, p); math.Abs(got-want) > 1e-9 {
			t.Errorf("Percentile(%v) = %v, want %v", p, got, want)
		}
	}
}

func TestIsOutlier(t *testing.T) {
	market := []float64{4200, 4300, 4400, 4500, 4500, 4600, 4700}
	if IsOutlier(market, 4800) {
		t.Error("4800 flagged, but it is a normal price")
	}
	if !IsOutlier(market, 45000) {
		t.Error("45000 (extra zero) not flagged")
	}
	if !IsOutlier(market, 450) {
		t.Error("450 (missing zero) not flagged")
	}
	if !IsOutlier(market[:4], 45000) || IsOutlier(market[:4], 5500) {
		t.Error("small-sample ±50% band is wrong")
	}
	if IsOutlier(market[:2], 45000) {
		t.Error("flagged with fewer than three prices")
	}
}

func TestIsOutlierIdenticalPrices(t *testing.T) {
	same := []float64{5000, 5000, 5000, 5000, 5000}
	if IsOutlier(same, 5000) || IsOutlier(same, 5100) {
		t.Error("identical or near-identical price flagged")
	}
	if !IsOutlier(same, 9000) {
		t.Error("far-off price not flagged when MAD is zero")
	}
}
