package httpapi

import (
	"context"
	"errors"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"bugun/internal/market"
	"bugun/internal/store/db"
)

func (s *Server) registerFeed(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "listListings", Method: http.MethodGet, Path: "/v1/listings",
		Summary: "Public listing feed with filters", Tags: []string{"browse"},
	}, s.listListings)

	huma.Register(api, huma.Operation{
		OperationID: "getListing", Method: http.MethodGet, Path: "/v1/listings/{id}",
		Summary: "Public listing page", Tags: []string{"browse"},
	}, s.getListing)
}

// ListingCard is a listing as shown in feeds.
type ListingCard struct {
	ID           int64     `json:"id"`
	SellerID     string    `json:"seller_id" doc:"Seller's public page id (/v1/sellers/{id})"`
	ProductID    int16     `json:"product_id"`
	ProductSlug  string    `json:"product_slug"`
	ProductName  string    `json:"product_name"`
	Category     string    `json:"category"`
	Variety      string    `json:"variety,omitempty"`
	PriceSom     int64     `json:"price_som"`
	Unit         string    `json:"unit"`
	TradeType    string    `json:"trade_type" enum:"ulgurji,chakana"`
	MinOrder     *float64  `json:"min_order,omitempty"`
	RegionName   string    `json:"region_name"`
	ProvinceName string    `json:"province_name,omitempty"`
	SellerName   string    `json:"seller_name"`
	CoverKey     string    `json:"cover_key,omitempty"`
	PhotoCount   int32     `json:"photo_count"`
	Median       int64     `json:"median,omitempty" doc:"Market median for this product (0 when fewer than 3 prices)"`
	DiffPercent  *int      `json:"diff_percent,omitempty" doc:"Price vs. market median, in percent"`
	Views        int32     `json:"views"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type FeedInput struct {
	Region    int16  `query:"region" minimum:"0"`
	Category  string `query:"category" enum:"sabzavot,meva,gosht,don,oziq_ovqat,qurilish,xizmat,"`
	Product   string `query:"product" maxLength:"60" doc:"Product slug"`
	Query     string `query:"q" maxLength:"60"`
	TradeType string `query:"trade" enum:"ulgurji,chakana,"`
	MinPrice  int64  `query:"min_price" minimum:"0"`
	MaxPrice  int64  `query:"max_price" minimum:"0"`
	Sort      string `query:"sort" enum:"new,price_asc,price_desc,popular,deal" default:"new"`
	Page      int32  `query:"page" minimum:"1" maximum:"500" default:"1"`
	PerPage   int32  `query:"per_page" minimum:"1" maximum:"60" default:"24"`
}

type FeedOutput struct {
	CacheControl string `header:"Cache-Control"`
	Body         struct {
		Listings []ListingCard `json:"listings"`
		Total    int32         `json:"total"`
		Page     int32         `json:"page"`
		PerPage  int32         `json:"per_page"`
	}
}

func optInt64(v int64) pgtype.Int8 { return pgtype.Int8{Int64: v, Valid: v > 0} }

func (s *Server) feed(ctx context.Context, p db.FeedListingsParams) ([]ListingCard, int32, error) {
	rows, err := s.q.FeedListings(ctx, p)
	if err != nil {
		return nil, 0, internalError(ctx, "feed listings", err)
	}
	cards := make([]ListingCard, len(rows))
	var total int32
	for i, r := range rows {
		total = r.Total
		c := ListingCard{
			ID: r.ID, SellerID: r.SellerID.String(), ProductID: r.ProductID, ProductSlug: r.ProductSlug, ProductName: r.ProductName, Category: r.Category,
			Variety: r.Variety, PriceSom: r.PriceSom, Unit: r.Unit, TradeType: r.TradeType, MinOrder: market.Float(r.MinOrder),
			RegionName: r.RegionName, ProvinceName: r.ProvinceName, SellerName: r.SellerName,
			CoverKey: r.CoverKey, PhotoCount: r.PhotoCount, Views: r.Views, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
		}
		if r.Median > 0 {
			c.Median = int64(math.Round(r.Median))
			d := int(math.Round(100 * (float64(r.PriceSom) - r.Median) / r.Median))
			c.DiffPercent = &d
		}
		cards[i] = c
	}
	return cards, total, nil
}

func (s *Server) listListings(ctx context.Context, in *FeedInput) (*FeedOutput, error) {
	cards, total, err := s.feed(ctx, db.FeedListingsParams{
		RegionID: optRegion(in.Region), Category: optText(in.Category), Product: optText(in.Product),
		TradeType: optText(in.TradeType), MinPrice: optInt64(in.MinPrice), MaxPrice: optInt64(in.MaxPrice),
		Query: optText(strings.TrimSpace(in.Query)), Sort: in.Sort,
		Lim: in.PerPage, Off: (in.Page - 1) * in.PerPage,
	})
	if err != nil {
		return nil, err
	}
	out := &FeedOutput{CacheControl: "public, max-age=30"}
	out.Body.Listings, out.Body.Total, out.Body.Page, out.Body.PerPage = cards, total, in.Page, in.PerPage
	return out, nil
}

type SellerInfo struct {
	ID          string        `json:"id" doc:"Seller's public page id (/v1/sellers/{id})"`
	Name        string        `json:"name"`
	RegionName  string        `json:"region_name"`
	MarketName  string        `json:"market_name,omitempty"`
	Since       time.Time     `json:"since"`
	Listings    int32         `json:"listings"`
	HasTelegram bool          `json:"has_telegram"`
	HasPhone    bool          `json:"has_phone"`
	Rating      *SellerRating `json:"rating,omitempty" doc:"Buyers' ratings of confirmed deals; omitted under 3 ratings"`
}

type ListingDetail struct {
	ListingCard
	Description string    `json:"description,omitempty"`
	Stock       *float64  `json:"stock,omitempty"`
	ValidUntil  time.Time `json:"valid_until"`
	Photos      []string  `json:"photos"`
}

type ListingPageInput struct {
	ID      int64  `path:"id" minimum:"1"`
	Region  int16  `query:"region" minimum:"0" doc:"Region for the market price comparison"`
	Session string `cookie:"bugun_session" doc:"Optional; counts the view once a day for this user"`
}

type ListingPageOutput struct {
	Body struct {
		Listing     ListingDetail `json:"listing"`
		Seller      SellerInfo    `json:"seller"`
		Price       Price         `json:"price" doc:"Market price for this product and trade type"`
		PriceScope  *Region       `json:"price_scope,omitempty"`
		FromSeller  []ListingCard `json:"from_seller"`
		OtherOffers []ListingCard `json:"other_offers" doc:"Same product from other sellers, cheapest first"`
	}
}

func (s *Server) getListing(ctx context.Context, in *ListingPageInput) (*ListingPageOutput, error) {
	l, err := s.q.PublicListing(ctx, in.ID)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && (l.Status != market.StatusActive || l.ValidUntil.Before(time.Now()))) {
		return nil, apiError(http.StatusNotFound, "listing_not_found", "E'lon topilmadi yoki muddati o'tgan.")
	}
	if err != nil {
		return nil, internalError(ctx, "public listing", err)
	}
	if vid, viewer := s.visitor(ctx, in.Session); viewer == nil || viewer.ID != l.SellerID {
		if err := s.q.IncrementListingView(ctx, l.ID); err != nil {
			return nil, internalError(ctx, "count view", err)
		}
		s.recordListingEvent(ctx, "view", l.ID, vid)
	}
	photos, err := s.q.ListListingPhotos(ctx, l.ID)
	if err != nil {
		return nil, internalError(ctx, "listing photos", err)
	}
	region := in.Region
	if region == 0 {
		region = l.RegionID
	}
	price, scope, err := s.marketPrice(ctx, l.ProductID, l.TradeType, region)
	if err != nil {
		return nil, err
	}

	out := &ListingPageOutput{}
	d := ListingDetail{
		ListingCard: ListingCard{
			ID: l.ID, SellerID: l.SellerID.String(), ProductID: l.ProductID, ProductSlug: l.ProductSlug, ProductName: l.ProductName, Category: l.Category,
			Variety: l.Variety, PriceSom: l.PriceSom, Unit: l.Unit, TradeType: l.TradeType, MinOrder: market.Float(l.MinOrder),
			RegionName: l.RegionName, ProvinceName: l.ProvinceName, SellerName: l.SellerName,
			PhotoCount: int32(len(photos)), Views: l.Views + 1, CreatedAt: l.CreatedAt, UpdatedAt: l.UpdatedAt,
		},
		Description: l.Description, Stock: market.Float(l.Stock), ValidUntil: l.ValidUntil, Photos: make([]string, len(photos)),
	}
	for i, ph := range photos {
		d.Photos[i] = ph.StorageKey
	}
	if len(photos) > 0 {
		d.CoverKey = photos[0].StorageKey
	}
	if price.Enough {
		d.Median = price.Median
		diff := int(math.Round(100 * float64(l.PriceSom-price.Median) / float64(price.Median)))
		d.DiffPercent = &diff
	}
	out.Body.Listing = d
	out.Body.Seller = SellerInfo{
		ID: l.SellerID.String(), Name: l.SellerName, RegionName: l.RegionName, MarketName: l.MarketName, Since: l.SellerSince,
		Listings: l.SellerListings, HasTelegram: l.HasTelegram, HasPhone: l.HasPhone,
	}
	if out.Body.Seller.Rating, err = s.sellerRating(ctx, l.SellerID); err != nil {
		return nil, internalError(ctx, "seller rating", err)
	}
	out.Body.Price, out.Body.PriceScope = price, scope

	exclude := pgtype.Int8{Int64: l.ID, Valid: true}
	if out.Body.FromSeller, _, err = s.feed(ctx, db.FeedListingsParams{
		SellerID: pgtype.UUID{Bytes: l.SellerID, Valid: true}, ExcludeID: exclude, Sort: "new", Lim: 8,
	}); err != nil {
		return nil, err
	}
	if out.Body.OtherOffers, _, err = s.feed(ctx, db.FeedListingsParams{
		Product: optText(l.ProductSlug), TradeType: optText(l.TradeType), ExcludeID: exclude, Sort: "price_asc", Lim: 8,
	}); err != nil {
		return nil, err
	}
	return out, nil
}
