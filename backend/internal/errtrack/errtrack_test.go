package errtrack

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/getsentry/sentry-go"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func TestDisabledWithoutDSN(t *testing.T) {
	if on, err := Init(Options{}); on || err != nil || Enabled() {
		t.Fatalf("Init without DSN = %v, %v", on, err)
	}
	h := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
	if got := Middleware(h); got == nil {
		t.Fatal("nil middleware")
	}
	run := func(context.Context) (int, error) { return 1, nil }
	if n, _ := Job("x", run)(context.Background()); n != 1 {
		t.Error("Job changed the run")
	}
	CaptureError(context.Background(), "op", errors.New("x")) // must not panic
}

func TestReporting(t *testing.T) {
	tr := &sentry.MockTransport{}
	if on, err := Init(Options{DSN: "https://public@sentry.example.com/1", Environment: "test", Transport: tr}); !on || err != nil {
		t.Fatalf("Init = %v, %v", on, err)
	}
	defer enabled.Store(false)

	r := chi.NewRouter()
	r.Use(middleware.Recoverer, Middleware)
	r.Get("/boom/{id}", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	})
	r.Get("/handled", func(w http.ResponseWriter, r *http.Request) {
		CaptureError(r.Context(), "save listing", errors.New("seller +998 90 000 00 00 failed"))
		w.WriteHeader(http.StatusInternalServerError)
	})
	r.Get("/panic", func(http.ResponseWriter, *http.Request) { panic("kaboom") })
	r.Get("/ok", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNotFound) })

	do := func(path string) int {
		req := httptest.NewRequest("GET", path+"?phone=998900000000", nil)
		req.Header.Set("Authorization", "Bearer secret")
		req.Header.Set("Cookie", "bugun_session=abc")
		req.Header.Set("X-Internal-Secret", "s")
		req.Header.Set("User-Agent", "test")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		return rec.Code
	}

	for path, want := range map[string]int{"/ok": 404, "/boom/7": 502, "/handled": 500, "/panic": 500} {
		if got := do(path); got != want {
			t.Errorf("%s: status %d, want %d", path, got, want)
		}
	}
	sentry.Flush(0)
	events := tr.Events()
	if len(events) != 3 {
		t.Fatalf("%d events, want 3 (502, the handled error once, the panic)", len(events))
	}
	var all strings.Builder
	for _, e := range events {
		all.WriteString(e.Message)
		for _, x := range e.Exception {
			all.WriteString(" " + x.Value)
		}
		if e.Request != nil {
			all.WriteString(" " + e.Request.QueryString + " " + e.Request.Cookies)
			for k, v := range e.Request.Headers {
				all.WriteString(" " + k + "=" + v)
			}
		}
	}
	text := all.String()
	for _, leak := range []string{"secret", "bugun_session", "abc", "000 00 00", "998900000000", "Authorization", "X-Internal-Secret"} {
		if strings.Contains(text, leak) {
			t.Errorf("event leaks %q: %s", leak, text)
		}
	}
	for _, want := range []string{"HTTP 502 GET /boom/{id}", "save listing: seller [phone] failed", "kaboom"} {
		if !strings.Contains(text, want) {
			t.Errorf("events miss %q: %s", want, text)
		}
	}
}

func TestJobReporting(t *testing.T) {
	tr := &sentry.MockTransport{}
	if _, err := Init(Options{DSN: "https://public@sentry.example.com/1", Transport: tr}); err != nil {
		t.Fatal(err)
	}
	defer enabled.Store(false)

	failing := Job("fx_daily", func(context.Context) (int, error) { return 0, errors.New("cbu down") })
	if _, err := failing(context.Background()); err == nil {
		t.Fatal("the error was swallowed")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _ = Job("fx_daily", func(ctx context.Context) (int, error) { return 0, ctx.Err() })(ctx)
	func() {
		defer func() {
			if recover() == nil {
				t.Error("the panic was swallowed")
			}
		}()
		_, _ = Job("weather", func(context.Context) (int, error) { panic("nil map") })(context.Background())
	}()
	sentry.Flush(0)
	events := tr.Events()
	if len(events) != 2 {
		t.Fatalf("%d events, want 2 (the failure and the panic, not the shutdown)", len(events))
	}
	if events[0].Tags["job"] != "fx_daily" || !strings.Contains(events[0].Exception[len(events[0].Exception)-1].Value, "cbu down") {
		t.Errorf("job event = %+v", events[0])
	}
}

func TestMask(t *testing.T) {
	for in, want := range map[string]string{
		"call +998 90 123-45-67 now":                           "call [phone] now",
		"(90) 123 45 67":                                       "([phone]",
		"listing 12345 at 2026-09-26":                          "listing 12345 at 2026-09-26",
		"token 123456789:AAHdF6IQAAAAAN0XohDhrOrcAAHdF6IQAAAA": "token [token]",
	} {
		if got := mask(in); got != want {
			t.Errorf("mask(%q) = %q, want %q", in, got, want)
		}
	}
}
