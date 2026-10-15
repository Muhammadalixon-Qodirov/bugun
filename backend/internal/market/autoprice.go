package market

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"

	"bugun/internal/analytics"
	"bugun/internal/store/db"
)

// AutoPriceRules are the tunable parts of automatic pricing (config:
// REPRICE_*, AUTO_PRICE_*).
type AutoPriceRules struct {
	analytics.RepriceRules
	// ManualPause is how long automatic pricing leaves a price set by hand.
	ManualPause time.Duration
	// MinMarketViews: below this the market's contact rate is too noisy to
	// compare with.
	MinMarketViews int
}

// DefaultAutoPriceRules: a price set by hand holds for 24 hours, and the
// market's contact rate counts from 100 views.
var DefaultAutoPriceRules = AutoPriceRules{
	RepriceRules: analytics.DefaultRepriceRules, ManualPause: 24 * time.Hour, MinMarketViews: 100,
}

// ManualPause is how long automatic pricing leaves a price set by hand.
func (s *Service) ManualPause() time.Duration { return s.auto.ManualPause }

// minStockPeers: with fewer other stocks there is no "usual" stock to compare with.
const minStockPeers = 3

var (
	// ErrStale: the price moved again after this change, so undoing it would
	// overwrite a newer price.
	ErrStale = errors.New("price changed again since")
	// ErrReverted: the change was already undone.
	ErrReverted = errors.New("price change already reverted")
)

// Reasons is what price_changes.reasons holds: the market the decision saw
// and the factors behind it, so the seller can see why.
type Reasons struct {
	Median  float64            `json:"median"`
	Peers   int                `json:"peers"`
	Factors []analytics.Factor `json:"factors"`
	Contrib []analytics.Factor `json:"contrib,omitempty"` // exact shares of the move; absent in changes before 2026-09-26
}

// AutoChange is one price the automatic pricing moved.
type AutoChange struct {
	Listing db.Listing // with the new price
	Change  db.PriceChange
	Reasons Reasons
}

// Reprice runs automatic pricing over every listing that has it on
// (DINAMIK_NARX.md). A listing that fails is logged and skipped, so one bad
// row does not stop the rest.
func (s *Service) Reprice(ctx context.Context) ([]AutoChange, error) {
	ids, err := s.q.AutoPricedListings(ctx, s.auto.ManualPause.Seconds())
	if err != nil {
		return nil, err
	}
	var changes []AutoChange
	failed := 0
	for _, id := range ids {
		var c *AutoChange
		err := s.inTx(ctx, func(q *db.Queries) error {
			var err error
			c, err = repriceListing(ctx, q, id, s.auto)
			return err
		})
		if err != nil {
			failed++
			slog.ErrorContext(ctx, "reprice listing", "listing", id, "err", err)
			continue
		}
		if c != nil {
			changes = append(changes, *c)
		}
	}
	if failed > 0 {
		return changes, fmt.Errorf("%d of %d listings failed", failed, len(ids))
	}
	return changes, nil
}

// repriceListing decides and applies one listing's price under a row lock, so
// a price the seller sets at the same moment is never overwritten. It returns
// nil when the price stays.
func repriceListing(ctx context.Context, q *db.Queries, id int64, rules AutoPriceRules) (*AutoChange, error) {
	l, err := q.LockAutoPricedListing(ctx, db.LockAutoPricedListingParams{ID: id, PauseSeconds: rules.ManualPause.Seconds()})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil // changed by hand or switched off since the list was read
	}
	if err != nil {
		return nil, err
	}
	in, err := repriceInput(ctx, q, l, rules.MinMarketViews)
	if err != nil {
		return nil, err
	}
	d := rules.Reprice(in)
	if !d.Change {
		return nil, nil
	}
	why := Reasons{Median: in.Median, Peers: in.Peers, Factors: d.Factors, Contrib: d.Contrib}
	reasons, err := json.Marshal(why)
	if err != nil {
		return nil, err
	}
	updated, err := q.SetAutoPrice(ctx, db.SetAutoPriceParams{ID: l.ID, PriceSom: d.Price})
	if err != nil {
		return nil, err
	}
	change, err := q.InsertPriceChange(ctx, db.InsertPriceChangeParams{
		ListingID: l.ID, OldPrice: l.PriceSom, NewPrice: d.Price, Reasons: reasons,
	})
	if err != nil {
		return nil, err
	}
	if err := recordPrice(ctx, q, updated, "", SourceSellerAuto); err != nil {
		return nil, err
	}
	return &AutoChange{Listing: updated, Change: change, Reasons: why}, nil
}

