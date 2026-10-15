package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5/pgtype"

	"bugun/internal/market"
	"bugun/internal/media"
	"bugun/internal/pricing"
	"bugun/internal/store/db"
)

func (s *Server) registerListings(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "createListing", Method: http.MethodPost, Path: "/v1/listings",
		Summary: "Create a listing (or update the seller's existing one for this product); entrepreneurs with a region, else 403 sellers_only",
		Tags:    []string{"listings"}, Security: authRequired,
	}, s.createListing)

	huma.Register(api, huma.Operation{
		OperationID: "listMyListings", Method: http.MethodGet, Path: "/v1/me/listings",
		Summary: "My listings", Tags: []string{"listings"}, Security: authRequired,
	}, s.myListings)

	huma.Register(api, huma.Operation{
		OperationID: "getMyListing", Method: http.MethodGet, Path: "/v1/me/listings/{id}",
		Summary: "One of my listings with photos", Tags: []string{"listings"}, Security: authRequired,
	}, s.myListing)

	huma.Register(api, huma.Operation{
		OperationID: "updateListing", Method: http.MethodPut, Path: "/v1/listings/{id}",
		Summary: "Edit a listing", Tags: []string{"listings"}, Security: authRequired,
	}, s.updateListing)

	huma.Register(api, huma.Operation{
		OperationID: "refreshListing", Method: http.MethodPost, Path: "/v1/listings/{id}/refresh",
		Summary: "Keep the price for another week", Tags: []string{"listings"}, Security: authRequired,
	}, s.refreshListing)

	huma.Register(api, huma.Operation{
		OperationID: "refreshDueListings", Method: http.MethodPost, Path: "/v1/me/listings/refresh",
		Summary: "Keep every listing that has expired or expires within two days for another week",
		Tags:    []string{"listings"}, Security: authRequired,
	}, s.refreshDueListings)

	huma.Register(api, huma.Operation{
		OperationID: "setListingPricing", Method: http.MethodPut, Path: "/v1/listings/{id}/pricing",
		Summary: "Turn automatic pricing within a min–max range on or off", Tags: []string{"listings"}, Security: authRequired,
	}, s.setListingPricing)

	huma.Register(api, huma.Operation{
		OperationID: "markListingSold", Method: http.MethodPost, Path: "/v1/listings/{id}/sold",
		Summary: "Record a sale from this listing and lower its stock (hides it when nothing is left)", Tags: []string{"listings"}, Security: authRequired,
	}, s.markListingSold)
	huma.Register(api, huma.Operation{
		OperationID: "listPriceChanges", Method: http.MethodGet, Path: "/v1/me/listings/{id}/price-changes",
		Summary: "Automatic price changes of a listing, newest first, with their reasons", Tags: []string{"listings"}, Security: authRequired,
	}, s.listPriceChanges)
	huma.Register(api, huma.Operation{
		OperationID: "revertPriceChange", Method: http.MethodPost, Path: "/v1/me/price-changes/{id}/revert",
		Summary: "Undo an automatic price change; auto pricing then waits 24 hours", Tags: []string{"listings"}, Security: authRequired,
	}, s.revertPriceChange)

	huma.Register(api, huma.Operation{
		OperationID: "setListingVisibility", Method: http.MethodPut, Path: "/v1/listings/{id}/visibility",
		Summary: "Hide or show a listing", Tags: []string{"listings"}, Security: authRequired,
		DefaultStatus: http.StatusNoContent,
	}, s.setListingVisibility)

	huma.Register(api, huma.Operation{
		OperationID: "deleteListing", Method: http.MethodDelete, Path: "/v1/listings/{id}",
		Summary: "Delete a listing", Tags: []string{"listings"}, Security: authRequired,
		DefaultStatus: http.StatusNoContent,
	}, s.deleteListing)

	huma.Register(api, huma.Operation{
		OperationID: "uploadListingPhoto", Method: http.MethodPost, Path: "/v1/listings/{id}/photos",
		Summary: "Upload a photo (JPEG, PNG or WebP, up to 10 MB by default; see /v1/meta/limits)", Tags: []string{"listings"}, Security: authRequired,
		MaxBodyBytes: s.mediaLimits().MaxBytes + 64<<10,
	}, s.uploadPhoto)

	huma.Register(api, huma.Operation{
		OperationID: "deleteListingPhoto", Method: http.MethodDelete, Path: "/v1/listings/{id}/photos/{photo_id}",
		Summary: "Remove a photo", Tags: []string{"listings"}, Security: authRequired,
		DefaultStatus: http.StatusNoContent,
	}, s.deletePhoto)
}

