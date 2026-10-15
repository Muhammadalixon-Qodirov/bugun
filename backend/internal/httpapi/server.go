// Package httpapi wires HTTP routes, middleware and the OpenAPI description.
package httpapi

import (
	"cmp"
	"context"
	"crypto/subtle"
	"net/http"
	"strings"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/go-chi/httprate"
	"github.com/jackc/pgx/v5/pgxpool"

	"bugun/internal/auth"
	"bugun/internal/config"
	"bugun/internal/deals"
	"bugun/internal/errtrack"
	"bugun/internal/insight"
	"bugun/internal/market"
	"bugun/internal/media"
	"bugun/internal/pricing"
	"bugun/internal/rfq"
	"bugun/internal/secure"
	"bugun/internal/store/db"
	"bugun/internal/watch"
)

type Server struct {
	cfg      config.Config
	pool     *pgxpool.Pool
	q        *db.Queries
	tg       *auth.Telegram
	sealer   *secure.Sealer
	prices   *pricing.Service
	listings *market.Service
	rfq      *rfq.Service
	deals    *deals.Service
	watches  *watch.Service
	insight  *insight.Service
	media    media.Store
	ai       *AIDeps // optional model features (ai.go)
}

// Deps are the services the HTTP layer needs.
type Deps struct {
	Sealer   *secure.Sealer
	Prices   *pricing.Service
	Listings *market.Service
	RFQ      *rfq.Service
	Deals    *deals.Service // optional: without it no follow-up questions are scheduled
	Watches  *watch.Service
	Insight  *insight.Service
	Media    media.Store
	AI       *AIDeps        // optional: model features (speech, photos, analyst, matching, advice, usage)
	Telegram *auth.Telegram // optional: built from the config when nil
}

// NewAPI builds the Huma API description on top of a chi router.
func NewAPI(r chi.Router) huma.API {
	huma.DefaultArrayNullable = false // handlers always return [] rather than null
	huma.NewError = newError          // every error body carries a translatable code
	hc := huma.DefaultConfig("Bugun API", "0.1.0")
	hc.Info.Description = "O'zbekiston bozorlari narx va talab tahlili"
	hc.CreateHooks = nil // no $schema links in response bodies
	hc.Components.SecuritySchemes = map[string]*huma.SecurityScheme{
		"session": {Type: "apiKey", In: "cookie", Name: sessionCookie},
	}
	return humachi.New(r, hc)
}

// Defaults for settings a bare config (tests, the OpenAPI export) leaves at zero.
const (
	defaultRequestTimeout = 15 * time.Second
	defaultRateLimit      = 120 // requests a minute per visitor
)

func New(cfg config.Config, pool *pgxpool.Pool, d Deps) http.Handler {
	s := &Server{
		cfg:      cfg,
		pool:     pool,
		q:        db.New(pool),
		sealer:   d.Sealer,
		prices:   d.Prices,
		listings: d.Listings,
		rfq:      d.RFQ,
		deals:    d.Deals,
		watches:  d.Watches,
		insight:  d.Insight,
		media:    d.Media,
		ai:       d.AI,
		tg:       d.Telegram,
	}
	if s.tg == nil {
		s.tg = auth.NewTelegram(cfg.TelegramClientID, cfg.TelegramClientSecret, cfg.PublicAPIURL()+"/v1/auth/telegram/callback")
	}

	trusted := cfg.TrustedProxies
	if trusted == nil {
		trusted = defaultTrustedProxies
	}
	r := chi.NewRouter()
	r.Use(middleware.RequestID, realIP(trusted), s.requestLogger)
	// CORS sits on the router itself, ahead of routing: middleware added inside
	// a chi Group only wraps matched handlers, so preflight OPTIONS requests
	// (which have no handler) got a bare 405 and browsers reported a CORS error.
	// Being outermost also puts the headers on 429, 500 and timeout responses.
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   append([]string{cfg.WebURL}, cfg.CORSOrigins...),
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Content-Type"},
		ExposedHeaders:   []string{"Location"},
		AllowCredentials: true,
		MaxAge:           300,
	}))
	// errtrack reports panics and 5xx answers when SENTRY_DSN is set; it sits
	// inside the recoverer (it panics again) and outside the timeout (to see its 504).
	r.Use(middleware.Recoverer, errtrack.Middleware, middleware.Timeout(cmp.Or(cfg.RequestTimeout, defaultRequestTimeout)))

	// Photos are immutable, randomly named and cached for a year; a catalog page
	// loads dozens, so they are kept out of the per-visitor API rate limit.
	r.Get("/media/{key}", s.serveMedia)

	r.Group(func(r chi.Router) {
		r.Use(httprate.Limit(cmp.Or(cfg.RateLimitPerMinute, defaultRateLimit), time.Minute, httprate.WithKeyFuncs(s.clientKey),
			httprate.WithLimitHandler(func(w http.ResponseWriter, _ *http.Request) { writeError(w, http.StatusTooManyRequests) })))
		r.Use(s.withVisitor)
		s.register(NewAPI(r))
	})
	r.NotFound(func(w http.ResponseWriter, _ *http.Request) { writeError(w, http.StatusNotFound) })
	r.MethodNotAllowed(func(w http.ResponseWriter, _ *http.Request) { writeError(w, http.StatusMethodNotAllowed) })
	return r
}

