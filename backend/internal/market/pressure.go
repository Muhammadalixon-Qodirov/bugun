package market

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"

	"bugun/internal/analytics"
	"bugun/internal/store/db"
)

// PressureWindows returns weighted demand (demand signals and confirmed
// deals) and the number of sellers per 7-day
// window for the pressure index (spec §10.3): index 0 is the last 7 days, 1–4
// the weeks before. Province 0 means the whole country.
func PressureWindows(ctx context.Context, q *db.Queries, productID, province int16, trade string) (demand, supply [5]float64, err error) {
	prov := pgtype.Int2{Int16: province, Valid: province != 0}
	drows, err := q.DemandWindows(ctx, db.DemandWindowsParams{ProductID: productID, ProvinceID: prov})
	if err != nil {
		return demand, supply, err
	}
	for _, r := range drows {
		demand[r.Win] += analytics.DemandWeight[r.Kind] * float64(r.N)
	}
	deals, err := q.DealWindows(ctx, db.DealWindowsParams{ProductID: productID, ProvinceID: prov})
	if err != nil {
		return demand, supply, err
	}
	for _, r := range deals {
		demand[r.Win] += analytics.DemandWeight["deal"] * float64(r.N)
	}
	srows, err := q.SupplyWindows(ctx, db.SupplyWindowsParams{ProductID: productID, TradeType: trade, ProvinceID: prov})
	if err != nil {
		return demand, supply, err
	}
	for _, r := range srows {
		supply[r.Win] = float64(r.Sellers)
	}
	return demand, supply, nil
}
