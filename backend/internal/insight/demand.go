package insight

import (
	"context"
	"fmt"
	"math"
	"sort"

	"github.com/jackc/pgx/v5/pgtype"

	"bugun/internal/analytics"
)

// TOP demand (TAHLIL_REJA.md #21): what people look for most this week, and
// what they started looking for.

// minDemandScore keeps a product with a handful of views off the list.
const minDemandScore = 5

type DemandItem struct {
	Product ProductRef `json:"product"`
	Score   float64    `json:"score" doc:"Weighted demand of the last 7 days: view 1, search 2, question 2, contact 3, watch 3, RFQ 5, deal 8"`
	Usual   float64    `json:"usual" doc:"Average weekly score over the 4 weeks before"`
	Growth  *float64   `json:"growth,omitempty" doc:"Score against usual: 0.8 = +80%; omitted when there was nothing before"`
	Median  *int64     `json:"median,omitempty" doc:"Price now (last 14 days) where known"`
}

type TopDemand struct {
	ProvinceID   int16        `json:"province_id,omitempty"`
	ProvinceName string       `json:"province_name,omitempty"`
	Top          []DemandItem `json:"top" doc:"Up to 10 products with the most demand this week"`
	Rising       []DemandItem `json:"rising" doc:"Up to 10 whose demand grew most against the usual week"`
}

// TopDemand ranks products by this week's weighted demand in a viloyat
// (0 = country).
func (s *Service) TopDemand(ctx context.Context, province int16) (*TopDemand, error) {
	key := fmt.Sprintf("demand|%d", province)
	if v, ok := s.cached(key); ok {
		return v.(*TopDemand), nil
	}
	prov := pgtype.Int2{Int16: province, Valid: province != 0}
	t := &TopDemand{Top: []DemandItem{}, Rising: []DemandItem{}}
	if province != 0 {
		r, err := s.q.GetRegion(ctx, province)
		if err != nil {
			return nil, err
		}
		t.ProvinceID, t.ProvinceName = r.ID, r.Name
	}
	var windows = map[int16]*[5]float64{}
	add := func(id int16, win int32, v float64) {
		if win < 0 || win > 4 {
			return
		}
		if windows[id] == nil {
			windows[id] = &[5]float64{}
		}
		windows[id][win] += v
	}
	rows, err := s.q.DemandByProduct(ctx, prov)
	if err != nil {
		return nil, err
	}
	for _, r := range rows {
		add(r.ProductID, r.Win, analytics.DemandWeight[r.Kind]*float64(r.N))
	}
	deals, err := s.q.DealsByProduct(ctx, prov)
	if err != nil {
		return nil, err
	}
	for _, d := range deals {
		add(d.ProductID, d.Win, analytics.DemandWeight["deal"]*float64(d.N))
	}
	products, err := s.products(ctx)
	if err != nil {
		return nil, err
	}
	mx, err := s.matrix(ctx, "ulgurji")
	if err != nil {
		return nil, err
	}
	var items []DemandItem
	for id, w := range windows {
		if w[0] < minDemandScore {
			continue
		}
		p := products[id]
		it := DemandItem{Product: ProductRef{ID: id, Slug: p.slug, Name: p.name, Category: p.cat, Unit: p.unt}, Score: w[0]}
		it.Usual = math.Round((w[1]+w[2]+w[3]+w[4])/4*10) / 10
		it.Growth = diff(w[0], it.Usual)
		var price float64
		if province != 0 {
			price = mx[id][province].median
		} else {
			price = countryOf(mx[id])
		}
		if price > 0 {
			m := rnd(price)
			it.Median = &m
		}
		items = append(items, it)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Score > items[j].Score })
	for _, it := range items {
		if len(t.Top) < topList {
			t.Top = append(t.Top, it)
		}
	}
	sort.SliceStable(items, func(i, j int) bool { return growth(items[i]) > growth(items[j]) })
	for _, it := range items {
		if it.Growth != nil && *it.Growth > 0 && len(t.Rising) < topList {
			t.Rising = append(t.Rising, it)
		}
	}
	s.store(key, t)
	return t, nil
}

func growth(it DemandItem) float64 {
	if it.Growth == nil {
		return math.Inf(-1)
	}
	return *it.Growth
}
