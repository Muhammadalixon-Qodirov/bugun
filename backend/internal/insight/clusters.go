package insight

import (
	"context"
	"fmt"
	"math"
	"sort"

	"bugun/internal/ml"
)

// Similar viloyatlar (ML_REJA.md 6, HUDUD's ml/clustering.py): each viloyat's
// price profile is how every common product's price there compares with the
// country (log ratio, last 14 days). K-Means groups the profiles averaged by
// category, which keeps a hundred noisy products from hiding the pattern,
// for k = 2..5 and keeps the k with the best silhouette; without a clear
// grouping it says so instead of inventing groups.

// ClusterRules are the grouping's parameters (config: CLUSTER_*).
type ClusterRules struct {
	// ProfileCoverage: a product enters the profile when at least this share
	// of viloyatlar have its price; a viloyat needs ProvinceCoverage of those
	// products.
	ProfileCoverage  float64
	ProvinceCoverage float64
	MinProfiles      int     // viloyatlar with a profile needed to group
	MinProfileItems  int     // products in the profile needed to group
	MaxClusters      int     // the largest k tried
	Top              int     // products named per group, similar viloyatlar listed
	NotableGap       float64 // a product must differ by this much to be named in a group
}

// DefaultClusterRules: products priced in 70% of viloyatlar, viloyatlar with
// half of them, at least 6 profiles of 5 products, k up to 5, top 3, 3% gap.
var DefaultClusterRules = ClusterRules{ProfileCoverage: 0.7, ProvinceCoverage: 0.5, MinProfiles: 6, MinProfileItems: 5,
	MaxClusters: 5, Top: 3, NotableGap: 0.03}

type ProductDiff struct {
	Product ProductRef `json:"product"`
	Diff    float64    `json:"diff" doc:"The group's price against the country: -0.12 = 12% cheaper"`
}

type ClusterGroup struct {
	ID        int           `json:"id"`
	Provinces []ProvinceRef `json:"provinces"`
	Level     float64       `json:"level" doc:"The group's prices against the country overall: -0.05 = 5% cheaper"`
	Cheaper   []ProductDiff `json:"cheaper" doc:"Up to 3 products the group sells cheapest"`
	Dearer    []ProductDiff `json:"dearer" doc:"Up to 3 products it sells dearest"`
}

type Clusters struct {
	Trade      string         `json:"trade"`
	Quality    string         `json:"quality" enum:"strong,weak,none" doc:"strong: silhouette ≥ 0.5; weak: ≥ 0.25; none: no clear groups, groups is empty"`
	Silhouette float64        `json:"silhouette" doc:"−1..1, how well separated the groups are"`
	K          int            `json:"k" doc:"Number of groups tried best; 0 without enough data"`
	Products   int            `json:"products" doc:"Products in the price profile"`
	Groups     []ClusterGroup `json:"groups"`
	Excluded   []ProvinceRef  `json:"excluded" doc:"Viloyatlar with too few prices for a profile"`
	Note       string         `json:"note"`
	DemoShare  float64        `json:"demo_share" doc:"Share of the prices in the profiles that are demo data (demo sellers or generated synthetic history), 0–1; 0 = all live. Show a Demo mark above 0"`
}

// SimilarProvince is a viloyat whose prices move like this one's.
type SimilarProvince struct {
	Province  ProvinceRef `json:"province"`
	Gap       float64     `json:"gap" doc:"Typical price difference over the common products: 0.04 ≈ 4%"`
	SameGroup bool        `json:"same_group" doc:"In the same group of /v1/analytics/clusters"`
}

type profileSet struct {
	provinces []ProvinceRef
	vectors   [][]float64
	products  []ProductRef
	excluded  []ProvinceRef
	demo      float64 // share of demo prices in the vectors
}

