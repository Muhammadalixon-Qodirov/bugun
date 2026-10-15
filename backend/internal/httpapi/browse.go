package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"bugun/internal/analytics"
	"bugun/internal/market"
	"bugun/internal/store/db"
)

func (s *Server) registerBrowse(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "browseProducts", Method: http.MethodGet, Path: "/v1/products",
		Summary: "Catalog with median prices for a region", Tags: []string{"browse"},
	}, s.browseProducts)

	huma.Register(api, huma.Operation{
		OperationID: "getProduct", Method: http.MethodGet, Path: "/v1/products/{slug}",
		Summary: "Product page: market price and every seller's offer", Tags: []string{"browse"},
	}, s.getProduct)

	huma.Register(api, huma.Operation{
		OperationID: "revealContact", Method: http.MethodPost, Path: "/v1/listings/{id}/contact",
		Summary: "Seller contacts for a listing (sign-in required)", Tags: []string{"browse"}, Security: authRequired,
	}, s.revealContact)
}

// Price is a market price with the context spec §7.4 requires next to every number.
type Price struct {
	Enough    bool      `json:"enough" doc:"false when fewer than 3 prices: no median is shown"`
	Offers    int32     `json:"offers"`
	Median    int64     `json:"median,omitempty"`
	P25       int64     `json:"p25,omitempty"`
	P75       int64     `json:"p75,omitempty"`
	Min       int64     `json:"min,omitempty"`
	Max       int64     `json:"max,omitempty"`
	FirstSeen time.Time `json:"first_seen,omitzero"`
	LastSeen  time.Time `json:"last_seen,omitzero"`
}

type ProductCard struct {
	Product
	Price    Price  `json:"price"`
	CoverKey string `json:"cover_key,omitempty"`
}

type BrowseInput struct {
	Category  string `query:"category" enum:"sabzavot,meva,gosht,don,oziq_ovqat,qurilish,xizmat," doc:"Empty for all"`
	Region    int16  `query:"region" minimum:"0" doc:"Viloyat or tuman id; 0 for the whole country"`
	Query     string `query:"q" maxLength:"60"`
	TradeType string `query:"trade" enum:"ulgurji,chakana" default:"ulgurji"`
	Sort      string `query:"sort" enum:"popular,price_asc,price_desc,name" default:"popular"`
	WithOffer bool   `query:"with_offers" doc:"Only products that have offers"`
	Session   string `cookie:"bugun_session" doc:"Optional; counts the search as demand for this user"`
}

type BrowseOutput struct {
	Body struct {
		Products []ProductCard `json:"products"`
	}
}

func optText(v string) pgtype.Text  { return pgtype.Text{String: v, Valid: v != ""} }
func optRegion(v int16) pgtype.Int2 { return pgtype.Int2{Int16: v, Valid: v > 0} }

func (s *Server) browseProducts(ctx context.Context, in *BrowseInput) (*BrowseOutput, error) {
	rows, err := s.q.BrowseProducts(ctx, db.BrowseProductsParams{
		Category: optText(in.Category), Query: optText(strings.TrimSpace(in.Query)), OnlyOffers: in.WithOffer,
		Sort: in.Sort, TradeType: in.TradeType, RegionID: optRegion(in.Region),
	})
	if err != nil {
		return nil, internalError(ctx, "browse products", err)
	}
	// A search that finds a product is demand for it: the exact name match when
	// there is one ("karam" is not "gulkaram"), else the top result.
	if q := strings.TrimSpace(in.Query); q != "" && len(rows) > 0 {
		match := rows[0].ID
		for _, r := range rows {
			if strings.EqualFold(r.Name, q) || strings.EqualFold(r.Slug, q) {
				match = r.ID
				break
			}
		}
		vid, viewer := s.visitor(ctx, in.Session)
		s.recordDemand(ctx, "search", match, s.demandProvince(ctx, in.Region, viewer), vid)
	}
	out := &BrowseOutput{}
	out.Body.Products = make([]ProductCard, len(rows))
	for i, r := range rows {
		c := ProductCard{
			Product:  Product{ID: r.ID, Slug: r.Slug, Name: r.Name, Category: r.Category, BaseUnit: r.BaseUnit},
			Price:    Price{Offers: r.Offers},
			CoverKey: r.CoverKey,
		}
		if r.Offers >= analytics.MinObservations {
			c.Price.Enough = true
			c.Price.Median = int64(math.Round(r.Median))
			c.Price.Min, c.Price.Max = r.MinPrice, r.MaxPrice
		}
		out.Body.Products[i] = c
	}
	return out, nil
}