type Photo struct {
	ID     int64  `json:"id"`
	Key    string `json:"key" doc:"Served at /media/{key}"`
	Width  int32  `json:"width"`
	Height int32  `json:"height"`
}

type MyListing struct {
	ID          int64     `json:"id"`
	ProductID   int16     `json:"product_id"`
	ProductName string    `json:"product_name"`
	ProductSlug string    `json:"product_slug"`
	Category    string    `json:"category"`
	Variety     string    `json:"variety"`
	Description string    `json:"description"`
	PriceSom    int64     `json:"price_som"`
	Unit        string    `json:"unit"`
	MinOrder    *float64  `json:"min_order,omitempty"`
	Stock       *float64  `json:"stock,omitempty"`
	TradeType   string    `json:"trade_type" enum:"ulgurji,chakana"`
	Status      string    `json:"status" enum:"active,review,hidden,rejected"`
	Expired     bool      `json:"expired" doc:"Past valid_until: hidden from buyers until refreshed"`
	ValidUntil  time.Time `json:"valid_until"`
	Views       int32     `json:"views"`
	UpdatedAt   time.Time `json:"updated_at"`
	CoverKey    string    `json:"cover_key,omitempty"`
	Photos      []Photo   `json:"photos"`
	Median      int64     `json:"median,omitempty" doc:"Market median for this product and trade type (0 when fewer than 3 prices)"`
	MedianScope string    `json:"median_scope,omitempty" enum:"province,country" doc:"Where the median comes from: the listing's viloyat, or nationwide when it has too few prices"`
	DiffPercent *int      `json:"diff_percent,omitempty" doc:"Price vs. the median, in percent"`
	// Position ranks a live listing among other sellers in its viloyat (spec §10.4).
	Position     *pricing.MarketPosition `json:"position,omitempty" doc:"Place among other sellers in the viloyat; omitted when not live or fewer than 3 other prices"`
	ReviewReason string                  `json:"review_reason,omitempty" enum:"jump_from_previous,far_from_market,far_from_reference,content_spam,content_abuse,content_prohibited,content_off_category" doc:"Why the automatic check held it, while status is review; content_* come from the listing text check"`
	ReviewNote   string                  `json:"review_note,omitempty" doc:"The admin's note to the seller, while status is rejected"`
	Pricing      ListingPricing          `json:"pricing" doc:"Automatic pricing settings (DINAMIK_NARX.md)"`
	Funnel7d     *Funnel                 `json:"funnel_7d,omitempty" doc:"Last 7 days: unique viewers, contacts, sales; only in the list"`
}

// ListingPricing is how the system may move a listing's price: in auto mode,
// within price_min–price_max and at most 5% a day.
type ListingPricing struct {
	Mode     string `json:"mode" enum:"manual,auto" doc:"auto lets the system move the price within the range"`
	Goal     string `json:"goal" enum:"fast,balanced,profit" doc:"fast sells sooner (a bit cheaper), profit holds a bit higher"`
	PriceMin *int64 `json:"price_min,omitempty" minimum:"50" maximum:"50000000" doc:"Never below this; required in auto mode"`
	PriceMax *int64 `json:"price_max,omitempty" minimum:"50" maximum:"50000000" doc:"Never above this; at most 3 × price_min"`
	// PausedUntil is set only in responses.
	PausedUntil *time.Time `json:"paused_until,omitempty" readOnly:"true" doc:"Auto mode waits until then because the price was set by hand (24 hours by default; see /v1/meta/limits)"`
}

