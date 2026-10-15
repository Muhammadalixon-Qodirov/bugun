package auth

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"net/url"
	"strings"
	"testing"

	"bugun/internal/auth/authtest"
)

const callback = "http://localhost:3000/api/v1/auth/telegram/callback"

func start(t *testing.T) (*authtest.Provider, *Telegram) {
	t.Helper()
	p := authtest.New("client-1", "secret-1")
	t.Cleanup(p.Close)
	return p, NewTelegram("client-1", "secret-1", callback, WithIssuer(p.URL))
}

func TestNotConfigured(t *testing.T) {
	tg := NewTelegram("", "", callback)
	if _, err := tg.AuthURL(context.Background(), "s", "n", "v"); !errors.Is(err, ErrNotConfigured) {
		t.Errorf("AuthURL without a client: %v", err)
	}
	if _, err := tg.Exchange(context.Background(), "c", "v", "n"); !errors.Is(err, ErrNotConfigured) {
		t.Errorf("Exchange without a client: %v", err)
	}
}

func TestAuthURLCarriesStateNonceAndPKCE(t *testing.T) {
	p, tg := start(t)
	raw, err := tg.AuthURL(context.Background(), "state-1", "nonce-1", "verifier-verifier-verifier-verifier-verifier")
	if err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(raw)
	q := u.Query()
	sum := sha256.Sum256([]byte("verifier-verifier-verifier-verifier-verifier"))
	for k, want := range map[string]string{
		"client_id": "client-1", "redirect_uri": callback, "response_type": "code", "state": "state-1", "nonce": "nonce-1",
		"code_challenge_method": "S256", "code_challenge": base64.RawURLEncoding.EncodeToString(sum[:]), "lang": "uz",
	} {
		if got := q.Get(k); got != want {
			t.Errorf("%s = %q, want %q", k, got, want)
		}
	}
	if !strings.HasPrefix(raw, p.URL+"/auth?") || !strings.Contains(q.Get("scope"), "openid") || !strings.Contains(q.Get("scope"), "phone") {
		t.Errorf("auth URL %s", raw)
	}
}

// login runs the browser's round trip and returns the code the provider
// sent back.
func login(t *testing.T, p *authtest.Provider, tg *Telegram, verifier, nonce string) string {
	t.Helper()
	raw, err := tg.AuthURL(context.Background(), "state", nonce, verifier)
	if err != nil {
		t.Fatal(err)
	}
	back, err := p.Authorize(raw, authtest.User{ID: 777000123, Name: "Test User", Username: "test_user", Phone: "998900000000"})
	if err != nil {
		t.Fatal(err)
	}
	return back.Query().Get("code")
}

const verifier = "a-verifier-long-enough-for-pkce-0123456789abcdef"

func TestExchange(t *testing.T) {
	p, tg := start(t)
	code := login(t, p, tg, verifier, "nonce-ok")
	id, err := tg.Exchange(context.Background(), code, verifier, "nonce-ok")
	if err != nil {
		t.Fatal(err)
	}
	if id != (Identity{TelegramID: 777000123, Username: "test_user", Name: "Test User", Phone: "998900000000"}) {
		t.Errorf("identity %+v", id)
	}
	// A code works once.
	if _, err := tg.Exchange(context.Background(), code, verifier, "nonce-ok"); err == nil {
		t.Error("a used code was accepted again")
	}
}

func TestExchangeRejects(t *testing.T) {
	ctx := context.Background()
	t.Run("wrong PKCE verifier", func(t *testing.T) {
		p, tg := start(t)
		code := login(t, p, tg, verifier, "n")
		if _, err := tg.Exchange(ctx, code, "another-verifier-of-the-same-sort-0123456789", "n"); err == nil {
			t.Error("accepted")
		}
	})
	t.Run("nonce mismatch", func(t *testing.T) {
		p, tg := start(t)
		code := login(t, p, tg, verifier, "n")
		if _, err := tg.Exchange(ctx, code, verifier, "other-nonce"); err == nil || !strings.Contains(err.Error(), "nonce") {
			t.Errorf("got %v, want a nonce error", err)
		}
	})
	t.Run("replayed id_token nonce", func(t *testing.T) {
		p, tg := start(t)
		p.Nonce = "stolen"
		code := login(t, p, tg, verifier, "n")
		if _, err := tg.Exchange(ctx, code, verifier, "n"); err == nil {
			t.Error("accepted")
		}
	})
	t.Run("token for another client", func(t *testing.T) {
		p, tg := start(t)
		p.Audience = "someone-else"
		code := login(t, p, tg, verifier, "n")
		if _, err := tg.Exchange(ctx, code, verifier, "n"); err == nil {
			t.Error("accepted")
		}
	})
	t.Run("forged signature", func(t *testing.T) {
		p, tg := start(t)
		p.SignWith = authtest.OtherKey()
		code := login(t, p, tg, verifier, "n")
		if _, err := tg.Exchange(ctx, code, verifier, "n"); err == nil {
			t.Error("accepted")
		}
	})
	t.Run("wrong client secret", func(t *testing.T) {
		p, _ := start(t)
		bad := NewTelegram("client-1", "not-the-secret", callback, WithIssuer(p.URL))
		code := login(t, p, bad, verifier, "n")
		if _, err := bad.Exchange(ctx, code, verifier, "n"); err == nil {
			t.Error("accepted")
		}
	})
	t.Run("unknown code", func(t *testing.T) {
		_, tg := start(t)
		if _, err := tg.Exchange(ctx, "made-up", verifier, "n"); err == nil {
			t.Error("accepted")
		}
	})
}

func TestProviderDown(t *testing.T) {
	p := authtest.New("client-1", "secret-1")
	tg := NewTelegram("client-1", "secret-1", callback, WithIssuer(p.URL))
	p.Close()
	if _, err := tg.AuthURL(context.Background(), "s", "n", verifier); err == nil || errors.Is(err, ErrNotConfigured) {
		t.Fatalf("AuthURL with the provider down: %v, want a discovery error", err)
	}
}
