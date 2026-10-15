package insight

import (
	"context"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"bugun/internal/analytics"
	"bugun/internal/geo"
	"bugun/internal/market"
	"bugun/internal/store/db"
)

// Regional statistics (TAHLIL_REJA.md §8): one viloyat's whole market.

// foodBasket are the categories of the food price index.
var foodBasket = map[string]bool{"sabzavot": true, "meva": true, "gosht": true, "don": true, "oziq_ovqat": true}

const (
	// opportunityGap: a product is an opportunity when another viloyat pays
	// at least this much more, after the road.
	opportunityGap = 0.10
	topList        = 10
	// basketCoverage: a product joins the basket index when it has prices in
	// at least this share of the weeks.
	basketCoverage = 0.9
)

type Count struct {
	Now  int32    `json:"now"`
	Prev *int32   `json:"prev,omitempty" doc:"The week before, where it is known"`
	Diff *float64 `json:"diff,omitempty"`
}

type Passport struct {
	Sellers  int32 `json:"sellers" doc:"Sellers with a live listing"`
	Listings int32 `json:"listings"`
	Markets  int32 `json:"markets" doc:"Bazaars with a live listing"`
	RFQs     Count `json:"rfqs" doc:"Buyer requests, last 7 days"`
	Deals    Count `json:"deals" doc:"Confirmed deals, last 7 days"`
	Demand   Count `json:"demand" doc:"Demand signals, last 7 days"`
}

type RegionProduct struct {
	ID        int16    `json:"id"`
	Slug      string   `json:"slug"`
	Name      string   `json:"name"`
	Category  string   `json:"category"`
	Unit      string   `json:"unit"`
	Median    int64    `json:"median" doc:"Last 14 days"`
	VsCountry *float64 `json:"vs_country,omitempty" doc:"Against the country: -0.08 = 8% cheaper here"`
	Week      *float64 `json:"week,omitempty" doc:"Last 7 days against the 7 before"`
	Zone      string   `json:"zone,omitempty" enum:"surplus,balanced,shortage"`
	Sellers   int32    `json:"sellers"`
	Offers    int32    `json:"offers"`
	DemoShare float64  `json:"demo_share" doc:"Share of the prices behind this that are demo data (demo sellers or generated synthetic history), 0–1; 0 = all live. Show a Demo mark above 0"`
}

type IndexPoint struct {
	Week    string   `json:"week"`
	Region  float64  `json:"region" doc:"First week = 100"`
	Country *float64 `json:"country,omitempty"`
}

type Opportunity struct {
	ProductID  int16   `json:"product_id"`
	Name       string  `json:"name"`
	Unit       string  `json:"unit"`
	Here       int64   `json:"here"`
	SellIn     string  `json:"sell_in"`
	SellInID   int16   `json:"sell_in_id"`
	There      int64   `json:"there"`
	DistanceKm int     `json:"distance_km"`
	NetPct     float64 `json:"net_pct" doc:"Gain after road cost at the default rate"`
}

type Activity struct {
	ID       int32  `json:"id"`
	Name     string `json:"name"`
	Listings int32  `json:"listings"`
	Sellers  int32  `json:"sellers"`
}

// RegionOverview is one viloyat's market page.
type RegionOverview struct {
	ProvinceID    int16             `json:"province_id"`
	ProvinceName  string            `json:"province_name"`
	Trade         string            `json:"trade"`
	Passport      Passport          `json:"passport"`
	Products      []RegionProduct   `json:"products" doc:"Products with at least 5 prices here in 14 days"`
	Index         []IndexPoint      `json:"index" doc:"Food basket price index, weekly over a year"`
	IndexBasket   int               `json:"index_basket" doc:"How many products the index is built from"`
	Shortages     []RegionProduct   `json:"shortages" doc:"Demand well above supply"`
	Surpluses     []RegionProduct   `json:"surpluses" doc:"Supply well above demand"`
	Opportunities []Opportunity     `json:"opportunities" doc:"Cheaper here than elsewhere by 10%+ after the road"`
	Districts     []Activity        `json:"districts"`
	Markets       []Activity        `json:"markets"`
	Similar       []SimilarProvince `json:"similar" doc:"Up to 3 viloyatlar with the nearest price profile (ML_REJA.md 6); empty with too few prices"`
	DemoShare     float64           `json:"demo_share" doc:"Share of the prices behind this that are demo data (demo sellers or generated synthetic history), 0–1; 0 = all live. Show a Demo mark above 0"`
}

