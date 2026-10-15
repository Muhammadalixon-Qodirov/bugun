package httpapi

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"bugun/internal/auth"
	"bugun/internal/config"
	"bugun/internal/secure"
	"bugun/internal/store"
	"bugun/internal/store/db"
)

// Signing in from the Telegram Mini App: valid init data creates the user
// and the same session cookie as the website login; forged, stale or
// unconfigured sign-ins are refused.
func TestWebAppLogin(t *testing.T) {
	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	pool, err := store.Open(ctx, dbURL)
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
	const token = "123456:TEST-token-for-tests-only"
	const tgID = 900000671
	cfg := config.Config{WebURL: "https://bugun.example.com", SessionTTL: time.Hour, TelegramBotToken: token, TelegramWebAppMaxAge: time.Hour}
	srv := httptest.NewServer(New(cfg, pool, Deps{Sealer: sealer}))
	t.Cleanup(srv.Close)
	t.Cleanup(func() {
		if u, err := q.GetUserByTelegramID(context.Background(), tgID); err == nil {
			_ = q.DeleteUser(context.Background(), u.ID)
		}
	})

	initData := func(age time.Duration, id int64) string {
		return auth.SignInitData(token, url.Values{
			"user":      {`{"id":` + strconv.FormatInt(id, 10) + `,"first_name":"Test","last_name":"Patient","username":"test_patient"}`},
			"auth_date": {strconv.FormatInt(time.Now().Add(-age).Unix(), 10)},
			"query_id":  {"Q"},
		})
	}
	post := func(url, data, returnTo string) (*http.Response, map[string]any) {
		t.Helper()
		b, _ := json.Marshal(map[string]string{"init_data": data, "return_to": returnTo})
		res, err := http.Post(url+"/v1/auth/telegram/webapp", "application/json", bytes.NewReader(b))
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		var body map[string]any
		_ = json.NewDecoder(res.Body).Decode(&body)
		return res, body
	}
	sessionOf := func(res *http.Response) *http.Cookie {
		for _, c := range res.Cookies() {
			if c.Name == sessionCookie {
				return c
			}
		}
		return nil
	}

	// A new user: signed in, sent to finish the profile.
	res, body := post(srv.URL, initData(time.Minute, tgID), "/mahsulotlarim/yangi")
	c := sessionOf(res)
	if res.StatusCode != 200 || c == nil || !c.HttpOnly || !c.Secure || c.SameSite != http.SameSiteLaxMode || c.Value == "" {
		t.Fatalf("sign-in: %d %+v %v", res.StatusCode, c, body)
	}
	if body["next"] != "/boshlash" || body["needs_onboarding"] != true {
		t.Errorf("new user body = %v", body)
	}
	u, err := q.GetUserByTelegramID(ctx, tgID)
	if err != nil || u.DisplayName != "Test Patient" || u.TelegramUsername.String != "test_patient" {
		t.Fatalf("user = %+v, %v", u, err)
	}
	if uid, err := q.GetSessionUserID(ctx, secure.HashToken(c.Value)); err != nil || uid != u.ID {
		t.Errorf("session belongs to %v (%v), want %v", uid, err, u.ID)
	}

	// With a role the Mini App goes where it asked; a phone the website
	// login stored stays.
	if _, err := q.UpdateProfile(ctx, db.UpdateProfileParams{ID: u.ID, Role: pgtype.Text{String: "entrepreneur", Valid: true}, RoleApproved: true,
		RegionID: pgtype.Int2{Int16: 2, Valid: true}, Categories: []string{"sabzavot"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE users SET phone_hash = $2 WHERE id = $1`, u.ID, sealer.Hash("+998900000000")); err != nil {
		t.Fatal(err)
	}
	res, body = post(srv.URL, initData(time.Minute, tgID), "/mahsulotlarim/yangi")
	if res.StatusCode != 200 || body["next"] != "/mahsulotlarim/yangi" || body["needs_onboarding"] != false {
		t.Errorf("seller sign-in: %d %v", res.StatusCode, body)
	}
	if u2, _ := q.GetUserByTelegramID(ctx, tgID); u2.PhoneHash == nil {
		t.Error("the Mini App sign-in dropped the stored phone")
	}
	if _, body = post(srv.URL, initData(time.Minute, tgID), "https://evil.example.com"); body["next"] != "/" {
		t.Errorf("foreign return_to kept: %v", body["next"])
	}

	// Refused: forged, stale, empty.
	forged := auth.SignInitData("654321:someone-else", url.Values{"user": {`{"id":900000672}`}, "auth_date": {strconv.FormatInt(time.Now().Unix(), 10)}})
	for name, data := range map[string]string{"forged": forged, "stale": initData(2*time.Hour, tgID), "junk": "hash=00"} {
		res, body := post(srv.URL, data, "")
		if res.StatusCode != http.StatusUnauthorized || body["code"] != "invalid_init_data" || sessionOf(res) != nil {
			t.Errorf("%s: %d %v", name, res.StatusCode, body)
		}
	}
	if res, _ := post(srv.URL, "", ""); res.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("empty init data: %d, want 422", res.StatusCode)
	}
	if _, err := q.GetUserByTelegramID(ctx, 900000672); err == nil {
		t.Error("a forged sign-in created a user")
	}

	// Without a bot token nothing can be checked.
	cfg.TelegramBotToken = ""
	off := httptest.NewServer(New(cfg, pool, Deps{Sealer: sealer}))
	defer off.Close()
	if res, body := post(off.URL, initData(time.Minute, tgID), ""); res.StatusCode != http.StatusServiceUnavailable || body["code"] != "telegram_not_configured" {
		t.Errorf("unconfigured: %d %v", res.StatusCode, body)
	}
}
