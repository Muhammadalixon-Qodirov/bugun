package insight

import (
	"math"
	"math/rand/v2"
	"testing"
)

// Two clear price levels become two groups, cheapest first; the nearest
// viloyatlar of a member are from its own group.
func TestClusterProfiles(t *testing.T) {
	r := rand.New(rand.NewPCG(9, 9))
	ps := &profileSet{}
	for j := 0; j < 12; j++ {
		cat := []string{"sabzavot", "meva", "gosht"}[j%3]
		ps.products = append(ps.products, ProductRef{ID: int16(j + 1), Name: "p", Category: cat})
	}
	for i := 0; i < 12; i++ {
		level := -0.1
		if i >= 6 {
			level = 0.1
		}
		v := make([]float64, len(ps.products))
		for j := range v {
			v[j] = level + r.NormFloat64()*0.01
		}
		ps.provinces = append(ps.provinces, ProvinceRef{ID: int16(100 + i)})
		ps.vectors = append(ps.vectors, v)
	}
	c := clusterProfiles(ps)
	if c.Quality != "strong" || len(c.Groups) != 2 {
		t.Fatalf("clusters %+v", c)
	}
	// Stricter rules: too few viloyatlar to group, and one named product each.
	if strict := (ClusterRules{ProfileCoverage: 0.7, ProvinceCoverage: 0.5, MinProfiles: 13, MinProfileItems: 5, MaxClusters: 5, Top: 3}).group(ps); strict.K != 0 || len(strict.Groups) != 0 {
		t.Errorf("MinProfiles 13 still grouped: %+v", strict)
	}
	one := DefaultClusterRules
	one.Top = 1
	for _, g := range one.group(ps).Groups {
		if len(g.Cheaper)+len(g.Dearer) > 1 {
			t.Errorf("Top 1 named %d products", len(g.Cheaper)+len(g.Dearer))
		}
	}
	if c.Groups[0].Level > -0.08 || c.Groups[1].Level < 0.08 || len(c.Groups[0].Provinces) != 6 {
		t.Errorf("groups %+v", c.Groups)
	}
	if len(c.Groups[0].Cheaper) != 3 || len(c.Groups[0].Dearer) != 0 {
		t.Errorf("cheap group products %+v / %+v", c.Groups[0].Cheaper, c.Groups[0].Dearer)
	}
	sim := similarTo(ps, c, 100)
	if len(sim) != 3 {
		t.Fatalf("similar %+v", sim)
	}
	for _, s := range sim {
		if !s.SameGroup || s.Province.ID >= 106 || s.Gap > 0.05 {
			t.Errorf("similar %+v", s)
		}
	}

	// One level for all: no clear groups, and no groups are invented.
	for i := range ps.vectors {
		for j := range ps.vectors[i] {
			ps.vectors[i][j] = r.NormFloat64() * 0.05
		}
	}
	if c := clusterProfiles(ps); c.Quality == "strong" {
		t.Errorf("noise grouped strongly: %+v", c)
	}
	// Too few viloyatlar.
	ps.vectors, ps.provinces = ps.vectors[:4], ps.provinces[:4]
	if c := clusterProfiles(ps); c.K != 0 || len(c.Groups) != 0 || math.IsNaN(c.Silhouette) {
		t.Errorf("few viloyatlar %+v", c)
	}
}