type product struct {
	id                   int16
	slug, name, cat, unt string
}

func (s *Service) products(ctx context.Context) (map[int16]product, error) {
	rows, err := s.q.ListProducts(ctx)
	if err != nil {
		return nil, err
	}
	out := make(map[int16]product, len(rows))
	for _, r := range rows {
		out[r.ID] = product{r.ID, r.Slug, r.Name, r.Category, r.BaseUnit}
	}
	return out, nil
}

// cell is a product's price in a viloyat.
type cell struct {
	median, demo float64
	offers       int32
}

// matrix returns product → viloyat → last 14 days, only from 5 prices.
func (s *Service) matrix(ctx context.Context, trade string) (map[int16]map[int16]cell, error) {
	rows, err := s.q.ProductProvinceMatrix(ctx, trade)
	if err != nil {
		return nil, err
	}
	m := map[int16]map[int16]cell{}
	for _, r := range rows {
		if r.Offers < minProvinceOffers {
			continue
		}
		if m[r.ProductID] == nil {
			m[r.ProductID] = map[int16]cell{}
		}
		m[r.ProductID][r.ProvinceID] = cell{r.Median, r.DemoShare, r.Offers}
	}
	return m, nil
}

// countryOf is the offers-weighted mean over viloyatlar.
func countryOf(cells map[int16]cell) float64 {
	var sum, w float64
	for _, c := range cells {
		sum += c.median * float64(c.offers)
		w += float64(c.offers)
	}
	if w == 0 {
		return 0
	}
	return sum / w
}

func (s *Service) weekChanges(ctx context.Context, trade string, province int16) (map[int16]*float64, error) {
	rows, err := s.q.ProductWeekChange(ctx, db.ProductWeekChangeParams{TradeType: trade, ProvinceID: pgtype.Int2{Int16: province, Valid: province != 0}})
	if err != nil {
		return nil, err
	}
	out := map[int16]*float64{}
	for _, r := range rows {
		if r.OffersNow >= minProvinceOffers && r.OffersPrev >= minProvinceOffers {
			out[r.ProductID] = diff(r.Now, r.Prev)
		}
	}
	return out, nil
}

