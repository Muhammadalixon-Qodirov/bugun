package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"bugun/internal/auth"
	"bugun/internal/secure"
	"bugun/internal/store/db"
)

const loginTTL = 10 * time.Minute

func (s *Server) registerAuth(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID:   "telegramLoginStart",
		Method:        http.MethodGet,
		Path:          "/v1/auth/telegram/start",
		Summary:       "Redirect to Telegram login",
		Tags:          []string{"auth"},
		DefaultStatus: http.StatusFound,
	}, s.loginStart)

	huma.Register(api, huma.Operation{
		OperationID:   "telegramLoginCallback",
		Method:        http.MethodGet,
		Path:          "/v1/auth/telegram/callback",
		Summary:       "Telegram login callback",
		Tags:          []string{"auth"},
		DefaultStatus: http.StatusFound,
	}, s.loginCallback)

	huma.Register(api, huma.Operation{
		OperationID:   "logout",
		Method:        http.MethodPost,
		Path:          "/v1/auth/logout",
		Summary:       "End the current session",
		Tags:          []string{"auth"},
		DefaultStatus: http.StatusNoContent,
	}, s.logout)

	s.registerWebAppAuth(api)
}

type RedirectOutput struct {
	Location  string      `header:"Location"`
	SetCookie http.Cookie `header:"Set-Cookie"`
}

type LoginStartInput struct {
	ReturnTo string `query:"return_to" maxLength:"200" doc:"Site path to open after login"`
}

func (s *Server) loginStart(ctx context.Context, in *LoginStartInput) (*RedirectOutput, error) {
	state, nonce, verifier := secure.Token(32), secure.Token(16), secure.Token(48)
	authURL, err := s.tg.AuthURL(ctx, state, nonce, verifier)
	if errors.Is(err, auth.ErrNotConfigured) {
		return s.loginFailed("sozlanmagan"), nil
	}
	if err != nil {
		slog.ErrorContext(ctx, "telegram auth url", "err", err)
		return s.loginFailed("telegram"), nil
	}
	err = s.q.CreateOAuthState(ctx, db.CreateOAuthStateParams{
		StateHash:    secure.HashToken(state),
		CodeVerifier: verifier,
		Nonce:        nonce,
		ReturnTo:     safeReturnTo(in.ReturnTo),
		ExpiresAt:    time.Now().Add(loginTTL),
	})
	if err != nil {
		return nil, internalError(ctx, "save oauth state", err)
	}
	return &RedirectOutput{Location: authURL}, nil
}

type LoginCallbackInput struct {
	Code  string `query:"code" maxLength:"2048"`
	State string `query:"state" maxLength:"256"`
	Error string `query:"error" maxLength:"256"`
}

func (s *Server) loginCallback(ctx context.Context, in *LoginCallbackInput) (*RedirectOutput, error) {
	if in.Error != "" || in.Code == "" || in.State == "" {
		return s.loginFailed("bekor"), nil
	}
	st, err := s.q.ConsumeOAuthState(ctx, secure.HashToken(in.State))
	if errors.Is(err, pgx.ErrNoRows) {
		return s.loginFailed("muddat"), nil
	}
	if err != nil {
		return nil, internalError(ctx, "consume oauth state", err)
	}

	id, err := s.tg.Exchange(ctx, in.Code, st.CodeVerifier, st.Nonce)
	if err != nil {
		slog.ErrorContext(ctx, "telegram exchange", "err", err)
		return s.loginFailed("telegram"), nil
	}

	params := db.UpsertTelegramUserParams{
		TelegramID:       id.TelegramID,
		TelegramUsername: pgtype.Text{String: id.Username, Valid: id.Username != ""},
		DisplayName:      id.Name,
	}
	if phone := normalizePhone(id.Phone); phone != "" {
		params.PhoneEnc = s.sealer.Seal(phone)
		params.PhoneHash = s.sealer.Hash(phone)
	}
	userID, err := s.q.UpsertTelegramUser(ctx, params)
	if err != nil {
		return nil, internalError(ctx, "upsert user", err)
	}

	token := secure.Token(32)
	expires := time.Now().Add(s.cfg.SessionTTL)
	if err := s.q.CreateSession(ctx, db.CreateSessionParams{
		TokenHash: secure.HashToken(token), UserID: userID, ExpiresAt: expires,
	}); err != nil {
		return nil, internalError(ctx, "create session", err)
	}
	slog.InfoContext(ctx, "user signed in", "user_id", userID)

	u, err := s.q.GetUser(ctx, userID)
	if err != nil {
		return nil, internalError(ctx, "get user", err)
	}
	next := st.ReturnTo
	if !u.Role.Valid {
		next = "/boshlash"
	}
	return &RedirectOutput{Location: s.cfg.WebURL + next, SetCookie: s.sessionCookie(token, expires)}, nil
}

type LogoutOutput struct {
	SetCookie http.Cookie `header:"Set-Cookie"`
}

func (s *Server) logout(ctx context.Context, in *SessionInput) (*LogoutOutput, error) {
	if in.Session != "" {
		if err := s.q.DeleteSession(ctx, secure.HashToken(in.Session)); err != nil {
			return nil, internalError(ctx, "delete session", err)
		}
	}
	return &LogoutOutput{SetCookie: s.clearedSessionCookie()}, nil
}

func (s *Server) loginFailed(reason string) *RedirectOutput {
	return &RedirectOutput{Location: s.cfg.WebURL + "/kirish?xato=" + url.QueryEscape(reason)}
}

// safeReturnTo only allows local paths, so the login cannot redirect to another site.
func safeReturnTo(p string) string {
	if !strings.HasPrefix(p, "/") || strings.HasPrefix(p, "//") || strings.Contains(p, "\\") {
		return "/"
	}
	return p
}

// normalizePhone keeps digits and a leading plus sign.
func normalizePhone(p string) string {
	var b strings.Builder
	for i, r := range strings.TrimSpace(p) {
		if (r >= '0' && r <= '9') || (r == '+' && i == 0) {
			b.WriteRune(r)
		}
	}
	out := b.String()
	if out != "" && !strings.HasPrefix(out, "+") {
		out = "+" + out
	}
	if len(out) < 8 {
		return ""
	}
	return out
}
