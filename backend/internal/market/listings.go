// Package market manages seller listings: every seller price is a listing
// linked to a catalog product, and each price change is recorded in
// price_observations so medians can be computed from history.
package market

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"bugun/internal/analytics"
	"bugun/internal/roles"
	"bugun/internal/store/db"
)

// Validity is how long a listing stays visible without a refresh (spec §5).
const Validity = 7 * 24 * time.Hour

// RenewWindow is how close to expiry a listing counts as due for renewal; the
// seller's dashboard flags these and "renew all" extends them.
const RenewWindow = 48 * time.Hour

const (
	StatusActive   = "active"
	StatusReview   = "review"
	StatusHidden   = "hidden"
	StatusRejected = "rejected"

	minPriceSom = 50
	maxPriceSom = 50_000_000
	maxQuantity = 10_000_000
)

var (
	ErrNotSeller      = errors.New("only entrepreneurs with a region can post listings")
	ErrNotFound       = errors.New("listing not found")
	ErrDuplicate      = errors.New("listing for this product, variety and trade type already exists")
	ErrUnknownProduct = errors.New("unknown product")
)

// ValidationError explains which input was rejected, in Uzbek for the user.
type ValidationError struct {
	Field   string
	Code    string         // stable code clients translate, e.g. "too_long"
	Params  map[string]any // limits for the message, e.g. {"max": 60}
	Message string
}

func (e *ValidationError) Error() string { return e.Field + ": " + e.Message }

// Input is what a seller submits from the website, the Mini App or the bot.
type Input struct {
	ProductID   int16
	Variety     string
	Description string
	PriceSom    int64
	MinOrder    *float64
	Stock       *float64
	TradeType   string
	// RawText is the original message when the price came from the bot.
	RawText string

	contentFlag string // set by checkContent: a review reason for the text
}

type Service struct {
	pool      *pgxpool.Pool
	q         *db.Queries
	auto      AutoPriceRules
	maxPhotos int
	content   ContentChecker // optional: listing text moderation (AI-7)
}

// Option changes a Service's defaults.
type Option func(*Service)

// WithAutoPrice sets the rules of automatic pricing.
func WithAutoPrice(r AutoPriceRules) Option { return func(s *Service) { s.auto = r } }

// WithMaxPhotos sets how many photos a listing may have.
func WithMaxPhotos(n int) Option { return func(s *Service) { s.maxPhotos = n } }

func NewService(pool *pgxpool.Pool, opts ...Option) *Service {
	s := &Service{pool: pool, q: db.New(pool), auto: DefaultAutoPriceRules, maxPhotos: DefaultMaxPhotos}
	for _, o := range opts {
		o(s)
	}
	return s
}

// CanSell reports whether the user may post listings, prices and RFQ
// offers: an entrepreneur who has set a region.
func CanSell(u db.User) bool {
	return u.Role.Valid && roles.IsEntrepreneur(u.Role.String) && u.RegionID.Valid
}

// Saved is the result of a create, update or upsert.
type Saved struct {
	ID         int64
	Created    bool
	Status     string
	ValidUntil time.Time
}

func (in *Input) normalize() error {
	in.Variety = strings.Join(strings.Fields(in.Variety), " ")
	in.Description = strings.TrimSpace(in.Description)
	switch {
	case len([]rune(in.Variety)) > 60:
		return &ValidationError{"variety", "too_long", map[string]any{"max": 60}, "Nav 60 belgidan oshmasin."}
	case len([]rune(in.Description)) > 1000:
		return &ValidationError{"description", "too_long", map[string]any{"max": 1000}, "Tavsif 1000 belgidan oshmasin."}
	case in.PriceSom < minPriceSom || in.PriceSom > maxPriceSom:
		return &ValidationError{"price_som", "invalid_price", nil, "Narx noto'g'ri ko'rinadi."}
	case in.TradeType != "ulgurji" && in.TradeType != "chakana":
		return &ValidationError{"trade_type", "required", nil, "Savdo turini tanlang."}
	}
	for field, v := range map[string]*float64{"min_order": in.MinOrder, "stock": in.Stock} {
		if v != nil && (*v <= 0 || *v > maxQuantity) {
			return &ValidationError{field, "invalid_amount", nil, "Miqdor noto'g'ri ko'rinadi."}
		}
	}
	return nil
}