// RegionOverview builds the page of a viloyat.
func (s *Service) RegionOverview(ctx context.Context, province int16, trade string) (*RegionOverview, error) {
	key := fmt.Sprintf("region|%d|%s", province, trade)
	if v, ok := s.cached(key); ok {
		return v.(*RegionOverview), nil
	}
	region, err := s.q.GetRegion(ctx, province)
	if err != nil {
		return nil, err
	}
	o := &RegionOverview{ProvinceID: region.ID, ProvinceName: region.Name, Trade: trade}
	if err := s.passport(ctx, o); err != nil {
		return nil, err
	}
	products, err := s.products(ctx)
	if err != nil {
		return nil, err
	}
	mx, err := s.matrix(ctx, trade)
	if err != nil {
		return nil, err
	}
	weeks, err := s.weekChanges(ctx, trade, province)
	if err != nil {
		return nil, err
	}
	sellerRows, err := s.q.RegionProductSellers(ctx, db.RegionProductSellersParams{TradeType: trade, ProvinceID: province})
	if err != nil {
		return nil, err
	}
	sellers := map[int16]int32{}
	for _, r := range sellerRows {
		sellers[r.ProductID] = r.Sellers
	}
	provinces, err := s.q.ListProvinces(ctx)
	if err != nil {
		return nil, err
	}
	centres, err := geo.Centroids()
	if err != nil {
		return nil, err
	}
	soato := map[int16]int64{}
	names := map[int16]string{}
	for _, p := range provinces {
		soato[p.ID], names[p.ID] = p.Soato.Int64, p.Name
	}

	o.Products, o.Shortages, o.Surpluses, o.Opportunities = []RegionProduct{}, []RegionProduct{}, []RegionProduct{}, []Opportunity{}
	var demoSum, offerSum float64
	for id, cells := range mx {
		here, ok := cells[province]
		if !ok {
			continue
		}
		p := products[id]
		rp := RegionProduct{
			ID: id, Slug: p.slug, Name: p.name, Category: p.cat, Unit: p.unt, Median: rnd(here.median),
			VsCountry: diff(here.median, countryOf(cells)), Week: weeks[id], Sellers: sellers[id],
			Offers: here.offers, DemoShare: math.Round(here.demo*100) / 100,
		}
		demand, supply, err := market.PressureWindows(ctx, s.q, id, province, trade)
		if err != nil {
			return nil, err
		}
		if _, zone, ok := analytics.Pressure(demand[0], demand[1:], supply[0], supply[1:]); ok {
			rp.Zone = zone
		}
		o.Products = append(o.Products, rp)
		switch rp.Zone {
		case analytics.ZoneShortage:
			o.Shortages = append(o.Shortages, rp)
		case analytics.ZoneSurplus:
			o.Surpluses = append(o.Surpluses, rp)
		}
		demoSum += here.demo * float64(here.offers)
		offerSum += float64(here.offers)

		// Where does it sell for the most, after the road?
		best, bestNet, bestKm := int16(0), 0.0, 0.0
		for pid, c := range cells {
			if pid == province {
				continue
			}
			a, ok1 := centres[soato[province]]
			b, ok2 := centres[soato[pid]]
			if !ok1 || !ok2 {
				continue
			}
			km := geo.RoadKm(a, b)
			net := (c.median - here.median - DefaultTransport*km/100) / here.median
			if net > bestNet {
				best, bestNet, bestKm = pid, net, km
			}
		}
		if best != 0 && bestNet >= opportunityGap && p.cat != "xizmat" { // a service cannot be carried
			o.Opportunities = append(o.Opportunities, Opportunity{
				ProductID: id, Name: p.name, Unit: p.unt, Here: rnd(here.median), SellIn: names[best], SellInID: best,
				There: rnd(cells[best].median), DistanceKm: int(math.Round(bestKm)), NetPct: math.Round(bestNet*1000) / 1000,
			})
		}
	}
	if offerSum > 0 {
		o.DemoShare = math.Round(demoSum/offerSum*100) / 100
	}
	sort.Slice(o.Products, func(i, j int) bool {
		if o.Products[i].Category != o.Products[j].Category {
			return o.Products[i].Category < o.Products[j].Category
		}
		return o.Products[i].Name < o.Products[j].Name
	})
	byOffers := func(xs []RegionProduct) []RegionProduct {
		sort.Slice(xs, func(i, j int) bool { return xs[i].Offers > xs[j].Offers })
		if len(xs) > topList {
			xs = xs[:topList]
		}
		return xs
	}
	o.Shortages, o.Surpluses = byOffers(o.Shortages), byOffers(o.Surpluses)
	sort.Slice(o.Opportunities, func(i, j int) bool { return o.Opportunities[i].NetPct > o.Opportunities[j].NetPct })
	if len(o.Opportunities) > topList {
		o.Opportunities = o.Opportunities[:topList]
	}

	if err := s.basketIndex(ctx, o, products, trade, province); err != nil {
		return nil, err
	}
	if err := s.activity(ctx, o, province); err != nil {
		return nil, err
	}
	ps, err := s.profiles(ctx, trade)
	if err != nil {
		return nil, err
	}
	o.Similar = s.clusterRules.similar(ps, s.clusterRules.group(ps), province)
	s.store(key, o)
	return o, nil
}

