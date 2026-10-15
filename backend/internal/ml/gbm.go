package ml

import (
	"math"
	"sort"
	"sync"
)

// Gradient-boosted regression trees (PROGNOZ_ML.md §2), the LightGBM idea in
// plain Go: every feature is cut into at most 255 bins by its quantiles, and
// each tree is grown on histograms of those bins, so training is linear in
// the rows. The loss is Huber: a residual larger than Delta counts linearly,
// so a few sharp price jumps do not steer the whole model. A value may be
// missing (NaN); each split learns which side missing values go to. Rows and
// features are sampled per tree and training stops when the validation loss
// has not improved for EarlyStop trees. All randomness comes from Seed.

// GBMParams are the training settings; zero fields take DefaultGBMParams.
type GBMParams struct {
	Trees        int     // at most this many trees
	Depth        int     // depth of each tree
	LearningRate float64 // shrinkage of each tree's step
	MinLeaf      int     // rows a leaf needs at least
	Subsample    float64 // share of rows each tree sees
	ColSample    float64 // share of features each tree may split on
	Delta        float64 // Huber threshold, in the target's unit
	EarlyStop    int     // trees without validation gain before stopping
}

// DefaultGBMParams suit weekly log price changes (a residual of 0.05 ≈ 5%).
var DefaultGBMParams = GBMParams{Trees: 600, Depth: 5, LearningRate: 0.05, MinLeaf: 50, Subsample: 0.8, ColSample: 0.8, Delta: 0.05, EarlyStop: 30}

const (
	maxBins    = 255 // bin 255 is kept for missing values
	missingBin = 255
)

func (p GBMParams) filled() GBMParams {
	d := DefaultGBMParams
	if p.Trees > 0 {
		d.Trees = p.Trees
	}
	if p.Depth > 0 {
		d.Depth = p.Depth
	}
	if p.LearningRate > 0 {
		d.LearningRate = p.LearningRate
	}
	if p.MinLeaf > 0 {
		d.MinLeaf = p.MinLeaf
	}
	if p.Subsample > 0 && p.Subsample <= 1 {
		d.Subsample = p.Subsample
	}
	if p.ColSample > 0 && p.ColSample <= 1 {
		d.ColSample = p.ColSample
	}
	if p.Delta > 0 {
		d.Delta = p.Delta
	}
	if p.EarlyStop > 0 {
		d.EarlyStop = p.EarlyStop
	}
	return d
}

// gbmNode is one node of a tree; a leaf has Feature -1.
type gbmNode struct {
	Feature     int     // split feature, -1 for a leaf
	Threshold   float64 // value < Threshold goes left
	MissingLeft bool    // where missing values go
	Left, Right int     // child indexes
	Value       float64 // leaf output (already shrunk)
	Cover       float64 // training rows that reached the node (for TreeSHAP)
}

type gbmTree []gbmNode

// GBM is a trained model.
type GBM struct {
	Base       float64 // the starting prediction (median of the targets)
	Trees      []gbmTree
	Features   int
	Importance []float64 // total split gain per feature, summed to 1
	TrainLoss  float64
	ValidLoss  float64 // at the best number of trees; NaN without validation rows
}

// binned holds every feature's cut points and each row's bin.
type binned struct {
	edges [][]float64 // per feature: upper bounds of bins 0..k-2 (bin k-1 is the rest)
	bins  [][]uint8   // [feature][row]
}

func binFeatures(x [][]float64, features int) binned {
	b := binned{edges: make([][]float64, features), bins: make([][]uint8, features)}
	n := len(x)
	for f := 0; f < features; f++ {
		vals := make([]float64, 0, n)
		for _, r := range x {
			if v := r[f]; !math.IsNaN(v) {
				vals = append(vals, v)
			}
		}
		sort.Float64s(vals)
		var edges []float64
		if len(vals) > 0 {
			for k := 1; k < maxBins; k++ {
				e := vals[k*len(vals)/maxBins]
				if len(edges) == 0 || e > edges[len(edges)-1] {
					edges = append(edges, e)
				}
			}
			if len(edges) > 0 && edges[0] == vals[0] {
				edges = edges[1:] // no empty first bin
			}
		}
		b.edges[f] = edges
		col := make([]uint8, n)
		for i, r := range x {
			col[i] = binOf(r[f], edges)
		}
		b.bins[f] = col
	}
	return b
}

