package httpapi

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"bugun/internal/pricing"
)

// How well price texts are understood (MATN_TUSHUNISH.md 1).

func (s *Server) registerParser(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "adminParserStats", Method: http.MethodGet, Path: "/v1/admin/parser",
		Summary: "How well price texts were understood: shares understood, cards confirmed at the first try, unknown words", Tags: []string{"admin"}, Security: authRequired,
	}, s.adminParserStats)
}

type ParserStatsInput struct {
	SessionInput
	Days int `query:"days" minimum:"1" maximum:"90" default:"7"`
}

type ParserStatsOutput struct {
	Body pricing.ParserStats
}

func (s *Server) adminParserStats(ctx context.Context, in *ParserStatsInput) (*ParserStatsOutput, error) {
	if _, err := s.currentAdmin(ctx, in.Session); err != nil {
		return nil, err
	}
	st, err := s.prices.Stats(ctx, in.Days)
	if err != nil {
		return nil, internalError(ctx, "parser stats", err)
	}
	return &ParserStatsOutput{Body: st}, nil
}
