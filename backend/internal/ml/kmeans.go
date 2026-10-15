package ml

import "math"

// K-Means always returns k groups, even when the data has none; the
// silhouette says whether the groups are real, so BestK only accepts a
// grouping above MinSilhouette.

// Silhouette bands (Rousseeuw 1987): from MinSilhouette the groups are weak
// but usable, from StrongSilhouette clear. Below MinSilhouette they overlap
// too much to name them.
const (
	MinSilhouette    = 0.25
	StrongSilhouette = 0.5
)

// Quality words a silhouette: strong, weak or none.
func Quality(silhouette float64) string {
	switch {
	case silhouette >= StrongSilhouette:
		return "strong"
	case silhouette >= MinSilhouette:
		return "weak"
	}
	return "none"
}

const kmeansRestarts = 20

// Clusters is a grouping of the rows.
type Clusters struct {
	K          int
	Labels     []int       // cluster of each row
	Centers    [][]float64 // mean of each cluster
	Silhouette float64     // mean silhouette, −1..1
}

func sqDist(a, b []float64) float64 {
	s := 0.0
	for i := range a {
		d := a[i] - b[i]
		s += d * d
	}
	return s
}

// KMeans groups rows into k clusters with k-means++ starts, keeping the best
// of 20 restarts by within-cluster distance.
func KMeans(rows [][]float64, k int) Clusters {
	r := newRand()
	best, bestCost := Clusters{}, math.Inf(1)
	n := len(rows)
	if k < 1 || n < k {
		return best
	}
	for rep := 0; rep < kmeansRestarts; rep++ {
		// k-means++: each next centre far from the ones already chosen.
		centers := [][]float64{append([]float64(nil), rows[r.IntN(n)]...)}
		for len(centers) < k {
			d := make([]float64, n)
			total := 0.0
			for i, x := range rows {
				d[i] = math.Inf(1)
				for _, c := range centers {
					d[i] = math.Min(d[i], sqDist(x, c))
				}
				total += d[i]
			}
			pick := 0
			if total > 0 {
				u := r.Float64() * total
				for i := range d {
					if u -= d[i]; u <= 0 {
						pick = i
						break
					}
				}
			} else {
				pick = r.IntN(n)
			}
			centers = append(centers, append([]float64(nil), rows[pick]...))
		}
		labels := make([]int, n)
		for iter := 0; iter < 100; iter++ {
			changed := false
			for i, x := range rows {
				bi, bd := 0, math.Inf(1)
				for c := range centers {
					if d := sqDist(x, centers[c]); d < bd {
						bi, bd = c, d
					}
				}
				if labels[i] != bi {
					labels[i], changed = bi, true
				}
			}
			for c := range centers {
				sum := make([]float64, len(rows[0]))
				cnt := 0
				for i, x := range rows {
					if labels[i] == c {
						for j := range x {
							sum[j] += x[j]
						}
						cnt++
					}
				}
				if cnt > 0 {
					for j := range sum {
						sum[j] /= float64(cnt)
					}
					centers[c] = sum
				}
			}
			if !changed && iter > 0 {
				break
			}
		}
		cost := 0.0
		for i, x := range rows {
			cost += sqDist(x, centers[labels[i]])
		}
		if cost < bestCost-1e-12 {
			bestCost = cost
			best = Clusters{K: k, Labels: labels, Centers: centers}
		}
	}
	best.Silhouette = Silhouette(rows, best.Labels, k)
	return best
}

// Silhouette is the mean over rows of (b−a)/max(a,b), where a is the mean
// distance to the row's own cluster and b to the nearest other cluster. A row
// alone in its cluster counts 0.
func Silhouette(rows [][]float64, labels []int, k int) float64 {
	n := len(rows)
	if n < 2 || k < 2 {
		return 0
	}
	total := 0.0
	for i := range rows {
		sums := make([]float64, k)
		cnts := make([]int, k)
		for j := range rows {
			if i == j {
				continue
			}
			sums[labels[j]] += math.Sqrt(sqDist(rows[i], rows[j]))
			cnts[labels[j]]++
		}
		own := labels[i]
		if cnts[own] == 0 {
			continue
		}
		a := sums[own] / float64(cnts[own])
		b := math.Inf(1)
		for c := 0; c < k; c++ {
			if c != own && cnts[c] > 0 {
				b = math.Min(b, sums[c]/float64(cnts[c]))
			}
		}
		if math.IsInf(b, 1) || math.Max(a, b) == 0 {
			continue
		}
		total += (b - a) / math.Max(a, b)
	}
	return total / float64(n)
}

// BestK tries k = 2..maxK and keeps the grouping with the highest silhouette.
// ok is false when none reaches MinSilhouette: the data has no clear groups.
func BestK(rows [][]float64, maxK int) (Clusters, bool) {
	best := Clusters{Silhouette: math.Inf(-1)}
	for k := 2; k <= maxK && k < len(rows); k++ {
		if c := KMeans(rows, k); c.Silhouette > best.Silhouette {
			best = c
		}
	}
	return best, best.K > 0 && best.Silhouette >= MinSilhouette
}
