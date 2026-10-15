// Package deals records sales someone said happened (QIZIQISH_VA_SAVDO.md).
// A contact reveal is interest, never a deal: a day later the buyer is asked
// whether they bought, and only an answer creates a deal.
package deals

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"bugun/internal/market"
	"bugun/internal/store/db"
	"bugun/internal/tz"
)

const (
	// AskAfter is how long after a contact reveal the buyer is asked.
	AskAfter = 24 * time.Hour
	// MaxPerDay caps the questions one buyer gets in a Tashkent day.
	MaxPerDay = 3
	// QuietFrom and QuietTo bound when the bot may ask (Tashkent hours).
	QuietTo, QuietFrom = 9, 20
	// MaxAsks: "still talking" gets one more question, then no more.
	MaxAsks = 2
	// DetailsWindow is how long a deal's quantity and price may be filled in.
	DetailsWindow = 7 * 24 * time.Hour

	maxQuantity = 10_000_000
	minPrice    = 50
	maxPrice    = 50_000_000
)

// Answers to a follow-up question.
const (
	AnswerYes     = "yes"
	AnswerTalking = "talking"
	AnswerNo      = "no"
)

var (
	ErrNotFound = errors.New("not found")
	ErrAnswered = errors.New("already answered")
	ErrClosed   = errors.New("too late to change")
	ErrInvalid  = errors.New("invalid quantity or price")
)

type Service struct {
	pool *pgxpool.Pool
	q    *db.Queries
}

func NewService(pool *pgxpool.Pool) *Service { return &Service{pool: pool, q: db.New(pool)} }

// ScheduleAfterContact asks the buyer about this listing a day from now,
// unless they were asked in the last week. The seller is never asked about
// their own listing.
func (s *Service) ScheduleAfterContact(ctx context.Context, buyerID uuid.UUID, listingID int64) error {
	return s.q.ScheduleDealPrompt(ctx, db.ScheduleDealPromptParams{
		ListingID: listingID, BuyerID: buyerID, DueAt: time.Now().Add(AskAfter),
	})
}

// InQuietHours reports whether the bot must not ask now.
func InQuietHours(now time.Time) bool {
	h := now.In(tz.Tashkent).Hour()
	return h < QuietTo || h >= QuietFrom
}

// TakeDue claims the questions to send now: at most MaxPerDay per buyer a
// day, none at night. Claimed questions are marked sent before they go out,
// so a crash may drop one but never sends it twice. Old unsent ones expire.
func (s *Service) TakeDue(ctx context.Context, now time.Time) ([]int64, error) {
	if _, err := s.q.ExpireDealPrompts(ctx); err != nil {
		return nil, err
	}
	if InQuietHours(now) {
		return nil, nil
	}
	var ids []int64
	err := s.inTx(ctx, func(q *db.Queries) error {
		due, err := q.LockDueDealPrompts(ctx)
		if err != nil || len(due) == 0 {
			return err
		}
		buyers := make([]uuid.UUID, 0, len(due))
		for _, d := range due {
			buyers = append(buyers, d.BuyerID)
		}
		counts, err := q.DealPromptsSentToday(ctx, buyers)
		if err != nil {
			return err
		}
		sent := map[uuid.UUID]int{}
		for _, c := range counts {
			sent[c.BuyerID] = int(c.N)
		}
		for _, d := range due {
			if sent[d.BuyerID] >= MaxPerDay {
				continue // tomorrow
			}
			sent[d.BuyerID]++
			ids = append(ids, d.ID)
		}
		if len(ids) == 0 {
			return nil
		}
		return q.MarkDealPromptsSent(ctx, ids)
	})
	return ids, err
}

// Answer records the buyer's answer. "yes" records a deal the buyer reports
// and returns it; "talking" asks again in 48 hours; "no" closes the question.
func (s *Service) Answer(ctx context.Context, buyer db.User, promptID int64, answer string) (*db.Deal, error) {
	var deal *db.Deal
	err := s.inTx(ctx, func(q *db.Queries) error {
		p, err := q.GetDealPromptForUpdate(ctx, db.GetDealPromptForUpdateParams{ID: promptID, BuyerID: buyer.ID})
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if p.Answer.Valid && p.Answer.String != AnswerTalking {
			return ErrAnswered
		}
		dealID := pgtype.Int8{}
		switch answer {
		case AnswerYes:
			d, err := buyerSaysYes(ctx, q, buyer, p.ListingID)
			if err != nil {
				return err
			}
			if err := market.SyncDealPrice(ctx, q, d.ID); err != nil {
				return err
			}
			deal, dealID = &d, pgtype.Int8{Int64: d.ID, Valid: true}
		case AnswerTalking:
			if p.Asks >= MaxAsks {
				return ErrAnswered
			}
		case AnswerNo:
		default:
			return ErrInvalid
		}
		return q.AnswerDealPrompt(ctx, db.AnswerDealPromptParams{ID: p.ID, Answer: answer, DealID: dealID})
	})
	return deal, err
}

