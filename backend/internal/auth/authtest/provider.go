// Package authtest runs a fake OpenID Connect provider for tests of the
// Telegram login: discovery, an authorize step, a token endpoint that checks
// the client secret and PKCE, and RS256-signed id_tokens.
package authtest

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"time"
)

// User is who signs in at the provider.
type User struct {
	ID       int64
	Name     string
	Username string
	Phone    string
}

// Provider is the fake. Tweak its fields to produce broken tokens.
type Provider struct {
	*httptest.Server
	ClientID, ClientSecret string

	// Knobs for failure cases, read when a token is issued.
	Nonce    string          // overrides the nonce in id_tokens
	Audience string          // overrides the aud claim
	SignWith *rsa.PrivateKey // signs with another key than the published one

	key   *rsa.PrivateKey
	mu    sync.Mutex
	codes map[string]grant
	used  map[string]bool
}

type grant struct {
	user                         User
	challenge, nonce, redirectTo string
}

// New starts a provider; Close it when done.
func New(clientID, clientSecret string) *Provider {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		panic(err)
	}
	p := &Provider{ClientID: clientID, ClientSecret: clientSecret, key: key, codes: map[string]grant{}, used: map[string]bool{}}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /.well-known/openid-configuration", p.discovery)
	mux.HandleFunc("GET /jwks", p.jwks)
	mux.HandleFunc("POST /token", p.token)
	p.Server = httptest.NewServer(mux)
	return p
}

func (p *Provider) discovery(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, map[string]any{
		"issuer":                                p.URL,
		"authorization_endpoint":                p.URL + "/auth",
		"token_endpoint":                        p.URL + "/token",
		"jwks_uri":                              p.URL + "/jwks",
		"id_token_signing_alg_values_supported": []string{"RS256"},
	})
}

func (p *Provider) jwks(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, map[string]any{"keys": []map[string]string{{
		"kty": "RSA", "kid": "k1", "alg": "RS256", "use": "sig",
		"n": b64(p.key.N.Bytes()), "e": b64(big.NewInt(int64(p.key.E)).Bytes()),
	}}})
}

// Authorize plays the user approving the login at authURL: it checks the
// request like Telegram would and returns the redirect back to the site
// (redirect_uri with code and state).
func (p *Provider) Authorize(authURL string, u User) (*url.URL, error) {
	a, err := url.Parse(authURL)
	if err != nil {
		return nil, err
	}
	q := a.Query()
	if q.Get("client_id") != p.ClientID || q.Get("response_type") != "code" || q.Get("code_challenge_method") != "S256" ||
		q.Get("code_challenge") == "" || q.Get("nonce") == "" || q.Get("state") == "" || !strings.Contains(q.Get("scope"), "openid") {
		return nil, &url.Error{Op: "authorize", URL: authURL, Err: errBadRequest}
	}
	code := b64(randomBytes(16))
	p.mu.Lock()
	p.codes[code] = grant{user: u, challenge: q.Get("code_challenge"), nonce: q.Get("nonce"), redirectTo: q.Get("redirect_uri")}
	p.mu.Unlock()
	back, err := url.Parse(q.Get("redirect_uri"))
	if err != nil {
		return nil, err
	}
	bq := back.Query()
	bq.Set("code", code)
	bq.Set("state", q.Get("state"))
	back.RawQuery = bq.Encode()
	return back, nil
}

func (p *Provider) token(w http.ResponseWriter, r *http.Request) {
	id, secret, ok := r.BasicAuth()
	if !ok || id != p.ClientID || secret != p.ClientSecret {
		oauthError(w, http.StatusUnauthorized, "invalid_client")
		return
	}
	if err := r.ParseForm(); err != nil || r.PostForm.Get("grant_type") != "authorization_code" {
		oauthError(w, http.StatusBadRequest, "invalid_request")
		return
	}
	code := r.PostForm.Get("code")
	p.mu.Lock()
	g, found := p.codes[code]
	reused := p.used[code]
	p.used[code] = true
	p.mu.Unlock()
	sum := sha256.Sum256([]byte(r.PostForm.Get("code_verifier")))
	if !found || reused || b64(sum[:]) != g.challenge || r.PostForm.Get("redirect_uri") != g.redirectTo {
		oauthError(w, http.StatusBadRequest, "invalid_grant")
		return
	}
	nonce, aud, signer := g.nonce, p.ClientID, p.key
	if p.Nonce != "" {
		nonce = p.Nonce
	}
	if p.Audience != "" {
		aud = p.Audience
	}
	if p.SignWith != nil {
		signer = p.SignWith
	}
	now := time.Now()
	claims := map[string]any{
		"iss": p.URL, "aud": aud, "sub": itoa(g.user.ID), "id": g.user.ID, "nonce": nonce,
		"iat": now.Unix(), "exp": now.Add(5 * time.Minute).Unix(),
		"name": g.user.Name, "preferred_username": g.user.Username, "phone_number": g.user.Phone,
	}
	writeJSON(w, map[string]any{"access_token": "at-" + code, "token_type": "Bearer", "expires_in": 300, "id_token": sign(signer, claims)})
}

// sign makes a compact RS256 JWS.
func sign(key *rsa.PrivateKey, claims map[string]any) string {
	header, _ := json.Marshal(map[string]string{"alg": "RS256", "kid": "k1", "typ": "JWT"})
	payload, _ := json.Marshal(claims)
	input := b64(header) + "." + b64(payload)
	sum := sha256.Sum256([]byte(input))
	sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, sum[:])
	if err != nil {
		panic(err)
	}
	return input + "." + b64(sig)
}

// OtherKey is a key the provider does not publish, for SignWith.
func OtherKey() *rsa.PrivateKey {
	k, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		panic(err)
	}
	return k
}

type constError string

func (e constError) Error() string { return string(e) }

const errBadRequest = constError("bad authorize request")

func oauthError(w http.ResponseWriter, status int, code string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": code})
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func b64(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }

func randomBytes(n int) []byte {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return b
}

func itoa(n int64) string { return big.NewInt(n).String() }