// Upsert creates the seller's listing for this product, variety and trade type,
// or updates it if it exists. The bot's text prices use this path.
func (s *Service) Upsert(ctx context.Context, u db.User, in Input) (Saved, error) {
	if err := in.normalize(); err != nil {
		return Saved{}, err
	}
	s.checkContent(ctx, in.ProductID, &in)
	var saved Saved
	err := s.inTx(ctx, func(q *db.Queries) error {
		var err error
		saved, err = UpsertTx(ctx, q, u, in)
		return err
	})
	return saved, err
}

// UpsertTx is Upsert inside the caller's transaction.
func UpsertTx(ctx context.Context, q *db.Queries, u db.User, in Input) (Saved, error) {
	if !CanSell(u) {
		return Saved{}, ErrNotSeller
	}
	if err := in.normalize(); err != nil {
		return Saved{}, err
	}
	p, err := q.GetProduct(ctx, in.ProductID)
	if errors.Is(err, pgx.ErrNoRows) {
		return Saved{}, ErrUnknownProduct
	}
	if err != nil {
		return Saved{}, err
	}
	// An existing listing is compared with its own last price and left out of its peers.
	listingID, err := q.SellerListingID(ctx, db.SellerListingIDParams{
		SellerID: u.ID, ProductID: in.ProductID, Variety: in.Variety, TradeType: in.TradeType,
	})
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return Saved{}, err
	}
	status, reason, err := reviewStatus(ctx, q, u, in, listingID)
	if err != nil {
		return Saved{}, err
	}
	validUntil := time.Now().Add(Validity)
	row, err := q.UpsertListing(ctx, db.UpsertListingParams{
		SellerID: u.ID, ProductID: in.ProductID, Variety: in.Variety, Description: in.Description,
		PriceSom: in.PriceSom, Unit: p.BaseUnit, MinOrder: numeric(in.MinOrder), Stock: numeric(in.Stock),
		TradeType: in.TradeType, RegionID: u.RegionID.Int16, MarketID: u.MarketID,
		Status: status, ValidUntil: validUntil, ReviewReason: reason,
	})
	if err != nil {
		return Saved{}, err
	}
	l, err := q.GetListing(ctx, row.ID)
	if err != nil {
		return Saved{}, err
	}
	if err := recordPrice(ctx, q, l, in.RawText, SourceSeller); err != nil {
		return Saved{}, err
	}
	return Saved{ID: l.ID, Created: row.Created, Status: l.Status, ValidUntil: l.ValidUntil}, nil
}

// Update edits one of the seller's listings; the product cannot change.
func (s *Service) Update(ctx context.Context, u db.User, id int64, in Input) (Saved, error) {
	if !CanSell(u) {
		return Saved{}, ErrNotSeller
	}
	if err := in.normalize(); err != nil {
		return Saved{}, err
	}
	if l, err := s.q.GetListing(ctx, id); err == nil && l.SellerID == u.ID {
		s.checkContent(ctx, l.ProductID, &in)
	}
	var saved Saved
	err := s.inTx(ctx, func(q *db.Queries) error {
		cur, err := q.GetListingForUpdate(ctx, db.GetListingForUpdateParams{ID: id, SellerID: u.ID})
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		in.ProductID = cur.ProductID
		status, reason, err := reviewStatus(ctx, q, u, in, cur.ID)
		if err != nil {
			return err
		}
		if cur.Status == StatusHidden {
			status = StatusHidden
		}
		l, err := q.UpdateListing(ctx, db.UpdateListingParams{
			ID: id, SellerID: u.ID, Variety: in.Variety, Description: in.Description, PriceSom: in.PriceSom,
			MinOrder: numeric(in.MinOrder), Stock: numeric(in.Stock), TradeType: in.TradeType,
			Status: status, ValidUntil: time.Now().Add(Validity), ReviewReason: reason,
		})
		if err != nil {
			return uniqueToDuplicate(err)
		}
		if err := recordPrice(ctx, q, l, "", SourceSeller); err != nil {
			return err
		}
		saved = Saved{ID: l.ID, Status: l.Status, ValidUntil: l.ValidUntil}
		return nil
	})
	return saved, err
}

