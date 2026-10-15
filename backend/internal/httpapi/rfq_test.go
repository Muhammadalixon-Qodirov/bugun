package httpapi

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"bugun/internal/config"
	"bugun/internal/market"
	"bugun/internal/rfq"
	"bugun/internal/secure"
	"bugun/internal/store"
	"bugun/internal/store/db"
)

// TestRFQEndpoints walks the whole RFQ flow through HTTP against a real
// database (TEST_DATABASE_URL): request, inbox, offer, pick, confirm, contact.
func TestRFQEndpoints(t *testing.T) {
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
	q := db.New(pool)
	sealer, err := secure.NewSealer(base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{7}, 32)))
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(New(config.Config{WebURL: "http://localhost:3000"}, pool, Deps{
		Sealer: sealer, Listings: market.NewService(pool), RFQ: rfq.NewService(pool, nil),
	}))
	t.Cleanup(srv.Close)

	// signIn creates a user with a live session and returns its cookie value.
	signIn := func(tgID int64, username, role string, region int16) (db.User, string) {
		id, err := q.UpsertTelegramUser(ctx, db.UpsertTelegramUserParams{
			TelegramID: tgID, DisplayName: "Test " + role, TelegramUsername: pgtype.Text{String: username, Valid: true},
		})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = q.DeleteUser(context.Background(), id) })
		u, err := q.UpdateProfile(ctx, db.UpdateProfileParams{
			ID: id, Role: pgtype.Text{String: role, Valid: true}, RoleApproved: true,
			RegionID: pgtype.Int2{Int16: region, Valid: true}, Categories: []string{"sabzavot"},
		})
		if err != nil {
			t.Fatal(err)
		}
		token := secure.Token(24)
		if err := q.CreateSession(ctx, db.CreateSessionParams{TokenHash: secure.HashToken(token), UserID: id, ExpiresAt: time.Now().Add(time.Hour)}); err != nil {
			t.Fatal(err)
		}
		return u, token
	}
	call := func(token, method, path string, body any, out any) int {
		t.Helper()
		var rd io.Reader
		if body != nil {
			b, _ := json.Marshal(body)
			rd = bytes.NewReader(b)
		}
		req, _ := http.NewRequest(method, srv.URL+path, rd)
		req.Header.Set("Content-Type", "application/json")
		req.AddCookie(&http.Cookie{Name: sessionCookie, Value: token})
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		if out != nil && res.StatusCode < 300 {
			if err := json.NewDecoder(res.Body).Decode(out); err != nil {
				t.Fatalf("%s %s: decode: %v", method, path, err)
			}
		}
		return res.StatusCode
	}

	_, buyerTok := signIn(900000401, "test_xaridor", "entrepreneur", 2)
	seller, sellerTok := signIn(900000402, "test_sotuvchi", "entrepreneur", 3)
	if _, err := market.NewService(pool).Upsert(ctx, seller, market.Input{ProductID: 1, PriceSom: 3300, TradeType: "ulgurji"}); err != nil {
		t.Fatal(err)
	}

	var created CreateRFQOutput
	if code := call(buyerTok, "POST", "/v1/rfqs", map[string]any{"product_id": 1, "quantity": 200}, &created.Body); code != 200 {
		t.Fatalf("create RFQ: status %d", code)
	}
	if created.Body.Sellers != 1 || created.Body.RFQ.Status != "open" || created.Body.RFQ.Unit != "kg" {
		t.Errorf("created: %+v", created.Body)
	}
	id := created.Body.RFQ.ID
	// Market staff may neither ask nor answer.
	_, staffTok := signIn(900000403, "test_bozor", "market_admin", 2)
	if code := call(staffTok, "POST", "/v1/rfqs", map[string]any{"product_id": 1, "quantity": 200}, nil); code != 403 {
		t.Errorf("market staff creating an RFQ: status %d, want 403", code)
	}
	if code := call(staffTok, "GET", "/v1/me/rfq-inbox", nil, nil); code != 403 {
		t.Errorf("market staff inbox: status %d, want 403", code)
	}
	// The seller is an entrepreneur too and may ask; their own request stays
	// out of their inbox and cannot be answered by them.
	var ownRFQ CreateRFQOutput
	if code := call(sellerTok, "POST", "/v1/rfqs", map[string]any{"product_id": 1, "quantity": 30}, &ownRFQ.Body); code != 200 {
		t.Fatalf("entrepreneur creating an RFQ: status %d, want 200", code)
	}
	if ownRFQ.Body.Sellers != 0 {
		t.Errorf("own RFQ reached %d sellers, want 0", ownRFQ.Body.Sellers)
	}
	if code := call(sellerTok, "POST", fmt.Sprintf("/v1/rfqs/%d/offer", ownRFQ.Body.RFQ.ID), map[string]any{"price_som": 3100}, nil); code != 404 {
		t.Errorf("offer on one's own RFQ: status %d, want 404", code)
	}

	var inbox InboxOutput
	call(sellerTok, "GET", "/v1/me/rfq-inbox", nil, &inbox.Body)
	if len(inbox.Body.RFQs) != 1 || inbox.Body.RFQs[0].ID != id || inbox.Body.RFQs[0].OfferStatus != "" {
		t.Fatalf("inbox: %+v", inbox.Body.RFQs)
	}
	var offer OfferState
	if code := call(sellerTok, "POST", fmt.Sprintf("/v1/rfqs/%d/offer", id), map[string]any{"price_som": 3100}, &offer.Body); code != 200 || offer.Body.Status != "sent" {
		t.Fatalf("offer: status %d %+v", code, offer.Body)
	}

	var detail RFQDetailOutput
	call(buyerTok, "GET", fmt.Sprintf("/v1/rfqs/%d", id), nil, &detail.Body)
	if len(detail.Body.Offers) != 1 || detail.Body.Offers[0].PriceSom != 3100 {
		t.Fatalf("detail offers: %+v", detail.Body.Offers)
	}
	if code := call(buyerTok, "GET", fmt.Sprintf("/v1/offers/%d/contact", offer.Body.ID), nil, nil); code != 409 {
		t.Errorf("contact before agreement: status %d, want 409", code)
	}
	if code := call(buyerTok, "POST", fmt.Sprintf("/v1/offers/%d/choose", offer.Body.ID), nil, nil); code != 200 {
		t.Fatalf("choose: status %d", code)
	}
	if code := call(sellerTok, "POST", fmt.Sprintf("/v1/offers/%d/answer", offer.Body.ID), map[string]any{"agree": true}, nil); code != 200 {
		t.Fatalf("answer: status %d", code)
	}
	var contact OfferContactOutput
	if code := call(buyerTok, "GET", fmt.Sprintf("/v1/offers/%d/contact", offer.Body.ID), nil, &contact.Body); code != 200 || contact.Body.TelegramURL != "https://t.me/test_sotuvchi" {
		t.Errorf("buyer sees seller contact: status %d %+v", code, contact.Body)
	}
	if code := call(sellerTok, "GET", fmt.Sprintf("/v1/offers/%d/contact", offer.Body.ID), nil, &contact.Body); code != 200 || contact.Body.TelegramURL != "https://t.me/test_xaridor" {
		t.Errorf("seller sees buyer contact: status %d %+v", code, contact.Body)
	}

	var mine MyRFQsOutput
	call(buyerTok, "GET", "/v1/me/rfqs", nil, &mine.Body)
	if len(mine.Body.RFQs) != 1 || mine.Body.RFQs[0].Offers != 1 || mine.Body.RFQs[0].BestPrice != 3100 {
		t.Errorf("my RFQs: %+v", mine.Body.RFQs)
	}
	if code := call(buyerTok, "POST", fmt.Sprintf("/v1/rfqs/%d/cancel", id), nil, nil); code != 204 && code != 200 {
		t.Errorf("cancel: status %d", code)
	}
}

