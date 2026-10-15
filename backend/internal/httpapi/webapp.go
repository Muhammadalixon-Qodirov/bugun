package httpapi

import (
	"cmp"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5/pgtype"

	"bugun/internal/auth"
	"bugun/internal/secure"
	"bugun/internal/store/db"
)

// defaultWebAppMaxAge applies when the config leaves TELEGRAM_WEBAPP_MAX_AGE at zero (tests).
const defaultWebAppMaxAge = time.Hour

func (s *Server) registerWebAppAuth(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "telegramWebAppLogin",
		Method:      http.MethodPost,
		Path:        "/v1/auth/telegram/webapp",
		Summary:     "Sign in from the Telegram Mini App",
		Description: "Takes the Mini App's Telegram.WebApp.initData, checks its signature with the bot token and its age " +
			"(TELEGRAM_WEBAPP_MAX_AGE, 1 hour by default), creates or updates the user by Telegram id like the website login " +
			"and sets the same session cookie. 401 invalid_init_data when the signature is wrong or the data too old; " +
			"503 telegram_not_configured without a bot token.",
		Tags: []string{"auth"},
	}, s.webAppLogin)
}

type WebAppLoginInput struct {
	Body struct {
		InitData string `json:"init_data" minLength:"1" maxLength:"4096" doc:"Telegram.WebApp.initData, unchanged"`
		ReturnTo string `json:"return_to,omitempty" maxLength:"200" doc:"Site path to open after sign-in; only local paths are kept"`
	}
}

type WebAppLoginOutput struct {
	SetCookie http.Cookie `header:"Set-Cookie"`
	Body      struct {
		Next            string `json:"next" example:"/mahsulotlarim/yangi" doc:"Where to go now: return_to, or /boshlash while the profile has no role"`
		NeedsOnboarding bool   `json:"needs_onboarding" doc:"The profile has no role yet"`
	}
}

func (s *Server) webAppLogin(ctx context.Context, in *WebAppLoginInput) (*WebAppLoginOutput, error) {
	id, err := auth.ValidateInitData(in.Body.InitData, s.cfg.TelegramBotToken, cmp.Or(s.cfg.TelegramWebAppMaxAge, defaultWebAppMaxAge), time.Now())
	switch {
	case errors.Is(err, auth.ErrNotConfigured):
		return nil, apiError(http.StatusServiceUnavailable, "telegram_not_configured", "Telegram orqali kirish hozircha sozlanmagan.")
	case err != nil:
		slog.InfoContext(ctx, "mini app sign-in refused", "reason", err)
		return nil, apiError(http.StatusUnauthorized, "invalid_init_data", "Telegram ma'lumoti tasdiqlanmadi. Mini ilovani qayta oching.")
	}

	userID, err := s.q.UpsertTelegramUser(ctx, db.UpsertTelegramUserParams{
		TelegramID:       id.TelegramID,
		TelegramUsername: pgtype.Text{String: id.Username, Valid: id.Username != ""},
		DisplayName:      id.Name,
	})
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
	slog.InfoContext(ctx, "user signed in", "user_id", userID, "via", "mini_app")

	u, err := s.q.GetUser(ctx, userID)
	if err != nil {
		return nil, internalError(ctx, "get user", err)
	}
	out := &WebAppLoginOutput{SetCookie: s.sessionCookie(token, expires)}
	out.Body.Next = safeReturnTo(in.Body.ReturnTo)
	if !u.Role.Valid {
		out.Body.Next, out.Body.NeedsOnboarding = "/boshlash", true
	}
	return out, nil
}