// Refresh extends a listing for another week ("prices unchanged").
func (s *Service) Refresh(ctx context.Context, u db.User, id int64) (time.Time, error) {
	var until time.Time
	err := s.inTx(ctx, func(q *db.Queries) error {
		var err error
		until, err = q.RefreshListing(ctx, db.RefreshListingParams{ID: id, SellerID: u.ID, ValidUntil: time.Now().Add(Validity)})
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		return q.SetListingObservationValidity(ctx, db.SetListingObservationValidityParams{
			ListingID: pgtype.Int8{Int64: id, Valid: true}, ValidUntil: until,
		})
	})
	return until, err
}

// RefreshDue renews, in one transaction, every live listing of the seller that
// has expired or expires within RenewWindow. It returns how many were renewed.
func (s *Service) RefreshDue(ctx context.Context, u db.User) (int, time.Time, error) {
	return s.renewBefore(ctx, u, time.Now().Add(RenewWindow))
}

// RenewAll renews every live listing of the seller for another week: the
// weekly "prices unchanged" answer (spec §8.3).
func (s *Service) RenewAll(ctx context.Context, u db.User) (int, time.Time, error) {
	return s.renewBefore(ctx, u, time.Now().Add(Validity+time.Hour))
}

// renewBefore renews the seller's live listings that expire before dueBefore.
func (s *Service) renewBefore(ctx context.Context, u db.User, dueBefore time.Time) (int, time.Time, error) {
	now := time.Now()
	until := now.Add(Validity)
	var ids []int64
	err := s.inTx(ctx, func(q *db.Queries) error {
		var err error
		ids, err = q.RefreshDueListings(ctx, db.RefreshDueListingsParams{ValidUntil: until, SellerID: u.ID, DueBefore: dueBefore})
		if err != nil || len(ids) == 0 {
			return err
		}
		return q.SetListingsObservationValidity(ctx, db.SetListingsObservationValidityParams{ValidUntil: until, ListingIds: ids})
	})
	return len(ids), until, err
}

// Review settles a listing held for review: approved it goes live and its
// price joins the median; rejected it stays hidden with the admin's note.
func (s *Service) Review(ctx context.Context, id int64, approve bool, note string) error {
	listing, observation := StatusRejected, "rejected"
	if approve {
		listing, observation = StatusActive, "active"
	}
	return s.inTx(ctx, func(q *db.Queries) error {
		_, err := q.DecideListingReview(ctx, db.DecideListingReviewParams{
			ID: id, Status: listing, Note: pgtype.Text{String: note, Valid: note != ""},
		})
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		return q.SetReviewedObservationStatus(ctx, db.SetReviewedObservationStatusParams{
			ListingID: pgtype.Int8{Int64: id, Valid: true}, Status: observation,
		})
	})
}

// SetVisible hides a listing from buyers or shows it again.
func (s *Service) SetVisible(ctx context.Context, u db.User, id int64, visible bool) error {
	l, err := s.q.GetListing(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && l.SellerID != u.ID) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	status := StatusHidden
	if visible {
		status = StatusActive
	}
	return s.q.SetListingStatus(ctx, db.SetListingStatusParams{ID: id, SellerID: u.ID, Status: status})
}

func (s *Service) Mine(ctx context.Context, sellerID uuid.UUID) ([]db.ListSellerListingsRow, error) {
	return s.q.ListSellerListings(ctx, sellerID)
}

// Owned returns one of the seller's listings with its photos.
func (s *Service) Owned(ctx context.Context, sellerID uuid.UUID, id int64) (db.Listing, []db.ListingPhoto, error) {
	l, err := s.q.GetListing(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && l.SellerID != sellerID) {
		return db.Listing{}, nil, ErrNotFound
	}
	if err != nil {
		return db.Listing{}, nil, err
	}
	photos, err := s.q.ListListingPhotos(ctx, id)
	return l, photos, err
}

