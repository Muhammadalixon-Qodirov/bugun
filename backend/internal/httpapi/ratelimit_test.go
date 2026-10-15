package httpapi

import (
	"net/http/httptest"
	"testing"

	"bugun/internal/config"
)

func TestClientKey(t *testing.T) {
	s := &Server{cfg: config.Config{InternalSecret: "s3cret"}}
	req := func(secret, ip string) string {
		r := httptest.NewRequest("GET", "/", nil)
		r.RemoteAddr = "10.0.0.1:1234"
		if secret != "" {
			r.Header.Set("X-Internal-Secret", secret)
		}
		r.Header.Set("X-Client-IP", ip)
		k, err := s.clientKey(r)
		if err != nil {
			t.Fatal(err)
		}
		return k
	}
	if k := req("s3cret", "203.0.113.7"); k != "fwd:203.0.113.7" {
		t.Errorf("trusted forward: key = %q", k)
	}
	if k := req("", "203.0.113.7"); k == "fwd:203.0.113.7" {
		t.Error("client IP accepted without the secret")
	}
	if k := req("wrong", "203.0.113.7"); k == "fwd:203.0.113.7" {
		t.Error("client IP accepted with a wrong secret")
	}
	for _, tc := range []struct{ secret, ip, want string }{
		{"s3cret", "203.0.113.7", "site"},
		{"wrong", "203.0.113.7", "site-unverified"},
		{"", "203.0.113.7", "site-unverified"},
		{"s3cret", "", "direct"},
	} {
		r := httptest.NewRequest("GET", "/", nil)
		r.Header.Set("X-Internal-Secret", tc.secret)
		r.Header.Set("X-Client-IP", tc.ip)
		if got := s.via(r); got != tc.want {
			t.Errorf("via(secret %q, ip %q) = %q, want %q", tc.secret, tc.ip, got, tc.want)
		}
	}
	s.cfg.InternalSecret = ""
	if k := req("", "203.0.113.7"); k == "fwd:203.0.113.7" {
		t.Error("forwarding must be off when no secret is configured")
	}
}
