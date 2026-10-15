package analytics

import (
	"math"
	"testing"
)

func TestPressure(t *testing.T) {
	cases := []struct {
		name  string
		tNow  float64
		tPast []float64
		sNow  float64
		sPast []float64
		value float64
		zone  string
		ok    bool
	}{
		{"steady market", 100, []float64{100, 100, 100, 100}, 10, []float64{10, 10, 10, 10}, 1, ZoneBalanced, true},
		{"demand doubled", 200, []float64{100, 100, 100, 100}, 10, []float64{10, 10, 10, 10}, 2, ZoneShortage, true},
		{"supply doubled", 100, []float64{100, 100, 100, 100}, 20, []float64{10, 10, 10, 10}, 0.5, ZoneSurplus, true},
		{"empty weeks are skipped in the average", 100, []float64{100, 0, 100, 0}, 10, []float64{10, 0, 10, 0}, 1, ZoneBalanced, true},
		{"one earlier week is not enough", 100, []float64{100, 0, 0, 0}, 10, []float64{10, 10, 10, 10}, 0, "", false},
		{"no sellers now", 100, []float64{100, 100}, 0, []float64{10, 10}, 0, "", false},
		{"no demand now is a surplus", 0, []float64{100, 100}, 10, []float64{10, 10}, 0, ZoneSurplus, true},
	}
	for _, c := range cases {
		v, z, ok := Pressure(c.tNow, c.tPast, c.sNow, c.sPast)
		if ok != c.ok || z != c.zone || math.Abs(v-c.value) > 1e-9 {
			t.Errorf("%s: Pressure = %v %q %v, want %v %q %v", c.name, v, z, ok, c.value, c.zone, c.ok)
		}
	}
}
