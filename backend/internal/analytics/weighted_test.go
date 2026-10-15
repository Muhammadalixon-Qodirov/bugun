package analytics

import "testing"

func TestWeightedQuantile(t *testing.T) {
	cases := []struct {
		name    string
		values  []float64
		weights []float64
		q       float64
		want    float64
	}{
		{"equal weights, odd count", []float64{5, 1, 3}, nil, 0.5, 3},
		{"equal weights, even count takes the lower middle", []float64{1, 2, 3, 4}, nil, 0.5, 2},
		{"fresh price outweighs stale ones", []float64{4000, 4200, 5000}, []float64{0.2, 0.2, 0.8}, 0.5, 5000},
		{"zero weight is ignored", []float64{1, 100, 2}, []float64{1, 0, 1}, 0.5, 1},
		{"first quartile", []float64{1, 2, 3, 4, 5, 6, 7, 8}, nil, 0.25, 2},
		{"third quartile", []float64{1, 2, 3, 4, 5, 6, 7, 8}, nil, 0.75, 6},
		{"nothing weighs", []float64{1, 2}, []float64{0, 0}, 0.5, 0},
		{"empty", nil, nil, 0.5, 0},
	}
	for _, c := range cases {
		if got := WeightedQuantile(c.values, c.weights, c.q); got != c.want {
			t.Errorf("%s: WeightedQuantile = %v, want %v", c.name, got, c.want)
		}
	}
}