type Offer struct {
	ID           int64     `json:"id"`
	Variety      string    `json:"variety,omitempty"`
	Description  string    `json:"description,omitempty"`
	PriceSom     int64     `json:"price_som"`
	Unit         string    `json:"unit"`
	MinOrder     *float64  `json:"min_order,omitempty"`
	Stock        *float64  `json:"stock,omitempty"`
	TradeType    string    `json:"trade_type" enum:"ulgurji,chakana"`
	SellerID     string    `json:"seller_id" doc:"Seller's public page id (/v1/sellers/{id})"`
	SellerName   string    `json:"seller_name"`
	RegionName   string    `json:"region_name"`
	ProvinceName string    `json:"province_name,omitempty"`
	MarketName   string    `json:"market_name,omitempty"`
	HasTelegram  bool      `json:"has_telegram"`
	HasPhone     bool      `json:"has_phone"`
	Photos       []string  `json:"photos"`
	DiffPercent  *int      `json:"diff_percent,omitempty" doc:"Difference from the market median, in percent"`
	UpdatedAt    time.Time `json:"updated_at"`
	ValidUntil   time.Time `json:"valid_until"`
}

type ProvinceMedian struct {
	RegionID int16 `json:"region_id"`
	Offers   int32 `json:"offers"`
	Median   int64 `json:"median"`
}

type ProductInput struct {
	Slug      string `path:"slug" maxLength:"60"`
	Region    int16  `query:"region" minimum:"0"`
	TradeType string `query:"trade" enum:"ulgurji,chakana" default:"ulgurji"`
	Session   string `cookie:"bugun_session" doc:"Optional; counts the view as demand for this user"`
}

type ProductOutput struct {
	Body struct {
		Product Product `json:"product"`
		Price   Price   `json:"price"`
		// PriceScope is the region the median was computed for. When the chosen
		// tuman has too few prices it widens to the viloyat, then the country.
		PriceScope *Region          `json:"price_scope,omitempty" doc:"Omitted when the scope is the whole country"`
		Offers     []Offer          `json:"offers"`
		Provinces  []ProvinceMedian `json:"provinces" doc:"Median per viloyat, only where there are at least 3 prices"`
		Interest   *Interest        `json:"interest,omitempty" doc:"Interest today and confirmed deals this week in the chosen viloyat (or the country); numbers under 5 are left out"`
	}
}

func (s *Server) getProduct(ctx context.Context, in *ProductInput) (*ProductOutput, error) {
	p, err := s.q.GetProductBySlug(ctx, in.Slug)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apiError(http.StatusNotFound, "product_not_found", "Mahsulot topilmadi.")
	}
	if err != nil {
		return nil, internalError(ctx, "get product", err)
	}
	out := &ProductOutput{}
	out.Body.Product = Product{ID: p.ID, Slug: p.Slug, Name: p.Name, Category: p.Category, BaseUnit: p.BaseUnit}

	out.Body.Price, out.Body.PriceScope, err = s.marketPrice(ctx, p.ID, in.TradeType, in.Region)
	if err != nil {
		return nil, err
	}

	rows, err := s.q.ProductListings(ctx, db.ProductListingsParams{
		ProductID: p.ID, TradeType: optText(in.TradeType), RegionID: optRegion(in.Region),
	})
	if err != nil {
		return nil, internalError(ctx, "product listings", err)
	}
	out.Body.Offers = make([]Offer, len(rows))
	for i, r := range rows {
		o := Offer{
			ID: r.ID, Variety: r.Variety, Description: r.Description, PriceSom: r.PriceSom, Unit: r.Unit,
			MinOrder: market.Float(r.MinOrder), Stock: market.Float(r.Stock), TradeType: r.TradeType,
			SellerID: r.SellerID.String(), SellerName: r.SellerName, RegionName: r.RegionName, ProvinceName: r.ProvinceName, MarketName: r.MarketName,
			HasTelegram: r.HasTelegram, HasPhone: r.HasPhone, Photos: r.PhotoKeys,
			UpdatedAt: r.UpdatedAt, ValidUntil: r.ValidUntil,
		}
		if out.Body.Price.Enough {
			d := int(math.Round(100 * float64(r.PriceSom-out.Body.Price.Median) / float64(out.Body.Price.Median)))
			o.DiffPercent = &d
		}
		out.Body.Offers[i] = o
	}

	provinces, err := s.q.ProductProvinceMedians(ctx, db.ProductProvinceMediansParams{ProductID: p.ID, TradeType: in.TradeType})
	if err != nil {
		return nil, internalError(ctx, "province medians", err)
	}
	out.Body.Provinces = []ProvinceMedian{}
	for _, pm := range provinces {
		if pm.Offers >= analytics.MinObservations {
			out.Body.Provinces = append(out.Body.Provinces, ProvinceMedian{RegionID: pm.ProvinceID, Offers: pm.Offers, Median: int64(math.Round(pm.Median))})
		}
	}
	vid, viewer := s.visitor(ctx, in.Session)
	s.recordDemand(ctx, "view", p.ID, s.demandProvince(ctx, in.Region, viewer), vid)
	var province int16
	if in.Region > 0 {
		province = s.demandProvince(ctx, in.Region, nil)
	}
	if out.Body.Interest, err = s.productInterest(ctx, p.ID, province); err != nil {
		return nil, internalError(ctx, "product interest", err)
	}
	return out, nil
}

