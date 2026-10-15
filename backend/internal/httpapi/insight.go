package httpapi

import (
	"bytes"
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5"

	"bugun/internal/insight"
	"bugun/internal/market"
)

func (s *Server) registerInsight(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "adminForecastModel", Method: http.MethodGet, Path: "/v1/admin/forecast/model",
		Summary: "The boosted-tree forecast model: last training, backtest error and what drives it (PROGNOZ_ML.md)", Tags: []string{"admin"}, Security: authRequired,
	}, s.adminForecastModel)
	huma.Register(api, huma.Operation{
		OperationID: "productAnalytics", Method: http.MethodGet, Path: "/v1/products/{slug}/analytics",
		Summary: "Product analytics: history, changes, records, trend and data quality (TAHLIL_REJA.md)", Tags: []string{"analytics"},
	}, s.productAnalytics)
	huma.Register(api, huma.Operation{
		OperationID: "regionOverview", Method: http.MethodGet, Path: "/v1/regions/{id}/overview",
		Summary: "One viloyat's market: passport, products against the country, food basket index, shortages, opportunities, activity", Tags: []string{"analytics"},
	}, s.regionOverview)
	huma.Register(api, huma.Operation{
		OperationID: "priceHeatmap", Method: http.MethodGet, Path: "/v1/analytics/heatmap",
		Summary: "Every product against every viloyat (last 14 days) and the week's biggest movers", Tags: []string{"analytics"},
	}, s.priceHeatmap)
	huma.Register(api, huma.Operation{
		OperationID: "provinceClusters", Method: http.MethodGet, Path: "/v1/analytics/clusters",
		Summary: "Viloyatlar grouped by their price profile (K-Means, k by silhouette), or a note that there are no clear groups", Tags: []string{"analytics"},
	}, s.provinceClusters)
	huma.Register(api, huma.Operation{
		OperationID: "topDemand", Method: http.MethodGet, Path: "/v1/analytics/top-demand",
		Summary: "Products people look for most this week, and those whose demand is rising", Tags: []string{"analytics"},
	}, s.topDemand)
	huma.Register(api, huma.Operation{
		OperationID: "compareProducts", Method: http.MethodGet, Path: "/v1/analytics/compare",
		Summary: "Up to 4 price lines on one chart: products in one place or one product in several viloyatlar", Tags: []string{"analytics"},
	}, s.compare)
	huma.Register(api, huma.Operation{
		OperationID: "productAnalyticsCSV", Method: http.MethodGet, Path: "/v1/products/{slug}/analytics.csv",
		Summary: "The analytics price series as CSV (opens in Excel)", Tags: []string{"analytics"},
	}, s.analyticsCSV)
}

type CompareInput struct {
	Products  string `query:"products" required:"true" maxLength:"300" doc:"Comma-separated product slugs"`
	Regions   string `query:"regions" maxLength:"100" doc:"Comma-separated viloyat or tuman ids; empty for the whole country"`
	TradeType string `query:"trade" enum:"ulgurji,chakana" default:"ulgurji"`
	Period    string `query:"period" enum:"30d,90d,1y,3y" default:"1y"`
}

type CompareOutput struct {
	Body insight.Comparison
}

func (s *Server) compare(ctx context.Context, in *CompareInput) (*CompareOutput, error) {
	var ids []int16
	for _, slug := range splitList(in.Products) {
		p, err := s.q.GetProductBySlug(ctx, slug)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apiError(http.StatusUnprocessableEntity, "unknown_product", "Bunday mahsulot katalogda yo'q.")
		} else if err != nil {
			return nil, internalError(ctx, "get product", err)
		}
		ids = append(ids, p.ID)
	}
	provinces := []int16{0}
	if list := splitList(in.Regions); len(list) > 0 {
		provinces = provinces[:0]
		for _, r := range list {
			id, err := strconv.ParseInt(r, 10, 16)
			if err != nil {
				return nil, apiError(http.StatusUnprocessableEntity, "unknown_region", "Bunday hudud topilmadi.")
			}
			p, err := market.ProvinceOf(ctx, s.q, int16(id))
			if errors.Is(err, pgx.ErrNoRows) {
				return nil, apiError(http.StatusUnprocessableEntity, "unknown_region", "Bunday hudud topilmadi.")
			} else if err != nil {
				return nil, internalError(ctx, "province", err)
			}
			provinces = append(provinces, p)
		}
	}
	if len(ids) == 0 || len(ids)*len(provinces) > insight.MaxLines {
		return nil, apiError(http.StatusUnprocessableEntity, "too_many_lines", "Bir grafikda 1 dan 4 tagacha chiziq bo'lishi mumkin.")
	}
	var specs []insight.LineSpec
	for _, id := range ids {
		for _, p := range provinces {
			specs = append(specs, insight.LineSpec{ProductID: id, Province: p})
		}
	}
	c, err := s.insight.Compare(ctx, specs, in.TradeType, in.Period)
	if err != nil {
		return nil, internalError(ctx, "compare", err)
	}
	return &CompareOutput{Body: *c}, nil
}