// reviewStatus holds a price for review when analytics.Screen flags it (spec
// §10.2): against the seller's last accepted price, other sellers in the same
// viloyat, and the official benchmark. The listing stays visible to its owner;
// the reason is stored for the admin.
func reviewStatus(ctx context.Context, q *db.Queries, u db.User, in Input, listingID int64) (string, pgtype.Text, error) {
	province, err := ProvinceOf(ctx, q, u.RegionID.Int16)
	if err != nil {
		return "", pgtype.Text{}, err
	}
	peers, _, err := peerPrices(ctx, q, province, in.ProductID, in.TradeType, listingID)
	if err != nil {
		return "", pgtype.Text{}, err
	}
	reference, err := q.ReferencePrice(ctx, db.ReferencePriceParams{
		ProvinceID: province, ProductID: in.ProductID, TradeType: in.TradeType,
	})
	if err != nil {
		return "", pgtype.Text{}, err
	}
	var previous int64
	if listingID != 0 {
		if previous, err = q.LastAcceptedPrice(ctx, pgtype.Int8{Int64: listingID, Valid: true}); err != nil {
			return "", pgtype.Text{}, err
		}
	}
	reason := analytics.Screen(analytics.Check{
		Peers: peers, Previous: float64(previous), Reference: reference,
	}, float64(in.PriceSom))
	if reason == analytics.ReasonNone {
		if status, text, ok := contentReview(in); ok {
			return status, text, nil
		}
		return StatusActive, pgtype.Text{}, nil
	}
	return StatusReview, pgtype.Text{String: string(reason), Valid: true}, nil
}

// PositionOf is where a live listing's price stands among other sellers in its
// viloyat (spec §10.4). Listings held for review are not ranked.
func PositionOf(ctx context.Context, q *db.Queries, l db.Listing) (analytics.Position, error) {
	if l.Status != StatusActive {
		return analytics.Position{}, nil
	}
	province, err := ProvinceOf(ctx, q, l.RegionID)
	if err != nil {
		return analytics.Position{}, err
	}
	peers, weights, err := peerPrices(ctx, q, province, l.ProductID, l.TradeType, l.ID)
	if err != nil {
		return analytics.Position{}, err
	}
	return analytics.PositionOf(peers, weights, float64(l.PriceSom)), nil
}

// ProvinceOf returns the viloyat of a region (the region itself for a viloyat).
func ProvinceOf(ctx context.Context, q *db.Queries, regionID int16) (int16, error) {
	region, err := q.GetRegion(ctx, regionID)
	if err != nil {
		return 0, err
	}
	if region.ParentID.Valid {
		return region.ParentID.Int16, nil
	}
	return region.ID, nil
}

// peerPrices are other sellers' live prices for the product in the viloyat and
// their median weights, leaving out the given listing (0 leaves out nothing).
func peerPrices(ctx context.Context, q *db.Queries, province, productID int16, tradeType string, listingID int64) ([]float64, []float64, error) {
	rows, err := q.PeerPrices(ctx, db.PeerPricesParams{
		ProductID: productID, TradeType: tradeType,
		ParentID:       pgtype.Int2{Int16: province, Valid: true},
		ExcludeListing: pgtype.Int8{Int64: listingID, Valid: listingID != 0},
	})
	if err != nil {
		return nil, nil, err
	}
	prices, weights := make([]float64, len(rows)), make([]float64, len(rows))
	for i, r := range rows {
		prices[i], weights[i] = r.Price, r.Weight
	}
	return prices, weights, nil
}

// Observation sources of a listing's price: set by the seller, or moved by
// automatic pricing (weighs half as much in the median).
const (
	SourceSeller     = "seller"
	SourceSellerAuto = "seller_auto"
)

// recordPrice retires the listing's previous observation and records the
// current price. Prices held for review never reach the median.
func recordPrice(ctx context.Context, q *db.Queries, l db.Listing, raw, source string) error {
	lid := pgtype.Int8{Int64: l.ID, Valid: true}
	if err := q.ExpireListingObservation(ctx, lid); err != nil {
		return err
	}
	status := "active"
	switch l.Status {
	case StatusReview:
		status = "review"
	case StatusRejected:
		status = "rejected"
	}
	return q.InsertListingObservation(ctx, db.InsertListingObservationParams{
		ProductID: l.ProductID, PriceSom: l.PriceSom, Unit: l.Unit, Quantity: l.Stock,
		TradeType: l.TradeType, RegionID: l.RegionID, MarketID: l.MarketID, Source: source,
		SellerID: pgtype.UUID{Bytes: l.SellerID, Valid: true}, ListingID: lid,
		ValidUntil: l.ValidUntil, RawText: pgtype.Text{String: raw, Valid: raw != ""}, Status: status,
	})
}

