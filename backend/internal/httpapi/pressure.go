package httpapi

import (
	"context"
	"errors"
	"math"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5"

	"bugun/internal/analytics"
	"bugun/internal/geo"
	"bugun/internal/market"
	"bugun/internal/store/db"
)

func (s *Server) registerPressure(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "productPressure", Method: http.MethodGet, Path: "/v1/products/{slug}/pressure",
		Summary: "Demand–supply pressure index for a product (spec 10.3)", Tags: []string{"browse"},
	}, s.productPressure)
	huma.Register(api, huma.Operation{
		OperationID: "provinceMap", Method: http.MethodGet, Path: "/v1/map/provinces",
		Summary: "Viloyat boundaries (GeoJSON) with median price and pressure for a product (W-04)", Tags: []string{"browse"},
	}, s.provinceMap)
}

// Pressure is the index for one product and area. Weeks are rolling 7-day
// windows ending now.
type Pressure struct {
	Enough     bool    `json:"enough" doc:"false until two earlier weeks have data; value and zone are then omitted"`
	Value      float64 `json:"value,omitempty" doc:"(demand now / usual) / (sellers now / usual); 1 is a normal week"`
	Zone       string  `json:"zone,omitempty" enum:"surplus,balanced,shortage" doc:"surplus < 0.8 (good time to buy), shortage > 1.2"`
	DemandNow  float64 `json:"demand_now" doc:"Weighted demand in the last 7 days"`
	DemandAvg  float64 `json:"demand_avg" doc:"Average weekly demand over the four weeks before"`
	SellersNow int     `json:"sellers_now" doc:"Sellers with a live price in the last 7 days"`
	SellersAvg float64 `json:"sellers_avg" doc:"Average over the four weeks before"`
}

// pressureFor computes the index; province 0 means the whole country.
func (s *Server) pressureFor(ctx context.Context, productID, province int16, trade string) (Pressure, error) {
	demand, supply, err := market.PressureWindows(ctx, s.q, productID, province, trade)
	if err != nil {
		return Pressure{}, err
	}
	p := Pressure{DemandNow: demand[0], SellersNow: int(supply[0]), DemandAvg: meanOf(demand[1:]), SellersAvg: meanOf(supply[1:])}
	if v, zone, ok := analytics.Pressure(demand[0], demand[1:], supply[0], supply[1:]); ok {
		p.Enough, p.Value, p.Zone = true, math.Round(v*100)/100, zone
	}
	return p, nil
}

func meanOf(xs []float64) float64 {
	sum, n := 0.0, 0
	for _, x := range xs {
		if x > 0 {
			sum += x
			n++
		}
	}
	if n == 0 {
		return 0
	}
	return math.Round(sum/float64(n)*10) / 10
}

type PressureInput struct {
	Slug      string `path:"slug" maxLength:"60"`
	Region    int16  `query:"region" minimum:"0" doc:"Viloyat or tuman id (its viloyat is used); 0 for the whole country"`
	TradeType string `query:"trade" enum:"ulgurji,chakana" default:"ulgurji"`
}

type PressureOutput struct {
	Body Pressure
}

func (s *Server) productPressure(ctx context.Context, in *PressureInput) (*PressureOutput, error) {
	p, err := s.q.GetProductBySlug(ctx, in.Slug)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, huma.Error404NotFound("Mahsulot topilmadi.")
	}
	if err != nil {
		return nil, internalError(ctx, "get product", err)
	}
	var province int16
	if in.Region > 0 {
		if province, err = market.ProvinceOf(ctx, s.q, in.Region); errors.Is(err, pgx.ErrNoRows) {
			return nil, huma.Error422UnprocessableEntity("Bunday hudud topilmadi.")
		} else if err != nil {
			return nil, internalError(ctx, "province", err)
		}
	}
	pr, err := s.pressureFor(ctx, p.ID, province, in.TradeType)
	if err != nil {
		return nil, internalError(ctx, "pressure", err)
	}
	return &PressureOutput{Body: pr}, nil
}

type MapInput struct {
	Product   string `query:"product" maxLength:"60" doc:"Product slug; empty returns boundaries and names only"`
	TradeType string `query:"trade" enum:"ulgurji,chakana" default:"ulgurji"`
}

type MapOutput struct {
	CacheControl string `header:"Cache-Control"`
	Body         struct {
		Type     string        `json:"type" enum:"FeatureCollection"`
		Features []geo.Feature `json:"features" doc:"One per viloyat; properties: region_id, soato, name and, with a product, offers, median, pressure, zone"`
	}
}

func (s *Server) provinceMap(ctx context.Context, in *MapInput) (*MapOutput, error) {
	shapes, err := geo.Provinces()
	if err != nil {
		return nil, internalError(ctx, "boundaries", err)
	}
	provinces, err := s.q.ListProvinces(ctx)
	if err != nil {
		return nil, internalError(ctx, "provinces", err)
	}
	var product *db.Product
	medians := map[int16]db.ProductProvinceMediansRow{}
	if in.Product != "" {
		p, err := s.q.GetProductBySlug(ctx, in.Product)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, huma.Error404NotFound("Mahsulot topilmadi.")
		}
		if err != nil {
			return nil, internalError(ctx, "get product", err)
		}
		product = &p
		rows, err := s.q.ProductProvinceMedians(ctx, db.ProductProvinceMediansParams{ProductID: p.ID, TradeType: in.TradeType})
		if err != nil {
			return nil, internalError(ctx, "province medians", err)
		}
		for _, r := range rows {
			medians[r.ProvinceID] = r
		}
	}
	out := &MapOutput{CacheControl: "public, max-age=300"}
	out.Body.Type = "FeatureCollection"
	out.Body.Features = []geo.Feature{}
	for _, pv := range provinces {
		f, ok := shapes[pv.Soato.Int64]
		if !ok {
			continue
		}
		f.Properties["region_id"] = pv.ID
		if product != nil {
			m := medians[pv.ID]
			f.Properties["offers"] = m.Offers
			if m.Offers >= analytics.MinObservations {
				f.Properties["median"] = int64(math.Round(m.Median))
			}
			pr, err := s.pressureFor(ctx, product.ID, pv.ID, in.TradeType)
			if err != nil {
				return nil, internalError(ctx, "pressure", err)
			}
			if pr.Enough {
				f.Properties["pressure"], f.Properties["zone"] = pr.Value, pr.Zone
			}
		}
		out.Body.Features = append(out.Body.Features, f)
	}
	return out, nil
}
