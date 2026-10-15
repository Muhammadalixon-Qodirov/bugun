package httpapi

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"bugun/internal/insight"
	"bugun/internal/market"
	"bugun/internal/rfq"
	"bugun/internal/watch"
)

// Business limits the site checks before sending a form, so it never keeps
// its own copy that could drift from the API's.

// Admin list sizes; the queries (AdminDeals, ListAnomalies) use the same.
const (
	adminDealsLimit     = 100
	adminAnomaliesLimit = 300
)

func (s *Server) registerMeta(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "metaLimits", Method: http.MethodGet, Path: "/v1/meta/limits",
		Summary:     "Business limits the site enforces in forms (RFQ, automatic pricing, photos, watches, comparisons)",
		Description: "Change only with a server restart; responses may be cached for 5 minutes.",
		Tags:        []string{"system"},
	}, s.metaLimits)
}

type RFQLimits struct {
	MaxOpen     int `json:"max_open" doc:"Open requests one buyer may have at a time"`
	DefaultDays int `json:"default_days" doc:"Answer deadline when the buyer does not choose one"`
	MinDays     int `json:"min_days"`
	MaxDays     int `json:"max_days"`
}

type AutoPriceLimits struct {
	MaxRatio         int     `json:"max_ratio" doc:"price_max may be at most this many times price_min"`
	DailyStep        float64 `json:"daily_step" doc:"Largest total change in one Tashkent day: 0.05 = 5%"`
	ManualPauseHours float64 `json:"manual_pause_hours" doc:"Automatic pricing waits this long after the seller sets a price by hand"`
}

type PhotoLimits struct {
	MaxPerListing int   `json:"max_per_listing"`
	MaxBytes      int64 `json:"max_bytes" doc:"Largest upload in bytes"`
	MaxPixels     int   `json:"max_pixels" doc:"Largest width × height"`
}

type Limits struct {
	RFQ       RFQLimits       `json:"rfq"`
	AutoPrice AutoPriceLimits `json:"auto_price"`
	Photos    PhotoLimits     `json:"photos"`
	// Fixed in code; listed so the site need not repeat them.
	WatchesMax          int `json:"watches_max" doc:"Watches (price alerts) one user may keep"`
	CompareMaxLines     int `json:"compare_max_lines" doc:"Lines on one /v1/analytics/compare chart"`
	AdminDealsLimit     int `json:"admin_deals_limit" doc:"Deals /v1/admin/deals returns at most"`
	AdminAnomaliesLimit int `json:"admin_anomalies_limit" doc:"Flags /v1/admin/anomalies returns at most"`
}

type LimitsOutput struct {
	CacheControl string `header:"Cache-Control"`
	Body         Limits
}

func (s *Server) metaLimits(context.Context, *struct{}) (*LimitsOutput, error) {
	auto := market.DefaultAutoPriceRules
	maxPhotos := market.DefaultMaxPhotos
	if s.listings != nil {
		auto = s.listings.AutoPriceRules()
		maxPhotos = s.listings.MaxPhotos()
	}
	l := rfq.DefaultLimits
	if s.rfq != nil {
		l = s.rfq.Limits()
	}
	m := s.mediaLimits()
	return &LimitsOutput{
		CacheControl: "public, max-age=300",
		Body: Limits{
			RFQ:                 RFQLimits{MaxOpen: l.MaxOpen, DefaultDays: l.DefaultDays, MinDays: l.MinDays, MaxDays: l.MaxDays},
			AutoPrice:           AutoPriceLimits{MaxRatio: market.MaxPriceRatio, DailyStep: auto.DailyStep, ManualPauseHours: auto.ManualPause.Hours()},
			Photos:              PhotoLimits{MaxPerListing: maxPhotos, MaxBytes: m.MaxBytes, MaxPixels: m.MaxPixels},
			WatchesMax:          watch.MaxPerUser,
			CompareMaxLines:     insight.MaxLines,
			AdminDealsLimit:     adminDealsLimit,
			AdminAnomaliesLimit: adminAnomaliesLimit,
		},
	}, nil
}
