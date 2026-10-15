package market

import (
	"context"
	"errors"
	"testing"
)

func i64(v int64) *int64 { return &v }

func TestPricingValidate(t *testing.T) {
	cases := []struct {
		name  string
		p     Pricing
		price int64
		field string // "" = valid
	}{
		{"manual needs nothing", Pricing{Mode: PricingManual}, 6000, ""},
		{"auto with a range around the price", Pricing{Mode: PricingAuto, PriceMin: i64(5500), PriceMax: i64(7000)}, 6000, ""},
		{"auto without bounds", Pricing{Mode: PricingAuto}, 6000, "price_min"},
		{"min above max", Pricing{Mode: PricingAuto, PriceMin: i64(7000), PriceMax: i64(5500)}, 6000, "price_max"},
		{"range wider than 3x", Pricing{Mode: PricingAuto, PriceMin: i64(2000), PriceMax: i64(7000)}, 6000, "price_max"},
		{"current price outside the range", Pricing{Mode: PricingAuto, PriceMin: i64(6500), PriceMax: i64(7000)}, 6000, "price_min"},
		{"unknown mode", Pricing{Mode: "robot"}, 6000, "mode"},
		{"unknown goal", Pricing{Mode: PricingManual, Goal: "greedy"}, 6000, "goal"},
		{"absurd bound", Pricing{Mode: PricingManual, PriceMin: i64(10)}, 6000, "price_min"},
	}
	for _, c := range cases {
		err := c.p.validate(c.price)
		var ve *ValidationError
		switch {
		case c.field == "" && err != nil:
			t.Errorf("%s: %v, want valid", c.name, err)
		case c.field != "" && (!errors.As(err, &ve) || ve.Field != c.field):
			t.Errorf("%s: %v, want an error on %s", c.name, err, c.field)
		}
	}
}

func TestSetPricingAndManualPriceTime(t *testing.T) {
	svc, u, pool := testSeller(t)
	ctx := context.Background()
	saved, err := svc.Upsert(ctx, u, Input{ProductID: 9, PriceSom: 6000, TradeType: "ulgurji"})
	if err != nil {
		t.Fatal(err)
	}
	l, err := svc.SetPricing(ctx, u, saved.ID, Pricing{Mode: PricingAuto, Goal: "profit", PriceMin: i64(5500), PriceMax: i64(7000)})
	if err != nil || l.PricingMode != PricingAuto || l.PriceMin.Int64 != 5500 {
		t.Fatalf("SetPricing: %+v %v", l, err)
	}
	// The database refuses auto mode without bounds even if the code forgot.
	if _, err := pool.Exec(ctx, `UPDATE listings SET price_min = NULL WHERE id = $1`, saved.ID); err == nil {
		t.Error("auto mode without a floor was accepted by the database")
	}
	// Changing the price by hand records when, so auto pricing waits.
	if _, err := svc.Upsert(ctx, u, Input{ProductID: 9, PriceSom: 6200, TradeType: "ulgurji"}); err != nil {
		t.Fatal(err)
	}
	var manual bool
	if err := pool.QueryRow(ctx, `SELECT manual_price_at > now() - interval '1 minute' FROM listings WHERE id = $1`, saved.ID).Scan(&manual); err != nil || !manual {
		t.Errorf("manual price time not recorded: %v %v", manual, err)
	}
	if _, err := svc.SetPricing(ctx, u, saved.ID, Pricing{Mode: PricingManual}); err != nil {
		t.Errorf("back to manual: %v", err)
	}
}