func splitList(s string) []string {
	var out []string
	for _, x := range strings.Split(s, ",") {
		if x = strings.TrimSpace(x); x != "" {
			out = append(out, x)
		}
	}
	return out
}

// analyticsCSV writes the series of the analytics page as CSV, with a BOM so
// Excel reads the UTF-8 names.
func (s *Server) analyticsCSV(ctx context.Context, in *AnalyticsInput) (*ReportOutput, error) {
	out, err := s.productAnalytics(ctx, in)
	if err != nil {
		return nil, err
	}
	a := out.Body
	var buf bytes.Buffer
	buf.WriteString("\ufeff")
	w := csv.NewWriter(&buf)
	where := a.Scope.ProvinceName
	if where == "" {
		where = "O'zbekiston"
	}
	_ = w.Write([]string{"mahsulot", "hudud", "savdo_turi", "sana", "median", "p25", "p75", "eng_past", "eng_yuqori", "narxlar_soni", "demo_ulushi"})
	for _, p := range a.Series {
		_ = w.Write([]string{
			a.Product.Name, where, a.Scope.Trade, p.Date,
			strconv.FormatInt(p.Median, 10), strconv.FormatInt(p.P25, 10), strconv.FormatInt(p.P75, 10),
			strconv.FormatInt(p.Min, 10), strconv.FormatInt(p.Max, 10), strconv.Itoa(int(p.Offers)),
			strconv.FormatFloat(p.DemoShare, 'f', -1, 64),
		})
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return nil, internalError(ctx, "csv", err)
	}
	name := fmt.Sprintf("bugun-%s-%d-%s.csv", a.Product.Slug, a.Scope.ProvinceID, a.Scope.Period)
	return &ReportOutput{
		ContentType: "text/csv; charset=utf-8", ContentDisposition: `attachment; filename="` + name + `"`,
		CacheControl: "private, max-age=600", Body: buf.Bytes(),
	}, nil
}

type TopDemandInput struct {
	Region int16 `query:"region" minimum:"0" doc:"Viloyat or tuman (its viloyat is used); 0 for the whole country"`
}

type TopDemandOutput struct {
	Body insight.TopDemand
}

func (s *Server) topDemand(ctx context.Context, in *TopDemandInput) (*TopDemandOutput, error) {
	var province int16
	if in.Region > 0 {
		p, err := market.ProvinceOf(ctx, s.q, in.Region)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apiError(http.StatusUnprocessableEntity, "unknown_region", "Bunday hudud topilmadi.")
		} else if err != nil {
			return nil, internalError(ctx, "province", err)
		}
		province = p
	}
	t, err := s.insight.TopDemand(ctx, province)
	if err != nil {
		return nil, internalError(ctx, "top demand", err)
	}
	return &TopDemandOutput{Body: *t}, nil
}

type HeatmapInput struct {
	TradeType string `query:"trade" enum:"ulgurji,chakana" default:"ulgurji"`
	Category  string `query:"category" enum:"sabzavot,meva,gosht,don,oziq_ovqat,qurilish,xizmat" doc:"Empty for all"`
}

type HeatmapOutput struct {
	Body insight.Heatmap
}