func (s *Server) listingPricing(mode, goal string, lo, hi pgtype.Int8, manualAt *time.Time) ListingPricing {
	p := ListingPricing{Mode: mode, Goal: goal}
	if manualAt != nil && mode == market.PricingAuto {
		if until := manualAt.Add(s.listings.ManualPause()); until.After(time.Now()) {
			p.PausedUntil = &until
		}
	}
	if lo.Valid {
		p.PriceMin = &lo.Int64
	}
	if hi.Valid {
		p.PriceMax = &hi.Int64
	}
	return p
}

type ListingBody struct {
	Variety     string   `json:"variety,omitempty" maxLength:"60" doc:"Nav, e.g. \"qizil\", \"Gala\""`
	Description string   `json:"description,omitempty" maxLength:"1000"`
	PriceSom    int64    `json:"price_som" minimum:"50" maximum:"50000000" doc:"Price per product unit"`
	MinOrder    *float64 `json:"min_order,omitempty" exclusiveMinimum:"0"`
	Stock       *float64 `json:"stock,omitempty" exclusiveMinimum:"0"`
	TradeType   string   `json:"trade_type" enum:"ulgurji,chakana"`
}

func (b ListingBody) input(productID int16) market.Input {
	return market.Input{
		ProductID: productID, Variety: b.Variety, Description: b.Description, PriceSom: b.PriceSom,
		MinOrder: b.MinOrder, Stock: b.Stock, TradeType: b.TradeType,
	}
}

type CreateListingInput struct {
	SessionInput
	Body struct {
		ProductID int16 `json:"product_id" minimum:"1"`
		ListingBody
	}
}

type ListingOutput struct {
	Body struct {
		Listing MyListing `json:"listing"`
		Created bool      `json:"created" doc:"false when an existing listing was updated"`
	}
}

func (s *Server) createListing(ctx context.Context, in *CreateListingInput) (*ListingOutput, error) {
	u, err := s.currentUser(ctx, in.Session)
	if err != nil {
		return nil, err
	}
	saved, err := s.listings.Upsert(ctx, u, in.Body.input(in.Body.ProductID))
	if err != nil {
		return nil, listingError(ctx, err)
	}
	return s.listingOutput(ctx, u, saved.ID, saved.Created)
}

type ListingIDInput struct {
	SessionInput
	ID int64 `path:"id" minimum:"1"`
}

func (s *Server) myListing(ctx context.Context, in *ListingIDInput) (*ListingOutput, error) {
	u, err := s.currentUser(ctx, in.Session)
	if err != nil {
		return nil, err
	}
	return s.listingOutput(ctx, u, in.ID, false)
}

type UpdateListingInput struct {
	SessionInput
	ID   int64 `path:"id" minimum:"1"`
	Body ListingBody
}

func (s *Server) updateListing(ctx context.Context, in *UpdateListingInput) (*ListingOutput, error) {
	u, err := s.currentUser(ctx, in.Session)
	if err != nil {
		return nil, err
	}
	if _, err := s.listings.Update(ctx, u, in.ID, in.Body.input(0)); err != nil {
		return nil, listingError(ctx, err)
	}
	return s.listingOutput(ctx, u, in.ID, false)
}

func (s *Server) refreshListing(ctx context.Context, in *ListingIDInput) (*ListingOutput, error) {
	u, err := s.currentUser(ctx, in.Session)
	if err != nil {
		return nil, err
	}
	if _, err := s.listings.Refresh(ctx, u, in.ID); err != nil {
		return nil, listingError(ctx, err)
	}
	return s.listingOutput(ctx, u, in.ID, false)
}

type RefreshDueOutput struct {
	Body struct {
		Renewed    int       `json:"renewed" doc:"How many listings were renewed"`
		ValidUntil time.Time `json:"valid_until" doc:"Their new expiry"`
	}
}

func (s *Server) refreshDueListings(ctx context.Context, in *SessionInput) (*RefreshDueOutput, error) {
	u, err := s.currentUser(ctx, in.Session)
	if err != nil {
		return nil, err
	}
	n, until, err := s.listings.RefreshDue(ctx, u)
	if err != nil {
		return nil, internalError(ctx, "refresh due listings", err)
	}
	out := &RefreshDueOutput{}
	out.Body.Renewed, out.Body.ValidUntil = n, until
	return out, nil
}

