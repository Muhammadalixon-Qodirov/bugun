package insight

import (
	"context"
	"math"
	"sort"

	"github.com/jackc/pgx/v5/pgtype"

	"bugun/internal/analytics"
	"bugun/internal/geo"
	"bugun/internal/store/db"
)

const (
	// minProvinceOffers: a viloyat is ranked from 5 prices in 14 days.
	minProvinceOffers = 5
	// minDeals: the bargain figure needs at least 5 deals.
	minDeals = 5
	// DefaultTransport is the road cost of 1 kg per 100 km, in so'm.
	DefaultTransport = 60
	arbitrageTop     = 5
)

// AreaPrice is one viloyat, tuman or bazaar in a ranking.
type AreaPrice struct {
	ID        int64    `json:"id"`
	Name      string   `json:"name"`
	Median    int64    `json:"median"`
	Diff      *float64 `json:"diff,omitempty" doc:"Against the chosen viloyat, or the country average: -0.11 = 11% cheaper"`
	Offers    int32    `json:"offers"`
	DemoShare float64  `json:"demo_share,omitempty" doc:"Share of the prices behind this that are demo data (demo sellers or generated synthetic history), 0–1; 0 = all live. Show a Demo mark above 0; omitted when 0"`
}

// Arbitrage is buying in one viloyat and selling in another.
type Arbitrage struct {
	BuyID      int16   `json:"buy_id"`
	Buy        string  `json:"buy"`
	BuyPrice   int64   `json:"buy_price"`
	SellID     int16   `json:"sell_id"`
	Sell       string  `json:"sell"`
	SellPrice  int64   `json:"sell_price"`
	Spread     int64   `json:"spread" doc:"Sell minus buy price, per unit"`
	DistanceKm int     `json:"distance_km" doc:"Estimated road distance between viloyat centres"`
	Transport  int64   `json:"transport" doc:"Road cost per unit at the chosen rate"`
	Net        int64   `json:"net" doc:"Spread minus transport, per unit"`
	NetPct     float64 `json:"net_pct" doc:"Net against the buy price"`
}

// Bargain is how far agreed prices sit from asking prices.
type Bargain struct {
	Deals       int     `json:"deals"`
	AvgDiscount float64 `json:"avg_discount" doc:"Mean of agreed/asked - 1: -0.035 = buyers got 3.5% off"`
}

// Regional is the geographic part of the analytics.
type Regional struct {
	Provinces    []AreaPrice `json:"provinces" doc:"Viloyatlar with at least 5 prices in 14 days, cheapest first"`
	Districts    []AreaPrice `json:"districts" doc:"Tumanlar of the chosen viloyat (at least 3 prices in 14 days); empty for the country"`
	Markets      []AreaPrice `json:"markets" doc:"Bazaars of the chosen viloyat, same rule"`
	Arbitrage    []Arbitrage `json:"arbitrage" doc:"Best buy/sell pairs after road cost; with a viloyat chosen, only pairs that include it"`
	Transport    float64     `json:"transport_rate" doc:"The road cost used: so'm per unit per 100 km"`
	RetailMarkup *float64    `json:"retail_markup,omitempty" doc:"Retail over wholesale in the last 30 days: 0.34 = 34% dearer"`
	Bargain      *Bargain    `json:"bargain,omitempty" doc:"Omitted with fewer than 5 confirmed deals in 90 days"`
}

