package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"bugun/internal/secure"
	"bugun/internal/store/db"
)

const sessionCookie = "bugun_session"

var authRequired = []map[string][]string{{"session": {}}}

// SessionInput is embedded in inputs of operations that need a signed-in user.
type SessionInput struct {
	Session string `cookie:"bugun_session" doc:"Session cookie set after Telegram login"`
}

func (s *Server) sessionCookie(token string, expires time.Time) http.Cookie {
	return http.Cookie{
		Name:     sessionCookie,
		Value:    token,
		Path:     "/",
		Domain:   s.cfg.CookieDomain,
		Expires:  expires,
		HttpOnly: true,
		Secure:   strings.HasPrefix(s.cfg.WebURL, "https://"),
		SameSite: http.SameSiteLaxMode,
	}
}

func (s *Server) clearedSessionCookie() http.Cookie {
	c := s.sessionCookie("", time.Unix(0, 0))
	c.MaxAge = -1
	return c
}

// currentUser resolves the session cookie to a user or returns 401.
func (s *Server) currentUser(ctx context.Context, token string) (db.User, error) {
	if token == "" {
		return db.User{}, errUnauthorized()
	}
	uid, err := s.q.GetSessionUserID(ctx, secure.HashToken(token))
	if errors.Is(err, pgx.ErrNoRows) {
		return db.User{}, errUnauthorized()
	}
	if err != nil {
		return db.User{}, internalError(ctx, "get session", err)
	}
	u, err := s.q.GetUser(ctx, uid)
	if errors.Is(err, pgx.ErrNoRows) {
		return db.User{}, errUnauthorized()
	}
	if err != nil {
		return db.User{}, internalError(ctx, "get user", err)
	}
	return u, nil
}

func errUnauthorized() error {
	return apiError(http.StatusUnauthorized, "unauthorized", "Avval Telegram orqali kiring.")
}
