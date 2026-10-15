package pricing

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"bugun/internal/analytics"
	"bugun/internal/market"
	"bugun/internal/store/db"
)

const (
	draftTTL      = time.Hour
	maxMessageLen = 2000
)

var (
	ErrNotSeller  = errors.New("only entrepreneurs with a region can submit prices")
	ErrDraftGone  = errors.New("draft not found or expired")
	ErrTooLong    = errors.New("message too long")
	ErrEmptyInput = errors.New("empty message")
)

// Service is shared by the website and the bot, so both produce the same
// numbers from the same text.
type Service struct {
	pool      *pgxpool.Pool
	q         *db.Queries
	catalog   *Catalog
	extractor Extractor      // optional language model (MATN_TUSHUNISH.md 3)
	matcher   ProductMatcher // optional: unknown words to catalog products (AI-4)

	mu       sync.Mutex
	shared   []Learned // names three sellers confirmed
	sharedAt time.Time
}

// NewService loads the product catalog once; it is reference data that only
// changes with a migration and a restart.
func NewService(ctx context.Context, pool *pgxpool.Pool) (*Service, error) {
	q := db.New(pool)
	rows, err := q.ListParserProducts(ctx)
	if err != nil {
		return nil, fmt.Errorf("load catalog: %w", err)
	}
	products := make([]Product, len(rows))
	for i, r := range rows {
		units := make(map[string]float64, len(r.Units))
		for k, u := range r.Units {
			units[u] = r.UnitSizes[k]
		}
		products[i] = Product{ID: r.ID, Slug: r.Slug, Name: r.Name, Category: r.Category, BaseUnit: r.BaseUnit, Aliases: r.Synonyms, Units: units}
	}
	return &Service{pool: pool, q: q, catalog: NewCatalog(products)}, nil
}

type Draft struct {
	ID       int64     `json:"id"`
	Items    []Item    `json:"items"`
	Problems []Problem `json:"problems"`
	// Suggestions are words that may be a catalog product, when nothing was
	// recognised; the seller picks one and sends the text again.
	Suggestions []Suggestion `json:"suggestions,omitempty"`
}

// CanSubmit reports whether the user may post prices.
func CanSubmit(u db.User) bool { return market.CanSell(u) }

// CreateDraft parses text and stores the recognised items for confirmation;
// via says where the text came from (ViaWeb, ViaBot). ID is zero when
// nothing was recognised. Every attempt is logged for the parser's stats.
func (s *Service) CreateDraft(ctx context.Context, u db.User, text, via string) (Draft, error) {
	if !CanSubmit(u) {
		return Draft{}, ErrNotSeller
	}
	if strings.TrimSpace(text) == "" {
		return Draft{}, ErrEmptyInput
	}
	if len([]rune(text)) > maxMessageLen {
		return Draft{}, ErrTooLong
	}
	und := s.understand(ctx, s.catalogFor(ctx, u.ID), text)
	res := und.Result
	var sugg []Suggestion
	if len(res.Items) == 0 {
		res, sugg = s.resolveUnknown(ctx, text, res)
	}
	d := Draft{Items: res.Items, Problems: res.Problems, Suggestions: sugg}
	if len(res.Items) == 0 {
		s.logParse(ctx, &u.ID, via, text, res, und.ByModel, "none")
		return d, nil
	}
	items, err := json.Marshal(res.Items)
	if err != nil {
		return d, err
	}
	learn, err := json.Marshal(append([]Learned{}, und.Learn...))
	if err != nil {
		return d, err
	}
	logID := s.logParse(ctx, &u.ID, via, text, res, und.ByModel, "pending")
	d.ID, err = s.q.CreatePriceDraft(ctx, db.CreatePriceDraftParams{
		UserID: u.ID, RawText: text, Items: items, ExpiresAt: time.Now().Add(draftTTL),
		LogID: pgtype.Int8{Int64: logID, Valid: logID != 0}, Learn: learn,
	})
	return d, err
}

type Saved struct {
	Item
	ListingID  int64           `json:"listing_id"`
	Status     string          `json:"status" doc:"active, or review when the price is far from the market"`
	ValidUntil time.Time       `json:"valid_until"`
	Position   *MarketPosition `json:"position,omitempty" doc:"Place among other sellers in the viloyat; omitted when held for review or fewer than 3 other prices"`
	PrevPrice  int64           `json:"prev_price,omitempty" doc:"The listing's last accepted price before this one; omitted for a new listing"`
}

