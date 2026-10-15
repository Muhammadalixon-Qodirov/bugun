package httpapi

import (
	"context"
	"math"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"

	"bugun/internal/store/db"
)

// Seller ratings (QIZIQISH_VA_SAVDO.md step 9): only buyers of confirmed
// deals rate, through the bot. Shown from minRatings up.
const minRatings = 3

func (s *Server) registerRatings(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "adminHeldRatings", Method: http.MethodGet, Path: "/v1/admin/ratings/held",
		Summary: "Low ratings with a comment waiting for review", Tags: []string{"admin"}, Security: authRequired,
	}, s.adminHeldRatings)
	huma.Register(api, huma.Operation{
		OperationID: "adminDecideRating", Method: http.MethodPost, Path: "/v1/admin/ratings/{id}/decision",
		Summary: "Show or hide a held rating comment", Tags: []string{"admin"}, Security: authRequired,
		DefaultStatus: http.StatusNoContent,
	}, s.adminDecideRating)
}

type RatingComment struct {
	Stars     int16     `json:"stars"`
	Comment   string    `json:"comment"`
	CreatedAt time.Time `json:"created_at"`
}

// SellerRating is shown once a seller has minRatings ratings.
type SellerRating struct {
	Average  float64         `json:"average" doc:"1–5, one decimal"`
	Count    int32           `json:"count"`
	Comments []RatingComment `json:"comments" doc:"Latest shown comments, at most 3"`
}

func (s *Server) sellerRating(ctx context.Context, sellerID uuid.UUID) (*SellerRating, error) {
	r, err := s.q.SellerRating(ctx, sellerID)
	if err != nil || r.Ratings < minRatings {
		return nil, err
	}
	rows, err := s.q.SellerRatingComments(ctx, sellerID)
	if err != nil {
		return nil, err
	}
	out := &SellerRating{Average: math.Round(r.Average*10) / 10, Count: r.Ratings, Comments: make([]RatingComment, len(rows))}
	for i, c := range rows {
		out.Comments[i] = RatingComment{Stars: c.Stars, Comment: c.Comment, CreatedAt: c.CreatedAt}
	}
	return out, nil
}

type HeldRating struct {
	ID          int64     `json:"id"`
	Stars       int16     `json:"stars"`
	Comment     string    `json:"comment"`
	SellerName  string    `json:"seller_name"`
	ProductName string    `json:"product_name"`
	CreatedAt   time.Time `json:"created_at"`
}

type HeldRatingsOutput struct {
	Body struct {
		Ratings []HeldRating `json:"ratings"`
	}
}

func (s *Server) adminHeldRatings(ctx context.Context, in *SessionInput) (*HeldRatingsOutput, error) {
	if _, err := s.currentAdmin(ctx, in.Session); err != nil {
		return nil, err
	}
	rows, err := s.q.HeldRatings(ctx)
	if err != nil {
		return nil, internalError(ctx, "held ratings", err)
	}
	out := &HeldRatingsOutput{}
	out.Body.Ratings = make([]HeldRating, len(rows))
	for i, r := range rows {
		out.Body.Ratings[i] = HeldRating{ID: r.ID, Stars: r.Stars, Comment: r.Comment, SellerName: r.SellerName, ProductName: r.ProductName, CreatedAt: r.CreatedAt}
	}
	return out, nil
}

type RatingDecisionInput struct {
	SessionInput
	ID   int64 `path:"id" minimum:"1"`
	Body struct {
		Publish bool `json:"publish" doc:"true shows the comment, false hides it (the stars still count)"`
	}
}

func (s *Server) adminDecideRating(ctx context.Context, in *RatingDecisionInput) (*struct{}, error) {
	if _, err := s.currentAdmin(ctx, in.Session); err != nil {
		return nil, err
	}
	status := "hidden"
	if in.Body.Publish {
		status = "published"
	}
	n, err := s.q.DecideRating(ctx, db.DecideRatingParams{ID: in.ID, Status: status})
	if err != nil {
		return nil, internalError(ctx, "decide rating", err)
	}
	if n == 0 {
		return nil, apiError(http.StatusNotFound, "rating_not_found", "Bu baho tekshiruvda emas.")
	}
	return nil, nil
}
