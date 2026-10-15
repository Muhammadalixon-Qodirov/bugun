package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"

	"bugun/internal/insight"
)

// Unusual sellers for admins (ML_REJA.md 5): the nightly Isolation Forest
// scores, highest first, with the features that stand out.

func (s *Server) registerSuspicious(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "adminSuspiciousSellers", Method: http.MethodGet, Path: "/v1/admin/sellers/suspicious",
		Summary: "Sellers unlike the rest (Isolation Forest over the last 60 days), a hint to check, not a verdict", Tags: []string{"admin"}, Security: authRequired,
	}, s.adminSuspiciousSellers)
}

type SuspiciousSeller struct {
	ID           uuid.UUID              `json:"id"`
	Name         string                 `json:"name"`
	Username     string                 `json:"username,omitempty" doc:"Telegram username without @"`
	Demo         bool                   `json:"demo"`
	Score        float64                `json:"score" doc:"0–1; from 0.6 unusual, near 1 very unusual"`
	Reasons      []string               `json:"reasons" doc:"Features far above the other sellers: review_share, deviation, below_share, above_share, per_listing, max_listings_day; empty means an unusual combination"`
	Features     insight.SellerFeatures `json:"features"`
	LiveListings int32                  `json:"live_listings"`
}

type SuspiciousOutput struct {
	Body struct {
		Scored     int                `json:"scored" doc:"Sellers scored in the last run; 0 while there are fewer than min_sellers"`
		MinSellers int                `json:"min_sellers"`
		Threshold  float64            `json:"threshold"`
		ComputedAt *time.Time         `json:"computed_at,omitempty"`
		Sellers    []SuspiciousSeller `json:"sellers" doc:"Score at or above threshold, highest first, up to 100"`
	}
}

func (s *Server) adminSuspiciousSellers(ctx context.Context, in *SessionInput) (*SuspiciousOutput, error) {
	if _, err := s.currentAdmin(ctx, in.Session); err != nil {
		return nil, err
	}
	stats, err := s.q.SellerScoreStats(ctx)
	if err != nil {
		return nil, internalError(ctx, "seller score stats", err)
	}
	rows, err := s.q.SuspiciousSellers(ctx, insight.SuspiciousScore)
	if err != nil {
		return nil, internalError(ctx, "suspicious sellers", err)
	}
	out := &SuspiciousOutput{}
	out.Body.Scored, out.Body.MinSellers, out.Body.Threshold = int(stats.Scored), insight.MinScoredSellers, insight.SuspiciousScore
	if stats.Scored > 0 {
		out.Body.ComputedAt = &stats.ComputedAt
	}
	out.Body.Sellers = make([]SuspiciousSeller, 0, len(rows))
	for _, r := range rows {
		var f insight.SellerFeatures
		if err := json.Unmarshal(r.Features, &f); err != nil {
			return nil, internalError(ctx, "seller features", err)
		}
		out.Body.Sellers = append(out.Body.Sellers, SuspiciousSeller{
			ID: r.SellerID, Name: r.DisplayName, Username: r.TelegramUsername.String, Demo: r.IsDemo, Score: r.Score,
			Reasons: r.Reasons, Features: f, LiveListings: r.LiveListings,
		})
	}
	return out, nil
}
