package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"bugun/internal/market"
	"bugun/internal/store/db"
)

// Market events log (TAHLIL_REJA.md #30): admins record what moved prices;
// the analytics page links events to price jumps.

func (s *Server) registerEvents(api huma.API) {
	tags := []string{"admin"}
	huma.Register(api, huma.Operation{
		OperationID: "adminListEvents", Method: http.MethodGet, Path: "/v1/admin/events",
		Summary: "Market events, newest first", Tags: tags, Security: authRequired,
	}, s.adminListEvents)
	huma.Register(api, huma.Operation{
		OperationID: "adminCreateEvent", Method: http.MethodPost, Path: "/v1/admin/events",
		Summary: "Record a market event (frost, export rule, holiday...)", Tags: tags, Security: authRequired,
	}, s.adminCreateEvent)
	huma.Register(api, huma.Operation{
		OperationID: "adminUpdateEvent", Method: http.MethodPut, Path: "/v1/admin/events/{id}",
		Summary: "Change a market event", Tags: tags, Security: authRequired,
	}, s.adminUpdateEvent)
	huma.Register(api, huma.Operation{
		OperationID: "adminDeleteEvent", Method: http.MethodDelete, Path: "/v1/admin/events/{id}",
		Summary: "Delete a market event", Tags: tags, Security: authRequired, DefaultStatus: http.StatusNoContent,
	}, s.adminDeleteEvent)
}

type EventBody struct {
	Title     string `json:"title" minLength:"3" maxLength:"200"`
	Kind      string `json:"kind" enum:"frost,heat,rain,export,import,fuel,currency,holiday,harvest,roads,policy,other"`
	StartsOn  string `json:"starts_on" format:"date" doc:"YYYY-MM-DD"`
	EndsOn    string `json:"ends_on,omitempty" format:"date" doc:"YYYY-MM-DD; empty for a one-day event"`
	RegionID  int16  `json:"region_id,omitempty" minimum:"0" doc:"Viloyat or tuman (its viloyat is kept); 0 = whole country"`
	ProductID int16  `json:"product_id,omitempty" minimum:"0" doc:"0 = not one product"`
	Category  string `json:"category,omitempty" enum:"sabzavot,meva,gosht,don,oziq_ovqat,qurilish,xizmat" doc:"Empty = every category"`
	SourceURL string `json:"source_url,omitempty" maxLength:"500" doc:"http(s) link to the news or decision"`
	Note      string `json:"note,omitempty" maxLength:"1000"`
}

type MarketEvent struct {
	ID          int64     `json:"id"`
	Title       string    `json:"title"`
	Kind        string    `json:"kind"`
	StartsOn    string    `json:"starts_on"`
	EndsOn      string    `json:"ends_on,omitempty"`
	ProvinceID  int16     `json:"province_id,omitempty"`
	Province    string    `json:"province,omitempty"`
	ProductID   int16     `json:"product_id,omitempty"`
	ProductName string    `json:"product_name,omitempty"`
	Category    string    `json:"category,omitempty"`
	SourceURL   string    `json:"source_url,omitempty"`
	Note        string    `json:"note,omitempty"`
	Demo        bool      `json:"demo"`
	CreatedAt   time.Time `json:"created_at"`
}

type EventsOutput struct {
	Body struct {
		Events []MarketEvent `json:"events"`
	}
}

type EventOutput struct {
	Body MarketEvent
}

func (s *Server) adminListEvents(ctx context.Context, in *SessionInput) (*EventsOutput, error) {
	if _, err := s.currentAdmin(ctx, in.Session); err != nil {
		return nil, err
	}
	rows, err := s.q.ListMarketEvents(ctx)
	if err != nil {
		return nil, internalError(ctx, "list events", err)
	}
	out := &EventsOutput{}
	out.Body.Events = make([]MarketEvent, len(rows))
	for i, r := range rows {
		e := eventOut(db.MarketEvent{
			ID: r.ID, Title: r.Title, Kind: r.Kind, StartsOn: r.StartsOn, EndsOn: r.EndsOn, ProvinceID: r.ProvinceID,
			ProductID: r.ProductID, Category: r.Category, SourceUrl: r.SourceUrl, Note: r.Note, IsDemo: r.IsDemo, CreatedAt: r.CreatedAt,
		})
		e.Province, e.ProductName = r.ProvinceName, r.ProductName
		out.Body.Events[i] = e
	}
	return out, nil
}

func eventOut(e db.MarketEvent) MarketEvent {
	m := MarketEvent{
		ID: e.ID, Title: e.Title, Kind: e.Kind, StartsOn: e.StartsOn.Time.Format(time.DateOnly),
		ProvinceID: e.ProvinceID.Int16, ProductID: e.ProductID.Int16, Category: e.Category.String,
		SourceURL: e.SourceUrl.String, Note: e.Note, Demo: e.IsDemo, CreatedAt: e.CreatedAt,
	}
	if e.EndsOn.Valid {
		m.EndsOn = e.EndsOn.Time.Format(time.DateOnly)
	}
	return m
}

// eventFields checks the body and turns it into column values.
type eventFields struct {
	starts, ends      pgtype.Date
	province, product pgtype.Int2
	category, source  pgtype.Text
}

