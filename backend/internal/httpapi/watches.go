package httpapi

import (
	"context"
	"errors"
	"math"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5"

	"bugun/internal/market"
	"bugun/internal/store/db"
	"bugun/internal/watch"
)

func (s *Server) registerWatches(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "addWatch", Method: http.MethodPost, Path: "/v1/watches",
		Summary: "Follow a product: the bot tells when a cheaper price appears or the median falls", Tags: []string{"watches"}, Security: authRequired,
	}, s.addWatch)
	huma.Register(api, huma.Operation{
		OperationID: "listWatches", Method: http.MethodGet, Path: "/v1/me/watches",
		Summary: "Products I follow", Tags: []string{"watches"}, Security: authRequired,
	}, s.listWatches)
	huma.Register(api, huma.Operation{
		OperationID: "deleteWatch", Method: http.MethodDelete, Path: "/v1/watches/{id}",
		Summary: "Stop following", Tags: []string{"watches"}, Security: authRequired, DefaultStatus: http.StatusNoContent,
	}, s.deleteWatch)
}

type WatchBody struct {
	ProductID int16  `json:"product_id" minimum:"1"`
	RegionID  int16  `json:"region_id,omitempty" minimum:"0" doc:"Viloyat or tuman (its viloyat is used); 0 or omitted for the whole country"`
	TradeType string `json:"trade_type,omitempty" enum:"ulgurji,chakana" default:"ulgurji"`
	MaxPrice  *int64 `json:"max_price,omitempty" minimum:"50" maximum:"50000000" doc:"Tell me about prices at or under this; omitted = 5% under the median"`
}

type Watch struct {
	ID           int64      `json:"id"`
	ProductID    int16      `json:"product_id"`
	ProductName  string     `json:"product_name"`
	ProductSlug  string     `json:"product_slug"`
	Unit         string     `json:"unit"`
	ProvinceID   int16      `json:"province_id,omitempty" doc:"Omitted for the whole country"`
	ProvinceName string     `json:"province_name,omitempty"`
	TradeType    string     `json:"trade_type"`
	MaxPrice     *int64     `json:"max_price,omitempty"`
	PeakMedian   *int64     `json:"peak_median,omitempty" doc:"The recent high a fall is measured from"`
	NotifiedAt   *time.Time `json:"notified_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
}

type AddWatchInput struct {
	SessionInput
	Body WatchBody
}

type WatchesOutput struct {
	Body struct {
		Watches []Watch `json:"watches"`
	}
}

func (s *Server) addWatch(ctx context.Context, in *AddWatchInput) (*WatchesOutput, error) {
	u, err := s.currentUser(ctx, in.Session)
	if err != nil {
		return nil, err
	}
	if _, err := s.q.GetProduct(ctx, in.Body.ProductID); errors.Is(err, pgx.ErrNoRows) {
		return nil, apiError(http.StatusUnprocessableEntity, "unknown_product", "Bunday mahsulot katalogda yo'q.")
	} else if err != nil {
		return nil, internalError(ctx, "get product", err)
	}
	var province int16
	if in.Body.RegionID > 0 {
		if province, err = market.ProvinceOf(ctx, s.q, in.Body.RegionID); errors.Is(err, pgx.ErrNoRows) {
			return nil, apiError(http.StatusUnprocessableEntity, "unknown_region", "Bunday hudud topilmadi.")
		} else if err != nil {
			return nil, internalError(ctx, "province", err)
		}
	}
	trade := in.Body.TradeType
	if trade == "" {
		trade = "ulgurji"
	}
	if _, err := s.watches.Add(ctx, u, in.Body.ProductID, province, trade, in.Body.MaxPrice); errors.Is(err, watch.ErrTooMany) {
		return nil, apiError(http.StatusConflict, "too_many_watches", "Ko'pi bilan 20 ta mahsulotni kuzatish mumkin.")
	} else if err != nil {
		return nil, internalError(ctx, "add watch", err)
	}
	s.recordDemand(ctx, "watch", in.Body.ProductID, province, "u:"+u.ID.String())
	return s.watchesOutput(ctx, u)
}

func (s *Server) listWatches(ctx context.Context, in *SessionInput) (*WatchesOutput, error) {
	u, err := s.currentUser(ctx, in.Session)
	if err != nil {
		return nil, err
	}
	return s.watchesOutput(ctx, u)
}

type WatchIDInput struct {
	SessionInput
	ID int64 `path:"id" minimum:"1"`
}

func (s *Server) deleteWatch(ctx context.Context, in *WatchIDInput) (*struct{}, error) {
	u, err := s.currentUser(ctx, in.Session)
	if err != nil {
		return nil, err
	}
	if err := s.watches.Remove(ctx, u, in.ID); errors.Is(err, watch.ErrNotFound) {
		return nil, apiError(http.StatusNotFound, "watch_not_found", "Kuzatuv topilmadi.")
	} else if err != nil {
		return nil, internalError(ctx, "delete watch", err)
	}
	return nil, nil
}

func (s *Server) watchesOutput(ctx context.Context, u db.User) (*WatchesOutput, error) {
	rows, err := s.watches.List(ctx, u)
	if err != nil {
		return nil, internalError(ctx, "list watches", err)
	}
	out := &WatchesOutput{}
	out.Body.Watches = make([]Watch, len(rows))
	for i, r := range rows {
		w := Watch{
			ID: r.ID, ProductID: r.ProductID, ProductName: r.ProductName, ProductSlug: r.ProductSlug, Unit: r.Unit,
			ProvinceID: r.ProvinceID.Int16, ProvinceName: r.ProvinceName, TradeType: r.TradeType,
			NotifiedAt: r.NotifiedAt, CreatedAt: r.CreatedAt,
		}
		if r.MaxPrice.Valid {
			w.MaxPrice = &r.MaxPrice.Int64
		}
		if r.PeakMedian.Valid {
			m := int64(math.Round(r.PeakMedian.Float64))
			w.PeakMedian = &m
		}
		out.Body.Watches[i] = w
	}
	return out, nil
}