// buyerSaysYes confirms the seller's own report of a sale on this listing
// when there is one from the last week, else records the buyer's report.
func buyerSaysYes(ctx context.Context, q *db.Queries, buyer db.User, listingID int64) (db.Deal, error) {
	d, err := q.LockSellerReportedDeal(ctx, pgtype.Int8{Int64: listingID, Valid: true})
	if err == nil {
		return q.ConfirmDealByBuyer(ctx, db.ConfirmDealByBuyerParams{ID: d.ID, BuyerID: buyer.ID})
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return db.Deal{}, err
	}
	return q.RecordBuyerContactDeal(ctx, db.RecordBuyerContactDealParams{BuyerID: buyer.ID, ListingID: listingID})
}

// SellerSold records the seller saying a listing sold (the evening question).
// It confirms the oldest unconfirmed sale a buyer reported on the listing in
// the last week, else records the seller's report for a buyer to confirm
// later. matched tells whether both sides have now said yes.
func (s *Service) SellerSold(ctx context.Context, seller db.User, listingID int64) (deal db.Deal, matched bool, err error) {
	err = s.inTx(ctx, func(q *db.Queries) error {
		if _, err := q.GetListingForUpdate(ctx, db.GetListingForUpdateParams{ID: listingID, SellerID: seller.ID}); errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		} else if err != nil {
			return err
		}
		d, err := q.LockBuyerReportedDeal(ctx, pgtype.Int8{Int64: listingID, Valid: true})
		switch {
		case err == nil:
			if deal, err = q.ConfirmDealBySeller(ctx, d.ID); err != nil {
				return err
			}
			matched = true
			return market.SyncDealPrice(ctx, q, deal.ID)
		case errors.Is(err, pgx.ErrNoRows):
			deal, err = q.RecordSellerContactDeal(ctx, listingID)
			return err
		}
		return err
	})
	return deal, matched, err
}

// SetDetails fills in how much was sold and at what price; either may be
// left out. The buyer's figures win: the seller only fills what is still
// empty. Only the two sides may, within DetailsWindow of the deal.
func (s *Service) SetDetails(ctx context.Context, u db.User, dealID int64, quantity *float64, price *int64) (db.Deal, error) {
	if (quantity != nil && (*quantity <= 0 || *quantity > maxQuantity)) || (price != nil && (*price < minPrice || *price > maxPrice)) {
		return db.Deal{}, ErrInvalid
	}
	var out db.Deal
	err := s.inTx(ctx, func(q *db.Queries) error {
		d, err := q.GetDealForUpdate(ctx, dealID)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		isBuyer := d.BuyerID.Valid && d.BuyerID.Bytes == u.ID
		if !isBuyer && d.SellerID != u.ID {
			return ErrNotFound
		}
		if d.Status == "disputed" || time.Since(d.CreatedAt) > DetailsWindow {
			return ErrClosed
		}
		qty := d.Quantity
		if quantity != nil && (isBuyer || !qty.Valid) {
			if err := qty.Scan(strconv.FormatFloat(*quantity, 'f', -1, 64)); err != nil {
				return err
			}
		}
		p := d.PriceSom
		if price != nil && (isBuyer || !p.Valid) {
			p = pgtype.Int8{Int64: *price, Valid: true}
		}
		if out, err = q.SetDealDetails(ctx, db.SetDealDetailsParams{ID: d.ID, Quantity: qty, PriceSom: p}); err != nil {
			return err
		}
		return market.SyncDealPrice(ctx, q, d.ID)
	})
	return out, err
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

// Ratings (QIZIQISH_VA_SAVDO.md step 9).

// ErrNotRatable: only the buyer of a confirmed deal may rate it.
var ErrNotRatable = errors.New("deal cannot be rated")

// MaxComment is the longest rating comment.
const MaxComment = 300

// TakeToRate claims the confirmed deals whose buyer should now be asked for
// a rating; each deal is asked about once.
func (s *Service) TakeToRate(ctx context.Context) ([]int64, error) {
	return s.q.ClaimDealsToRate(ctx)
}

// Rate records (or changes) the buyer's stars for a confirmed deal.
func (s *Service) Rate(ctx context.Context, buyer db.User, dealID int64, stars int) (db.Rating, error) {
	if stars < 1 || stars > 5 {
		return db.Rating{}, ErrInvalid
	}
	r, err := s.q.RateDeal(ctx, db.RateDealParams{Stars: int16(stars), DealID: dealID, BuyerID: buyer.ID})
	if errors.Is(err, pgx.ErrNoRows) {
		return r, ErrNotRatable
	}
	return r, err
}

// Comment adds the buyer's words to their rating. A comment on a 1–2 star
// rating waits for an admin before it is shown.
func (s *Service) Comment(ctx context.Context, buyer db.User, dealID int64, text string) (db.Rating, error) {
	text = strings.TrimSpace(text)
	if utf8.RuneCountInString(text) > MaxComment {
		return db.Rating{}, ErrInvalid
	}
	r, err := s.q.CommentRating(ctx, db.CommentRatingParams{Comment: text, DealID: dealID, BuyerID: buyer.ID})
	if errors.Is(err, pgx.ErrNoRows) {
		return r, ErrNotRatable
	}
	return r, err
}