// binOf is the bin of v: the first edge greater than v, or missingBin.
func binOf(v float64, edges []float64) uint8 {
	if math.IsNaN(v) {
		return missingBin
	}
	return uint8(sort.Search(len(edges), func(i int) bool { return v < edges[i] }))
}

// huberGrad is the clipped residual: the step each row asks for.
func huberGrad(r, delta float64) float64 {
	return math.Max(-delta, math.Min(delta, r))
}

func huberLoss(r, delta float64) float64 {
	a := math.Abs(r)
	if a <= delta {
		return 0.5 * r * r
	}
	return delta * (a - 0.5*delta)
}

// TrainGBM fits the model on x → y and stops early on the validation rows
// (vx, vy; may be empty, then all trees are kept).
func TrainGBM(x [][]float64, y []float64, vx [][]float64, vy []float64, params GBMParams) *GBM {
	p := params.filled()
	n := len(x)
	if n == 0 {
		return &GBM{ValidLoss: math.NaN()}
	}
	features := len(x[0])
	m := &GBM{Base: Median(y), Features: features, Importance: make([]float64, features), ValidLoss: math.NaN()}
	b := binFeatures(x, features)
	pred := make([]float64, n)
	for i := range pred {
		pred[i] = m.Base
	}
	vpred := make([]float64, len(vx))
	for i := range vpred {
		vpred[i] = m.Base
	}
	r := newRand()
	grad := make([]float64, n)
	all := make([]int, n)
	for i := range all {
		all[i] = i
	}
	best, bestTrees, since := math.Inf(1), 0, 0
	gains := make([]float64, features)
	var bestGains []float64
	for t := 0; t < p.Trees; t++ {
		for i := range grad {
			grad[i] = huberGrad(y[i]-pred[i], p.Delta)
		}
		// Rows and features for this tree.
		r.Shuffle(n, func(i, j int) { all[i], all[j] = all[j], all[i] })
		rows := append([]int(nil), all[:max(1, int(float64(n)*p.Subsample))]...)
		cols := r.Perm(features)[:max(1, int(math.Round(float64(features)*p.ColSample)))]
		sort.Ints(cols)
		tree := growTree(b, grad, rows, cols, p, gains)
		m.Trees = append(m.Trees, tree)
		for i, row := range x {
			pred[i] += tree.predict(row)
		}
		if len(vx) == 0 {
			continue
		}
		loss := 0.0
		for i, row := range vx {
			vpred[i] += tree.predict(row)
			loss += huberLoss(vy[i]-vpred[i], p.Delta)
		}
		loss /= float64(len(vx))
		if loss < best-1e-12 {
			best, bestTrees, since = loss, len(m.Trees), 0
			bestGains = append(bestGains[:0], gains...)
		} else if since++; since >= p.EarlyStop {
			break
		}
	}
	if len(vx) > 0 {
		m.Trees = m.Trees[:bestTrees]
		m.ValidLoss = best
		gains = bestGains
		if gains == nil {
			gains = make([]float64, features)
		}
	}
	total := 0.0
	for _, g := range gains {
		total += g
	}
	for f := range gains {
		if total > 0 {
			m.Importance[f] = gains[f] / total
		}
	}
	loss := 0.0
	for i, row := range x {
		loss += huberLoss(y[i]-m.Predict(row), p.Delta)
	}
	m.TrainLoss = loss / float64(n)
	return m
}

type split struct {
	gain        float64
	feature     int
	bin         int // rows with bin < this go left
	missingLeft bool
}