type VisibilityInput struct {
	SessionInput
	ID   int64 `path:"id" minimum:"1"`
	Body struct {
		Visible bool `json:"visible"`
	}
}

func (s *Server) setListingVisibility(ctx context.Context, in *VisibilityInput) (*struct{}, error) {
	u, err := s.currentUser(ctx, in.Session)
	if err != nil {
		return nil, err
	}
	if err := s.listings.SetVisible(ctx, u, in.ID, in.Body.Visible); err != nil {
		return nil, listingError(ctx, err)
	}
	return nil, nil
}

func (s *Server) deleteListing(ctx context.Context, in *ListingIDInput) (*struct{}, error) {
	u, err := s.currentUser(ctx, in.Session)
	if err != nil {
		return nil, err
	}
	if err := s.listings.DeleteListing(ctx, s.media, u, in.ID); err != nil {
		return nil, listingError(ctx, err)
	}
	return nil, nil
}

type MyListingsOutput struct {
	Body struct {
		Listings []MyListing `json:"listings"`
	}
}

func (s *Server) myListings(ctx context.Context, in *SessionInput) (*MyListingsOutput, error) {
	u, err := s.currentUser(ctx, in.Session)
	if err != nil {
		return nil, err
	}
	rows, err := s.listings.Mine(ctx, u.ID)
	if err != nil {
		return nil, internalError(ctx, "list listings", err)
	}
	funnels, err := s.sellerFunnels(ctx, u)
	if err != nil {
		return nil, internalError(ctx, "funnels", err)
	}
	out := &MyListingsOutput{}
	out.Body.Listings = make([]MyListing, len(rows))
	now := time.Now()
	for i, r := range rows {
		f := funnels[r.ID]
		out.Body.Listings[i] = MyListing{
			ID: r.ID, ProductID: r.ProductID, ProductName: r.ProductName, ProductSlug: r.ProductSlug,
			Category: r.ProductCategory, Variety: r.Variety, Description: r.Description, PriceSom: r.PriceSom,
			Unit: r.Unit, MinOrder: market.Float(r.MinOrder), Stock: market.Float(r.Stock), TradeType: r.TradeType,
			Status: r.Status, Expired: r.ValidUntil.Before(now), ValidUntil: r.ValidUntil, Views: r.Views,
			UpdatedAt: r.UpdatedAt, CoverKey: r.CoverKey, Photos: []Photo{},
			Pricing:  s.listingPricing(r.PricingMode, r.PricingGoal, r.PriceMin, r.PriceMax, r.ManualPriceAt),
			Funnel7d: &f,
		}
		if r.Median > 0 {
			l := &out.Body.Listings[i]
			l.Median = int64(math.Round(r.Median))
			l.MedianScope = "country"
			if r.MedianInProvince {
				l.MedianScope = "province"
			}
			d := int(math.Round(100 * (float64(r.PriceSom) - r.Median) / r.Median))
			l.DiffPercent = &d
		}
		l := &out.Body.Listings[i]
		if r.Status == market.StatusReview {
			l.ReviewReason = r.ReviewReason.String
		}
		if r.Status == market.StatusRejected {
			l.ReviewNote = r.ReviewNote.String
		}
		if r.Status == market.StatusActive && !l.Expired {
			pos, err := market.PositionOf(ctx, s.q, db.Listing{
				ID: r.ID, Status: r.Status, RegionID: r.RegionID, ProductID: r.ProductID,
				TradeType: r.TradeType, PriceSom: r.PriceSom,
			})
			if err != nil {
				return nil, internalError(ctx, "listing position", err)
			}
			l.Position = pricing.NewMarketPosition(pos)
		}
	}
	return out, nil
}

type UploadPhotoInput struct {
	SessionInput
	ID      int64 `path:"id" minimum:"1"`
	RawBody huma.MultipartFormFiles[struct {
		File huma.FormFile `form:"file" contentType:"image/jpeg,image/png,image/webp" required:"true"`
	}]
}

type PhotoOutput struct {
	Body Photo
}

