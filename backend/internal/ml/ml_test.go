package ml

import (
	"math"
	"math/rand/v2"
	"testing"
)

func TestRobustZMasking(t *testing.T) {
	// The mean-based z of 100 in [4 5 5 6 100] is only 2; the robust one is large.
	z, ok := RobustZ(100, []float64{4, 5, 5, 6, 100})
	if !ok || z < 50 {
		t.Fatalf("robust z = %v %v, want > 50", z, ok)
	}
	if _, ok := RobustZ(1, []float64{1, 2, 3}); ok {
		t.Fatal("fewer than 5 values must not score")
	}
	if _, ok := RobustZ(1, []float64{2, 2, 2, 2, 2}); ok {
		t.Fatal("no spread must not score")
	}
	if m := Median([]float64{3, 1, 2, 10}); m != 2.5 {
		t.Fatalf("median = %v", m)
	}
}

func TestShapleyAddsUp(t *testing.T) {
	// price = 100 × (1+a) × (1+b), then capped at 110.
	shares := []float64{0.08, 0.06}
	v := func(m uint) float64 {
		p := 100.0
		for i, s := range shares {
			if m&(1<<i) != 0 {
				p *= 1 + s
			}
		}
		return math.Min(p, 110)
	}
	phi := Shapley(2, v)
	if got := phi[0] + phi[1]; math.Abs(got-(v(3)-v(0))) > 1e-9 {
		t.Fatalf("sum %v, want %v", got, v(3)-v(0))
	}
	if phi[0] <= phi[1] {
		t.Fatalf("the bigger factor must get more: %v", phi)
	}
	// Symmetric players get equal values.
	sym := Shapley(3, func(m uint) float64 { return float64(popcount(m) * popcount(m)) })
	if math.Abs(sym[0]-sym[1]) > 1e-9 || math.Abs(sym[1]-sym[2]) > 1e-9 || math.Abs(sym[0]*3-9) > 1e-9 {
		t.Fatalf("symmetric values %v", sym)
	}
}

func TestForestFindsLoneOutlier(t *testing.T) {
	r := rand.New(rand.NewPCG(1, 2))
	rows := make([][]float64, 0, 201)
	for i := 0; i < 200; i++ {
		rows = append(rows, []float64{r.NormFloat64(), r.NormFloat64(), r.NormFloat64()})
	}
	rows = append(rows, []float64{8, -8, 8})
	f := NewForest(rows)
	out := f.Score(rows[200])
	normal := f.Score([]float64{0, 0, 0})
	if out < 0.65 || normal > 0.5 || out <= normal {
		t.Fatalf("outlier %.3f, normal %.3f", out, normal)
	}
	if again := NewForest(rows).Score(rows[200]); again != out {
		t.Fatalf("not deterministic: %v vs %v", again, out)
	}
}

func TestKMeansAndSilhouette(t *testing.T) {
	r := rand.New(rand.NewPCG(3, 4))
	var rows [][]float64
	for _, c := range [][2]float64{{0, 0}, {5, 5}, {0, 6}} {
		for i := 0; i < 10; i++ {
			rows = append(rows, []float64{c[0] + r.NormFloat64()*0.3, c[1] + r.NormFloat64()*0.3})
		}
	}
	best, ok := BestK(rows, 5)
	if !ok || best.K != 3 || best.Silhouette < 0.7 {
		t.Fatalf("k=%d silhouette=%.2f ok=%v", best.K, best.Silhouette, ok)
	}
	for g := 0; g < 3; g++ {
		for i := g * 10; i < g*10+10; i++ {
			if best.Labels[i] != best.Labels[g*10] {
				t.Fatalf("group %d split", g)
			}
		}
	}
	if Quality(best.Silhouette) != "strong" {
		t.Fatalf("clear groups: %s", Quality(best.Silhouette))
	}
	// Uniform noise never looks like clear groups.
	var noise [][]float64
	for i := 0; i < 40; i++ {
		noise = append(noise, []float64{r.Float64(), r.Float64(), r.Float64(), r.Float64()})
	}
	if c, _ := BestK(noise, 5); Quality(c.Silhouette) == "strong" {
		t.Fatalf("noise grouped: k=%d silhouette=%.2f", c.K, c.Silhouette)
	}
}

func TestLocalMoranHotSpot(t *testing.T) {
	// A 5×5 grid, rook neighbours: the top-left 3×3 block is dear.
	n := 5
	var x []float64
	var nb [][]int
	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			v := 0.0
			if i < 3 && j < 3 {
				v = 10
			}
			x = append(x, v+float64((i*7+j*3)%5)*0.1)
			var ns []int
			for _, d := range [][2]int{{-1, 0}, {1, 0}, {0, -1}, {0, 1}} {
				a, b := i+d[0], j+d[1]
				if a >= 0 && a < n && b >= 0 && b < n {
					ns = append(ns, a*n+b)
				}
			}
			nb = append(nb, ns)
		}
	}
	spots := LocalMoran(x, nb)
	if spots[1*n+1].Kind != "hot" {
		t.Fatalf("centre of the dear block: %+v", spots[1*n+1])
	}
	if spots[4*n+4].Kind != "" && spots[4*n+4].Kind != "cold" {
		t.Fatalf("far corner: %+v", spots[4*n+4])
	}
	if spots[0].I <= 0 {
		t.Fatalf("corner of the block must be positive: %+v", spots[0])
	}
}

func TestStandardize(t *testing.T) {
	s := Standardize([][]float64{{1, 100}, {2, 200}, {3, 300}, {4, 400}, {5, 5000}})
	if s[2][0] != 0 || s[2][1] != 0 {
		t.Fatalf("median row must be 0: %v", s[2])
	}
	if math.Abs(s[0][0]-s[0][1]) > 1e-9 {
		t.Fatalf("scale must not depend on the unit: %v", s[0])
	}
}
