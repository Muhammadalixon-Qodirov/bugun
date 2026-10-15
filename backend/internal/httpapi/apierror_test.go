package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/danielgtaylor/huma/v2"

	"bugun/internal/config"
)

func TestTranslateDetail(t *testing.T) {
	for _, tc := range []struct {
		msg, loc     string
		code, wantAt string
		params       map[string]any
	}{
		{"expected required property phone to be present", "body", "required", "body.phone", nil},
		{"expected length <= 60", "body.variety", "too_long", "body.variety", map[string]any{"max": 60.0}},
		{"expected number >= 50", "body.price_som", "too_small", "body.price_som", map[string]any{"min": 50.0}},
		{"expected number <= 50000000", "body.price_som", "too_large", "body.price_som", map[string]any{"max": 5e7}},
		{"expected boolean", "body.telegram", "invalid_type", "body.telegram", nil},
		{`expected value to be one of "ulgurji, chakana"`, "body.trade_type", "invalid_choice", "body.trade_type", nil},
		{"something new in a future Huma", "body.x", "invalid", "body.x", nil},
	} {
		got := translateDetail(&huma.ErrorDetail{Message: tc.msg, Location: tc.loc})
		if got.Code != tc.code || got.Location != tc.wantAt || got.Message == "" {
			t.Errorf("%q: got %s at %s (%q), want %s at %s", tc.msg, got.Code, got.Location, got.Message, tc.code, tc.wantAt)
		}
		for k, v := range tc.params {
			if got.Params[k] != v {
				t.Errorf("%q: params[%s] = %v, want %v", tc.msg, k, got.Params[k], v)
			}
		}
	}
}

// Every error the API sends, whoever raises it, carries a code.
func TestErrorsCarryCodes(t *testing.T) {
	h := New(config.Config{WebURL: "https://bugun.example.uz"}, nil, Deps{})
	for _, tc := range []struct {
		name, method, path, body string
		status                   int
		code                     string
		fieldCodes               []string
	}{
		{"huma validation", http.MethodPut, "/v1/me/contact", `{"telegram":"x"}`, 422, "validation_failed", []string{"required", "invalid_type"}},
		{"no session", http.MethodPut, "/v1/me/contact", `{"telegram":true,"phone":false}`, 401, "unauthorized", nil},
		{"unknown route", http.MethodGet, "/v1/nothing-here", "", 404, "not_found", nil},
		{"wrong method", http.MethodPatch, "/v1/catalog", "", 405, "method_not_allowed", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)

			var p APIError
			if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
				t.Fatalf("body is not JSON: %q", rec.Body.String())
			}
			if rec.Code != tc.status || p.Status != tc.status || p.Code != tc.code || p.Detail == "" {
				t.Fatalf("got %d %+v, want %d %s with a detail", rec.Code, p, tc.status, tc.code)
			}
			var codes []string
			for _, f := range p.Errors {
				codes = append(codes, f.Code)
			}
			if strings.Join(codes, ",") != strings.Join(tc.fieldCodes, ",") {
				t.Errorf("field codes %v, want %v", codes, tc.fieldCodes)
			}
		})
	}
}
