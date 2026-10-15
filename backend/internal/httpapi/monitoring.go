package httpapi

import (
	"context"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
)

// System monitoring for admins (ML_REJA.md 3): open and recently resolved
// alerts, and the last run of every scheduled job.

func (s *Server) registerMonitoring(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "adminMonitoring", Method: http.MethodGet, Path: "/v1/admin/monitoring",
		Summary: "System health: open alerts, alerts resolved in the last 7 days and the last run of each job", Tags: []string{"admin"}, Security: authRequired,
	}, s.adminMonitoring)
}

type Alert struct {
	ID         int64      `json:"id"`
	Kind       string     `json:"kind" doc:"source_silent, source_drop, source_spike, review_share, job_failed, job_stuck, job_overdue, fx_missing, fx_stale, weather_missing, weather_stale"`
	Subject    string     `json:"subject,omitempty" doc:"The price source or job"`
	Level      string     `json:"level" enum:"medium,high,critical"`
	Value      *float64   `json:"value,omitempty" doc:"What was seen: prices in the last 24 hours, a share, or days"`
	Baseline   *float64   `json:"baseline,omitempty" doc:"What is usual"`
	Message    string     `json:"message" doc:"Plain Uzbek sentence"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at" doc:"Last check that still saw it"`
	NotifiedAt *time.Time `json:"notified_at,omitempty" doc:"When admins got it in the bot (high and critical only)"`
	ResolvedAt *time.Time `json:"resolved_at,omitempty"`
}

type JobRun struct {
	Job        string     `json:"job"`
	StartedAt  time.Time  `json:"started_at"`
	FinishedAt *time.Time `json:"finished_at,omitempty" doc:"Absent while it runs"`
	Error      string     `json:"error,omitempty"`
}

type MonitoringOutput struct {
	Body struct {
		Alerts []Alert  `json:"alerts" doc:"Open first, worst first"`
		Jobs   []JobRun `json:"jobs"`
	}
}

func (s *Server) adminMonitoring(ctx context.Context, in *SessionInput) (*MonitoringOutput, error) {
	if _, err := s.currentAdmin(ctx, in.Session); err != nil {
		return nil, err
	}
	alerts, err := s.q.ListAlerts(ctx)
	if err != nil {
		return nil, internalError(ctx, "list alerts", err)
	}
	runs, err := s.q.LatestJobRuns(ctx)
	if err != nil {
		return nil, internalError(ctx, "job runs", err)
	}
	out := &MonitoringOutput{}
	out.Body.Alerts = make([]Alert, len(alerts))
	for i, a := range alerts {
		out.Body.Alerts[i] = Alert{
			ID: a.ID, Kind: a.Kind, Subject: a.Subject, Level: a.Level, Message: a.Message,
			CreatedAt: a.CreatedAt, UpdatedAt: a.UpdatedAt, NotifiedAt: a.NotifiedAt, ResolvedAt: a.ResolvedAt,
		}
		if a.Value.Valid {
			out.Body.Alerts[i].Value = &a.Value.Float64
		}
		if a.Baseline.Valid {
			out.Body.Alerts[i].Baseline = &a.Baseline.Float64
		}
	}
	out.Body.Jobs = make([]JobRun, len(runs))
	for i, r := range runs {
		out.Body.Jobs[i] = JobRun{Job: r.Job, StartedAt: r.StartedAt, FinishedAt: r.FinishedAt, Error: r.Error.String}
	}
	return out, nil
}