// MarketPosition is "12 sellers, you are 9th" (spec §8.2, §10.4).
type MarketPosition struct {
	Rank        int   `json:"rank" doc:"1 = cheapest"`
	Total       int   `json:"total" doc:"Sellers compared, including this one"`
	Median      int64 `json:"median" doc:"Median of the other sellers' prices"`
	DiffPercent int   `json:"diff_percent" doc:"This price vs. that median, in percent"`
}

// NewMarketPosition converts an analytics position for API output; nil when
// there is not enough data to rank.
func NewMarketPosition(p analytics.Position) *MarketPosition {
	if !p.Enough {
		return nil
	}
	return &MarketPosition{Rank: p.Rank, Total: p.Total, Median: int64(math.Round(p.Median)), DiffPercent: p.DiffPct}
}

// Confirm turns a draft into listings in one transaction: each item creates the
// seller's listing for that product or updates its price.
func (s *Service) Confirm(ctx context.Context, u db.User, draftID int64) ([]Saved, error) {
	if !CanSubmit(u) {
		return nil, ErrNotSeller
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	q := s.q.WithTx(tx)

	draft, err := q.ConsumePriceDraft(ctx, db.ConsumePriceDraftParams{ID: draftID, UserID: u.ID})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrDraftGone
	}
	if err != nil {
		return nil, err
	}
	var items []Item
	if err := json.Unmarshal(draft.Items, &items); err != nil {
		return nil, fmt.Errorf("decode draft: %w", err)
	}

	saved := make([]Saved, 0, len(items))
	for _, it := range items {
		var prev int64
		if id, err := q.SellerListingID(ctx, db.SellerListingIDParams{SellerID: u.ID, ProductID: it.ProductID, TradeType: it.TradeType}); err == nil {
			if prev, err = q.LastAcceptedPrice(ctx, pgtype.Int8{Int64: id, Valid: true}); err != nil {
				return nil, err
			}
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return nil, err
		}
		res, err := market.UpsertTx(ctx, q, u, market.Input{
			ProductID: it.ProductID, PriceSom: it.PriceSom, Stock: it.Quantity,
			TradeType: it.TradeType, RawText: draft.RawText,
		})
		if err != nil {
			return nil, err
		}
		saved = append(saved, Saved{Item: it, ListingID: res.ID, Status: res.Status, ValidUntil: res.ValidUntil, PrevPrice: prev})
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	s.setOutcome(ctx, draft.LogID, "confirmed")
	// The seller confirmed the model's reading: remember its names for them.
	var learned []Learned
	if json.Unmarshal(draft.Learn, &learned) == nil {
		for _, l := range learned {
			if err := s.q.InsertLearnedAlias(ctx, db.InsertLearnedAliasParams{Phrase: l.Phrase, ProductID: l.ProductID, UserID: u.ID}); err != nil {
				slog.WarnContext(ctx, "learn alias", "err", err)
			}
		}
	}
	// Ranking is a read after the commit; failing it must not undo the save.
	for i := range saved {
		l, err := s.q.GetListing(ctx, saved[i].ListingID)
		if err != nil {
			slog.WarnContext(ctx, "position after confirm", "err", err)
			continue
		}
		pos, err := market.PositionOf(ctx, s.q, l)
		if err != nil {
			slog.WarnContext(ctx, "position after confirm", "err", err)
			continue
		}
		saved[i].Position = NewMarketPosition(pos)
	}
	return saved, nil
}

// Discard throws a card away; the seller will usually write the text again.
func (s *Service) Discard(ctx context.Context, userID uuid.UUID, draftID int64) error {
	logID, err := s.q.DeletePriceDraft(ctx, db.DeletePriceDraftParams{ID: draftID, UserID: userID})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil // already gone
	}
	if err != nil {
		return err
	}
	s.setOutcome(ctx, logID, "discarded")
	return nil
}

// FormatSom formats an amount as "4 500".
func FormatSom(n int64) string { return groupDigits(n) }

// ParseDemand reads a buyer's request ("kartoshka 200 kg") against the catalog.
func (s *Service) ParseDemand(text string) (Demand, *Problem) { return s.catalog.ParseDemand(text) }

// Parse reads prices from free text without saving a draft, for posts in
// connected Telegram channels.
func (s *Service) Parse(text string) Result { return s.catalog.Parse(text) }
