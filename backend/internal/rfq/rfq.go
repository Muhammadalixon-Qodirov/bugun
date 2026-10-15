// Package rfq runs buyer requests for quotes (spec §8.4): a buyer asks for a
// product and amount, sellers with a live listing of it in the same viloyat
// answer with a price, the buyer picks offers and the seller confirms; only
// then do both see each other's contacts. Every offer price joins the market
// median as source rfq_offer (weight 1.0, spec §10.1).
package rfq

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"bugun/internal/analytics"
	"bugun/internal/market"
	"bugun/internal/roles"
	"bugun/internal/secure"
	"bugun/internal/store/db"
	"bugun/internal/tz"
)

// Limits bound a buyer's requests (config: RFQ_*).
type Limits struct {
	// MaxOpen is how many open RFQs one buyer may have at a time (spec §15:
	// three RFQs a week on the free plan).
	MaxOpen int
	// DefaultDays, MinDays and MaxDays: how long sellers may answer.
	DefaultDays, MinDays, MaxDays int
}

// DefaultLimits: three open requests, answered within 1 to 7 days, 3 unless
// the buyer chooses.
var DefaultLimits = Limits{MaxOpen: 3, DefaultDays: 3, MinDays: 1, MaxDays: 7}

const (
	maxNote     = 300
	maxQuantity = 10_000_000
	minPriceSom = 50
	maxPriceSom = 50_000_000
)

// Offer statuses.
const (
	StatusSent      = "sent"      // seller quoted a price
	StatusDeclined  = "declined"  // seller said "Menda yo'q"
	StatusChosen    = "chosen"    // buyer picked it; waiting for the seller
	StatusConfirmed = "confirmed" // seller agreed; contacts are exchanged
	StatusRejected  = "rejected"  // seller refused after being picked
)

var (
	ErrNotBuyer    = errors.New("only entrepreneurs with a region can request quotes")
	ErrNotSeller   = errors.New("only entrepreneurs with a region can answer requests")
	ErrTooManyOpen = errors.New("too many open requests")
	ErrNotFound    = errors.New("request or offer not found")
	ErrClosed      = errors.New("request is closed")
	ErrNotEligible = errors.New("seller has no live listing of this product in the viloyat")
	ErrOfferLocked = errors.New("offer was already picked")
	ErrBadState    = errors.New("offer is not in a state for this action")
)

// TooManyOpenError is ErrTooManyOpen with the limit in force, for messages.
type TooManyOpenError struct{ Max int }

func (e *TooManyOpenError) Error() string        { return ErrTooManyOpen.Error() }
func (e *TooManyOpenError) Is(target error) bool { return target == ErrTooManyOpen }

// OpenLimit is the limit behind an ErrTooManyOpen, or DefaultLimits.MaxOpen.
func OpenLimit(err error) int {
	var e *TooManyOpenError
	if errors.As(err, &e) {
		return e.Max
	}
	return DefaultLimits.MaxOpen
}

// ValidationError explains which input was rejected, in Uzbek for the user.
type ValidationError struct {
	Field   string
	Code    string
	Params  map[string]any
	Message string
}

func (e *ValidationError) Error() string { return e.Field + ": " + e.Message }

// Notifier tells people about RFQ events, e.g. through the Telegram bot. It
// is called after the change is committed and must not block for long.
type Notifier interface {
	RFQCreated(ctx context.Context, r db.Rfq, sellers []uuid.UUID)
	OfferMade(ctx context.Context, r db.Rfq, o db.Offer)
	OfferChosen(ctx context.Context, r db.Rfq, o db.Offer)
	OfferAnswered(ctx context.Context, r db.Rfq, o db.Offer)
}

type Service struct {
	pool      *pgxpool.Pool
	q         *db.Queries
	notify    Notifier
	demandKey func(string) []byte
	limits    Limits
}

// Option changes a Service's defaults.
type Option func(*Service)

// WithLimits sets the request limits.
func WithLimits(l Limits) Option { return func(s *Service) { s.limits = l } }

// NewService builds the service; notify may be nil when the bot is off.
func NewService(pool *pgxpool.Pool, notify Notifier, opts ...Option) *Service {
	s := &Service{pool: pool, q: db.New(pool), notify: notify, demandKey: secure.HashToken, limits: DefaultLimits}
	for _, o := range opts {
		o(s)
	}
	return s
}

// Limits are the request limits in force.
func (s *Service) Limits() Limits { return s.limits }