func (s *Server) uploadPhoto(ctx context.Context, in *UploadPhotoInput) (*PhotoOutput, error) {
	u, err := s.currentUser(ctx, in.Session)
	if err != nil {
		return nil, err
	}
	f := in.RawBody.Data().File
	defer f.Close()
	lim := s.mediaLimits()
	p, err := lim.Process(f)
	switch {
	case errors.Is(err, media.ErrTooLarge):
		return nil, apiError(http.StatusRequestEntityTooLarge, "photo_too_large", fmt.Sprintf("Rasm juda katta. %d MB gacha va %d megapikselgacha rasm yuklang.", lim.MaxBytes>>20, lim.MaxPixels/1_000_000))
	case errors.Is(err, media.ErrUnsupported):
		return nil, apiError(http.StatusUnsupportedMediaType, "photo_unsupported", "Faqat JPEG, PNG yoki WebP rasm yuklash mumkin.")
	case err != nil:
		return nil, internalError(ctx, "process photo", err)
	}
	row, err := s.listings.AddPhoto(ctx, s.media, u, in.ID, p)
	if errors.Is(err, market.ErrTooManyPhotos) {
		return nil, apiError(http.StatusUnprocessableEntity, "photo_limit", fmt.Sprintf("Bitta e'longa ko'pi bilan %d ta rasm yuklash mumkin.", s.listings.MaxPhotos()))
	}
	if err != nil {
		return nil, listingError(ctx, err)
	}
	return &PhotoOutput{Body: Photo{ID: row.ID, Key: row.StorageKey, Width: row.Width, Height: row.Height}}, nil
}

type PhotoIDInput struct {
	SessionInput
	ID      int64 `path:"id" minimum:"1"`
	PhotoID int64 `path:"photo_id" minimum:"1"`
}

func (s *Server) deletePhoto(ctx context.Context, in *PhotoIDInput) (*struct{}, error) {
	u, err := s.currentUser(ctx, in.Session)
	if err != nil {
		return nil, err
	}
	if err := s.listings.DeletePhoto(ctx, s.media, u, in.PhotoID); err != nil {
		return nil, listingError(ctx, err)
	}
	return nil, nil
}

func (s *Server) listingOutput(ctx context.Context, u db.User, id int64, created bool) (*ListingOutput, error) {
	l, photos, err := s.listings.Owned(ctx, u.ID, id)
	if err != nil {
		return nil, listingError(ctx, err)
	}
	p, err := s.q.GetProduct(ctx, l.ProductID)
	if err != nil {
		return nil, internalError(ctx, "get product", err)
	}
	out := &ListingOutput{}
	out.Body.Created = created
	out.Body.Listing = MyListing{
		ID: l.ID, ProductID: l.ProductID, ProductName: p.Name, ProductSlug: p.Slug, Category: p.Category,
		Variety: l.Variety, Description: l.Description, PriceSom: l.PriceSom, Unit: l.Unit,
		MinOrder: market.Float(l.MinOrder), Stock: market.Float(l.Stock), TradeType: l.TradeType,
		Status: l.Status, Expired: l.ValidUntil.Before(time.Now()), ValidUntil: l.ValidUntil, Views: l.Views,
		UpdatedAt: l.UpdatedAt, Photos: make([]Photo, len(photos)),
		Pricing: s.listingPricing(l.PricingMode, l.PricingGoal, l.PriceMin, l.PriceMax, l.ManualPriceAt),
	}
	if l.Status == market.StatusRejected {
		out.Body.Listing.ReviewNote = l.ReviewNote.String
	}
	for i, ph := range photos {
		out.Body.Listing.Photos[i] = Photo{ID: ph.ID, Key: ph.StorageKey, Width: ph.Width, Height: ph.Height}
	}
	if len(photos) > 0 {
		out.Body.Listing.CoverKey = photos[0].StorageKey
	}
	return out, nil
}