func (s *Service) inTx(ctx context.Context, fn func(q *db.Queries) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := fn(s.q.WithTx(tx)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func uniqueToDuplicate(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return ErrDuplicate
	}
	return err
}

func numeric(v *float64) pgtype.Numeric {
	var n pgtype.Numeric
	if v != nil {
		_ = n.Scan(strconv.FormatFloat(*v, 'f', -1, 64))
	}
	return n
}

// Float converts a nullable numeric column for API output.
func Float(n pgtype.Numeric) *float64 {
	if !n.Valid {
		return nil
	}
	f, err := n.Float64Value()
	if err != nil || !f.Valid {
		return nil
	}
	return &f.Float64
}

// Pricing modes and goals (DINAMIK_NARX.md).
const (
	PricingManual = "manual"
	PricingAuto   = "auto"
)

var pricingGoals = map[string]bool{"fast": true, "balanced": true, "profit": true}

// Pricing is how a listing's price may move on its own.
type Pricing struct {
	Mode     string // manual or auto
	Goal     string // fast, balanced or profit
	PriceMin *int64 // required in auto mode
	PriceMax *int64
}

// MaxPriceRatio keeps the range meaningful: the top may be at most three
// times the floor, so a typo cannot hand the system a free hand.
const MaxPriceRatio = 3

func (p *Pricing) validate(current int64) error {
	if p.Goal == "" {
		p.Goal = "balanced"
	}
	switch {
	case p.Mode != PricingManual && p.Mode != PricingAuto:
		return &ValidationError{"mode", "invalid_choice", nil, "Narx rejimini tanlang."}
	case !pricingGoals[p.Goal]:
		return &ValidationError{"goal", "invalid_choice", nil, "Narx maqsadini tanlang."}
	}
	for field, v := range map[string]*int64{"price_min": p.PriceMin, "price_max": p.PriceMax} {
		if v != nil && (*v < minPriceSom || *v > maxPriceSom) {
			return &ValidationError{field, "invalid_price", nil, "Narx noto'g'ri ko'rinadi."}
		}
	}
	if p.Mode != PricingAuto {
		return nil
	}
	switch {
	case p.PriceMin == nil || p.PriceMax == nil:
		return &ValidationError{"price_min", "required", nil, "Avtomatik narx uchun eng past va eng yuqori narxni kiriting."}
	case *p.PriceMin > *p.PriceMax:
		return &ValidationError{"price_max", "below_min", nil, "Eng yuqori narx eng pastdan kam bo'lmasin."}
	case *p.PriceMax > *p.PriceMin*MaxPriceRatio:
		return &ValidationError{"price_max", "range_too_wide", map[string]any{"max_ratio": MaxPriceRatio}, "Eng yuqori narx eng pastdan 3 barobardan oshmasin."}
	case current < *p.PriceMin || current > *p.PriceMax:
		return &ValidationError{"price_min", "price_outside_range", map[string]any{"price": current}, "Hozirgi narx kiritilgan oraliq ichida bo'lsin."}
	}
	return nil
}

// SetPricing saves a listing's pricing settings; auto mode needs a min–max
// range around the current price.
func (s *Service) SetPricing(ctx context.Context, u db.User, id int64, p Pricing) (db.Listing, error) {
	if !CanSell(u) {
		return db.Listing{}, ErrNotSeller
	}
	var l db.Listing
	err := s.inTx(ctx, func(q *db.Queries) error {
		cur, err := q.GetListingForUpdate(ctx, db.GetListingForUpdateParams{ID: id, SellerID: u.ID})
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if err := p.validate(cur.PriceSom); err != nil {
			return err
		}
		l, err = q.SetListingPricing(ctx, db.SetListingPricingParams{
			ID: id, SellerID: u.ID, Mode: p.Mode, Goal: p.Goal,
			PriceMin: optInt8(p.PriceMin), PriceMax: optInt8(p.PriceMax),
		})
		return err
	})
	return l, err
}

func optInt8(v *int64) pgtype.Int8 {
	if v == nil {
		return pgtype.Int8{}
	}
	return pgtype.Int8{Int64: *v, Valid: true}
}
