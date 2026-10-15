package ml

import (
	"math"
	"math/rand/v2"
)

// Isolation Forest (Liu, Ting, Zhou 2008): an outlier is easy to cut off from
// the rest with a few random splits, so its average path in random trees is
// short. It needs no labels. It finds lone points, not a tight group of
// similar ones: a group is a pattern, not an outlier.

const (
	forestTrees  = 100
	forestSample = 256
)

type isoNode struct {
	feature     int
	split       float64
	left, right *isoNode
	size        int // leaves: points that reached them
}

// Forest is a trained Isolation Forest.
type Forest struct {
	trees  []*isoNode
	sample int
}

// NewForest trains 100 trees on up to 256 rows each.
func NewForest(rows [][]float64) *Forest {
	r := newRand()
	sample := min(forestSample, len(rows))
	limit := int(math.Ceil(math.Log2(float64(max(sample, 2)))))
	f := &Forest{sample: sample}
	idx := make([]int, len(rows))
	for i := range idx {
		idx[i] = i
	}
	for t := 0; t < forestTrees; t++ {
		r.Shuffle(len(idx), func(i, j int) { idx[i], idx[j] = idx[j], idx[i] })
		pick := make([][]float64, sample)
		for i := range pick {
			pick[i] = rows[idx[i]]
		}
		f.trees = append(f.trees, grow(r, pick, 0, limit))
	}
	return f
}

func grow(r *rand.Rand, rows [][]float64, depth, limit int) *isoNode {
	if depth >= limit || len(rows) <= 1 {
		return &isoNode{size: len(rows)}
	}
	d := len(rows[0])
	// Pick a feature that still varies among these rows.
	for _, j := range r.Perm(d) {
		lo, hi := rows[0][j], rows[0][j]
		for _, x := range rows {
			lo, hi = math.Min(lo, x[j]), math.Max(hi, x[j])
		}
		if lo == hi {
			continue
		}
		split := lo + r.Float64()*(hi-lo)
		var left, right [][]float64
		for _, x := range rows {
			if x[j] < split {
				left = append(left, x)
			} else {
				right = append(right, x)
			}
		}
		return &isoNode{feature: j, split: split, left: grow(r, left, depth+1, limit), right: grow(r, right, depth+1, limit)}
	}
	return &isoNode{size: len(rows)}
}

// avgPath is the average path length of an unsuccessful search in a binary
// search tree of n points: the expected depth still to go in a leaf of n.
func avgPath(n int) float64 {
	if n <= 1 {
		return 0
	}
	if n == 2 {
		return 1
	}
	h := math.Log(float64(n-1)) + 0.5772156649
	return 2*h - 2*float64(n-1)/float64(n)
}

func pathLength(n *isoNode, x []float64, depth int) float64 {
	for n.left != nil {
		if x[n.feature] < n.split {
			n = n.left
		} else {
			n = n.right
		}
		depth++
	}
	return float64(depth) + avgPath(n.size)
}

// Score is the anomaly score in 0–1: near 1 is easily isolated (an outlier),
// around 0.5 or below is ordinary.
func (f *Forest) Score(x []float64) float64 {
	if len(f.trees) == 0 {
		return 0
	}
	sum := 0.0
	for _, t := range f.trees {
		sum += pathLength(t, x, 0)
	}
	mean := sum / float64(len(f.trees))
	c := avgPath(f.sample)
	if c == 0 {
		return 0
	}
	return math.Pow(2, -mean/c)
}