// marketPrice is the median for a product, widening the scope until there are
// enough prices: tuman -> viloyat -> whole country. The returned region is the
// scope used (nil for the whole country).
func (s *Server) marketPrice(ctx context.Context, productID int16, trade string, region int16) (Price, *Region, error) {
	scopes := []int16{}
	if region > 0 {
		r, err := s.q.GetRegion(ctx, region)
		if errors.Is(err, pgx.ErrNoRows) {
			return Price{}, nil, apiError(http.StatusUnprocessableEntity, "region_not_found", "Bunday hudud topilmadi.")
		}
		if err != nil {
			return Price{}, nil, internalError(ctx, "get region", err)
		}
		scopes = append(scopes, r.ID)
		if r.ParentID.Valid {
			scopes = append(scopes, r.ParentID.Int16)
		}
	}
	scopes = append(scopes, 0)
	var price Price
	for _, sc := range scopes {
		st, err := s.q.ProductStats(ctx, db.ProductStatsParams{ProductID: productID, TradeType: trade, RegionID: optRegion(sc)})
		if err != nil {
			return Price{}, nil, internalError(ctx, "product stats", err)
		}
		price = Price{Offers: st.Offers}
		if st.Offers < analytics.MinObservations {
			continue
		}
		price = Price{
			Enough: true, Offers: st.Offers, Median: int64(math.Round(st.Median)),
			P25: int64(math.Round(st.P25)), P75: int64(math.Round(st.P75)),
			Min: st.MinPrice, Max: st.MaxPrice, FirstSeen: st.FirstSeen, LastSeen: st.LastSeen,
		}
		if sc == 0 {
			return price, nil, nil
		}
		r, err := s.q.GetRegion(ctx, sc)
		if err != nil {
			return Price{}, nil, internalError(ctx, "get region", err)
		}
		return price, &Region{ID: r.ID, Slug: r.Slug, Name: r.Name, Kind: r.Kind}, nil
	}
	return price, nil, nil
}

type ContactOutput struct {
	Body struct {
		TelegramURL string `json:"telegram_url,omitempty"`
		Phone       string `json:"phone,omitempty"`
	}
}

func (s *Server) revealContact(ctx context.Context, in *ListingIDInput) (*ContactOutput, error) {
	u, err := s.currentUser(ctx, in.Session)
	if err != nil {
		return nil, err
	}
	c, err := s.q.ListingContact(ctx, in.ID)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && c.Status != market.StatusActive && c.SellerID != u.ID) {
		return nil, apiError(http.StatusNotFound, "listing_not_found", "E'lon topilmadi.")
	}
	if err != nil {
		return nil, internalError(ctx, "listing contact", err)
	}
	out := &ContactOutput{}
	if c.ContactTelegram && c.TelegramUsername.Valid {
		out.Body.TelegramURL = "https://t.me/" + c.TelegramUsername.String
	}
	if c.ContactPhone && len(c.PhoneEnc) > 0 {
		phone, err := s.sealer.Open(c.PhoneEnc)
		if err != nil {
			return nil, internalError(ctx, "open phone", err)
		}
		out.Body.Phone = phone
	}
	if out.Body.TelegramURL == "" && out.Body.Phone == "" {
		return nil, apiError(http.StatusNotFound, "contact_hidden", "Sotuvchi hozircha bog'lanish usulini ochmagan.")
	}
	// Every reveal is a buyer's interest, whether by phone or Telegram.
	if c.SellerID != u.ID {
		if err := s.q.LogContactReveal(ctx, db.LogContactRevealParams{
			ListingID: in.ID, ViewerID: pgtype.UUID{Bytes: u.ID, Valid: true},
		}); err != nil {
			return nil, internalError(ctx, "log reveal", err)
		}
		s.recordDemand(ctx, "contact", c.ProductID, s.demandProvince(ctx, c.RegionID, nil), "u:"+u.ID.String())
		s.recordListingEvent(ctx, "contact", in.ID, "u:"+u.ID.String())
		if s.deals != nil { // a day later the bot asks whether they bought
			if err := s.deals.ScheduleAfterContact(ctx, u.ID, in.ID); err != nil {
				slog.WarnContext(ctx, "schedule deal prompt", "err", err)
			}
		}
	}
	return out, nil
}