// Views, searches, contact reveals and RFQs are counted once per visitor,
// product and day, without storing who or which IP.
func TestDemandSignals(t *testing.T) {
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
	sealer, _ := secure.NewSealer(base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{7}, 32)))
	srv := httptest.NewServer(New(config.Config{WebURL: "http://localhost:3000"}, pool, Deps{
		Sealer: sealer, Listings: market.NewService(pool), RFQ: rfq.NewService(pool, nil),
	}))
	t.Cleanup(srv.Close)
	// Product 6 is used only here, so counts are not disturbed by other tests.
	if _, err := pool.Exec(ctx, `DELETE FROM demand_events WHERE product_id = 6`); err != nil {
		t.Fatal(err)
	}
	var slug string
	if err := pool.QueryRow(ctx, `SELECT slug FROM products WHERE id = 6`).Scan(&slug); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ { // the same visitor three times counts once
		res, err := http.Get(srv.URL + "/v1/products/" + slug + "?region=2")
		if err != nil || res.StatusCode != 200 {
			t.Fatalf("product page: %v %v", err, res)
		}
		res.Body.Close()
	}
	res, err := http.Get(srv.URL + "/v1/products?q=" + slug)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()

	counts := map[string]int{}
	rows, err := pool.Query(ctx, `SELECT kind, count(*), bool_and(province_id = 1) FROM demand_events WHERE product_id = 6 GROUP BY kind`)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var kind string
		var n int
		var inNamangan *bool
		if err := rows.Scan(&kind, &n, &inNamangan); err != nil {
			t.Fatal(err)
		}
		counts[kind] = n
		if kind == "view" && (inNamangan == nil || !*inNamangan) {
			t.Error("view not attributed to the Namangan viloyat")
		}
	}
	if counts["view"] != 1 || counts["search"] != 1 {
		t.Errorf("demand counts %v, want one view and one search", counts)
	}
}