// SetDemandKey sets how demand signal keys are hashed, the same keyed hash
// the site uses, so one person's signals on a day share one key.
func (s *Service) SetDemandKey(hash func(string) []byte) { s.demandKey = hash }

// SetNotifier attaches the notifier once the bot has started.
func (s *Service) SetNotifier(n Notifier) { s.notify = n }

// CanBuy reports whether the user may request quotes: an entrepreneur who
// has set a region (the same people who may sell).
func CanBuy(u db.User) bool {
	return u.Role.Valid && roles.IsEntrepreneur(u.Role.String) && u.RegionID.Valid
}

// Input is a new RFQ. Quantity is in the product's base unit; the RFQ goes to
// the buyer's viloyat.
type Input struct {
	ProductID   int16
	Quantity    float64
	Days        int
	MaxPriceSom *int64
	Note        string
}

// Created is a new RFQ and how many sellers it reached.
type Created struct {
	RFQ     db.Rfq
	Sellers int
}

func (in *Input) normalize(l Limits) error {
	in.Note = strings.TrimSpace(in.Note)
	if in.Days == 0 {
		in.Days = l.DefaultDays
	}
	switch {
	case in.Quantity <= 0 || in.Quantity > maxQuantity:
		return &ValidationError{"quantity", "invalid_amount", nil, "Miqdor noto'g'ri ko'rinadi."}
	case in.Days < l.MinDays || in.Days > l.MaxDays:
		return &ValidationError{"days", "out_of_range", map[string]any{"min": l.MinDays, "max": l.MaxDays},
			fmt.Sprintf("Muddat %d kundan %d kungacha bo'lsin.", l.MinDays, l.MaxDays)}
	case in.MaxPriceSom != nil && (*in.MaxPriceSom < minPriceSom || *in.MaxPriceSom > maxPriceSom):
		return &ValidationError{"max_price_som", "invalid_price", nil, "Narx noto'g'ri ko'rinadi."}
	case utf8.RuneCountInString(in.Note) > maxNote:
		return &ValidationError{"note", "too_long", map[string]any{"max": maxNote}, "Izoh 300 belgidan oshmasin."}
	}
	return nil
}

// Create opens an RFQ for the buyer's viloyat and notifies matching sellers.
func (s *Service) Create(ctx context.Context, u db.User, in Input) (Created, error) {
	if !CanBuy(u) {
		return Created{}, ErrNotBuyer
	}
	if err := in.normalize(s.limits); err != nil {
		return Created{}, err
	}
	var out Created
	var sellers []uuid.UUID
	err := s.inTx(ctx, func(q *db.Queries) error {
		p, err := q.GetProduct(ctx, in.ProductID)
		if errors.Is(err, pgx.ErrNoRows) {
			return &ValidationError{"product_id", "unknown_product", nil, "Bunday mahsulot katalogda yo'q."}
		}
		if err != nil {
			return err
		}
		if err := q.LockUser(ctx, u.ID); err != nil {
			return err
		}
		open, err := q.CountOpenRFQs(ctx, u.ID)
		if err != nil {
			return err
		}
		if int(open) >= s.limits.MaxOpen {
			return &TooManyOpenError{Max: s.limits.MaxOpen}
		}
		out.RFQ, err = q.CreateRFQ(ctx, db.CreateRFQParams{
			BuyerID: u.ID, ProductID: in.ProductID, Quantity: numeric(in.Quantity), Unit: p.BaseUnit,
			RegionID: u.RegionID.Int16, Deadline: time.Now().Add(time.Duration(in.Days) * 24 * time.Hour),
			MaxPriceSom: optInt8(in.MaxPriceSom), Note: pgtype.Text{String: in.Note, Valid: in.Note != ""},
		})
		if err != nil {
			return err
		}
		province, err := market.ProvinceOf(ctx, q, out.RFQ.RegionID)
		if err != nil {
			return err
		}
		rows, err := q.RFQRecipients(ctx, db.RFQRecipientsParams{ProductID: in.ProductID, ProvinceID: province, BuyerID: u.ID})
		if err != nil {
			return err
		}
		for _, r := range rows {
			sellers = append(sellers, r.ID)
		}
		out.Sellers = len(rows)
		// An RFQ is the strongest demand signal (spec §10.3). The key follows
		// the day like other signals; the RFQ row already names the buyer.
		day := time.Now().In(tz.Tashkent).Format(time.DateOnly)
		return q.RecordDemand(ctx, db.RecordDemandParams{
			Kind: "rfq", ProductID: in.ProductID, ProvinceID: pgtype.Int2{Int16: province, Valid: true},
			ViewerKey: s.demandKey(day + "|u:" + u.ID.String()),
		})
	})
	if err != nil {
		return Created{}, err
	}
	if s.notify != nil && len(sellers) > 0 {
		s.notify.RFQCreated(detach(ctx), out.RFQ, sellers)
	}
	return out, nil
}

