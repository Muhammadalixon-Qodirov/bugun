package httpapi

import (
	"context"
	"errors"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"bugun/internal/pricing"
)

func (s *Server) registerPrices(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "createPriceDraft",
		Method:      http.MethodPost,
		Path:        "/v1/prices/drafts",
		Summary:     "Parse a price message into a draft for confirmation (entrepreneurs with a region, else 403 sellers_only)",
		Tags:        []string{"prices"},
		Security:    authRequired,
	}, s.createDraft)

	huma.Register(api, huma.Operation{
		OperationID: "confirmPriceDraft",
		Method:      http.MethodPost,
		Path:        "/v1/prices/drafts/{id}/confirm",
		Summary:     "Save a draft's prices",
		Tags:        []string{"prices"},
		Security:    authRequired,
	}, s.confirmDraft)

	huma.Register(api, huma.Operation{
		OperationID:   "discardPriceDraft",
		Method:        http.MethodDelete,
		Path:          "/v1/prices/drafts/{id}",
		Summary:       "Discard a draft",
		Tags:          []string{"prices"},
		Security:      authRequired,
		DefaultStatus: http.StatusNoContent,
	}, s.discardDraft)
}

type CreateDraftInput struct {
	SessionInput
	Body struct {
		Text string `json:"text" minLength:"1" maxLength:"2000" doc:"Free text, e.g. \"kartoshka 4500, 2 tonna\""`
	}
}

type DraftOutput struct {
	Body pricing.Draft
}

func (s *Server) createDraft(ctx context.Context, in *CreateDraftInput) (*DraftOutput, error) {
	u, err := s.currentUser(ctx, in.Session)
	if err != nil {
		return nil, err
	}
	d, err := s.prices.CreateDraft(ctx, u, in.Body.Text, pricing.ViaWeb)
	switch {
	case errors.Is(err, pricing.ErrNotSeller):
		return nil, apiError(http.StatusForbidden, "sellers_only", "Narx yuborish uchun profilda «Tadbirkor» rolini tanlang va hududingizni kiriting.")
	case errors.Is(err, pricing.ErrEmptyInput), errors.Is(err, pricing.ErrTooLong):
		return nil, apiError(http.StatusUnprocessableEntity, "price_text_invalid", "Xabar bo'sh yoki juda uzun.")
	case err != nil:
		return nil, internalError(ctx, "create draft", err)
	}
	return &DraftOutput{Body: d}, nil
}

type DraftIDInput struct {
	SessionInput
	ID int64 `path:"id" minimum:"1"`
}

type ConfirmOutput struct {
	Body struct {
		Saved []pricing.Saved `json:"saved"`
	}
}

func (s *Server) confirmDraft(ctx context.Context, in *DraftIDInput) (*ConfirmOutput, error) {
	u, err := s.currentUser(ctx, in.Session)
	if err != nil {
		return nil, err
	}
	saved, err := s.prices.Confirm(ctx, u, in.ID)
	switch {
	case errors.Is(err, pricing.ErrNotSeller):
		return nil, apiError(http.StatusForbidden, "sellers_only", "Narx yuborish uchun profilda «Tadbirkor» rolini tanlang va hududingizni kiriting.")
	case errors.Is(err, pricing.ErrDraftGone):
		return nil, apiError(http.StatusConflict, "draft_stale", "Bu kartochka eskirgan yoki allaqachon saqlangan. Narxni qayta yuboring.")
	case err != nil:
		return nil, internalError(ctx, "confirm draft", err)
	}
	out := &ConfirmOutput{}
	out.Body.Saved = saved
	return out, nil
}

func (s *Server) discardDraft(ctx context.Context, in *DraftIDInput) (*struct{}, error) {
	u, err := s.currentUser(ctx, in.Session)
	if err != nil {
		return nil, err
	}
	if err := s.prices.Discard(ctx, u.ID, in.ID); err != nil {
		return nil, internalError(ctx, "discard draft", err)
	}
	return nil, nil
}