func (s *Service) passport(ctx context.Context, o *RegionOverview) error {
	p, err := s.q.RegionPassport(ctx, o.ProvinceID)
	if err != nil {
		return err
	}
	count := func(now, prev int32) Count {
		c := Count{Now: now, Prev: &prev}
		c.Diff = diff(float64(now), float64(prev))
		return c
	}
	o.Passport = Passport{
		Sellers: p.Sellers, Listings: p.Listings, Markets: p.Markets,
		RFQs: count(p.Rfqs, p.RfqsPrev), Deals: count(p.Deals, p.DealsPrev), Demand: count(p.Demand, p.DemandPrev),
	}
	return nil
}

// basketIndex is the food basket price index: for every food product with
// prices in most weeks of the year, each week's price over its first week,
// averaged and scaled to 100; the country the same way.
func (s *Service) basketIndex(ctx context.Context, o *RegionOverview, products map[int16]product, trade string, province int16) error {
	region, n, err := s.index(ctx, products, trade, province)
	if err != nil {
		return err
	}
	country, _, err := s.index(ctx, products, trade, 0)
	if err != nil {
		return err
	}
	o.Index, o.IndexBasket = []IndexPoint{}, n
	for _, w := range sortedWeeks(region) {
		p := IndexPoint{Week: w.Format(time.DateOnly), Region: math.Round(region[w]*10) / 10}
		if c, ok := country[w]; ok {
			c = math.Round(c*10) / 10
			p.Country = &c
		}
		o.Index = append(o.Index, p)
	}
	return nil
}

func (s *Service) index(ctx context.Context, products map[int16]product, trade string, province int16) (map[time.Time]float64, int, error) {
	rows, err := s.q.WeeklyProductPrices(ctx, db.WeeklyProductPricesParams{TradeType: trade, ProvinceID: pgtype.Int2{Int16: province, Valid: province != 0}})
	if err != nil {
		return nil, 0, err
	}
	series := map[int16]map[time.Time]float64{}
	weeks := map[time.Time]bool{}
	for _, r := range rows {
		if !foodBasket[products[r.ProductID].cat] {
			continue
		}
		if series[r.ProductID] == nil {
			series[r.ProductID] = map[time.Time]float64{}
		}
		series[r.ProductID][r.Week.Time] = r.Median
		weeks[r.Week.Time] = true
	}
	all := make([]time.Time, 0, len(weeks))
	for w := range weeks {
		all = append(all, w)
	}
	sort.Slice(all, func(i, j int) bool { return all[i].Before(all[j]) })
	out := map[time.Time]float64{}
	if len(all) == 0 {
		return out, 0, nil
	}
	first := all[0]
	var basket []map[time.Time]float64
	for _, sr := range series {
		if sr[first] > 0 && float64(len(sr)) >= basketCoverage*float64(len(all)) {
			basket = append(basket, sr)
		}
	}
	if len(basket) == 0 {
		return out, 0, nil
	}
	for _, w := range all {
		sum, n := 0.0, 0
		for _, sr := range basket {
			if v, ok := sr[w]; ok {
				sum += v / sr[first]
				n++
			}
		}
		if n > 0 {
			out[w] = 100 * sum / float64(n)
		}
	}
	return out, len(basket), nil
}

func sortedWeeks(m map[time.Time]float64) []time.Time {
	out := make([]time.Time, 0, len(m))
	for w := range m {
		out = append(out, w)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Before(out[j]) })
	return out
}

func (s *Service) activity(ctx context.Context, o *RegionOverview, province int16) error {
	ds, err := s.q.RegionDistrictActivity(ctx, province)
	if err != nil {
		return err
	}
	o.Districts = make([]Activity, len(ds))
	for i, d := range ds {
		o.Districts[i] = Activity{ID: int32(d.ID), Name: d.Name, Listings: d.Listings, Sellers: d.Sellers}
	}
	ms, err := s.q.RegionMarketActivity(ctx, province)
	if err != nil {
		return err
	}
	o.Markets = make([]Activity, len(ms))
	for i, m := range ms {
		o.Markets[i] = Activity{ID: int32(m.ID), Name: m.Name, Listings: m.Listings, Sellers: m.Sellers}
	}
	return nil
}
