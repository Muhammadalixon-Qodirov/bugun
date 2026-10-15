package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"bugun/internal/config"
)

// Browsers send a preflight before PUT/DELETE and before JSON POSTs; it must
// be answered with CORS headers for the allowed site and refused for others.
func TestPreflightAllowsSiteOrigin(t *testing.T) {
	h := New(config.Config{WebURL: "https://bugun.example.uz", CORSOrigins: []string{"https://preview.example.uz"}}, nil, Deps{})

	for _, tc := range []struct {
		origin  string
		allowed bool
	}{
		{"https://bugun.example.uz", true},
		{"https://preview.example.uz", true},
		{"https://evil.example.com", false},
	} {
		for _, path := range []string{"/v1/me/profile", "/v1/listings/1", "/v1/catalog"} {
			req := httptest.NewRequest(http.MethodOptions, path, nil)
			req.Header.Set("Origin", tc.origin)
			req.Header.Set("Access-Control-Request-Method", http.MethodPut)
			req.Header.Set("Access-Control-Request-Headers", "content-type")
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)

			got := rec.Header().Get("Access-Control-Allow-Origin")
			if tc.allowed {
				if rec.Code >= 300 || got != tc.origin || rec.Header().Get("Access-Control-Allow-Credentials") != "true" {
					t.Errorf("%s from %s: status %d, allow-origin %q; want 2xx with the origin and credentials", path, tc.origin, rec.Code, got)
				}
			} else if got != "" {
				t.Errorf("%s from %s: allow-origin %q; want none", path, tc.origin, got)
			}
		}
	}
}
