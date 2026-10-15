package httpapi

import (
	"context"
	"math"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5/pgtype"

	"bugun/internal/store/db"
)

// Daily price anomalies for admins (ML_REJA.md 4): confirm a flag to keep the
// day out of the charts for good, or dismiss it to put the day back.

func (s *Server) registerAnomalies(api huma.API) {
	tags := []string{"admin"}
	huma.Register(api, huma.Operation{
		OperationID: "adminListAnomalies", Method: http.MethodGet, Path: "/v1/admin/anomalies",
		Summary: "Suspicious viloyat-days in the daily price history, newest first", Tags: tags, Security: authRequired,
	}, s.adminListAnomalies)
	huma.Register(api, huma.Operation{
		OperationID: "adminDecideAnomaly", Method: http.MethodPost, Path: "/v1/admin/anomalies/decision",
		Summary: "Confirm a flag (the day stays out of the charts) or dismiss it (the day comes back)", Tags: tags, Security: authRequired,
		DefaultStatus: http.StatusNoContent,
	}, s.adminDecideAnomaly)
}

type AnomaliesInput struct {
	SessionInput
	Status string `query:"status" enum:"open,confirmed,dismissed" doc:"Empty = all"`
}

type PriceAnomaly struct {
	ProductID    int16      `json:"product_id"`
	ProductSlug  string     `json:"product_slug"`
	ProductName  string     `json:"product_name"`
	TradeType    string     `json:"trade_type"`
	ProvinceID   int16      `json:"province_id"`
	ProvinceName string     `json:"province_name"`
	Day          string     `json:"day"`
	Layer        string     `json:"layer" enum:"temporal,cross_section" doc:"temporal: far from the viloyat's own last 4 weeks; cross_section: jumped while the other viloyatlar did not"`
	Value        int64      `json:"value" doc:"The day's median"`
	Reference    int64      `json:"reference" doc:"What was expected"`
	Z            float64    `json:"z" doc:"Robust z-score; beyond ±ANOMALY_Z (3.5 by default) is flagged"`
	Settled      bool       `json:"settled" doc:"Four later days were seen; until then the flag is provisional and the day stays in the charts"`
	Status       string     `json:"status" enum:"open,confirmed,dismissed"`
	FoundAt      time.Time  `json:"found_at"`
	DecidedAt    *time.Time `json:"decided_at,omitempty"`
	DemoShare    float64    `json:"demo_share" doc:"Share of the day's prices that are demo data (demo sellers or generated synthetic history), 0–1; 0 = all live"`
}

type AnomaliesOutput struct {
	Body struct {
		Anomalies []PriceAnomaly `json:"anomalies" doc:"Up to 300"`
	}
}

func (s *Server) adminListAnomalies(ctx context.Context, in *AnomaliesInput) (*AnomaliesOutput, error) {
	if _, err := s.currentAdmin(ctx, in.Session); err != nil {
		return nil, err
	}
	rows, err := s.q.ListAnomalies(ctx, pgtype.Text{String: in.Status, Valid: in.Status != ""})
	if err != nil {
		return nil, internalError(ctx, "list anomalies", err)
	}
	out := &AnomaliesOutput{}
	out.Body.Anomalies = make([]PriceAnomaly, len(rows))
	for i, r := range rows {
		out.Body.Anomalies[i] = PriceAnomaly{
			ProductID: r.ProductID, ProductSlug: r.ProductSlug, ProductName: r.ProductName, TradeType: r.TradeType,
			ProvinceID: r.ProvinceID, ProvinceName: r.ProvinceName, Day: r.Day.Time.Format(time.DateOnly), Layer: r.Layer,
			Value: int64(r.Value + 0.5), Reference: int64(r.Reference + 0.5), Z: r.Z, Settled: r.Settled, Status: r.Status,
			FoundAt: r.FoundAt, DecidedAt: r.DecidedAt, DemoShare: math.Round(r.DemoShare*100) / 100,
		}
	}
	return out, nil
}

type AnomalyDecisionInput struct {
	SessionInput
	Body struct {
		ProductID  int16  `json:"product_id" minimum:"1"`
		TradeType  string `json:"trade_type" enum:"ulgurji,chakana"`
		ProvinceID int16  `json:"province_id" minimum:"1"`
		Day        string `json:"day" format:"date"`
		Layer      string `json:"layer" enum:"temporal,cross_section"`
		Status     string `json:"status" enum:"confirmed,dismissed"`
	}
}

func (s *Server) adminDecideAnomaly(ctx context.Context, in *AnomalyDecisionInput) (*struct{}, error) {
	u, err := s.currentAdmin(ctx, in.Session)
	if err != nil {
		return nil, err
	}
	day, err := time.Parse(time.DateOnly, in.Body.Day)
	if err != nil {
		return nil, apiError(http.StatusUnprocessableEntity, "bad_date", "Sana YYYY-MM-DD koʻrinishida boʻlsin.")
	}
	n, err := s.q.DecideAnomaly(ctx, db.DecideAnomalyParams{
		Status: in.Body.Status, DecidedBy: pgtype.UUID{Bytes: u.ID, Valid: true}, ProductID: in.Body.ProductID,
		TradeType: in.Body.TradeType, ProvinceID: in.Body.ProvinceID, Day: pgtype.Date{Time: day, Valid: true}, Layer: in.Body.Layer,
	})
	if err != nil {
		return nil, internalError(ctx, "decide anomaly", err)
	}
	if n == 0 {
		return nil, apiError(http.StatusNotFound, "anomaly_not_found", "Bunday belgi topilmadi.")
	}
	s.insight.Invalidate()
	return nil, nil
}
