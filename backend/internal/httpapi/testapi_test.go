package httpapi

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"bugun/internal/config"
	"bugun/internal/deals"
	"bugun/internal/insight"
	"bugun/internal/market"
	"bugun/internal/media"
	"bugun/internal/secure"
	"bugun/internal/store"
	"bugun/internal/store/db"
	"bugun/internal/watch"
)

// testAPI is the HTTP API on a real database (TEST_DATABASE_URL) with
// helpers to sign in and call it.
type testAPI struct {
	t        *testing.T
	srv      *httptest.Server
	pool     *pgxpool.Pool
	q        *db.Queries
	listings *market.Service
}

func newTestAPI(t *testing.T) *testAPI { t.Helper(); return newTestAPIWithMedia(t, nil) }

// newTestAPIWithMedia is newTestAPI with a photo store.
func newTestAPIWithMedia(t *testing.T, photos media.Store) *testAPI {
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
	sealer, err := secure.NewSealer(base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{7}, 32)))
	if err != nil {
		t.Fatal(err)
	}
	listings := market.NewService(pool)
	srv := httptest.NewServer(New(config.Config{WebURL: "http://localhost:3000"}, pool, Deps{Sealer: sealer, Listings: listings, Deals: deals.NewService(pool), Watches: watch.NewService(pool), Insight: insight.NewService(pool), Media: photos}))
	t.Cleanup(srv.Close)
	return &testAPI{t: t, srv: srv, pool: pool, q: db.New(pool), listings: listings}
}

// signIn creates a user with a role in a region and a live session.
func (a *testAPI) signIn(tgID int64, role string, region int16) (db.User, string) {
	a.t.Helper()
	ctx := context.Background()
	id, err := a.q.UpsertTelegramUser(ctx, db.UpsertTelegramUserParams{TelegramID: tgID, DisplayName: "Test " + role})
	if err != nil {
		a.t.Fatal(err)
	}
	a.t.Cleanup(func() { _ = a.q.DeleteUser(context.Background(), id) })
	u, err := a.q.UpdateProfile(ctx, db.UpdateProfileParams{
		ID: id, Role: pgtype.Text{String: role, Valid: true}, RoleApproved: true,
		RegionID: pgtype.Int2{Int16: region, Valid: true}, Categories: []string{"sabzavot"},
	})
	if err != nil {
		a.t.Fatal(err)
	}
	token := secure.Token(24)
	if err := a.q.CreateSession(ctx, db.CreateSessionParams{TokenHash: secure.HashToken(token), UserID: id, ExpiresAt: time.Now().Add(time.Hour)}); err != nil {
		a.t.Fatal(err)
	}
	return u, token
}

// call sends a request with the session (none when token is "") and decodes
// a successful JSON answer into out.
func (a *testAPI) call(token, method, path string, body, out any) int {
	a.t.Helper()
	var rd io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	}
	req, _ := http.NewRequest(method, a.srv.URL+path, rd)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.AddCookie(&http.Cookie{Name: sessionCookie, Value: token})
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		a.t.Fatal(err)
	}
	defer res.Body.Close()
	if out != nil && res.StatusCode < 300 {
		if err := json.NewDecoder(res.Body).Decode(out); err != nil {
			a.t.Fatalf("%s %s: decode: %v", method, path, err)
		}
	} else {
		_, _ = io.Copy(io.Discard, res.Body)
	}
	return res.StatusCode
}
