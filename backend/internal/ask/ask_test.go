package ask

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"

	"bugun/internal/pricing"
	"bugun/internal/store"
	"bugun/internal/store/db"
)

func TestIntentOf(t *testing.T) {
	cases := map[string]Intent{
		"namanganda piyoz qancha":            IntentPrice,
		"piyoz narxi":                        IntentPrice,
		"eng arzon kartoshka kimda":          IntentCheapest,
		"kartoshkani qayerdan olsam bo'ladi": IntentCheapest,
		"qaysi tumanda piyoz arzon":          IntentAreas,
		"piyoz qaysi viloyatda arzonroq":     IntentAreas,
	}
	for q, want := range cases {
		if got := intentOf(pricing.Normalize(q)); got != want {
			t.Errorf("intentOf(%q) = %s, want %s", q, got, want)
		}
	}
}

func setup(t *testing.T) (*Service, *db.Queries, context.Context) {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	pool, err := store.Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err := store.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	prices, err := pricing.NewService(ctx, pool)
	if err != nil {
		t.Fatal(err)
	}
	q := db.New(pool)
	s, err := NewService(ctx, q, prices)
	if err != nil {
		t.Fatal(err)
	}
	return s, q, ctx
}

func TestFindRegion(t *testing.T) {
	s, _, _ := setup(t)
	cases := map[string]string{
		"namanganda piyoz qancha":     "Namangan viloyati",
		"namangan shahrida piyoz":     "Namangan shahri",
		"namangan tumanida kartoshka": "Namangan tumani",
		"chortoqdagi pomidor":         "Chortoq",
		"toshkentda go'sht narxi":     "Toshkent shahri",
		"toshkent viloyatida un":      "Toshkent viloyati",
		"farg'onada olma":             "Farg'ona viloyati",
		"uchqo'rg'onda sabzi":         "Uchqo'rg'on",
		"samarqandda piyoz qancha":    "Samarqand viloyati",
		"urgutda eng arzon kartoshka": "Urgut",
		"piyoz qancha":                "",
	}
	for q, want := range cases {
		got := s.findRegion(pricing.Normalize(q))
		switch {
		case want == "" && got != nil:
			t.Errorf("%q: region %s, want none", q, got.Name)
		case want != "" && (got == nil || got.Name != want):
			t.Errorf("%q: region %v, want %s", q, got, want)
		}
	}
}

func TestAsk(t *testing.T) {
	s, q, ctx := setup(t)
	// Four sellers of onions (product 2) in Chortoq, Namangan.
	for i, price := range []int64{1800, 1900, 2000, 2100} {
		id, err := q.UpsertTelegramUser(ctx, db.UpsertTelegramUserParams{TelegramID: int64(900000601 + i), DisplayName: "Test Sotuvchi"})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = q.DeleteUser(context.Background(), id) })
		u, err := q.UpdateProfile(ctx, db.UpdateProfileParams{
			ID: id, Role: pgtype.Text{String: "entrepreneur", Valid: true}, RoleApproved: true,
			RegionID: pgtype.Int2{Int16: 3, Valid: true}, Categories: []string{"sabzavot"},
		})
		if err != nil {
			t.Fatal(err)
		}
		d, err := s.prices.CreateDraft(ctx, u, "piyoz "+pricing.FormatSom(price)+" optom", pricing.ViaWeb)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := s.prices.Confirm(ctx, u, d.ID); err != nil {
			t.Fatal(err)
		}
	}

	a, err := s.Ask(ctx, nil, "Chortoqda piyoz qancha?")
	if err != nil {
		t.Fatal(err)
	}
	if a.Product.Name != "Piyoz" || a.Region == nil || a.Region.Name != "Chortoq" || !a.Stats.Enough || a.Stats.Offers < 4 {
		t.Errorf("price answer: %+v", a)
	}
	a, err = s.Ask(ctx, nil, "eng arzon piyoz kimda, Chortoqda")
	if err != nil || a.Intent != IntentCheapest || len(a.Cheapest) == 0 || a.Cheapest[0].PriceSom != 1800 {
		t.Errorf("cheapest answer: %+v %v", a.Cheapest, err)
	}
	a, err = s.Ask(ctx, nil, "Namangan viloyatida qaysi tumanda piyoz arzon?")
	if err != nil || a.Intent != IntentAreas || len(a.Areas) == 0 || a.Areas[0].Name != "Chortoq" {
		t.Errorf("areas answer: %+v %v", a.Areas, err)
	}
	if _, err := s.Ask(ctx, nil, "ob-havo qanday?"); !errors.Is(err, ErrNoProduct) {
		t.Errorf("question without a product: %v, want ErrNoProduct", err)
	}
}
