package ml

// TreeSHAP (Lundberg, Erion, Lee 2018): each feature's exact Shapley value
// for one prediction of a tree ensemble, in time polynomial in the depth
// instead of exponential in the features. "Exact" is for the trees' own
// conditional expectation (a feature left out follows both children by the
// training rows that went each way), and the values add up to the
// prediction minus the model's expected output. This is the shap library's
// tree_shap_recursive.

type pathElem struct {
	feature int
	zero    float64 // share of the rows that would pass without the feature
	one     float64 // 1 if the explained row passes, else 0
	weight  float64
}

func extendPath(p []pathElem, depth int, zero, one float64, feature int) {
	p[depth] = pathElem{feature: feature, zero: zero, one: one}
	if depth == 0 {
		p[0].weight = 1
	}
	for i := depth - 1; i >= 0; i-- {
		p[i+1].weight += one * p[i].weight * float64(i+1) / float64(depth+1)
		p[i].weight = zero * p[i].weight * float64(depth-i) / float64(depth+1)
	}
}

func unwindPath(p []pathElem, depth, index int) {
	one, zero := p[index].one, p[index].zero
	next := p[depth].weight
	for i := depth - 1; i >= 0; i-- {
		if one != 0 {
			tmp := p[i].weight
			p[i].weight = next * float64(depth+1) / (float64(i+1) * one)
			next = tmp - p[i].weight*zero*float64(depth-i)/float64(depth+1)
		} else {
			p[i].weight = p[i].weight * float64(depth+1) / (zero * float64(depth-i))
		}
	}
	for i := index; i < depth; i++ {
		p[i].feature, p[i].zero, p[i].one = p[i+1].feature, p[i+1].zero, p[i+1].one
	}
}

func unwoundPathSum(p []pathElem, depth, index int) float64 {
	one, zero := p[index].one, p[index].zero
	next := p[depth].weight
	total := 0.0
	if one != 0 {
		for i := depth - 1; i >= 0; i-- {
			tmp := next / (float64(i+1) * one)
			total += tmp
			next = p[i].weight - tmp*zero*float64(depth-i)
		}
	} else if zero != 0 {
		for i := depth - 1; i >= 0; i-- {
			total += p[i].weight / (zero * float64(depth-i))
		}
	}
	return total * float64(depth+1)
}

// shap adds the tree's contributions for x to phi.
func (t gbmTree) shap(x []float64, phi []float64) {
	maxDepth := 0
	var depthOf func(i, d int)
	depthOf = func(i, d int) {
		if d > maxDepth {
			maxDepth = d
		}
		if t[i].Feature >= 0 {
			depthOf(t[i].Left, d+1)
			depthOf(t[i].Right, d+1)
		}
	}
	depthOf(0, 0)
	// One block per level; each recursion copies its parent's path.
	store := make([]pathElem, (maxDepth+3)*(maxDepth+4))
	var rec func(node, depth, offset int, parent []pathElem, zero, one float64, feature int)
	rec = func(node, depth, offset int, parent []pathElem, zero, one float64, feature int) {
		path := store[offset : offset+depth+2]
		copy(path, parent[:depth])
		extendPath(path, depth, zero, one, feature)
		n := t[node]
		if n.Feature < 0 {
			for i := 1; i <= depth; i++ {
				w := unwoundPathSum(path, depth, i)
				e := path[i]
				phi[e.feature] += w * (e.one - e.zero) * n.Value
			}
			return
		}
		hot, cold := n.Right, n.Left
		v := x[n.Feature]
		if (v != v && n.MissingLeft) || (v == v && v < n.Threshold) {
			hot, cold = n.Left, n.Right
		}
		inZero, inOne := 1.0, 1.0
		d := depth
		for k := 0; k <= depth; k++ {
			if path[k].feature == n.Feature {
				inZero, inOne = path[k].zero, path[k].one
				unwindPath(path, depth, k)
				d--
				break
			}
		}
		next := offset + depth + 2
		rec(hot, d+1, next, path, t[hot].Cover/n.Cover*inZero, inOne, n.Feature)
		rec(cold, d+1, next, path, t[cold].Cover/n.Cover*inZero, 0, n.Feature)
	}
	rec(0, 0, 0, nil, 1, 1, -1)
}

// expected is the tree's mean output over its training rows.
func (t gbmTree) expected() float64 {
	sum := 0.0
	for _, n := range t {
		if n.Feature < 0 {
			sum += n.Value * n.Cover
		}
	}
	return sum / t[0].Cover
}

// Explain returns each feature's contribution to the prediction for x and
// the expected output they start from: Predict(x) = expected + Σ phi.
func (m *GBM) Explain(x []float64) (phi []float64, expected float64) {
	phi = make([]float64, m.Features)
	expected = m.Base
	for _, t := range m.Trees {
		t.shap(x, phi)
		expected += t.expected()
	}
	return phi, expected
}