func (s *Server) eventFields(ctx context.Context, b EventBody) (eventFields, error) {
	var f eventFields
	var bad []*APIFieldError
	start, err := time.Parse(time.DateOnly, b.StartsOn)
	if err != nil {
		bad = append(bad, fieldError("starts_on", "invalid_date", "Sanani YYYY-MM-DD shaklida yozing.", nil))
	}
	f.starts = pgtype.Date{Time: start, Valid: err == nil}
	if b.EndsOn != "" {
		end, err := time.Parse(time.DateOnly, b.EndsOn)
		switch {
		case err != nil:
			bad = append(bad, fieldError("ends_on", "invalid_date", "Sanani YYYY-MM-DD shaklida yozing.", nil))
		case end.Before(start):
			bad = append(bad, fieldError("ends_on", "before_start", "Tugash sanasi boshlanishidan oldin boʻlmasin.", nil))
		}
		f.ends = pgtype.Date{Time: end, Valid: err == nil}
	}
	if b.SourceURL != "" {
		if !strings.HasPrefix(b.SourceURL, "http://") && !strings.HasPrefix(b.SourceURL, "https://") {
			bad = append(bad, fieldError("source_url", "invalid_url", "Havola http:// yoki https:// bilan boshlansin.", nil))
		}
		f.source = pgtype.Text{String: b.SourceURL, Valid: true}
	}
	if b.RegionID > 0 {
		p, err := market.ProvinceOf(ctx, s.q, b.RegionID)
		if errors.Is(err, pgx.ErrNoRows) {
			bad = append(bad, fieldError("region_id", "unknown_region", "Bunday hudud topilmadi.", nil))
		} else if err != nil {
			return f, internalError(ctx, "province", err)
		}
		f.province = pgtype.Int2{Int16: p, Valid: err == nil}
	}
	if b.ProductID > 0 {
		if _, err := s.q.GetProduct(ctx, b.ProductID); errors.Is(err, pgx.ErrNoRows) {
			bad = append(bad, fieldError("product_id", "unknown_product", "Bunday mahsulot katalogda yoʻq.", nil))
		} else if err != nil {
			return f, internalError(ctx, "get product", err)
		}
		f.product = pgtype.Int2{Int16: b.ProductID, Valid: true}
	}
	f.category = pgtype.Text{String: b.Category, Valid: b.Category != ""}
	if len(bad) > 0 {
		return f, invalidInput(bad...)
	}
	return f, nil
}

type CreateEventInput struct {
	SessionInput
	Body EventBody
}

func (s *Server) adminCreateEvent(ctx context.Context, in *CreateEventInput) (*EventOutput, error) {
	u, err := s.currentAdmin(ctx, in.Session)
	if err != nil {
		return nil, err
	}
	f, err := s.eventFields(ctx, in.Body)
	if err != nil {
		return nil, err
	}
	e, err := s.q.CreateMarketEvent(ctx, db.CreateMarketEventParams{
		Title: strings.TrimSpace(in.Body.Title), Kind: in.Body.Kind, StartsOn: f.starts, EndsOn: f.ends,
		ProvinceID: f.province, ProductID: f.product, Category: f.category, SourceUrl: f.source,
		Note: strings.TrimSpace(in.Body.Note), CreatedBy: pgtype.UUID{Bytes: u.ID, Valid: true},
	})
	if err != nil {
		return nil, internalError(ctx, "create event", err)
	}
	return &EventOutput{Body: eventOut(e)}, nil
}

type UpdateEventInput struct {
	SessionInput
	ID   int64 `path:"id" minimum:"1"`
	Body EventBody
}

func (s *Server) adminUpdateEvent(ctx context.Context, in *UpdateEventInput) (*EventOutput, error) {
	if _, err := s.currentAdmin(ctx, in.Session); err != nil {
		return nil, err
	}
	f, err := s.eventFields(ctx, in.Body)
	if err != nil {
		return nil, err
	}
	e, err := s.q.UpdateMarketEvent(ctx, db.UpdateMarketEventParams{
		ID: in.ID, Title: strings.TrimSpace(in.Body.Title), Kind: in.Body.Kind, StartsOn: f.starts, EndsOn: f.ends,
		ProvinceID: f.province, ProductID: f.product, Category: f.category, SourceUrl: f.source, Note: strings.TrimSpace(in.Body.Note),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apiError(http.StatusNotFound, "event_not_found", "Voqea topilmadi.")
	}
	if err != nil {
		return nil, internalError(ctx, "update event", err)
	}
	return &EventOutput{Body: eventOut(e)}, nil
}

type EventIDInput struct {
	SessionInput
	ID int64 `path:"id" minimum:"1"`
}

func (s *Server) adminDeleteEvent(ctx context.Context, in *EventIDInput) (*struct{}, error) {
	if _, err := s.currentAdmin(ctx, in.Session); err != nil {
		return nil, err
	}
	n, err := s.q.DeleteMarketEvent(ctx, in.ID)
	if err != nil {
		return nil, internalError(ctx, "delete event", err)
	}
	if n == 0 {
		return nil, apiError(http.StatusNotFound, "event_not_found", "Voqea topilmadi.")
	}
	return nil, nil
}