// listingError maps service errors to polite Uzbek API errors.
func listingError(ctx context.Context, err error) error {
	var ve *market.ValidationError
	switch {
	case errors.As(err, &ve):
		return invalidInput(fieldError(ve.Field, ve.Code, ve.Message, ve.Params))
	case errors.Is(err, market.ErrNotSeller):
		return apiError(http.StatusForbidden, "sellers_only", "E'lon joylash uchun profilda «Tadbirkor» rolini tanlang va hududingizni kiriting.")
	case errors.Is(err, market.ErrNotFound):
		return apiError(http.StatusNotFound, "listing_not_found", "E'lon topilmadi.")
	case errors.Is(err, market.ErrUnknownProduct):
		return apiError(http.StatusUnprocessableEntity, "unknown_product", "Bunday mahsulot katalogda yo'q.")
	case errors.Is(err, market.ErrDuplicate):
		return apiError(http.StatusConflict, "listing_exists", "Bu mahsulot, nav va savdo turi uchun e'loningiz allaqachon bor.")
	case errors.Is(err, market.ErrStale):
		return apiError(http.StatusConflict, "price_changed_since", "Narx shundan keyin yana o'zgargan, qaytarib bo'lmaydi.")
	case errors.Is(err, market.ErrReverted):
		return apiError(http.StatusConflict, "already_reverted", "Bu o'zgarish allaqachon qaytarilgan.")
	case errors.Is(err, market.ErrTooManyPhotos):
		return apiError(http.StatusUnprocessableEntity, "photo_limit", "Bitta e'longa ko'pi bilan 5 ta rasm yuklash mumkin.")
	}
	return internalError(ctx, "listing", err)
}

type PricingInput struct {
	SessionInput
	ID   int64 `path:"id" minimum:"1"`
	Body ListingPricing
}

// setListingPricing turns automatic pricing on or off for one listing.
func (s *Server) setListingPricing(ctx context.Context, in *PricingInput) (*ListingOutput, error) {
	u, err := s.currentUser(ctx, in.Session)
	if err != nil {
		return nil, err
	}
	if _, err := s.listings.SetPricing(ctx, u, in.ID, market.Pricing{
		Mode: in.Body.Mode, Goal: in.Body.Goal, PriceMin: in.Body.PriceMin, PriceMax: in.Body.PriceMax,
	}); err != nil {
		return nil, listingError(ctx, err)
	}
	return s.listingOutput(ctx, u, in.ID, false)
}

// PriceChange is one automatic change and why it happened.
type PriceChange struct {
	ID         int64      `json:"id"`
	OldPrice   int64      `json:"old_price"`
	NewPrice   int64      `json:"new_price"`
	Status     string     `json:"status" enum:"applied,reverted"`
	CreatedAt  time.Time  `json:"created_at"`
	RevertedAt *time.Time `json:"reverted_at,omitempty"`
	CanRevert  bool       `json:"can_revert" doc:"true only for the latest change while the price is still the one it set"`
	Median     float64    `json:"median" doc:"The viloyat's weighted median the decision followed"`
	Peers      int        `json:"peers" doc:"How many other prices that median came from"`
	Factors    []Factor   `json:"factors"`
	Contrib    []Factor   `json:"contrib" doc:"Why the price moved, as exact Shapley shares of the old price that add up to new/old − 1 even when a cap cut the move; codes as in factors plus market (moving to the viloyat median) and rounding; empty for changes made before this was recorded"`
}

// Factor is one reason as a signed share of the price (0.04 = +4%).
type Factor struct {
	Code  string  `json:"code" enum:"market,goal,pressure,trend,response,rfq,sales,stock,cap,rounding" doc:"market: moving to the viloyat median (contrib only); rounding: to a bazaar price step (contrib only); goal: the seller's goal; pressure: demand vs supply; trend: the market's week; response: many views, few contacts; rfq: buyers picked others; sales: selling faster than others, or contacts without sales; stock: own stock vs usual; cap: range, market upper quartile or the daily 5% limit"`
	Share float64 `json:"share"`
}

type PriceChangesOutput struct {
	Body struct {
		Changes []PriceChange `json:"changes"`
	}
}

