package httpapi

import (
	"net"
	"net/http"
	"net/netip"
	"strings"
)

// defaultTrustedProxies: nginx on the same host.
var defaultTrustedProxies = []netip.Prefix{netip.MustParsePrefix("127.0.0.1/32"), netip.MustParsePrefix("::1/128")}

// realIP replaces chi's middleware.RealIP, which believes X-Forwarded-For from
// anyone: a client talking to the API directly could pick a new address for
// every request and never meet the rate limit. The headers are read only
// when the connection comes from a trusted proxy (TRUSTED_PROXIES); then
// r.RemoteAddr becomes the client's address, which the rate limiter and the
// logs use.
func realIP(trusted []netip.Prefix) func(http.Handler) http.Handler {
	isTrusted := func(a netip.Addr) bool {
		a = a.Unmap()
		for _, p := range trusted {
			if p.Contains(a) {
				return true
			}
		}
		return false
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if peer, ok := parseAddr(r.RemoteAddr); ok && isTrusted(peer) {
				if ip, ok := forwardedFor(r, isTrusted); ok {
					r.RemoteAddr = ip.String()
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

// forwardedFor is the client address a trusted proxy reported: X-Real-IP
// (nginx sets it to the address it saw), else the right-most address in
// X-Forwarded-For that is not one of our proxies. Entries left of that were
// written by the client and prove nothing.
func forwardedFor(r *http.Request, isTrusted func(netip.Addr) bool) (netip.Addr, bool) {
	if a, ok := parseAddr(r.Header.Get("X-Real-IP")); ok {
		return a, true
	}
	var hops []string
	for _, h := range r.Header.Values("X-Forwarded-For") {
		hops = append(hops, strings.Split(h, ",")...)
	}
	var last netip.Addr
	for i := len(hops) - 1; i >= 0; i-- {
		a, ok := parseAddr(hops[i])
		if !ok {
			break // a garbled hop: trust nothing further left
		}
		last = a
		if !isTrusted(a) {
			return a, true
		}
	}
	return last, last.IsValid()
}

// parseAddr reads an IP address with or without a port.
func parseAddr(s string) (netip.Addr, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return netip.Addr{}, false
	}
	if host, _, err := net.SplitHostPort(s); err == nil {
		s = host
	}
	a, err := netip.ParseAddr(strings.Trim(s, "[]"))
	if err != nil {
		return netip.Addr{}, false
	}
	return a.Unmap(), true
}