// growTree grows one tree depth by depth on the histograms of grad.
func growTree(b binned, grad []float64, rows, cols []int, p GBMParams, gains []float64) gbmTree {
	tree := gbmTree{}
	var grow func(rows []int, depth int) int
	grow = func(rows []int, depth int) int {
		sum := 0.0
		for _, i := range rows {
			sum += grad[i]
		}
		id := len(tree)
		tree = append(tree, gbmNode{Feature: -1, Value: p.LearningRate * sum / float64(len(rows)), Cover: float64(len(rows))})
		if depth >= p.Depth || len(rows) < 2*p.MinLeaf {
			return id
		}
		s := bestSplit(b, grad, rows, cols, sum, p.MinLeaf)
		if s.gain <= 1e-12 {
			return id
		}
		var left, right []int
		for _, i := range rows {
			bin := int(b.bins[s.feature][i])
			if (bin == missingBin && s.missingLeft) || (bin != missingBin && bin < s.bin) {
				left = append(left, i)
			} else {
				right = append(right, i)
			}
		}
		gains[s.feature] += s.gain
		threshold := math.Inf(1)
		if edges := b.edges[s.feature]; s.bin-1 < len(edges) && s.bin >= 1 {
			threshold = edges[s.bin-1]
		}
		tree[id].Feature, tree[id].Threshold, tree[id].MissingLeft = s.feature, threshold, s.missingLeft
		l := grow(left, depth+1)
		rt := grow(right, depth+1)
		tree[id].Left, tree[id].Right = l, rt
		return id
	}
	grow(rows, 0)
	return tree
}

// bestSplit scans each sampled feature's histogram for the cut with the
// largest drop in squared error, trying missing values on either side.
func bestSplit(b binned, grad []float64, rows, cols []int, total float64, minLeaf int) split {
	results := make([]split, len(cols))
	var wg sync.WaitGroup
	n := float64(len(rows))
	parent := total * total / n
	for k, f := range cols {
		wg.Add(1)
		go func(k, f int) {
			defer wg.Done()
			var sums [256]float64
			var counts [256]int
			col := b.bins[f]
			for _, i := range rows {
				sums[col[i]] += grad[i]
				counts[col[i]]++
			}
			best := split{feature: f}
			nb := len(b.edges[f]) + 1
			missSum, missN := sums[missingBin], counts[missingBin]
			var ls float64
			var ln int
			for cut := 1; cut < nb; cut++ {
				ls += sums[cut-1]
				ln += counts[cut-1]
				for _, mLeft := range []bool{false, true} {
					l, lc := ls, ln
					if mLeft {
						l, lc = ls+missSum, ln+missN
					}
					rc := len(rows) - lc
					if lc < minLeaf || rc < minLeaf {
						continue
					}
					rs := total - l
					g := l*l/float64(lc) + rs*rs/float64(rc) - parent
					if g > best.gain {
						best = split{gain: g, feature: f, bin: cut, missingLeft: mLeft}
					}
				}
			}
			results[k] = best
		}(k, f)
	}
	wg.Wait()
	best := split{}
	for _, s := range results {
		if s.gain > best.gain {
			best = s
		}
	}
	return best
}

func (t gbmTree) leaf(x []float64) int {
	i := 0
	for t[i].Feature >= 0 {
		n := t[i]
		v := x[n.Feature]
		if math.IsNaN(v) {
			if n.MissingLeft {
				i = n.Left
			} else {
				i = n.Right
			}
		} else if v < n.Threshold {
			i = n.Left
		} else {
			i = n.Right
		}
	}
	return i
}

func (t gbmTree) predict(x []float64) float64 { return t[t.leaf(x)].Value }

// Predict is the model's output for one row.
func (m *GBM) Predict(x []float64) float64 {
	out := m.Base
	for _, t := range m.Trees {
		out += t.predict(x)
	}
	return out
}
