package httpapi

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
)

type Region struct {
	ID       int16  `json:"id"`
	Slug     string `json:"slug"`
	Name     string `json:"name"`
	Kind     string `json:"kind" enum:"respublika,viloyat,tuman,shahar"`
	ParentID *int16 `json:"parent_id,omitempty"`
}

type Market struct {
	ID       int16  `json:"id"`
	Slug     string `json:"slug"`
	Name     string `json:"name"`
	RegionID int16  `json:"region_id"`
	Kind     string `json:"kind" enum:"dehqon,ulgurji,qurilish"`
}

type Product struct {
	ID       int16  `json:"id"`
	Slug     string `json:"slug"`
	Name     string `json:"name"`
	Category string `json:"category" enum:"sabzavot,meva,gosht,don,oziq_ovqat,qurilish,xizmat"`
	BaseUnit string `json:"base_unit" enum:"kg,litr,dona,m2,m3,xizmat"`
}

type Category struct {
	Slug string `json:"slug"`
	Name string `json:"name"`
}

// Categories are fixed by the spec (§7.1), so they live in code, not in the database.
var Categories = []Category{
	{"sabzavot", "Sabzavot"},
	{"meva", "Meva"},
	{"gosht", "Go'sht"},
	{"don", "Don"},
	{"oziq_ovqat", "Oziq-ovqat"},
	{"qurilish", "Qurilish mollari"},
	{"xizmat", "Xizmatlar"},
}

type CatalogOutput struct {
	CacheControl string `header:"Cache-Control"`
	Body         struct {
		Regions    []Region   `json:"regions"`
		Markets    []Market   `json:"markets"`
		Products   []Product  `json:"products"`
		Categories []Category `json:"categories"`
	}
}

func (s *Server) registerCatalog(api huma.API) {
	s.registerReferences(api) // benchmark prices of a catalog product
	huma.Register(api, huma.Operation{
		OperationID: "getCatalog",
		Method:      http.MethodGet,
		Path:        "/v1/catalog",
		Summary:     "Regions, markets, products and categories",
		Tags:        []string{"catalog"},
	}, s.catalog)
}

func (s *Server) catalog(ctx context.Context, _ *struct{}) (*CatalogOutput, error) {
	regions, err := s.q.ListRegions(ctx)
	if err != nil {
		return nil, internalError(ctx, "list regions", err)
	}
	markets, err := s.q.ListMarkets(ctx)
	if err != nil {
		return nil, internalError(ctx, "list markets", err)
	}
	products, err := s.q.ListProducts(ctx)
	if err != nil {
		return nil, internalError(ctx, "list products", err)
	}

	out := &CatalogOutput{CacheControl: "public, max-age=300"}
	out.Body.Regions = make([]Region, len(regions))
	for i, r := range regions {
		out.Body.Regions[i] = Region{ID: r.ID, Slug: r.Slug, Name: r.Name, Kind: r.Kind}
		if r.ParentID.Valid {
			out.Body.Regions[i].ParentID = &r.ParentID.Int16
		}
	}
	out.Body.Markets = make([]Market, len(markets))
	for i, m := range markets {
		out.Body.Markets[i] = Market{ID: m.ID, Slug: m.Slug, Name: m.Name, RegionID: m.RegionID, Kind: m.Kind}
	}
	out.Body.Products = make([]Product, len(products))
	for i, p := range products {
		out.Body.Products[i] = Product{ID: p.ID, Slug: p.Slug, Name: p.Name, Category: p.Category, BaseUnit: p.BaseUnit}
	}
	out.Body.Categories = Categories
	return out, nil
}