// Four quiet weeks and a busy one with the same sellers: pressure 2, shortage.
func TestPressureAndMap(t *testing.T) {
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
	q := db.New(pool)
	sealer, _ := secure.NewSealer(base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{7}, 32)))
	srv := httptest.NewServer(New(config.Config{WebURL: "http://localhost:3000"}, pool, Deps{Sealer: sealer}))
	t.Cleanup(srv.Close)

	const product = 9 // used only here
	var slug string
	if err := pool.QueryRow(ctx, `SELECT slug FROM products WHERE id = $1`, product).Scan(&slug); err != nil {
		t.Fatal(err)
	}
	_, _ = pool.Exec(ctx, `DELETE FROM demand_events WHERE product_id = $1`, product)
	_, _ = pool.Exec(ctx, `DELETE FROM price_observations WHERE product_id = $1`, product)
	// Demand: 10 views a week for four weeks, 20 in the last one (Namangan).
	for week, views := range []int{20, 10, 10, 10, 10} {
		for i := 0; i < views; i++ {
			if _, err := pool.Exec(ctx, `INSERT INTO demand_events (kind, product_id, province_id, viewer_key, created_at, day)
				VALUES ('view', $1, 1, $2, now() - make_interval(days => $3), current_date - $3)`,
				product, []byte(fmt.Sprintf("w%d-%d", week, i)), week*7+3); err != nil {
				t.Fatal(err)
			}
		}
	}
	// Supply: the same two sellers live the whole time.
	for i := 0; i < 2; i++ {
		id, err := q.UpsertTelegramUser(ctx, db.UpsertTelegramUserParams{TelegramID: int64(900000501 + i), DisplayName: "Test Sotuvchi"})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = q.DeleteUser(context.Background(), id) })
		if _, err := pool.Exec(ctx, `INSERT INTO price_observations (product_id, price_som, unit, trade_type, region_id, source, seller_id, observed_at, valid_until)
			VALUES ($1, 5000, 'kg', 'ulgurji', 3, 'seller', $2, now() - interval '40 days', now() + interval '3 days')`, product, id); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM price_observations WHERE product_id = $1`, product)
	})

	var pr Pressure
	res, err := http.Get(srv.URL + "/v1/products/" + slug + "/pressure?region=2")
	if err != nil {
		t.Fatal(err)
	}
	_ = json.NewDecoder(res.Body).Decode(&pr)
	res.Body.Close()
	if !pr.Enough || pr.Value != 2 || pr.Zone != "shortage" || pr.SellersNow != 2 || pr.DemandAvg != 10 {
		t.Errorf("pressure = %+v, want 2 (shortage) with 2 sellers and usual demand 10", pr)
	}

	var fc struct {
		Features []struct {
			Properties map[string]any `json:"properties"`
		} `json:"features"`
	}
	res, err = http.Get(srv.URL + "/v1/map/provinces?product=" + slug)
	if err != nil {
		t.Fatal(err)
	}
	_ = json.NewDecoder(res.Body).Decode(&fc)
	res.Body.Close()
	if len(fc.Features) != 14 {
		t.Fatalf("map has %d viloyatlar, want 14", len(fc.Features))
	}
	for _, f := range fc.Features {
		if f.Properties["name"] == "Namangan viloyati" && f.Properties["zone"] != "shortage" {
			t.Errorf("Namangan on the map: %v, want zone shortage", f.Properties)
		}
	}
}
