package analytics

import "testing"

func TestScreen(t *testing.T) {
	market := []float64{4000, 4100, 4200, 4200, 4300, 4400}
	few := []float64{4000, 4200, 4400}

	cases := []struct {
		name  string
		check Check
		price float64
		want  Reason
	}{
		{"normal price among many sellers", Check{Peers: market}, 4600, ReasonNone},
		{"missing zero among many sellers", Check{Peers: market}, 600, ReasonFarFromMarket},
		{"extra zero against own last price", Check{Peers: market, Previous: 4200}, 42000, ReasonJumpFromPrevious},
		{"missing zero against own last price", Check{Previous: 4200}, 420, ReasonJumpFromPrevious},
		{"normal change from own last price", Check{Peers: market, Previous: 4000}, 4400, ReasonNone},
		{"jump just under the limit is allowed", Check{Previous: 1000}, 4900, ReasonNone},
		{"no sellers, close to reference", Check{Reference: 4200}, 5000, ReasonNone},
		{"no sellers, far from reference", Check{Reference: 4200}, 9000, ReasonFarFromReference},
		{"few sellers use the reference first", Check{Peers: few, Reference: 9000}, 9500, ReasonNone},
		{"few sellers without reference use their median", Check{Peers: few}, 9500, ReasonFarFromMarket},
		{"many sellers win over the reference", Check{Peers: market, Reference: 9000}, 4300, ReasonNone},
		{"nothing to compare with", Check{}, 123456, ReasonNone},
		{"zero price is left to input validation", Check{Peers: market, Previous: 4200}, 0, ReasonNone},
	}
	for _, c := range cases {
		if got := Screen(c.check, c.price); got != c.want {
			t.Errorf("%s: Screen = %q, want %q", c.name, got, c.want)
		}
	}
}

// Spec §16.3: at least 90% of deliberately broken prices must be caught.
func TestScreenCatchesTypos(t *testing.T) {
	market := []float64{3800, 3900, 4000, 4100, 4200, 4200, 4300, 4400, 4500}
	caught, total := 0, 0
	for _, real := range market {
		for _, typo := range []float64{real * 10, real / 10, real * 100, real / 100} {
			total++
			if Screen(Check{Peers: market}, typo) != ReasonNone {
				caught++
			}
		}
	}
	if caught*100 < total*90 {
		t.Errorf("caught %d of %d typos, want at least 90%%", caught, total)
	}
	for _, real := range market {
		if r := Screen(Check{Peers: market}, real); r != ReasonNone {
			t.Errorf("real price %v flagged as %q", real, r)
		}
	}
}