func (s *Server) priceHeatmap(ctx context.Context, in *HeatmapInput) (*HeatmapOutput, error) {
	h, err := s.insight.Heatmap(ctx, in.TradeType, in.Category)
	if err != nil {
		return nil, internalError(ctx, "heatmap", err)
	}
	return &HeatmapOutput{Body: *h}, nil
}

type ClustersInput struct {
	TradeType string `query:"trade" enum:"ulgurji,chakana" default:"ulgurji"`
}

type ClustersOutput struct {
	Body insight.Clusters
}

func (s *Server) provinceClusters(ctx context.Context, in *ClustersInput) (*ClustersOutput, error) {
	c, err := s.insight.Clusters(ctx, in.TradeType)
	if err != nil {
		return nil, internalError(ctx, "clusters", err)
	}
	return &ClustersOutput{Body: *c}, nil
}

type RegionOverviewInput struct {
	ID        int16  `path:"id" minimum:"1" doc:"Viloyat or tuman (its viloyat is used)"`
	TradeType string `query:"trade" enum:"ulgurji,chakana" default:"ulgurji"`
}

type RegionOverviewOutput struct {
	Body insight.RegionOverview
}

func (s *Server) regionOverview(ctx context.Context, in *RegionOverviewInput) (*RegionOverviewOutput, error) {
	province, err := market.ProvinceOf(ctx, s.q, in.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apiError(http.StatusNotFound, "region_not_found", "Bunday hudud topilmadi.")
	}
	if err != nil {
		return nil, internalError(ctx, "province", err)
	}
	o, err := s.insight.RegionOverview(ctx, province, in.TradeType)
	if err != nil {
		return nil, internalError(ctx, "region overview", err)
	}
	return &RegionOverviewOutput{Body: *o}, nil
}

type AnalyticsInput struct {
	Slug      string  `path:"slug" maxLength:"60"`
	Region    int16   `query:"region" minimum:"0" doc:"Viloyat or tuman (its viloyat is used); 0 for the whole country"`
	TradeType string  `query:"trade" enum:"ulgurji,chakana" default:"ulgurji"`
	Period    string  `query:"period" enum:"30d,90d,1y,3y" default:"1y"`
	Transport float64 `query:"transport" minimum:"0" maximum:"5000" default:"60" doc:"Road cost for arbitrage: so'm per unit per 100 km"`
}

type AnalyticsOutput struct {
	Body struct {
		Product Product `json:"product"`
		insight.Analytics
	}
}

func (s *Server) productAnalytics(ctx context.Context, in *AnalyticsInput) (*AnalyticsOutput, error) {
	p, err := s.q.GetProductBySlug(ctx, in.Slug)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apiError(http.StatusNotFound, "product_not_found", "Mahsulot topilmadi.")
	}
	if err != nil {
		return nil, internalError(ctx, "get product", err)
	}
	var province int16
	if in.Region > 0 {
		if province, err = market.ProvinceOf(ctx, s.q, in.Region); errors.Is(err, pgx.ErrNoRows) {
			return nil, apiError(http.StatusUnprocessableEntity, "unknown_region", "Bunday hudud topilmadi.")
		} else if err != nil {
			return nil, internalError(ctx, "province", err)
		}
	}
	a, err := s.insight.Analytics(ctx, insight.Params{ProductID: p.ID, Province: province, Trade: in.TradeType, Period: in.Period, Transport: in.Transport})
	if err != nil {
		return nil, internalError(ctx, "analytics", err)
	}
	out := &AnalyticsOutput{}
	out.Body.Product = Product{ID: p.ID, Slug: p.Slug, Name: p.Name, Category: p.Category, BaseUnit: p.BaseUnit}
	out.Body.Analytics = *a
	return out, nil
}

type ForecastModelOutput struct {
	Body struct {
		Runs []insight.ModelRun `json:"runs" doc:"The latest train and backtest runs; empty before the first night"`
	}
}

func (s *Server) adminForecastModel(ctx context.Context, in *SessionInput) (*ForecastModelOutput, error) {
	if _, err := s.currentAdmin(ctx, in.Session); err != nil {
		return nil, err
	}
	runs, err := s.insight.ModelReport(ctx)
	if err != nil {
		return nil, internalError(ctx, "forecast model", err)
	}
	out := &ForecastModelOutput{}
	out.Body.Runs = runs
	return out, nil
}
