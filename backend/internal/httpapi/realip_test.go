package httpapi

import (
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"

	"bugun/internal/config"
)

func TestRealIPOnlyBelievesTrustedProxies(t *testing.T) {
	trusted := []netip.Prefix{netip.MustParsePrefix("127.0.0.1/32"), netip.MustParsePrefix("::1/128"), netip.MustParsePrefix("10.1.0.0/16")}
	s := &Server{cfg: config.Config{}}
	for _, tc := range []struct {
		name, peer, realIP, xff, want string
	}{
		{"direct client spoofs XFF", "203.0.113.9:5555", "", "1.2.3.4", "203.0.113.9"},
		{"direct client spoofs X-Real-IP", "203.0.113.9:5555", "1.2.3.4", "", "203.0.113.9"},
		{"nginx sets X-Real-IP", "127.0.0.1:40000", "198.51.100.7", "6.6.6.6, 198.51.100.7", "198.51.100.7"},
		{"nginx appends to XFF", "127.0.0.1:40000", "", "198.51.100.7", "198.51.100.7"},
		{"client-written XFF entries are skipped", "127.0.0.1:40000", "", "6.6.6.6, 198.51.100.7", "198.51.100.7"},
		{"chain of trusted proxies", "[::1]:40000", "", "198.51.100.7, 10.1.2.3", "198.51.100.7"},
		{"garbage in XFF", "127.0.0.1:40000", "", "not-an-ip", "127.0.0.1"},
		{"garbage between hops stops the walk", "127.0.0.1:40000", "", "6.6.6.6, junk, 10.1.2.3", "10.1.2.3"},
		{"IPv4-mapped peer", "[::ffff:127.0.0.1]:40000", "", "198.51.100.7", "198.51.100.7"},
		{"trusted proxy without headers", "127.0.0.1:40000", "", "", "127.0.0.1"},
	} {
		var got string
		h := realIP(trusted)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
			got, _ = s.clientKey(r)
		}))
		r := httptest.NewRequest("GET", "/", nil)
		r.RemoteAddr = tc.peer
		if tc.realIP != "" {
			r.Header.Set("X-Real-IP", tc.realIP)
		}
		if tc.xff != "" {
			r.Header.Set("X-Forwarded-For", tc.xff)
		}
		h.ServeHTTP(httptest.NewRecorder(), r)
		if got != tc.want {
			t.Errorf("%s: rate limit key = %q, want %q", tc.name, got, tc.want)
		}
	}
}

// A client that reaches the API directly cannot dodge the rate limit by
// sending a new X-Forwarded-For with every request.
func TestRateLimitIgnoresSpoofedHeaders(t *testing.T) {
	h := New(config.Config{WebURL: "http://localhost:3000", RateLimitPerMinute: 3}, nil, Deps{})
	codes := map[int]int{}
	for i := range 6 {
		r := httptest.NewRequest("GET", "/healthz", nil)
		r.RemoteAddr = "203.0.113.9:5555"
		r.Header.Set("X-Forwarded-For", "10.0.0."+string(rune('1'+i)))
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		codes[w.Code]++
	}
	if codes[http.StatusTooManyRequests] != 3 {
		t.Errorf("status counts %v, want 3 requests over the limit", codes)
	}
}
