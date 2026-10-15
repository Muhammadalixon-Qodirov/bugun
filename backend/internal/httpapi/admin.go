package httpapi

import (
	"context"
	"errors"
	"math"
	"net/http"
	"slices"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"bugun/internal/market"
	"bugun/internal/roles"
	"bugun/internal/store/db"
)

func (s *Server) registerAdmin(api huma.API) {
	tags := []string{"admin"}
	huma.Register(api, huma.Operation{
		OperationID: "adminSummary", Method: http.MethodGet, Path: "/v1/admin/summary",
		Summary: "Counts for the admin dashboard", Tags: tags, Security: authRequired,
	}, s.adminSummary)
	huma.Register(api, huma.Operation{
		OperationID: "adminReviewQueue", Method: http.MethodGet, Path: "/v1/admin/listings/review",
		Summary: "Listings held for review, oldest first", Tags: tags, Security: authRequired,
	}, s.adminReviewQueue)
	huma.Register(api, huma.Operation{
		OperationID: "adminDecideListing", Method: http.MethodPost, Path: "/v1/admin/listings/{id}/decision",
		Summary: "Approve or reject a listing held for review", Tags: tags, Security: authRequired,
		DefaultStatus: http.StatusNoContent,
	}, s.adminDecideListing)
	huma.Register(api, huma.Operation{
		OperationID: "adminRoleRequests", Method: http.MethodGet, Path: "/v1/admin/role-requests",
		Summary: "Market staff waiting for role approval", Tags: tags, Security: authRequired,
	}, s.adminRoleRequests)
	huma.Register(api, huma.Operation{
		OperationID: "adminDecideRole", Method: http.MethodPost, Path: "/v1/admin/role-requests/{id}/decision",
		Summary: "Approve a requested role, or make the user an entrepreneur", Tags: tags, Security: authRequired,
		DefaultStatus: http.StatusNoContent,
	}, s.adminDecideRole)
}

// isAdmin: role "admin" or a Telegram id listed in ADMIN_TELEGRAM_IDS.
func (s *Server) isAdmin(u db.User) bool {
	return (u.Role.Valid && u.Role.String == roles.Admin) || slices.Contains(s.cfg.AdminTelegramIDs, u.TelegramID)
}

func (s *Server) currentAdmin(ctx context.Context, token string) (db.User, error) {
	u, err := s.currentUser(ctx, token)
	if err != nil {
		return u, err
	}
	if !s.isAdmin(u) {
		return u, apiError(http.StatusForbidden, "admins_only", "Bu bo'lim faqat administratorlar uchun.")
	}
	return u, nil
}

type AdminSummaryOutput struct {
	Body struct {
		ReviewListings int32 `json:"review_listings" doc:"Listings waiting for review"`
		PendingRoles   int32 `json:"pending_roles" doc:"Role requests waiting for approval"`
		Users          int32 `json:"users" doc:"Registered users"`
		Sellers        int32 `json:"sellers" doc:"Users who have posted at least one listing (any status)"`
		LiveListings   int32 `json:"live_listings" doc:"Listings buyers can see now"`
	}
}

func (s *Server) adminSummary(ctx context.Context, in *SessionInput) (*AdminSummaryOutput, error) {
	if _, err := s.currentAdmin(ctx, in.Session); err != nil {
		return nil, err
	}
	r, err := s.q.AdminSummary(ctx)
	if err != nil {
		return nil, internalError(ctx, "admin summary", err)
	}
	out := &AdminSummaryOutput{}
	out.Body.ReviewListings, out.Body.PendingRoles, out.Body.Users, out.Body.Sellers, out.Body.LiveListings =
		r.ReviewListings, r.PendingRoles, r.Users, r.Sellers, r.LiveListings
	return out, nil
}

