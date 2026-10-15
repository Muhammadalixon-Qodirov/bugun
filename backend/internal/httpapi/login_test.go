package httpapi

import (
	"bytes"
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"bugun/internal/auth"
	"bugun/internal/auth/authtest"
	"bugun/internal/config"
	"bugun/internal/secure"
	"bugun/internal/store"
	"bugun/internal/store/db"
)

// The whole "Log in with Telegram" round trip against a fake provider:
// start, the provider's redirect back, the session cookie, and the ways a
// callback is refused.
func TestTelegramLoginFlow(t *testing.T) {
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

	const web = "http://localhost:3000"
	provider := authtest.New("client-1", "secret-1")
	t.Cleanup(provider.Close)
	var h http.Handler
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { h.ServeHTTP(w, r) }))
	t.Cleanup(srv.Close)
	tg := auth.NewTelegram("client-1", "secret-1", srv.URL+"/v1/auth/telegram/callback", auth.WithIssuer(provider.URL))
	h = New(config.Config{WebURL: web, SessionTTL: time.Hour}, pool, Deps{Sealer: sealer, Telegram: tg})

	const tgID = 900000651
	user := authtest.User{ID: tgID, Name: "Test Patient", Username: "test_patient", Phone: "+998 90 000 00 00"}
	t.Cleanup(func() {
		if u, err := q.GetUserByTelegramID(context.Background(), tgID); err == nil {
			_ = q.DeleteUser(context.Background(), u.ID)
		}
	})

	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	get := func(u string) *http.Response {
		t.Helper()
		res, err := client.Get(u)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		return res
	}
	session := func(res *http.Response) string {
		for _, c := range res.Cookies() {
			if c.Name == sessionCookie && c.Value != "" {
				return c.Value
			}
		}
		return ""
	}
	// begin starts a login and returns the provider's redirect back to us.
	begin := func(returnTo string) *url.URL {
		t.Helper()
		res := get(srv.URL + "/v1/auth/telegram/start?return_to=" + url.QueryEscape(returnTo))
		if res.StatusCode != http.StatusFound || !strings.HasPrefix(res.Header.Get("Location"), provider.URL+"/auth?") {
			t.Fatalf("start: %d %s", res.StatusCode, res.Header.Get("Location"))
		}
		back, err := provider.Authorize(res.Header.Get("Location"), user)
		if err != nil {
			t.Fatal(err)
		}
		return back
	}
	failed := func(name string, res *http.Response, reason string) {
		t.Helper()
		if res.StatusCode != http.StatusFound || res.Header.Get("Location") != web+"/kirish?xato="+reason || session(res) != "" {
			t.Errorf("%s: %d %s (cookie %v), want a redirect to /kirish?xato=%s without a session", name, res.StatusCode, res.Header.Get("Location"), session(res) != "", reason)
		}
	}

	// A first login: new users finish sign-up first, whatever return_to says.
	back := begin("/katalog")
	res := get(back.String())
	token := session(res)
	if res.StatusCode != http.StatusFound || res.Header.Get("Location") != web+"/boshlash" || token == "" {
		t.Fatalf("callback: %d %s, session %q", res.StatusCode, res.Header.Get("Location"), token)
	}
	u, err := q.GetUserByTelegramID(ctx, tgID)
	if err != nil || u.DisplayName != "Test Patient" || len(u.PhoneHash) == 0 {
		t.Fatalf("user after login: %+v %v", u, err)
	}
	req, _ := http.NewRequest("GET", srv.URL+"/v1/me", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookie, Value: token})
	if me, err := client.Do(req); err != nil || me.StatusCode != http.StatusOK {
		t.Fatalf("/v1/me with the new session: %v %v", me, err)
	} else {
		me.Body.Close()
	}

	// The same callback again: the state was used up.
	failed("replayed callback", get(back.String()), "muddat")
	// A state the API never issued.
	forged := *back
	fq := forged.Query()
	fq.Set("state", "not-a-state-we-issued")
	forged.RawQuery = fq.Encode()
	failed("unknown state", get(forged.String()), "muddat")
	// The user cancelled at Telegram.
	failed("cancelled", get(srv.URL+"/v1/auth/telegram/callback?error=access_denied&state=x"), "bekor")
	failed("no code", get(srv.URL+"/v1/auth/telegram/callback?state=x"), "bekor")
	// A code from one login with the state of another: PKCE does not match.
	a, b := begin("/"), begin("/")
	mixed := *a
	mq := mixed.Query()
	mq.Set("state", b.Query().Get("state"))
	mixed.RawQuery = mq.Encode()
	failed("code and state from different logins", get(mixed.String()), "telegram")

	// A registered user goes back where they were, but only to a local path.
	if _, err := q.UpdateProfile(ctx, db.UpdateProfileParams{
		ID: u.ID, Role: pgtype.Text{String: "entrepreneur", Valid: true}, RoleApproved: true,
		RegionID: pgtype.Int2{Int16: 2, Valid: true}, Categories: []string{"sabzavot"},
	}); err != nil {
		t.Fatal(err)
	}
	for returnTo, want := range map[string]string{
		"/mahsulot/piyoz":      web + "/mahsulot/piyoz",
		"//evil.example/x":     web + "/",
		"https://evil.example": web + "/",
		"/\\evil.example":      web + "/",
	} {
		res := get(begin(returnTo).String())
		if res.StatusCode != http.StatusFound || res.Header.Get("Location") != want || session(res) == "" {
			t.Errorf("return_to %q: %d %s, want %s with a session", returnTo, res.StatusCode, res.Header.Get("Location"), want)
		}
	}
}