// Offer records or changes the seller's price for an RFQ; price 0 means
// "Menda yo'q". The price joins the market median unless it is an outlier.
func (s *Service) Offer(ctx context.Context, u db.User, rfqID int64, priceSom int64, quantity *float64) (db.Offer, error) {
	if !market.CanSell(u) {
		return db.Offer{}, ErrNotSeller
	}
	if priceSom != 0 && (priceSom < minPriceSom || priceSom > maxPriceSom) {
		return db.Offer{}, &ValidationError{"price_som", "invalid_price", nil, "Narx noto'g'ri ko'rinadi."}
	}
	if quantity != nil && (*quantity <= 0 || *quantity > maxQuantity) {
		return db.Offer{}, &ValidationError{"quantity", "invalid_amount", nil, "Miqdor noto'g'ri ko'rinadi."}
	}
	var r db.Rfq
	var o db.Offer
	err := s.inTx(ctx, func(q *db.Queries) error {
		var err error
		r, err = q.GetRFQ(ctx, rfqID)
		if errors.Is(err, pgx.ErrNoRows) || (err == nil && r.BuyerID == u.ID) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if r.Status != "open" || !r.Deadline.After(time.Now()) {
			return ErrClosed
		}
		province, err := market.ProvinceOf(ctx, q, r.RegionID)
		if err != nil {
			return err
		}
		ok, err := q.SellerCanOffer(ctx, db.SellerCanOfferParams{SellerID: u.ID, ProductID: r.ProductID, ProvinceID: province})
		if err != nil {
			return err
		}
		if !ok {
			return ErrNotEligible
		}
		status := StatusSent
		price := pgtype.Int8{Int64: priceSom, Valid: priceSom != 0}
		if priceSom == 0 {
			status, quantity = StatusDeclined, nil
		}
		o, err = q.UpsertOffer(ctx, db.UpsertOfferParams{
			RfqID: rfqID, SellerID: u.ID, PriceSom: price, Quantity: optNumeric(quantity), Status: status,
		})
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrOfferLocked
		}
		if err != nil {
			return err
		}
		return s.recordOfferPrice(ctx, q, u, r, o, province)
	})
	if err != nil {
		return db.Offer{}, err
	}
	if s.notify != nil && o.Status == StatusSent {
		s.notify.OfferMade(detach(ctx), r, o)
	}
	return o, nil
}

// recordOfferPrice replaces the offer's observation in the market history.
// An outlier is kept out of the median, like a listing held for review.
func (s *Service) recordOfferPrice(ctx context.Context, q *db.Queries, u db.User, r db.Rfq, o db.Offer, province int16) error {
	oid := pgtype.Int8{Int64: o.ID, Valid: true}
	if err := q.ExpireOfferObservation(ctx, oid); err != nil {
		return err
	}
	if o.Status != StatusSent {
		return nil
	}
	rows, err := q.PeerPrices(ctx, db.PeerPricesParams{
		ProductID: r.ProductID, TradeType: "ulgurji", ParentID: pgtype.Int2{Int16: province, Valid: true},
	})
	if err != nil {
		return err
	}
	peers := make([]float64, len(rows))
	for i, p := range rows {
		peers[i] = p.Price
	}
	status := "active"
	if analytics.IsOutlier(peers, float64(o.PriceSom.Int64)) {
		status = "review"
	}
	return q.InsertOfferObservation(ctx, db.InsertOfferObservationParams{
		ProductID: r.ProductID, PriceSom: o.PriceSom.Int64, Unit: r.Unit, Quantity: o.Quantity,
		RegionID: u.RegionID.Int16, MarketID: u.MarketID, SellerID: pgtype.UUID{Bytes: u.ID, Valid: true},
		OfferID: oid, ValidUntil: r.Deadline, Status: status,
	})
}

// Choose is the buyer picking an offer; the seller then confirms or refuses.
func (s *Service) Choose(ctx context.Context, u db.User, offerID int64) (db.Offer, error) {
	return s.transition(ctx, offerID, func(row db.GetOfferForUpdateRow) (string, error) {
		if row.BuyerID != u.ID {
			return "", ErrNotFound
		}
		if row.Status != StatusSent {
			return "", ErrBadState
		}
		return StatusChosen, nil
	}, func(ctx context.Context, r db.Rfq, o db.Offer) { s.notify.OfferChosen(ctx, r, o) })
}