func (s *Server) listPriceChanges(ctx context.Context, in *ListingIDInput) (*PriceChangesOutput, error) {
	u, err := s.currentUser(ctx, in.Session)
	if err != nil {
		return nil, err
	}
	l, _, err := s.listings.Owned(ctx, u.ID, in.ID)
	if err != nil {
		return nil, listingError(ctx, err)
	}
	rows, err := s.q.ListPriceChanges(ctx, l.ID)
	if err != nil {
		return nil, internalError(ctx, "price changes", err)
	}
	out := &PriceChangesOutput{}
	out.Body.Changes = make([]PriceChange, len(rows))
	for i, c := range rows {
		var why market.Reasons
		if err := json.Unmarshal(c.Reasons, &why); err != nil {
			return nil, internalError(ctx, "price change reasons", err)
		}
		pc := PriceChange{
			ID: c.ID, OldPrice: c.OldPrice, NewPrice: c.NewPrice, Status: c.Status, CreatedAt: c.CreatedAt,
			RevertedAt: c.RevertedAt, CanRevert: i == 0 && c.Status == "applied" && l.PriceSom == c.NewPrice,
			Median: math.Round(why.Median), Peers: why.Peers, Factors: make([]Factor, len(why.Factors)),
			Contrib: make([]Factor, len(why.Contrib)),
		}
		for j, f := range why.Factors {
			pc.Factors[j] = Factor{Code: f.Code, Share: math.Round(f.Share*1000) / 1000}
		}
		for j, f := range why.Contrib {
			pc.Contrib[j] = Factor{Code: f.Code, Share: math.Round(f.Share*1000) / 1000}
		}
		out.Body.Changes[i] = pc
	}
	return out, nil
}

type PriceChangeIDInput struct {
	SessionInput
	ID int64 `path:"id" minimum:"1"`
}

func (s *Server) revertPriceChange(ctx context.Context, in *PriceChangeIDInput) (*ListingOutput, error) {
	u, err := s.currentUser(ctx, in.Session)
	if err != nil {
		return nil, err
	}
	l, _, err := s.listings.RevertAutoPrice(ctx, u, in.ID)
	if err != nil {
		return nil, listingError(ctx, err)
	}
	return s.listingOutput(ctx, u, l.ID, false)
}

type SoldInput struct {
	SessionInput
	ID   int64 `path:"id" minimum:"1"`
	Body struct {
		Quantity float64 `json:"quantity" exclusiveMinimum:"0" maximum:"10000000" doc:"How much was sold, in the listing's unit"`
		PriceSom *int64  `json:"price_som,omitempty" minimum:"50" maximum:"50000000" doc:"Price per unit agreed, if the seller wants to say"`
	}
}

type SoldOutput struct {
	Body struct {
		DealID    int64     `json:"deal_id"`
		Confirmed bool      `json:"confirmed" doc:"true when a buyer had reported this sale, so both sides have now confirmed it"`
		SoldOut   bool      `json:"sold_out" doc:"Stock reached 0: the listing is now hidden"`
		Listing   MyListing `json:"listing"`
	}
}

// markListingSold is the seller's "sold" button (QIZIQISH_VA_SAVDO.md step 5).
func (s *Server) markListingSold(ctx context.Context, in *SoldInput) (*SoldOutput, error) {
	u, err := s.currentUser(ctx, in.Session)
	if err != nil {
		return nil, err
	}
	sold, err := s.listings.MarkSold(ctx, u, in.ID, in.Body.Quantity, in.Body.PriceSom)
	if err != nil {
		return nil, listingError(ctx, err)
	}
	l, err := s.listingOutput(ctx, u, in.ID, false)
	if err != nil {
		return nil, err
	}
	out := &SoldOutput{}
	out.Body.DealID, out.Body.Confirmed, out.Body.SoldOut = sold.Deal.ID, sold.Matched, sold.SoldOut
	out.Body.Listing = l.Body.Listing
	return out, nil
}

// mediaLimits bound photo uploads; a bare config gets the defaults.
func (s *Server) mediaLimits() media.Limits {
	l := s.cfg.MediaLimits()
	if l.MaxBytes <= 0 {
		l.MaxBytes = media.DefaultLimits.MaxBytes
	}
	if l.MaxPixels <= 0 {
		l.MaxPixels = media.DefaultLimits.MaxPixels
	}
	return l
}
