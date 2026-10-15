package insight

import (
	"context"
	"math"
	"sort"

	"bugun/internal/geo"
	"bugun/internal/ml"
	"bugun/internal/store/db"
)

// Heatmap (TAHLIL_REJA.md §8 H.2): every product against every viloyat.

type ProvinceRef struct {
	ID   int16  `json:"id"`
	Name string `json:"name"`
}

type ProductRef struct {
	ID       int16  `json:"id"`
	Slug     string `json:"slug"`
	Name     string `json:"name"`
	Category string `json:"category"`
	Unit     string `json:"unit"`
}

type HeatCell struct {
	Median int64    `json:"median"`
	Offers int32    `json:"offers"`
	Diff   *float64 `json:"diff,omitempty" doc:"Against the country"`
	Spot   string   `json:"spot,omitempty" enum:"hot,cold,high_outlier,low_outlier" doc:"Local Moran's I (ML_REJA.md 7), p < 0.01 and at least 5% from the country: hot = dear among dear neighbours, cold = cheap among cheap, high_outlier = dear among cheap, low_outlier = cheap among dear; empty when not significant or under 8 viloyatlar"`
}

type HeatRow struct {
	Product ProductRef  `json:"product"`
	Country int64       `json:"country" doc:"Offers-weighted mean of the viloyatlar shown"`
	Week    *float64    `json:"week,omitempty" doc:"Country, last 7 days against the 7 before"`
	Cells   []*HeatCell `json:"cells" doc:"One per viloyat in the order of provinces; null under 5 prices"`
}

type Mover struct {
	Product ProductRef `json:"product"`
	Country int64      `json:"country"`
	Week    float64    `json:"week"`
}

type Heatmap struct {
	Trade     string        `json:"trade"`
	Category  string        `json:"category,omitempty"`
	Provinces []ProvinceRef `json:"provinces"`
	Rows      []HeatRow     `json:"rows"`
	Up        []Mover       `json:"up" doc:"Up to 10 products that rose most this week"`
	Down      []Mover       `json:"down" doc:"Up to 10 that fell most"`
	DemoShare float64       `json:"demo_share" doc:"Share of the prices behind this that are demo data (demo sellers or generated synthetic history), 0–1; 0 = all live. Show a Demo mark above 0"`
}

// Heatmap builds the product × viloyat table for a trade type and,
// optionally, one category, with the week's biggest movers.
func (s *Service) Heatmap(ctx context.Context, trade, category string) (*Heatmap, error) {
	key := "heatmap|" + trade + "|" + category
	if v, ok := s.cached(key); ok {
		return v.(*Heatmap), nil
	}
	products, err := s.products(ctx)
	if err != nil {
		return nil, err
	}
	mx, err := s.matrix(ctx, trade)
	if err != nil {
		return nil, err
	}
	weeks, err := s.weekChanges(ctx, trade, 0)
	if err != nil {
		return nil, err
	}
	provinces, err := s.q.ListProvinces(ctx)
	if err != nil {
		return nil, err
	}
	neighbours, err := provinceNeighbours(provinces)
	if err != nil {
		return nil, err
	}
	h := &Heatmap{Trade: trade, Category: category, Provinces: make([]ProvinceRef, len(provinces)), Rows: []HeatRow{}, Up: []Mover{}, Down: []Mover{}}
	for i, p := range provinces {
		h.Provinces[i] = ProvinceRef{ID: p.ID, Name: p.Name}
	}
	var movers []Mover
	var demoSum, offerSum float64
	for id, cells := range mx {
		p := products[id]
		if category != "" && p.cat != category {
			continue
		}
		ref := ProductRef{ID: id, Slug: p.slug, Name: p.name, Category: p.cat, Unit: p.unt}
		country := countryOf(cells)
		row := HeatRow{Product: ref, Country: rnd(country), Week: weeks[id], Cells: make([]*HeatCell, len(provinces))}
		for i, pv := range provinces {
			if c, ok := cells[pv.ID]; ok {
				row.Cells[i] = &HeatCell{Median: rnd(c.median), Offers: c.offers, Diff: diff(c.median, country)}
				demoSum += c.demo * float64(c.offers)
				offerSum += float64(c.offers)
			}
		}
		markSpots(row.Cells, neighbours)
		h.Rows = append(h.Rows, row)
		if w := weeks[id]; w != nil && *w != 0 {
			movers = append(movers, Mover{Product: ref, Country: rnd(country), Week: *w})
		}
	}
	sort.Slice(h.Rows, func(i, j int) bool {
		a, b := h.Rows[i].Product, h.Rows[j].Product
		if a.Category != b.Category {
			return a.Category < b.Category
		}
		return a.Name < b.Name
	})
	sort.Slice(movers, func(i, j int) bool { return movers[i].Week > movers[j].Week })
	for _, m := range movers {
		if m.Week > 0 && len(h.Up) < topList {
			h.Up = append(h.Up, m)
		}
	}
	for i := len(movers) - 1; i >= 0; i-- {
		if movers[i].Week < 0 && len(h.Down) < topList {
			h.Down = append(h.Down, movers[i])
		}
	}
	if offerSum > 0 {
		h.DemoShare = math.Round(demoSum/offerSum*100) / 100
	}
	s.store(key, h)
	return h, nil
}

const (
	// minSpotProvinces: Local Moran's I needs this many viloyatlar with a price.
	minSpotProvinces = 8
	// A hundred products × 14 viloyatlar are many tests: at p < 0.05 about 5%
	// of cells would be marked by chance. A spot needs p < 0.01 and a price
	// at least 5% away from the country.
	spotAlpha = 0.01
	spotGap   = 0.05
)

// provinceNeighbours maps each viloyat's index in provinces to the indices of
// the viloyatlar it borders.
func provinceNeighbours(provinces []db.ListProvincesRow) ([][]int, error) {
	nb, err := geo.Neighbors()
	if err != nil {
		return nil, err
	}
	index := map[int64]int{}
	for i, p := range provinces {
		index[p.Soato.Int64] = i
	}
	out := make([][]int, len(provinces))
	for i, p := range provinces {
		for _, s := range nb[p.Soato.Int64] {
			if j, ok := index[s]; ok {
				out[i] = append(out[i], j)
			}
		}
	}
	return out, nil
}

// markSpots runs Local Moran's I over the viloyatlar with a price, on the log
// of their price against the country, and marks the significant cells.
func markSpots(cells []*HeatCell, neighbours [][]int) {
	var have []int // indices into provinces
	pos := map[int]int{}
	for i, c := range cells {
		if c != nil && c.Diff != nil {
			pos[i] = len(have)
			have = append(have, i)
		}
	}
	if len(have) < minSpotProvinces {
		return
	}
	x := make([]float64, len(have))
	nb := make([][]int, len(have))
	for k, i := range have {
		x[k] = math.Log1p(*cells[i].Diff)
		for _, j := range neighbours[i] {
			if p, ok := pos[j]; ok {
				nb[k] = append(nb[k], p)
			}
		}
	}
	for k, sp := range ml.LocalMoran(x, nb) {
		if sp.P < spotAlpha && math.Abs(*cells[have[k]].Diff) >= spotGap {
			cells[have[k]].Spot = sp.Kind
		}
	}
}
