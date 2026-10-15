package analytics

import "testing"

func TestPositionOf(t *testing.T) {
	peers := []float64{4000, 4100, 4200, 4200, 4300, 4400, 4500, 4600, 4700, 4800, 4900}

	cases := []struct {
		name  string
		peers []float64
		price float64
		want  Position
	}{
		{"cheapest", peers, 3900, Position{Enough: true, Rank: 1, Total: 12, Median: 4400, DiffPct: -11}},
		{"ninth of twelve", peers, 4650, Position{Enough: true, Rank: 9, Total: 12, Median: 4400, DiffPct: 6}},
		{"equal prices share a rank", peers, 4200, Position{Enough: true, Rank: 3, Total: 12, Median: 4400, DiffPct: -5}},
		{"most expensive", peers, 5000, Position{Enough: true, Rank: 12, Total: 12, Median: 4400, DiffPct: 14}},
		{"too few others", []float64{4000, 4100}, 4200, Position{Total: 3}},
		{"nobody else", nil, 4200, Position{Total: 1}},
	}
	for _, c := range cases {
		if got := PositionOf(c.peers, nil, c.price); got != c.want {
			t.Errorf("%s: PositionOf = %+v, want %+v", c.name, got, c.want)
		}
	}
}

func TestPositionUsesWeightedMedian(t *testing.T) {
	// Two stale cheap prices and one fresh expensive price: the market median
	// follows the fresh one, the rank still counts every seller.
	got := PositionOf([]float64{4000, 4100, 5000}, []float64{0.2, 0.2, 0.8}, 4800)
	want := Position{Enough: true, Rank: 3, Total: 4, Median: 5000, DiffPct: -4}
	if got != want {
		t.Errorf("PositionOf = %+v, want %+v", got, want)
	}
}
