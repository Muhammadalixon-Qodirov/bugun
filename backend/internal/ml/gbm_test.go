package ml

import (
	"math"
	"math/rand/v2"
	"testing"
	"time"
)

// pricesLike makes rows like the forecast's: frost days, last week's change,
// a holiday flag and noise columns; the target is next week's log change.
func pricesLike(n int, seed uint64, noise float64) ([][]float64, []float64) {
	r := rand.New(rand.NewPCG(seed, seed+1))
	x := make([][]float64, n)
	y := make([]float64, n)
	for i := range x {
		frost := float64(r.IntN(8))        // frost days last week
		last := r.NormFloat64() * 0.04     // last week's change
		holiday := float64(r.IntN(10) / 9) // a holiday soon, 1 in 10
		row := []float64{frost, last, holiday, r.Float64(), r.NormFloat64()}
		if r.IntN(5) == 0 {
			row[1] = math.NaN() // last week unknown for some series
		}
		x[i] = row
		t := 0.0
		if frost >= 4 {
			t += 0.08 // a hard frost week: +8%
		}
		if !math.IsNaN(row[1]) {
			t += 0.5 * row[1] // momentum
		}
		t += 0.05 * holiday
		y[i] = t + r.NormFloat64()*noise
	}
	return x, y
}

func TestGBMLearnsTheRules(t *testing.T) {
	x, y := pricesLike(20000, 1, 0.01)
	vx, vy := pricesLike(3000, 2, 0.01)
	started := time.Now()
	m := TrainGBM(x, y, vx, vy, GBMParams{})
	took := time.Since(started)
	tx, ty := pricesLike(3000, 3, 0.01)
	var sse, sst, mean float64
	for _, v := range ty {
		mean += v / float64(len(ty))
	}
	for i, row := range tx {
		d := m.Predict(row) - ty[i]
		sse += d * d
		sst += (ty[i] - mean) * (ty[i] - mean)
	}
	if r2 := 1 - sse/sst; r2 < 0.85 {
		t.Errorf("R² %.3f on new rows (trees %d)", r2, len(m.Trees))
	}
	// The frost effect is learned: same row, frost 6 against frost 0.
	base := []float64{0, 0, 0, 0.5, 0}
	hard := []float64{6, 0, 0, 0.5, 0}
	if d := m.Predict(hard) - m.Predict(base); math.Abs(d-0.08) > 0.015 {
		t.Errorf("frost effect %.3f, want ≈0.08", d)
	}
	if m.Importance[0] < m.Importance[3] || m.Importance[0] < m.Importance[4] {
		t.Errorf("noise columns more important than frost: %v", m.Importance)
	}
	// A row with last week unknown still gets a prediction.
	if v := m.Predict([]float64{6, math.NaN(), 0, 0.5, 0}); math.IsNaN(v) || math.Abs(v-0.08) > 0.03 {
		t.Errorf("missing value prediction %.3f", v)
	}
	if took > 20*time.Second {
		t.Errorf("training 20 000 rows took %s", took)
	}
	t.Logf("trees %d, valid loss %.6f, took %s", len(m.Trees), m.ValidLoss, took)
}

func TestGBMDoesNotLearnNoise(t *testing.T) {
	r := rand.New(rand.NewPCG(7, 8))
	mk := func(n int) ([][]float64, []float64) {
		x := make([][]float64, n)
		y := make([]float64, n)
		for i := range x {
			x[i] = []float64{r.Float64(), r.Float64(), r.NormFloat64()}
			y[i] = r.NormFloat64() * 0.03
		}
		return x, y
	}
	x, y := mk(5000)
	vx, vy := mk(1000)
	m := TrainGBM(x, y, vx, vy, GBMParams{})
	if len(m.Trees) > 60 {
		t.Errorf("kept %d trees on pure noise; early stopping should cut it short", len(m.Trees))
	}
	tx, ty := mk(1000)
	var modelErr, zeroErr float64
	for i, row := range tx {
		modelErr += math.Abs(m.Predict(row) - ty[i])
		zeroErr += math.Abs(ty[i])
	}
	if modelErr > zeroErr*1.02 {
		t.Errorf("worse than predicting nothing on noise: %.4f vs %.4f", modelErr, zeroErr)
	}
}

func TestGBMDeterministic(t *testing.T) {
	x, y := pricesLike(3000, 4, 0.02)
	vx, vy := pricesLike(500, 5, 0.02)
	a := TrainGBM(x, y, vx, vy, GBMParams{Trees: 80})
	b := TrainGBM(x, y, vx, vy, GBMParams{Trees: 80})
	row := []float64{5, 0.01, 1, 0.3, -1}
	if a.Predict(row) != b.Predict(row) || len(a.Trees) != len(b.Trees) {
		t.Errorf("two runs differ: %v vs %v", a.Predict(row), b.Predict(row))
	}
	if m := TrainGBM(nil, nil, nil, nil, GBMParams{}); m.Predict(nil) != 0 {
		t.Error("an empty model predicts 0")
	}
}