func (s *Service) regional(ctx context.Context, a *Analytics, p Params, prov pgtype.Int2) error {
	r := &a.Regional
	r.Transport = p.Transport
	rows, err := s.q.ProvincePrices(ctx, db.ProvincePricesParams{ProductID: p.ProductID, TradeType: p.Trade})
	if err != nil {
		return err
	}
	var ranked []db.ProvincePricesRow
	var sum, weight float64
	for _, row := range rows {
		if row.Offers < minProvinceOffers {
			continue
		}
		ranked = append(ranked, row)
		sum += row.Median * float64(row.Offers)
		weight += float64(row.Offers)
	}
	ref := 0.0
	if weight > 0 {
		ref = sum / weight
	}
	if p.Province != 0 {
		ref = 0
		for _, row := range ranked {
			if row.ID == p.Province {
				ref = row.Median
			}
		}
	}
	r.Provinces = make([]AreaPrice, len(ranked))
	for i, row := range ranked {
		r.Provinces[i] = AreaPrice{ID: int64(row.ID), Name: row.Name, Median: rnd(row.Median), Diff: diff(row.Median, ref),
			Offers: row.Offers, DemoShare: math.Round(row.DemoShare*100) / 100}
	}
	if r.Arbitrage, err = arbitrage(ranked, p.Province, p.Transport); err != nil {
		return err
	}

	r.Districts, r.Markets = []AreaPrice{}, []AreaPrice{}
	if p.Province != 0 {
		ds, err := s.q.DistrictPrices(ctx, db.DistrictPricesParams{ProductID: p.ProductID, TradeType: p.Trade, ProvinceID: p.Province, MinOffers: analytics.MinObservations})
		if err != nil {
			return err
		}
		for _, d := range ds {
			r.Districts = append(r.Districts, AreaPrice{ID: int64(d.ID), Name: d.Name, Median: rnd(d.Median), Diff: diff(d.Median, ref), Offers: d.Offers})
		}
		ms, err := s.q.MarketPrices(ctx, db.MarketPricesParams{ProductID: p.ProductID, TradeType: p.Trade, ProvinceID: p.Province, MinOffers: analytics.MinObservations})
		if err != nil {
			return err
		}
		for _, m := range ms {
			r.Markets = append(r.Markets, AreaPrice{ID: int64(m.ID), Name: m.Name, Median: rnd(m.Median), Diff: diff(m.Median, ref), Offers: m.Offers})
		}
	}

	trades, err := s.q.TradeMedians(ctx, db.TradeMediansParams{ProductID: p.ProductID, ProvinceID: prov})
	if err != nil {
		return err
	}
	var whole, retail float64
	for _, t := range trades {
		if t.Offers < minProvinceOffers {
			continue
		}
		switch t.TradeType {
		case "ulgurji":
			whole = t.Median
		case "chakana":
			retail = t.Median
		}
	}
	r.RetailMarkup = diff(retail, whole)

	deals, err := s.q.DealDiscounts(ctx, db.DealDiscountsParams{ProductID: p.ProductID, ProvinceID: prov})
	if err != nil {
		return err
	}
	var disc float64
	n := 0
	for _, d := range deals {
		if d.Asked > 0 {
			disc += d.Agreed/d.Asked - 1
			n++
		}
	}
	if n >= minDeals {
		r.Bargain = &Bargain{Deals: n, AvgDiscount: math.Round(disc/float64(n)*1000) / 1000}
	}
	return nil
}

// arbitrage lists the best pairs of viloyatlar to buy in and sell in, after
// the road cost of rate so'm per unit per 100 km.
func arbitrage(ranked []db.ProvincePricesRow, only int16, rate float64) ([]Arbitrage, error) {
	centres, err := geo.Centroids()
	if err != nil {
		return nil, err
	}
	out := []Arbitrage{}
	for _, b := range ranked {
		for _, s := range ranked {
			if b.ID == s.ID || s.Median <= b.Median || (only != 0 && b.ID != only && s.ID != only) {
				continue
			}
			cb, ok1 := centres[b.Soato]
			cs, ok2 := centres[s.Soato]
			if !ok1 || !ok2 {
				continue
			}
			km := geo.RoadKm(cb, cs)
			transport := rate * km / 100
			net := s.Median - b.Median - transport
			if net <= 0 {
				continue
			}
			out = append(out, Arbitrage{
				BuyID: b.ID, Buy: b.Name, BuyPrice: rnd(b.Median), SellID: s.ID, Sell: s.Name, SellPrice: rnd(s.Median),
				Spread: rnd(s.Median - b.Median), DistanceKm: int(math.Round(km)), Transport: rnd(transport),
				Net: rnd(net), NetPct: math.Round(net/b.Median*1000) / 1000,
			})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].NetPct > out[j].NetPct })
	if len(out) > arbitrageTop {
		out = out[:arbitrageTop]
	}
	return out, nil
}

// diff is x against ref as a share, or nil when either is missing.
func diff(x, ref float64) *float64 {
	if x <= 0 || ref <= 0 {
		return nil
	}
	v := math.Round((x/ref-1)*1000) / 1000
	return &v
}
