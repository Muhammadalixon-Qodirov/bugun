package httpapi

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"bugun/internal/store/db"
)

// Public seller page (spec W-09): name, market, live prices and activity.
// Phone and Telegram details stay private; contacts go through listings.
func (s *Server) registerSellers(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "getSeller", Method: http.MethodGet, Path: "/v1/sellers/{id}",
		Summary: "Public seller page with live listings", Tags: []string{"sellers"},
	}, s.getSeller)
}

// sellerPageListings caps the listings shown on a seller page.
const sellerPageListings = 100

type SellerProfile struct {
	ID           string        `json:"id"`
	Name         string        `json:"name"`
	RegionName   string        `json:"region_name,omitempty"`
	ProvinceName string        `json:"province_name,omitempty"`
	MarketName   string        `json:"market_name,omitempty"`
	Since        time.Time     `json:"since"`
	Categories   []string      `json:"categories" doc:"Categories of live listings (the profile's categories when there are none)"`
	Listings     int32         `json:"listings" doc:"Live listings"`
	Products     int32         `json:"products" doc:"Distinct products in live listings"`
	HasTelegram  bool          `json:"has_telegram"`
	HasPhone     bool          `json:"has_phone"`
	Rating       *SellerRating `json:"rating,omitempty" doc:"Buyers' ratings of confirmed deals; omitted under 3 ratings"`
	LastActive   *time.Time    `json:"last_active,omitempty" doc:"Latest price the seller set"`
}

type SellerPageInput struct {
	ID string `path:"id" maxLength:"36" doc:"Seller's user id"`
}

type SellerPageOutput struct {
	CacheControl string `header:"Cache-Control"`
	Body         struct {
		Seller   SellerProfile `json:"seller"`
		Listings []ListingCard `json:"listings" doc:"Live listings, newest first, at most 100"`
	}
}

func (s *Server) getSeller(ctx context.Context, in *SellerPageInput) (*SellerPageOutput, error) {
	notFound := apiError(http.StatusNotFound, "seller_not_found", "Sotuvchi topilmadi.")
	id, err := uuid.Parse(in.ID)
	if err != nil {
		return nil, notFound
	}
	u, err := s.q.PublicSeller(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, notFound
	}
	if err != nil {
		return nil, internalError(ctx, "public seller", err)
	}
	out := &SellerPageOutput{CacheControl: "public, max-age=30"}
	p := SellerProfile{
		ID: u.ID.String(), Name: u.Name, RegionName: u.RegionName, ProvinceName: u.ProvinceName, MarketName: u.MarketName,
		Since: u.CreatedAt, Categories: u.Categories, Listings: u.Listings, Products: u.Products,
		HasTelegram: u.HasTelegram, HasPhone: u.HasPhone,
	}
	if u.LastActive.After(time.Unix(0, 0)) {
		p.LastActive = &u.LastActive
	}
	if p.Rating, err = s.sellerRating(ctx, u.ID); err != nil {
		return nil, internalError(ctx, "seller rating", err)
	}
	out.Body.Seller = p
	if out.Body.Listings, _, err = s.feed(ctx, db.FeedListingsParams{
		SellerID: pgtype.UUID{Bytes: u.ID, Valid: true}, Sort: "new", Lim: sellerPageListings,
	}); err != nil {
		return nil, err
	}
	return out, nil
}
