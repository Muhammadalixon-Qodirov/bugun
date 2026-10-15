package httpapi

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5"

	"bugun/internal/market"
	"bugun/internal/report"
	"bugun/internal/roles"
	"bugun/internal/store/db"
	"bugun/internal/tz"
)

func (s *Server) registerReport(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "weeklyReport", Method: http.MethodGet, Path: "/v1/reports/weekly",
		Summary: "Weekly viloyat price report as PDF (W-10): approved market staff (own viloyat) and admins",
		Tags:    []string{"reports"}, Security: authRequired,
	}, s.weeklyReport)
}

type ReportInput struct {
	SessionInput
	Region int16 `query:"region" minimum:"0" doc:"Viloyat id (a tuman means its viloyat); 0 = the user's own viloyat"`
}

type ReportOutput struct {
	ContentType        string `header:"Content-Type"`
	ContentDisposition string `header:"Content-Disposition"`
	CacheControl       string `header:"Cache-Control"`
	Body               []byte
}

// canReport: admins for any viloyat, approved market staff for their own.
func (s *Server) canReport(u db.User, province, own int16) bool {
	if s.isAdmin(u) {
		return true
	}
	return u.Role.Valid && u.Role.String == roles.MarketAdmin && u.RoleApproved && province == own
}

func (s *Server) weeklyReport(ctx context.Context, in *ReportInput) (*ReportOutput, error) {
	u, err := s.currentUser(ctx, in.Session)
	if err != nil {
		return nil, err
	}
	var own int16
	if u.RegionID.Valid {
		if own, err = market.ProvinceOf(ctx, s.q, u.RegionID.Int16); err != nil {
			return nil, internalError(ctx, "own province", err)
		}
	}
	province := own
	if in.Region > 0 {
		province, err = market.ProvinceOf(ctx, s.q, in.Region)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, huma.Error422UnprocessableEntity("Bunday hudud topilmadi.")
		}
		if err != nil {
			return nil, internalError(ctx, "province", err)
		}
	}
	if province == 0 {
		return nil, apiError(http.StatusUnprocessableEntity, "region_required", "Hisobot uchun viloyatni tanlang.")
	}
	if !s.canReport(u, province, own) {
		return nil, apiError(http.StatusForbidden, "reports_only", "Hisobotlar tasdiqlangan bozor ma'muriyati xodimlari uchun.")
	}
	pv, err := s.q.GetRegion(ctx, province)
	if err != nil {
		return nil, internalError(ctx, "get region", err)
	}
	sum, err := s.q.ReportSummary(ctx, province)
	if err != nil {
		return nil, internalError(ctx, "report summary", err)
	}
	rows, err := s.q.ReportProducts(ctx, province)
	if err != nil {
		return nil, internalError(ctx, "report products", err)
	}
	rep := report.Report{
		Province: pv.Name, GeneratedAt: time.Now(),
		Summary: report.Summary{Sellers: int(sum.Sellers), Listings: int(sum.Listings), RFQs: int(sum.Rfqs), Demand: int(sum.Demand), Deals: int(sum.Deals)},
	}
	dealRows, err := s.q.ReportDeals(ctx, province)
	if err != nil {
		return nil, internalError(ctx, "report deals", err)
	}
	for _, d := range dealRows {
		dr := report.DealRow{Name: d.Name, Unit: d.Unit, Deals: int(d.Deals), Quantity: d.Quantity}
		if d.Priced >= 5 {
			dr.Median = d.Median
		}
		rep.Deals = append(rep.Deals, dr)
	}
	for _, r := range rows {
		pr, err := s.pressureFor(ctx, r.ID, province, "ulgurji")
		if err != nil {
			return nil, internalError(ctx, "report pressure", err)
		}
		rep.Rows = append(rep.Rows, report.Row{
			Name: r.Name, Unit: r.BaseUnit, Offers: int(r.Offers), Median: r.Median, PrevMedian: r.PrevMedian,
			Min: r.MinPrice, Max: r.MaxPrice, Zone: pr.Zone,
		})
	}
	pdf, err := report.Render(rep, tz.Tashkent)
	if err != nil {
		return nil, internalError(ctx, "render report", err)
	}
	name := fmt.Sprintf("bugun-hisobot-%s-%s.pdf", pv.Slug, time.Now().In(tz.Tashkent).Format(time.DateOnly))
	return &ReportOutput{
		ContentType: "application/pdf", ContentDisposition: `attachment; filename="` + name + `"`,
		CacheControl: "private, no-store", Body: pdf,
	}, nil
}
