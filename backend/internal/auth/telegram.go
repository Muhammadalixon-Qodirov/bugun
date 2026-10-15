// Package auth implements "Log in with Telegram" over OpenID Connect.
package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

// Issuer is Telegram's OpenID Connect provider.
const Issuer = "https://oauth.telegram.org"

// Scopes: identity, phone number, and permission for our bot to message the
// user, which links the bot to the profile without a separate step.
var scopes = []string{oidc.ScopeOpenID, "profile", "phone", "telegram:bot_access"}

var ErrNotConfigured = errors.New("telegram login is not configured")

type Identity struct {
	TelegramID int64
	Username   string
	Name       string
	Phone      string
}

type Telegram struct {
	clientID     string
	clientSecret string
	redirectURL  string
	issuer       string

	mu       sync.Mutex
	ready    bool
	oauth    oauth2.Config
	verifier *oidc.IDTokenVerifier
}

// Option changes a Telegram login's defaults.
type Option func(*Telegram)

// WithIssuer points the login at another OpenID Connect provider (tests).
func WithIssuer(url string) Option { return func(t *Telegram) { t.issuer = url } }

func NewTelegram(clientID, clientSecret, redirectURL string, opts ...Option) *Telegram {
	t := &Telegram{clientID: clientID, clientSecret: clientSecret, redirectURL: redirectURL, issuer: Issuer}
	for _, o := range opts {
		o(t)
	}
	return t
}

// init fetches the discovery document on first use, so the API starts even
// when Telegram is unreachable; a failed attempt is retried on the next call.
func (t *Telegram) init() error {
	if t.clientID == "" || t.clientSecret == "" {
		return ErrNotConfigured
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.ready {
		return nil
	}
	// The provider keeps its context for later JWKS refreshes, so it must not be
	// the request context.
	pctx := oidc.ClientContext(context.Background(), &http.Client{Timeout: 10 * time.Second})
	provider, err := oidc.NewProvider(pctx, t.issuer)
	if err != nil {
		return fmt.Errorf("telegram oidc discovery: %w", err)
	}
	ep := provider.Endpoint()
	ep.AuthStyle = oauth2.AuthStyleInHeader
	t.oauth = oauth2.Config{
		ClientID:     t.clientID,
		ClientSecret: t.clientSecret,
		RedirectURL:  t.redirectURL,
		Endpoint:     ep,
		Scopes:       scopes,
	}
	t.verifier = provider.Verifier(&oidc.Config{ClientID: t.clientID})
	t.ready = true
	return nil
}

// AuthURL returns the Telegram consent URL for a login attempt.
func (t *Telegram) AuthURL(ctx context.Context, state, nonce, verifier string) (string, error) {
	if err := t.init(); err != nil {
		return "", err
	}
	return t.oauth.AuthCodeURL(state,
		oauth2.S256ChallengeOption(verifier),
		oidc.Nonce(nonce),
		oauth2.SetAuthURLParam("lang", "uz"),
	), nil
}

// Exchange trades the code for tokens and returns the verified identity.
func (t *Telegram) Exchange(ctx context.Context, code, verifier, nonce string) (Identity, error) {
	if err := t.init(); err != nil {
		return Identity{}, err
	}
	tok, err := t.oauth.Exchange(ctx, code, oauth2.VerifierOption(verifier))
	if err != nil {
		return Identity{}, fmt.Errorf("exchange code: %w", err)
	}
	raw, ok := tok.Extra("id_token").(string)
	if !ok {
		return Identity{}, errors.New("no id_token in token response")
	}
	idt, err := t.verifier.Verify(ctx, raw)
	if err != nil {
		return Identity{}, fmt.Errorf("verify id_token: %w", err)
	}
	if idt.Nonce != nonce {
		return Identity{}, errors.New("id_token nonce mismatch")
	}

	var c struct {
		ID                json.Number `json:"id"`
		Name              string      `json:"name"`
		GivenName         string      `json:"given_name"`
		PreferredUsername string      `json:"preferred_username"`
		PhoneNumber       string      `json:"phone_number"`
	}
	if err := idt.Claims(&c); err != nil {
		return Identity{}, fmt.Errorf("decode claims: %w", err)
	}
	idStr := c.ID.String()
	if idStr == "" {
		idStr = idt.Subject
	}
	tgID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || tgID <= 0 {
		return Identity{}, fmt.Errorf("unexpected telegram id %q", idStr)
	}
	name := c.Name
	if name == "" {
		name = c.GivenName
	}
	if name == "" {
		name = c.PreferredUsername
	}
	if name == "" {
		name = "Foydalanuvchi"
	}
	return Identity{TelegramID: tgID, Username: c.PreferredUsername, Name: name, Phone: c.PhoneNumber}, nil
}
