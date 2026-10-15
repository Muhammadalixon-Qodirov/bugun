// Package errtrack reports errors to Sentry when SENTRY_DSN is set: panics
// and 5xx answers of the API, and failed scheduled jobs. Without a DSN every
// function here does nothing.
//
// Personal data never leaves: cookies, Authorization and forwarding headers,
// request bodies and the user's IP are dropped, and phone numbers and bot
// tokens in any text are masked before an event is sent (Scrub).
package errtrack

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"sync/atomic"
	"time"

	"github.com/getsentry/sentry-go"
	sentryhttp "github.com/getsentry/sentry-go/http"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// Options configure the reporting (config: SENTRY_*).
type Options struct {
	DSN              string
	Environment      string
	Release          string
	TracesSampleRate float64 // 0 sends no performance traces
	// Transport replaces the network transport (tests).
	Transport sentry.Transport
}

var enabled atomic.Bool

// Init starts reporting; with an empty DSN it does nothing and reports false.
func Init(o Options) (bool, error) {
	if o.DSN == "" {
		return false, nil
	}
	err := sentry.Init(sentry.ClientOptions{
		Dsn:              o.DSN,
		Environment:      o.Environment,
		Release:          o.Release,
		EnableTracing:    o.TracesSampleRate > 0,
		TracesSampleRate: o.TracesSampleRate,
		SendDefaultPII:   false,
		AttachStacktrace: true,
		Transport:        o.Transport,
		BeforeSend: func(e *sentry.Event, _ *sentry.EventHint) *sentry.Event {
			return Scrub(e)
		},
		BeforeSendTransaction: func(e *sentry.Event, _ *sentry.EventHint) *sentry.Event {
			return Scrub(e)
		},
		BeforeBreadcrumb: func(b *sentry.Breadcrumb, _ *sentry.BreadcrumbHint) *sentry.Breadcrumb {
			b.Message = mask(b.Message)
			for k, v := range b.Data {
				if s, ok := v.(string); ok {
					b.Data[k] = mask(s)
				}
			}
			return b
		},
	})
	if err != nil {
		return false, fmt.Errorf("sentry: %w", err)
	}
	enabled.Store(true)
	return true, nil
}

// Enabled reports whether Init turned reporting on.
func Enabled() bool { return enabled.Load() }

// Flush waits up to timeout for queued events, at shutdown.
func Flush(timeout time.Duration) {
	if Enabled() {
		sentry.Flush(timeout)
	}
}

// Middleware reports panics (and panics again, for the recoverer outside it)
// and 5xx answers that CaptureError has not reported already.
func Middleware(next http.Handler) http.Handler {
	if !Enabled() {
		return next
	}
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen := new(atomic.Bool)
		r = r.WithContext(context.WithValue(r.Context(), capturedKey{}, seen))
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(ww, r)
		if ww.Status() >= 500 && !seen.Load() {
			route := r.URL.Path
			if rc := chi.RouteContext(r.Context()); rc != nil && rc.RoutePattern() != "" {
				route = rc.RoutePattern()
			}
			hub := hubOf(r.Context())
			hub.WithScope(func(s *sentry.Scope) {
				s.SetTag("http.status", fmt.Sprint(ww.Status()))
				s.SetTag("route", route)
				hub.CaptureMessage(fmt.Sprintf("HTTP %d %s %s", ww.Status(), r.Method, route))
			})
		}
	})
	return sentryhttp.New(sentryhttp.Options{Repanic: true}).Handle(inner)
}

type capturedKey struct{}

func hubOf(ctx context.Context) *sentry.Hub {
	if h := sentry.GetHubFromContext(ctx); h != nil {
		return h
	}
	return sentry.CurrentHub()
}

// CaptureError reports an error a handler turned into a 5xx answer; op says
// what failed.
func CaptureError(ctx context.Context, op string, err error) {
	if !Enabled() || err == nil {
		return
	}
	if seen, ok := ctx.Value(capturedKey{}).(*atomic.Bool); ok {
		seen.Store(true)
	}
	hub := hubOf(ctx)
	hub.WithScope(func(s *sentry.Scope) {
		s.SetTag("op", op)
		hub.CaptureException(fmt.Errorf("%s: %w", op, err))
	})
}

// Job wraps a scheduled job so a failed or panicking run is reported.
func Job(name string, run func(context.Context) (int, error)) func(context.Context) (int, error) {
	if !Enabled() {
		return run
	}
	return func(ctx context.Context) (n int, err error) {
		hub := sentry.CurrentHub().Clone()
		hub.Scope().SetTag("job", name)
		defer func() {
			if p := recover(); p != nil {
				hub.Recover(p)
				hub.Flush(2 * time.Second)
				panic(p)
			}
		}()
		n, err = run(ctx)
		if err != nil && ctx.Err() == nil { // a run cut off by shutdown is no failure
			hub.CaptureException(fmt.Errorf("job %s: %w", name, err))
		}
		return n, err
	}
}

var (
	// Bot tokens first: their digits would otherwise pass for a phone number.
	tokenRe = regexp.MustCompile(`\d{6,12}:[A-Za-z0-9_-]{30,}`)
	// Phone numbers: 9 or more digits, spaces, dashes and brackets between.
	phoneRe = regexp.MustCompile(`\+?\d(?:[\s()-]*\d){8,}`)
	// Headers that carry credentials or who the visitor is.
	dropHeaders = []string{"authorization", "cookie", "set-cookie", "x-internal-secret", "x-client-ip",
		"x-forwarded-for", "x-real-ip", "forwarded", "cf-connecting-ip", "true-client-ip"}
)

func mask(s string) string {
	s = tokenRe.ReplaceAllString(s, "[token]")
	return phoneRe.ReplaceAllString(s, "[phone]")
}

// Scrub removes personal data and credentials from an event.
func Scrub(e *sentry.Event) *sentry.Event {
	if e == nil {
		return nil
	}
	e.User = sentry.User{ID: e.User.ID}
	e.ServerName = ""
	e.Message = mask(e.Message)
	for i := range e.Exception {
		e.Exception[i].Value = mask(e.Exception[i].Value)
	}
	for _, b := range e.Breadcrumbs {
		b.Message = mask(b.Message)
	}
	for k, v := range e.Tags {
		e.Tags[k] = mask(v)
	}
	if r := e.Request; r != nil {
		r.Cookies = ""
		r.Data = ""
		r.Env = nil
		r.QueryString = mask(r.QueryString)
		r.URL = mask(r.URL)
		for k := range r.Headers {
			for _, d := range dropHeaders {
				if strings.EqualFold(k, d) {
					delete(r.Headers, k)
				}
			}
		}
	}
	return e
}