// Answer is the seller confirming (contacts are exchanged) or refusing a
// picked offer.
func (s *Service) Answer(ctx context.Context, u db.User, offerID int64, agree bool) (db.Offer, error) {
	return s.transition(ctx, offerID, func(row db.GetOfferForUpdateRow) (string, error) {
		if row.SellerID != u.ID {
			return "", ErrNotFound
		}
		if row.Status != StatusChosen {
			return "", ErrBadState
		}
		if agree {
			return StatusConfirmed, nil
		}
		return StatusRejected, nil
	}, func(ctx context.Context, r db.Rfq, o db.Offer) { s.notify.OfferAnswered(ctx, r, o) })
}

func (s *Service) transition(ctx context.Context, offerID int64, next func(db.GetOfferForUpdateRow) (string, error), after func(context.Context, db.Rfq, db.Offer)) (db.Offer, error) {
	var r db.Rfq
	var o db.Offer
	err := s.inTx(ctx, func(q *db.Queries) error {
		row, err := q.GetOfferForUpdate(ctx, offerID)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		status, err := next(row)
		if err != nil {
			return err
		}
		if row.RfqStatus == "cancelled" {
			return ErrClosed
		}
		if err := q.SetOfferStatus(ctx, db.SetOfferStatusParams{ID: offerID, Status: status}); err != nil {
			return err
		}
		if status == StatusConfirmed { // both sides said yes: a deal
			if err := q.RecordRFQDeal(ctx, offerID); err != nil {
				return err
			}
		}
		if r, err = q.GetRFQ(ctx, row.RfqID); err != nil {
			return err
		}
		o = db.Offer{
			ID: row.ID, RfqID: row.RfqID, SellerID: row.SellerID, PriceSom: row.PriceSom,
			Quantity: row.Quantity, Status: status, CreatedAt: row.CreatedAt, UpdatedAt: time.Now(),
		}
		return nil
	})
	if err != nil {
		return db.Offer{}, err
	}
	if s.notify != nil {
		after(detach(ctx), r, o)
	}
	return o, nil
}

// Cancel closes the buyer's open RFQ.
func (s *Service) Cancel(ctx context.Context, u db.User, rfqID int64) error {
	n, err := s.q.CancelRFQ(ctx, db.CancelRFQParams{ID: rfqID, BuyerID: u.ID})
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// Get returns the buyer's own RFQ.
func (s *Service) Get(ctx context.Context, u db.User, rfqID int64) (db.Rfq, error) {
	r, err := s.q.GetRFQ(ctx, rfqID)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && r.BuyerID != u.ID) {
		return db.Rfq{}, ErrNotFound
	}
	return r, err
}

// Parties returns both sides of a confirmed offer; only they may read it.
func (s *Service) Parties(ctx context.Context, u db.User, offerID int64) (db.OfferPartiesRow, error) {
	p, err := s.q.OfferParties(ctx, offerID)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && p.BuyerID != u.ID && p.SellerID != u.ID) {
		return db.OfferPartiesRow{}, ErrNotFound
	}
	if err != nil {
		return db.OfferPartiesRow{}, err
	}
	if p.Status != StatusConfirmed {
		return db.OfferPartiesRow{}, ErrBadState
	}
	return p, nil
}

// Savings is spec §10.6: (market median − cheapest live offer) × quantity,
// never negative.
func Savings(median float64, cheapest int64, quantity float64) int64 {
	if median <= 0 || cheapest <= 0 || float64(cheapest) >= median {
		return 0
	}
	return int64((median - float64(cheapest)) * quantity)
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

// detach keeps request values but not its cancellation, so notifications
// finish after the HTTP response is sent.
func detach(ctx context.Context) context.Context { return context.WithoutCancel(ctx) }

func numeric(v float64) pgtype.Numeric {
	var n pgtype.Numeric
	if err := n.Scan(strconv.FormatFloat(v, 'f', -1, 64)); err != nil {
		slog.Warn("numeric", "value", v, "err", err)
	}
	return n
}

func optNumeric(v *float64) pgtype.Numeric {
	if v == nil {
		return pgtype.Numeric{}
	}
	return numeric(*v)
}

func optInt8(v *int64) pgtype.Int8 {
	if v == nil {
		return pgtype.Int8{}
	}
	return pgtype.Int8{Int64: *v, Valid: true}
}