// profiles builds the price profile of every viloyat with enough prices.
func (s *Service) profiles(ctx context.Context, trade string) (*profileSet, error) {
	products, err := s.products(ctx)
	if err != nil {
		return nil, err
	}
	mx, err := s.matrix(ctx, trade)
	if err != nil {
		return nil, err
	}
	provinces, err := s.q.ListProvinces(ctx)
	if err != nil {
		return nil, err
	}
	var ids []int16
	for id, cells := range mx {
		if float64(len(cells)) >= s.clusterRules.ProfileCoverage*float64(len(provinces)) {
			ids = append(ids, id)
		}
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	ps := &profileSet{excluded: []ProvinceRef{}}
	for _, id := range ids {
		p := products[id]
		ps.products = append(ps.products, ProductRef{ID: id, Slug: p.slug, Name: p.name, Category: p.cat, Unit: p.unt})
	}
	var demoSum, offerSum float64
	for _, pv := range provinces {
		ref := ProvinceRef{ID: pv.ID, Name: pv.Name}
		v := make([]float64, len(ids))
		have := 0
		var demo, offers float64
		for j, id := range ids {
			if c, ok := mx[id][pv.ID]; ok && c.median > 0 {
				if country := countryOf(mx[id]); country > 0 {
					v[j] = math.Log(c.median / country)
					have++
					demo += c.demo * float64(c.offers)
					offers += float64(c.offers)
				}
			}
		}
		if len(ids) == 0 || float64(have) < s.clusterRules.ProvinceCoverage*float64(len(ids)) {
			ps.excluded = append(ps.excluded, ref)
			continue
		}
		demoSum += demo
		offerSum += offers
		ps.provinces = append(ps.provinces, ref)
		ps.vectors = append(ps.vectors, v)
	}
	if offerSum > 0 {
		ps.demo = math.Round(demoSum/offerSum*100) / 100
	}
	return ps, nil
}

// Clusters groups the viloyatlar by their price profile.
func (s *Service) Clusters(ctx context.Context, trade string) (*Clusters, error) {
	key := "clusters|" + trade
	if v, ok := s.cached(key); ok {
		return v.(*Clusters), nil
	}
	ps, err := s.profiles(ctx, trade)
	if err != nil {
		return nil, err
	}
	c := s.clusterRules.group(ps)
	c.Trade = trade
	c.DemoShare = ps.demo
	s.store(key, c)
	return c, nil
}

func clusterProfiles(ps *profileSet) *Clusters { return DefaultClusterRules.group(ps) }

// group is clusterProfiles under these rules.
func (cr ClusterRules) group(ps *profileSet) *Clusters {
	c := &Clusters{Quality: "none", Products: len(ps.products), Groups: []ClusterGroup{}, Excluded: ps.excluded}
	if len(ps.vectors) < cr.MinProfiles || len(ps.products) < cr.MinProfileItems {
		c.Note = fmt.Sprintf("Guruhlash uchun maʼlumot yetarli emas: kamida %d viloyatda %d ta umumiy mahsulot narxi kerak.", cr.MinProfiles, cr.MinProfileItems)
		return c
	}
	best, ok := ml.BestK(byCategory(ps), min(cr.MaxClusters, len(ps.vectors)-1))
	c.K, c.Silhouette, c.Quality = best.K, round3(best.Silhouette), ml.Quality(best.Silhouette)
	if !ok {
		c.Note = "Viloyatlar narx profili boʻyicha aniq guruhlarga boʻlinmaydi: farqlar uzluksiz."
		return c
	}
	c.Note = "Guruhlar oxirgi 14 kun narxlaridan tuzilgan. Bir guruhdagi viloyatlarda mahsulotlar mamlakatga nisbatan oʻxshash darajada arzon yoki qimmat."
	for g := 0; g < best.K; g++ {
		grp := ClusterGroup{ID: g + 1, Cheaper: []ProductDiff{}, Dearer: []ProductDiff{}}
		mean := make([]float64, len(ps.products))
		n := 0
		for i, l := range best.Labels {
			if l != g {
				continue
			}
			grp.Provinces = append(grp.Provinces, ps.provinces[i])
			for j, x := range ps.vectors[i] {
				mean[j] += x
			}
			n++
		}
		if n == 0 {
			continue
		}
		total := 0.0
		order := make([]int, len(mean))
		for j := range mean {
			mean[j] /= float64(n)
			total += mean[j] / float64(len(mean))
			order[j] = j
		}
		grp.Level = round3(math.Exp(total) - 1)
		sort.SliceStable(order, func(a, b int) bool { return mean[order[a]] < mean[order[b]] })
		for _, j := range order {
			if d := math.Exp(mean[j]) - 1; d < -cr.NotableGap && len(grp.Cheaper) < cr.Top {
				grp.Cheaper = append(grp.Cheaper, ProductDiff{ps.products[j], round3(d)})
			}
		}
		for k := len(order) - 1; k >= 0; k-- {
			if d := math.Exp(mean[order[k]]) - 1; d > cr.NotableGap && len(grp.Dearer) < cr.Top {
				grp.Dearer = append(grp.Dearer, ProductDiff{ps.products[order[k]], round3(d)})
			}
		}
		c.Groups = append(c.Groups, grp)
	}
	sort.SliceStable(c.Groups, func(a, b int) bool { return c.Groups[a].Level < c.Groups[b].Level })
	for i := range c.Groups {
		c.Groups[i].ID = i + 1 // cheapest group first
	}
	return c
}

// byCategory averages each profile over the products of every category.
func byCategory(ps *profileSet) [][]float64 {
	cats := map[string]int{}
	var order []string
	for _, p := range ps.products {
		if _, ok := cats[p.Category]; !ok {
			cats[p.Category] = len(order)
			order = append(order, p.Category)
		}
	}
	out := make([][]float64, len(ps.vectors))
	for i, v := range ps.vectors {
		sum := make([]float64, len(order))
		n := make([]float64, len(order))
		for j, x := range v {
			c := cats[ps.products[j].Category]
			sum[c] += x
			n[c]++
		}
		for c := range sum {
			sum[c] /= n[c]
		}
		out[i] = sum
	}
	return out
}

// similarTo lists the 3 viloyatlar whose profile is nearest to province's.
func similarTo(ps *profileSet, c *Clusters, province int16) []SimilarProvince {
	return DefaultClusterRules.similar(ps, c, province)
}

// similar is similarTo under these rules: cr.Top viloyatlar.
func (cr ClusterRules) similar(ps *profileSet, c *Clusters, province int16) []SimilarProvince {
	me := -1
	for i, p := range ps.provinces {
		if p.ID == province {
			me = i
		}
	}
	out := []SimilarProvince{}
	if me < 0 {
		return out
	}
	group := map[int16]int{}
	for _, g := range c.Groups {
		for _, p := range g.Provinces {
			group[p.ID] = g.ID
		}
	}
	type near struct {
		i   int
		gap float64
	}
	var ns []near
	for i, v := range ps.vectors {
		if i == me {
			continue
		}
		sq := 0.0
		for j := range v {
			d := v[j] - ps.vectors[me][j]
			sq += d * d
		}
		ns = append(ns, near{i, math.Sqrt(sq / float64(len(v)))})
	}
	sort.Slice(ns, func(a, b int) bool { return ns[a].gap < ns[b].gap })
	for _, n := range ns[:min(cr.Top, len(ns))] {
		p := ps.provinces[n.i]
		g, ok := group[p.ID]
		out = append(out, SimilarProvince{Province: p, Gap: round3(math.Exp(n.gap) - 1), SameGroup: ok && g == group[province]})
	}
	return out
}
