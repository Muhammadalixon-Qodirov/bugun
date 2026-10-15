package httpapi

import (
	"context"
	"math"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5/pgtype"

	"bugun/internal/market"
	"bugun/internal/store/db"
)

// Interest and deal funnels (QIZIQISH_VA_SAVDO.md step 6). A seller sees all
// of their own numbers; public and comparison numbers appear only from
// minPublic up, so no single buyer or seller can be singled out.
const minPublic = 5

func (s *Server) registerFunnel(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "listingStats", Method: http.MethodGet, Path: "/v1/me/listings/{id}/stats",
		Summary: "Views, contacts and deals of one of my listings per day, with the viloyat's average", Tags: []string{"listings"}, Security: authRequired,
	}, s.listingStats)
	huma.Register(api, huma.Operation{
		OperationID: "adminDeals", Method: http.MethodGet, Path: "/v1/admin/deals",
		Summary: "Latest deals, optionally by status", Tags: []string{"admin"}, Security: authRequired,
	}, s.adminDeals)
}

// Funnel is a listing's last 7 days in "Mahsulotlarim".
type Funnel struct {
	Views    int32 `json:"views" doc:"Unique visitors of the listing page"`
	Contacts int32 `json:"contacts" doc:"Unique visitors who opened the contact"`
	Deals    int32 `json:"deals" doc:"Sales reported or confirmed"`
}

type StatsInput struct {
	SessionInput
	ID   int64 `path:"id" minimum:"1"`
	Days int32 `query:"days" enum:"7,30" default:"7"`
}

type StatsDay struct {
	Date     string `json:"date" doc:"Tashkent day, YYYY-MM-DD"`
	Views    int32  `json:"views"`
	Contacts int32  `json:"contacts"`
}

// MarketStats is the average other live listing of the product in the viloyat.
type MarketStats struct {
	Listings           int32   `json:"listings"`
	ViewsPerListing    float64 `json:"views_per_listing"`
	ContactsPerListing float64 `json:"contacts_per_listing"`
	ContactRate        float64 `json:"contact_rate" doc:"Contacts per view, 0–1"`
}

type StatsOutput struct {
	Body struct {
		Days           int32        `json:"days"`
		Views          int32        `json:"views"`
		Contacts       int32        `json:"contacts"`
		ContactRate    float64      `json:"contact_rate" doc:"Contacts per view, 0–1; 0 without views"`
		DealsConfirmed int32        `json:"deals_confirmed" doc:"Both sides said yes (or an RFQ agreement)"`
		DealsReported  int32        `json:"deals_reported" doc:"Only one side has said so yet"`
		SoldQuantity   float64      `json:"sold_quantity" doc:"Sum of amounts given for those deals, in unit"`
		Unit           string       `json:"unit"`
		Series         []StatsDay   `json:"series" doc:"One entry per day, oldest first"`
		Market         *MarketStats `json:"market,omitempty" doc:"Omitted when the viloyat has fewer than 5 other live listings of the product"`
	}
}

func (s *Server) listingStats(ctx context.Context, in *StatsInput) (*StatsOutput, error) {
	u, err := s.currentUser(ctx, in.Session)
	if err != nil {
		return nil, err
	}
	l, _, err := s.listings.Owned(ctx, u.ID, in.ID)
	if err != nil {
		return nil, listingError(ctx, err)
	}
	days, err := s.q.ListingFunnelDays(ctx, db.ListingFunnelDaysParams{ListingID: l.ID, NumDays: in.Days})
	if err != nil {
		return nil, internalError(ctx, "funnel days", err)
	}
	totals, err := s.q.ListingDealTotals(ctx, db.ListingDealTotalsParams{ListingID: pgtype.Int8{Int64: l.ID, Valid: true}, NumDays: in.Days})
	if err != nil {
		return nil, internalError(ctx, "deal totals", err)
	}
	out := &StatsOutput{}
	b := &out.Body
	b.Days, b.Unit = in.Days, l.Unit
	b.DealsConfirmed, b.DealsReported, b.SoldQuantity = totals.Confirmed, totals.Reported, totals.Quantity
	b.Series = make([]StatsDay, len(days))
	for i, d := range days {
		b.Series[i] = StatsDay{Date: d.Day.Time.Format(time.DateOnly), Views: d.Views, Contacts: d.Contacts}
		b.Views += d.Views
		b.Contacts += d.Contacts
	}
	b.ContactRate = rate(b.Contacts, b.Views)

	province, err := market.ProvinceOf(ctx, s.q, l.RegionID)
	if err != nil {
		return nil, internalError(ctx, "province", err)
	}
	m, err := s.q.MarketFunnel(ctx, db.MarketFunnelParams{
		NumDays: in.Days, ProductID: l.ProductID, TradeType: l.TradeType, ListingID: l.ID, ProvinceID: province,
	})
	if err != nil {
		return nil, internalError(ctx, "market funnel", err)
	}
	if m.Listings >= minPublic {
		b.Market = &MarketStats{
			Listings:           m.Listings,
			ViewsPerListing:    math.Round(float64(m.Views)/float64(m.Listings)*10) / 10,
			ContactsPerListing: math.Round(float64(m.Contacts)/float64(m.Listings)*10) / 10,
			ContactRate:        rate(m.Contacts, m.Views),
		}
	}
	return out, nil
}

