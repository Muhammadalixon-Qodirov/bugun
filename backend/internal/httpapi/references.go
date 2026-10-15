package httpapi

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"bugun/internal/market"
	"bugun/internal/store/db"
)

func (s *Server) registerReferences(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "productReferences", Method: http.MethodGet, Path: "/v1/products/{slug}/references",
		Summary: "Official benchmark prices for a product (UzEx, ...)",
		Description: "The latest quote of every benchmark source, converted to the product's base unit. Benchmarks never enter " +
			"medians; show them as a separate reference line with their source and date. Empty until a connector " +
			"(UZEX_ENABLED) has loaded quotes.",
		Tags: []string{"browse"},
	}, s.productReferences)
}

type ReferencePrice struct {
	Source    string    `json:"source" enum:"uzex,narxtahlil"`
	PriceSom  int64     `json:"price_som" doc:"Per the product's base unit"`
	QuoteName string    `json:"quote_name" doc:"The item as the source names it, e.g. Bug'doy 3 sinf"`
	QuoteUnit string    `json:"quote_unit" doc:"The unit the source quotes in, e.g. tonna"`
	QuoteSom  float64   `json:"quote_som" doc:"The source's own price per quote_unit"`
	QuotedOn  string    `json:"quoted_on" format:"date" doc:"Tashkent day of the quote"`
	FetchedAt time.Time `json:"fetched_at"`
	SourceURL string    `json:"source_url,omitempty"`
	National  bool      `json:"national" doc:"A country-wide price (the exchange) rather than the viloyat's"`
}

type ReferencesInput struct {
	Slug      string `path:"slug" maxLength:"60"`
	Region    int16  `query:"region" minimum:"0" doc:"Viloyat or tuman id (its viloyat is used); 0 for national prices only"`
	TradeType string `query:"trade" enum:"ulgurji,chakana" default:"ulgurji"`
}

type ReferencesOutput struct {
	Body struct {
		References []ReferencePrice `json:"references"`
	}
}

func (s *Server) productReferences(ctx context.Context, in *ReferencesInput) (*ReferencesOutput, error) {
	p, err := s.q.GetProductBySlug(ctx, in.Slug)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apiError(http.StatusNotFound, "product_not_found", "Mahsulot topilmadi.")
	}
	if err != nil {
		return nil, internalError(ctx, "get product", err)
	}
	var province pgtype.Int2
	if in.Region > 0 {
		id, err := market.ProvinceOf(ctx, s.q, in.Region)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, huma.Error422UnprocessableEntity("Bunday hudud topilmadi.")
		} else if err != nil {
			return nil, internalError(ctx, "province", err)
		}
		province = pgtype.Int2{Int16: id, Valid: true}
	}
	rows, err := s.q.LatestReferencePrices(ctx, db.LatestReferencePricesParams{ProductID: p.ID, TradeType: in.TradeType, ProvinceID: province})
	if err != nil {
		return nil, internalError(ctx, "reference prices", err)
	}
	out := &ReferencesOutput{}
	out.Body.References = make([]ReferencePrice, 0, len(rows))
	for _, r := range rows {
		out.Body.References = append(out.Body.References, ReferencePrice{
			Source: r.Source, PriceSom: r.PriceSom, QuoteName: r.QuoteName, QuoteUnit: r.QuoteUnit, QuoteSom: r.QuoteSom,
			QuotedOn: r.QuotedOn.Time.Format(time.DateOnly), FetchedAt: r.FetchedAt, SourceURL: r.SourceUrl, National: !r.RegionID.Valid,
		})
	}
	return out, nil
}