// RevertAutoPrice undoes one of the seller's automatic changes. The old price
// comes back as the seller's own, which pauses automatic pricing (ManualPause).
func (s *Service) RevertAutoPrice(ctx context.Context, u db.User, changeID int64) (db.Listing, db.PriceChange, error) {
	var l db.Listing
	var c db.PriceChange
	err := s.inTx(ctx, func(q *db.Queries) error {
		var err error
		c, err = q.GetPriceChangeForUpdate(ctx, db.GetPriceChangeForUpdateParams{ID: changeID, SellerID: u.ID})
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if c.Status == "reverted" {
			return ErrReverted
		}
		cur, err := q.GetListingForUpdate(ctx, db.GetListingForUpdateParams{ID: c.ListingID, SellerID: u.ID})
		if err != nil {
			return err
		}
		if cur.PriceSom != c.NewPrice {
			return ErrStale
		}
		if l, err = q.RevertAutoPrice(ctx, db.RevertAutoPriceParams{ID: cur.ID, PriceSom: c.OldPrice}); err != nil {
			return err
		}
		if err := q.MarkPriceChangeReverted(ctx, c.ID); err != nil {
			return err
		}
		c.Status = "reverted"
		return recordPrice(ctx, q, l, "", SourceSeller)
	})
	return l, c, err
}

// repriceInput gathers what one decision looks at: the viloyat's prices now
// and a week ago, demand pressure, how often viewers call, sales, lost RFQs
// and stock.
func repriceInput(ctx context.Context, q *db.Queries, l db.Listing, minMarketViews int) (analytics.RepriceInput, error) {
	in := analytics.RepriceInput{
		Current: l.PriceSom, DayStart: l.PriceSom, Min: l.PriceMin.Int64, Max: l.PriceMax.Int64, Goal: l.PricingGoal,
	}
	province, err := ProvinceOf(ctx, q, l.RegionID)
	if err != nil {
		return in, err
	}
	prices, weights, err := peerPrices(ctx, q, province, l.ProductID, l.TradeType, l.ID)
	if err != nil {
		return in, err
	}
	in.Peers = len(prices)
	if in.Peers < analytics.MinRepricePeers {
		return in, nil // Reprice holds; the rest would be wasted work
	}
	in.Median = analytics.WeightedQuantile(prices, weights, 0.5)
	in.P75 = analytics.WeightedQuantile(prices, weights, 0.75)

	prev, err := q.PeerPricesWeekAgo(ctx, db.PeerPricesWeekAgoParams{
		ProductID: l.ProductID, TradeType: l.TradeType, ProvinceID: province, ListingID: l.ID,
	})
	if err != nil {
		return in, err
	}
	if len(prev) >= analytics.MinRepricePeers {
		pp, pw := make([]float64, len(prev)), make([]float64, len(prev))
		for i, r := range prev {
			pp[i], pw[i] = r.Price, r.Weight
		}
		in.MedianPrev = analytics.WeightedQuantile(pp, pw, 0.5)
	}

	if start, err := q.DayStartPrice(ctx, l.ID); err != nil {
		return in, err
	} else if start > 0 {
		in.DayStart = start
	}

	demand, supply, err := PressureWindows(ctx, q, l.ProductID, province, l.TradeType)
	if err != nil {
		return in, err
	}
	in.Pressure, _, in.PressureOK = analytics.Pressure(demand[0], demand[1:], supply[0], supply[1:])

	resp, err := q.ResponseCounts(ctx, db.ResponseCountsParams{
		ProductID: l.ProductID, TradeType: l.TradeType, ListingID: l.ID, ProvinceID: province,
	})
	if err != nil {
		return in, err
	}
	in.Views, in.Contacts = int(resp.Views), int(resp.Contacts)
	if int(resp.MarketViews) >= minMarketViews {
		in.MarketRate = float64(resp.MarketContacts) / float64(resp.MarketViews)
	}

	sales, err := q.SalesCounts(ctx, db.SalesCountsParams{
		ProductID: l.ProductID, TradeType: l.TradeType, ListingID: l.ID, ProvinceID: province,
	})
	if err != nil {
		return in, err
	}
	in.Sales = int(sales.Sales)
	if sales.Peers > 0 {
		in.MarketSales, in.SalesKnown = float64(sales.PeerSales)/float64(sales.Peers), true
	}

	rfq, err := q.SellerRFQOutcomes(ctx, db.SellerRFQOutcomesParams{SellerID: l.SellerID, ProductID: l.ProductID})
	if err != nil {
		return in, err
	}
	in.RFQOffers, in.RFQPicked = int(rfq.Offers), int(rfq.Picked)

	if stock := Float(l.Stock); stock != nil && *stock > 0 {
		peers, err := q.PeerStock(ctx, db.PeerStockParams{
			ProductID: l.ProductID, TradeType: l.TradeType, Unit: l.Unit, ListingID: l.ID, ProvinceID: province,
		})
		if err != nil {
			return in, err
		}
		if peers.N >= minStockPeers && peers.Median > 0 {
			in.StockRatio = *stock / peers.Median
		}
	}
	return in, nil
}

// AutoPriceRules are the automatic pricing rules in force.
func (s *Service) AutoPriceRules() AutoPriceRules { return s.auto }
