package httpapi

import (
	"bytes"
	"context"
	"encoding/base64"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"bugun/internal/config"
	"bugun/internal/secure"
	"bugun/internal/store"
	"bugun/internal/store/db"
)

func TestWeeklyReportAccess(t *testing.T) {
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

	session := func(tgID int64, role string, approved bool, region int16) string {
		id, err := q.UpsertTelegramUser(ctx, db.UpsertTelegramUserParams{TelegramID: tgID, DisplayName: "Test " + role})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = q.DeleteUser(context.Background(), id) })
		if _, err := q.UpdateProfile(ctx, db.UpdateProfileParams{
			ID: id, Role: pgtype.Text{String: role, Valid: true}, RoleApproved: approved,
			RegionID: pgtype.Int2{Int16: region, Valid: true}, Categories: []string{},
		}); err != nil {
			t.Fatal(err)
		}
		token := secure.Token(24)
		if err := q.CreateSession(ctx, db.CreateSessionParams{TokenHash: secure.HashToken(token), UserID: id, ExpiresAt: time.Now().Add(time.Hour)}); err != nil {
			t.Fatal(err)
		}
		return token
	}
	get := func(token, query string) (*http.Response, []byte) {
		req, _ := http.NewRequest("GET", srv.URL+"/v1/reports/weekly"+query, nil)
		req.AddCookie(&http.Cookie{Name: sessionCookie, Value: token})
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		b, _ := io.ReadAll(res.Body)
		return res, b
	}

	staff := session(900000701, "market_admin", true, 2)
	res, body := get(staff, "?region=1")
	if res.StatusCode != 200 || res.Header.Get("Content-Type") != "application/pdf" || !bytes.HasPrefix(body, []byte("%PDF-")) {
		t.Fatalf("market staff report: %d %s %q", res.StatusCode, res.Header.Get("Content-Type"), body[:min(20, len(body))])
	}
	if cd := res.Header.Get("Content-Disposition"); cd == "" {
		t.Error("no file name")
	}

	for name, c := range map[string]struct {
		token, query string
		want         int
	}{
		"entrepreneur":                 {session(900000702, "entrepreneur", true, 2), "?region=1", 403},
		"unapproved market staff":      {session(900000703, "market_admin", false, 2), "", 403},
		"market staff, other viloyat":  {session(900000704, "market_admin", true, 2), "?region=100", 403},
		"market staff, own viloyat":    {session(900000705, "market_admin", true, 3), "", 200},
		"market staff, unknown region": {staff, "?region=9999", 422},
	} {
		if res, _ := get(c.token, c.query); res.StatusCode != c.want {
			t.Errorf("%s: status %d, want %d", name, res.StatusCode, c.want)
		}
	}
}
