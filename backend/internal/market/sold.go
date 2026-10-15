package market

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"bugun/internal/store/db"
)

// Sold is the outcome of the seller's "sold" button.
type Sold struct {
	Deal    db.Deal
	Matched bool // a buyer had reported this sale: now confirmed by both
	SoldOut bool // stock reached 0 and the listing was hidden
	Listing db.Listing
}

// MarkSold records that the seller sold quantity (in the listing's unit),
// optionally at a price (QIZIQISH_VA_SAVDO.md step 5). It confirms a sale a
// buyer reported on the listing in the last week when there is one, lowers
// the stock, and hides the listing when nothing is left.
func (s *Service) MarkSold(ctx context.Context, u db.User, id int64, quantity float64, price *int64) (Sold, error) {
	if quantity <= 0 || quantity > maxQuantity {
		return Sold{}, &ValidationError{"quantity", "invalid_amount", nil, "Miqdor noto'g'ri ko'rinadi."}
	}
	if price != nil && (*price < minPriceSom || *price > maxPriceSom) {
		return Sold{}, &ValidationError{"price_som", "invalid_price", nil, "Narx noto'g'ri ko'rinadi."}
	}
	var out Sold
	err := s.inTx(ctx, func(q *db.Queries) error {
		l, err := q.GetListingForUpdate(ctx, db.GetListingForUpdateParams{ID: id, SellerID: u.ID})
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		stock := Float(l.Stock)
		if stock != nil && quantity > *stock {
			return &ValidationError{"quantity", "more_than_stock", map[string]any{"stock": *stock},
				"Zaxiradagidan ko'p sotib bo'lmaydi. Avval zaxirani yangilang."}
		}
		qty := numeric(&quantity)
		p := pgtype.Int8{}
		if price != nil {
			p = pgtype.Int8{Int64: *price, Valid: true}
		}

		lid := pgtype.Int8{Int64: l.ID, Valid: true}
		reported, err := q.LockBuyerReportedDeal(ctx, lid)
		switch {
		case err == nil:
			if out.Deal, err = q.ConfirmDealBySeller(ctx, reported.ID); err != nil {
				return err
			}
			out.Matched = true
			// The buyer's figures win; the seller fills only what is empty.
			if out.Deal.Quantity.Valid {
				qty = out.Deal.Quantity
			}
			if out.Deal.PriceSom.Valid {
				p = out.Deal.PriceSom
			}
			if out.Deal, err = q.SetDealDetails(ctx, db.SetDealDetailsParams{ID: out.Deal.ID, Quantity: qty, PriceSom: p}); err != nil {
				return err
			}
		case errors.Is(err, pgx.ErrNoRows):
			if out.Deal, err = q.RecordSellerMarkedDeal(ctx, db.RecordSellerMarkedDealParams{Quantity: qty, PriceSom: p, ListingID: l.ID}); err != nil {
				return err
			}
		default:
			return err
		}

		if err := SyncDealPrice(ctx, q, out.Deal.ID); err != nil {
			return err
		}
		out.Listing = l
		if stock != nil {
			left := *stock - quantity
			status := l.Status
			if left <= 0 {
				out.SoldOut, status = true, StatusHidden
			}
			var n pgtype.Numeric
			if left > 0 {
				n = numeric(&left)
			}
			if out.Listing, err = q.SetListingStockAndStatus(ctx, db.SetListingStockAndStatusParams{ID: l.ID, Stock: n, Status: status}); err != nil {
				return err
			}
		}
		return nil
	})
	return out, err
}

// SyncDealPrice puts a deal's agreed price into the median, or takes it out,
// after any change to the deal. Only confirmed, priced, non-RFQ deals count.
func SyncDealPrice(ctx context.Context, q *db.Queries, dealID int64) error {
	id := pgtype.Int8{Int64: dealID, Valid: true}
	if err := q.ExpireDealObservation(ctx, id); err != nil {
		return err
	}
	return q.InsertDealObservation(ctx, dealID)
}