type ReviewItem struct {
	ID           int64     `json:"id"`
	ProductName  string    `json:"product_name"`
	ProductSlug  string    `json:"product_slug"`
	Category     string    `json:"category"`
	Variety      string    `json:"variety"`
	Description  string    `json:"description"`
	PriceSom     int64     `json:"price_som"`
	Unit         string    `json:"unit"`
	TradeType    string    `json:"trade_type" enum:"ulgurji,chakana"`
	SellerName   string    `json:"seller_name"`
	RegionName   string    `json:"region_name"`
	ProvinceName string    `json:"province_name,omitempty"`
	CoverKey     string    `json:"cover_key,omitempty"`
	Median       int64     `json:"median,omitempty" doc:"Live median in the listing's viloyat (0 when there is none)"`
	Offers       int32     `json:"offers" doc:"How many live prices the median is based on"`
	DiffPercent  *int      `json:"diff_percent,omitempty" doc:"Price vs. that median, in percent"`
	Reason       string    `json:"reason,omitempty" enum:"jump_from_previous,far_from_market,far_from_reference,content_spam,content_abuse,content_prohibited,content_off_category" doc:"Why the automatic check held it: 5x jump from the seller's last price, far from other sellers, far from the official benchmark, or the listing text (content_*: spam, abuse, prohibited goods, not about the product)"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type ReviewQueueOutput struct {
	Body struct {
		Listings []ReviewItem `json:"listings"`
	}
}

func (s *Server) adminReviewQueue(ctx context.Context, in *SessionInput) (*ReviewQueueOutput, error) {
	if _, err := s.currentAdmin(ctx, in.Session); err != nil {
		return nil, err
	}
	rows, err := s.q.AdminReviewQueue(ctx)
	if err != nil {
		return nil, internalError(ctx, "review queue", err)
	}
	out := &ReviewQueueOutput{}
	out.Body.Listings = make([]ReviewItem, len(rows))
	for i, r := range rows {
		it := ReviewItem{
			ID: r.ID, ProductName: r.ProductName, ProductSlug: r.ProductSlug, Category: r.Category, Variety: r.Variety,
			Description: r.Description, PriceSom: r.PriceSom, Unit: r.Unit, TradeType: r.TradeType, SellerName: r.SellerName,
			RegionName: r.RegionName, ProvinceName: r.ProvinceName, CoverKey: r.CoverKey, Offers: r.Offers, Reason: r.ReviewReason,
			UpdatedAt: r.UpdatedAt,
		}
		if r.Median > 0 {
			it.Median = int64(math.Round(r.Median))
			d := int(math.Round(100 * (float64(r.PriceSom) - r.Median) / r.Median))
			it.DiffPercent = &d
		}
		out.Body.Listings[i] = it
	}
	return out, nil
}

type ListingDecisionInput struct {
	SessionInput
	ID   int64 `path:"id" minimum:"1"`
	Body struct {
		Approve bool   `json:"approve" doc:"true publishes the listing, false rejects it"`
		Note    string `json:"note,omitempty" maxLength:"300" doc:"Why it was rejected; the seller sees it"`
	}
}

func (s *Server) adminDecideListing(ctx context.Context, in *ListingDecisionInput) (*struct{}, error) {
	if _, err := s.currentAdmin(ctx, in.Session); err != nil {
		return nil, err
	}
	err := s.listings.Review(ctx, in.ID, in.Body.Approve, in.Body.Note)
	if errors.Is(err, market.ErrNotFound) {
		return nil, apiError(http.StatusNotFound, "review_not_found", "Bu e'lon tekshiruvda emas: ehtimol boshqa administrator hal qilgan.")
	}
	if err != nil {
		return nil, internalError(ctx, "review listing", err)
	}
	return nil, nil
}

type RoleRequest struct {
	ID               uuid.UUID `json:"id"`
	DisplayName      string    `json:"display_name"`
	Role             string    `json:"role" enum:"market_admin"`
	TelegramUsername string    `json:"telegram_username,omitempty"`
	RegionName       string    `json:"region_name,omitempty"`
	ProvinceName     string    `json:"province_name,omitempty"`
	MarketName       string    `json:"market_name,omitempty"`
	RequestedAt      time.Time `json:"requested_at"`
}

type RoleRequestsOutput struct {
	Body struct {
		Requests []RoleRequest `json:"requests"`
	}
}

func (s *Server) adminRoleRequests(ctx context.Context, in *SessionInput) (*RoleRequestsOutput, error) {
	if _, err := s.currentAdmin(ctx, in.Session); err != nil {
		return nil, err
	}
	rows, err := s.q.AdminRoleRequests(ctx)
	if err != nil {
		return nil, internalError(ctx, "role requests", err)
	}
	out := &RoleRequestsOutput{}
	out.Body.Requests = make([]RoleRequest, len(rows))
	for i, r := range rows {
		out.Body.Requests[i] = RoleRequest{
			ID: r.ID, DisplayName: r.DisplayName, Role: r.Role.String, TelegramUsername: r.TelegramUsername.String,
			RegionName: r.RegionName, ProvinceName: r.ProvinceName, MarketName: r.MarketName, RequestedAt: r.UpdatedAt,
		}
	}
	return out, nil
}

type RoleDecisionInput struct {
	SessionInput
	ID   string `path:"id" format:"uuid"`
	Body struct {
		Approve bool `json:"approve" doc:"true confirms the role, false makes the user an entrepreneur"`
	}
}

func (s *Server) adminDecideRole(ctx context.Context, in *RoleDecisionInput) (*struct{}, error) {
	if _, err := s.currentAdmin(ctx, in.Session); err != nil {
		return nil, err
	}
	id, err := uuid.Parse(in.ID)
	if err != nil {
		return nil, apiError(http.StatusNotFound, "role_request_not_found", "Bu so'rov allaqachon hal qilingan.")
	}
	_, err = s.q.DecideRoleRequest(ctx, db.DecideRoleRequestParams{ID: id, Approved: in.Body.Approve})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apiError(http.StatusNotFound, "role_request_not_found", "Bu so'rov allaqachon hal qilingan.")
	}
	if err != nil {
		return nil, internalError(ctx, "decide role", err)
	}
	return nil, nil
}