func rate(part, whole int32) float64 {
	if whole == 0 {
		return 0
	}
	return math.Round(float64(part)/float64(whole)*1000) / 1000
}

// sellerFunnels maps each of the seller's listings to its last 7 days.
func (s *Server) sellerFunnels(ctx context.Context, u db.User) (map[int64]Funnel, error) {
	rows, err := s.q.SellerFunnels(ctx, u.ID)
	if err != nil {
		return nil, err
	}
	m := make(map[int64]Funnel, len(rows))
	for _, r := range rows {
		m[r.ListingID] = Funnel{Views: r.Views, Contacts: r.Contacts, Deals: r.Deals}
	}
	return m, nil
}

// Interest is public interest in a product. Each number is omitted below 5.
type Interest struct {
	VisitorsToday *int32 `json:"visitors_today,omitempty" doc:"Distinct people who viewed, searched, asked or contacted about the product today"`
	Deals7d       *int32 `json:"deals_7d,omitempty" doc:"Confirmed deals in the last 7 days"`
	DealMedian7d  *int64 `json:"deal_median_7d,omitempty" doc:"Median agreed price of those deals that gave one"`
}

// productInterest returns nil when there is nothing to show.
func (s *Server) productInterest(ctx context.Context, productID, province int16) (*Interest, error) {
	r, err := s.q.ProductInterest(ctx, db.ProductInterestParams{
		ProductID: productID, ProvinceID: pgtype.Int2{Int16: province, Valid: province != 0},
	})
	if err != nil {
		return nil, err
	}
	var in Interest
	if r.VisitorsToday >= minPublic {
		in.VisitorsToday = &r.VisitorsToday
	}
	if r.Deals >= minPublic {
		in.Deals7d = &r.Deals
	}
	if r.Priced >= minPublic {
		m := int64(math.Round(r.Median))
		in.DealMedian7d = &m
	}
	if in == (Interest{}) {
		return nil, nil
	}
	return &in, nil
}

type AdminDealsInput struct {
	SessionInput
	Status string `query:"status" enum:"reported,confirmed,disputed" doc:"Empty for all"`
}

type AdminDeal struct {
	ID              int64      `json:"id"`
	Status          string     `json:"status"`
	Source          string     `json:"source" enum:"rfq,contact,seller_marked"`
	ProductName     string     `json:"product_name"`
	SellerName      string     `json:"seller_name"`
	RegionName      string     `json:"region_name"`
	ListingID       *int64     `json:"listing_id,omitempty"`
	HasBuyer        bool       `json:"has_buyer"`
	Quantity        *float64   `json:"quantity,omitempty"`
	Unit            string     `json:"unit"`
	PriceSom        *int64     `json:"price_som,omitempty"`
	BuyerConfirmed  *time.Time `json:"buyer_confirmed_at,omitempty"`
	SellerConfirmed *time.Time `json:"seller_confirmed_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
}

type AdminDealsOutput struct {
	Body struct {
		Deals []AdminDeal `json:"deals"`
	}
}

func (s *Server) adminDeals(ctx context.Context, in *AdminDealsInput) (*AdminDealsOutput, error) {
	if _, err := s.currentAdmin(ctx, in.Session); err != nil {
		return nil, err
	}
	rows, err := s.q.AdminDeals(ctx, pgtype.Text{String: in.Status, Valid: in.Status != ""})
	if err != nil {
		return nil, internalError(ctx, "admin deals", err)
	}
	out := &AdminDealsOutput{}
	out.Body.Deals = make([]AdminDeal, len(rows))
	for i, r := range rows {
		d := AdminDeal{
			ID: r.ID, Status: r.Status, Source: r.Source, ProductName: r.ProductName, SellerName: r.SellerName,
			RegionName: r.RegionName, HasBuyer: r.HasBuyer, Quantity: market.Float(r.Quantity), Unit: r.Unit,
			BuyerConfirmed: r.BuyerConfirmedAt, SellerConfirmed: r.SellerConfirmedAt, CreatedAt: r.CreatedAt,
		}
		if r.ListingID.Valid {
			d.ListingID = &r.ListingID.Int64
		}
		if r.PriceSom.Valid {
			d.PriceSom = &r.PriceSom.Int64
		}
		out.Body.Deals[i] = d
	}
	return out, nil
}