func (s *Server) register(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "health",
		Method:      http.MethodGet,
		Path:        "/healthz",
		Summary:     "Service health",
		Description: "200 when the database answers; 503 with the same body (database: unavailable) when it does not, so load balancers and health checks see an outage.",
		Tags:        []string{"system"},
		Responses: map[string]*huma.Response{
			"503": {Description: "The database is unavailable", Content: map[string]*huma.MediaType{
				"application/json": {Schema: &huma.Schema{Ref: "#/components/schemas/HealthOutputBody"}},
			}},
		},
	}, s.health)
	s.registerMeta(api)
	s.registerCatalog(api)
	s.registerAuth(api)
	s.registerMe(api)
	s.registerPrices(api)
	s.registerListings(api)
	s.registerBrowse(api)
	s.registerFeed(api)
	s.registerSellers(api)
	s.registerAdmin(api)
	s.registerRFQ(api)
	s.registerFunnel(api)
	s.registerWatches(api)
	s.registerRatings(api)
	s.registerInsight(api)
	s.registerEvents(api)
	s.registerMonitoring(api)
	s.registerAnomalies(api)
	s.registerSuspicious(api)
	s.registerParser(api)
	s.registerPressure(api)
	s.registerReport(api)
	s.registerAI(api)
	s.registerChat(api)
}

type HealthOutput struct {
	Status int
	Body   struct {
		Status   string `json:"status" example:"ok" doc:"The API process answers; always ok"`
		Database string `json:"database" example:"ok" enum:"ok,unavailable" doc:"unavailable comes with HTTP 503"`
	}
}

// health answers 503 when the database does not, so Docker's HEALTHCHECK and
// the deploy script see a broken database instead of a cheerful 200.
func (s *Server) health(ctx context.Context, _ *struct{}) (*HealthOutput, error) {
	out := &HealthOutput{Status: http.StatusOK}
	out.Body.Status = "ok"
	out.Body.Database = "ok"
	ok := s.pool != nil
	if ok {
		ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()
		ok = s.pool.Ping(ctx) == nil
	}
	if !ok {
		out.Status = http.StatusServiceUnavailable
		out.Body.Database = "unavailable"
	}
	return out, nil
}

// Spec returns the OpenAPI document without starting a server or touching the database.
func Spec() ([]byte, error) {
	s := &Server{}
	api := NewAPI(chi.NewRouter())
	s.register(api)
	return api.OpenAPI().MarshalJSON()
}

// clientKey identifies the visitor for rate limiting. Requests from the site's
// own server carry the shared secret and the visitor's IP; everyone else is
// keyed by the connection address (the client's, as a trusted proxy reported
// it: see realIP).
func (s *Server) clientKey(r *http.Request) (string, error) {
	if s.via(r) == "site" {
		return "fwd:" + strings.TrimSpace(r.Header.Get("X-Client-IP")), nil
	}
	return httprate.KeyByIP(r)
}

// via tells where a request came from, for rate limiting and the access log:
// "site" (the website's server, secret verified), "site-unverified" (claims a
// visitor IP but the secret is missing or wrong, so all its visitors share one
// limit) or "direct".
func (s *Server) via(r *http.Request) string {
	if strings.TrimSpace(r.Header.Get("X-Client-IP")) == "" {
		return "direct"
	}
	if sec := s.cfg.InternalSecret; sec != "" &&
		subtle.ConstantTimeCompare([]byte(r.Header.Get("X-Internal-Secret")), []byte(sec)) == 1 {
		return "site"
	}
	return "site-unverified"
}
