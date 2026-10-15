package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"bugun/internal/media"
	"bugun/internal/roles"
	"bugun/internal/store/db"
)

type Me struct {
	ID               string     `json:"id"`
	DisplayName      string     `json:"display_name"`
	TelegramUsername *string    `json:"telegram_username,omitempty"`
	PhoneMasked      *string    `json:"phone_masked,omitempty" doc:"Only the last digits are shown"`
	Role             *string    `json:"role,omitempty" enum:"entrepreneur,market_admin,admin" doc:"entrepreneur (Tadbirkor) sells and buys; market_admin oversees a market once approved; admin is the system role. Absent until onboarding."`
	RoleApproved     bool       `json:"role_approved"`
	RegionID         *int16     `json:"region_id,omitempty"`
	MarketID         *int16     `json:"market_id,omitempty"`
	Categories       []string   `json:"categories"`
	PublicName       *string    `json:"public_name,omitempty"`
	ConsentAt        *time.Time `json:"consent_at,omitempty"`
	BotEnabled       bool       `json:"bot_enabled"`
	ContactTelegram  bool       `json:"contact_telegram" doc:"Buyers may open a Telegram chat (needs a username)"`
	ContactPhone     bool       `json:"contact_phone" doc:"Buyers may reveal the phone number"`
	Onboarded        bool       `json:"onboarded"`
	IsAdmin          bool       `json:"is_admin" doc:"May open the admin pages"`
	CreatedAt        time.Time  `json:"created_at"`
}

type MeOutput struct {
	Body Me
}

func (s *Server) registerMe(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "getMe",
		Method:      http.MethodGet,
		Path:        "/v1/me",
		Summary:     "Current user's profile",
		Tags:        []string{"profile"},
		Security:    authRequired,
	}, s.getMe)

	huma.Register(api, huma.Operation{
		OperationID: "updateProfile",
		Method:      http.MethodPut,
		Path:        "/v1/me/profile",
		Summary:     "Set role, region, market and categories (onboarding and settings)",
		Tags:        []string{"profile"},
		Security:    authRequired,
	}, s.updateProfile)

	huma.Register(api, huma.Operation{
		OperationID: "setBotEnabled",
		Method:      http.MethodPut,
		Path:        "/v1/me/bot",
		Summary:     "Connect or disconnect the Telegram bot",
		Tags:        []string{"profile"},
		Security:    authRequired,
	}, s.setBot)

	huma.Register(api, huma.Operation{
		OperationID: "setContactPrefs",
		Method:      http.MethodPut,
		Path:        "/v1/me/contact",
		Summary:     "Choose how buyers can reach me",
		Tags:        []string{"profile"},
		Security:    authRequired,
	}, s.setContact)

	huma.Register(api, huma.Operation{
		OperationID:   "deleteMe",
		Method:        http.MethodDelete,
		Path:          "/v1/me",
		Summary:       "Delete my account and personal data",
		Tags:          []string{"profile"},
		Security:      authRequired,
		DefaultStatus: http.StatusNoContent,
	}, s.deleteMe)
}

func (s *Server) getMe(ctx context.Context, in *SessionInput) (*MeOutput, error) {
	u, err := s.currentUser(ctx, in.Session)
	if err != nil {
		return nil, err
	}
	return &MeOutput{Body: s.toMe(u)}, nil
}

type UpdateProfileInput struct {
	SessionInput
	Body struct {
		Role       string   `json:"role" enum:"entrepreneur,market_admin" doc:"entrepreneur is active at once; market_admin waits for an admin"`
		RegionID   int16    `json:"region_id" minimum:"1"`
		MarketID   *int16   `json:"market_id,omitempty" minimum:"1"`
		Categories []string `json:"categories,omitempty" maxItems:"7" uniqueItems:"true" doc:"Optional product categories"`
		PublicName string   `json:"public_name,omitempty" maxLength:"60"`
		Consent    bool     `json:"consent" doc:"Consent to processing of personal data"`
	}
}

func (s *Server) updateProfile(ctx context.Context, in *UpdateProfileInput) (*MeOutput, error) {
	u, err := s.currentUser(ctx, in.Session)
	if err != nil {
		return nil, err
	}
	b := in.Body
	var errs []*APIFieldError
	if !b.Consent {
		errs = append(errs, fieldError("consent", "consent_required", "Shaxsiy ma'lumotlarni qayta ishlashga rozilik kerak.", nil))
	}
	for _, c := range b.Categories {
		if !slices.ContainsFunc(Categories, func(k Category) bool { return k.Slug == c }) {
			errs = append(errs, fieldError("categories", "unknown_category", "Noma'lum toifa.", map[string]any{"value": c}))
		}
	}
	// Markets are only catalogued for the pilot region, so only market staff must pick one.
	if b.Role == roles.MarketAdmin && b.MarketID == nil {
		errs = append(errs, fieldError("market_id", "market_required", "Bozorni tanlang.", nil))
	}
	// Profiles point at a tuman or shahar, never at a whole viloyat.
	switch r, err := s.q.GetRegion(ctx, b.RegionID); {
	case errors.Is(err, pgx.ErrNoRows):
		errs = append(errs, fieldError("region_id", "region_not_found", "Bunday hudud topilmadi.", nil))
	case err != nil:
		return nil, internalError(ctx, "get region", err)
	case !r.ParentID.Valid:
		errs = append(errs, fieldError("region_id", "district_required", "Tuman yoki shaharni tanlang.", nil))
	}
	var market pgtype.Int2
	if b.MarketID != nil {
		m, err := s.q.GetMarket(ctx, *b.MarketID)
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			errs = append(errs, fieldError("market_id", "market_not_found", "Bunday bozor topilmadi.", nil))
		case err != nil:
			return nil, internalError(ctx, "get market", err)
		case m.RegionID != b.RegionID:
			errs = append(errs, fieldError("market_id", "market_outside_region", "Bozor tanlangan hududda emas.", nil))
		default:
			market = pgtype.Int2{Int16: m.ID, Valid: true}
		}
	}
	if len(errs) > 0 {
		return nil, invalidInput(errs...)
	}

	// Market administration is activated by an admin (spec §7.1). Changing to
	// it resets approval; keeping the same role keeps it. Entrepreneurs are
	// approved at once.
	approved := !roles.NeedsApproval(b.Role) || (u.Role.Valid && u.Role.String == b.Role && u.RoleApproved)
	categories := b.Categories
	if categories == nil {
		categories = []string{}
	}
	updated, err := s.q.UpdateProfile(ctx, db.UpdateProfileParams{
		ID:           u.ID,
		Role:         pgtype.Text{String: b.Role, Valid: true},
		RoleApproved: approved,
		RegionID:     pgtype.Int2{Int16: b.RegionID, Valid: true},
		MarketID:     market,
		Categories:   categories,
		PublicName:   pgtype.Text{String: strings.TrimSpace(b.PublicName), Valid: strings.TrimSpace(b.PublicName) != ""},
	})
	if err != nil {
		return nil, internalError(ctx, "update profile", err)
	}
	return &MeOutput{Body: s.toMe(updated)}, nil
}

type SetBotInput struct {
	SessionInput
	Body struct {
		Enabled bool `json:"enabled"`
	}
}

func (s *Server) setBot(ctx context.Context, in *SetBotInput) (*MeOutput, error) {
	u, err := s.currentUser(ctx, in.Session)
	if err != nil {
		return nil, err
	}
	if err := s.q.SetBotEnabled(ctx, db.SetBotEnabledParams{ID: u.ID, BotEnabled: in.Body.Enabled}); err != nil {
		return nil, internalError(ctx, "set bot enabled", err)
	}
	u.BotEnabled = in.Body.Enabled
	return &MeOutput{Body: s.toMe(u)}, nil
}

type SetContactInput struct {
	SessionInput
	Body struct {
		Telegram bool `json:"telegram"`
		Phone    bool `json:"phone"`
	}
}

func (s *Server) setContact(ctx context.Context, in *SetContactInput) (*MeOutput, error) {
	u, err := s.currentUser(ctx, in.Session)
	if err != nil {
		return nil, err
	}
	b := in.Body
	switch {
	case !b.Telegram && !b.Phone:
		return nil, apiError(http.StatusUnprocessableEntity, "contact_method_required", "Kamida bitta bog'lanish usulini qoldiring.")
	case b.Telegram && !u.TelegramUsername.Valid:
		return nil, apiError(http.StatusUnprocessableEntity, "telegram_username_missing", "Telegram hisobingizda username yo'q. Telegram sozlamalarida username qo'ying yoki telefonni tanlang.")
	case b.Phone && len(u.PhoneEnc) == 0:
		return nil, apiError(http.StatusUnprocessableEntity, "phone_missing", "Telefon raqamingiz bizda yo'q. Qayta kirib, raqamni ulashishga ruxsat bering.")
	}
	if err := s.q.SetContactPrefs(ctx, db.SetContactPrefsParams{ID: u.ID, ContactTelegram: b.Telegram, ContactPhone: b.Phone}); err != nil {
		return nil, internalError(ctx, "set contact", err)
	}
	u.ContactTelegram, u.ContactPhone = b.Telegram, b.Phone
	return &MeOutput{Body: s.toMe(u)}, nil
}

func (s *Server) deleteMe(ctx context.Context, in *SessionInput) (*LogoutOutput, error) {
	u, err := s.currentUser(ctx, in.Session)
	if err != nil {
		return nil, err
	}
	// The photo rows cascade with the listings; the files go after them.
	photos, err := s.q.ListSellerPhotoKeys(ctx, u.ID)
	if err != nil {
		return nil, internalError(ctx, "list photos", err)
	}
	// Sessions, RFQs and offers cascade; price observations keep the number but lose the link.
	if err := s.q.DeleteUser(ctx, u.ID); err != nil {
		return nil, internalError(ctx, "delete user", err)
	}
	if s.media != nil {
		for _, key := range photos {
			if err := media.Remove(ctx, s.media, key); err != nil {
				slog.WarnContext(ctx, "delete photo file", "err", err)
			}
		}
	}
	slog.InfoContext(ctx, "user deleted", "user_id", u.ID)
	return &LogoutOutput{SetCookie: s.clearedSessionCookie()}, nil
}

func (s *Server) toMe(u db.User) Me {
	m := Me{
		ID:              u.ID.String(),
		DisplayName:     u.DisplayName,
		RoleApproved:    u.RoleApproved,
		Categories:      u.Categories,
		ConsentAt:       u.ConsentAt,
		BotEnabled:      u.BotEnabled,
		ContactTelegram: u.ContactTelegram,
		ContactPhone:    u.ContactPhone,
		Onboarded:       u.Role.Valid && u.ConsentAt != nil,
		IsAdmin:         s.isAdmin(u),
		CreatedAt:       u.CreatedAt,
	}
	if u.TelegramUsername.Valid {
		m.TelegramUsername = &u.TelegramUsername.String
	}
	if u.Role.Valid {
		role := u.Role.String
		if roles.IsEntrepreneur(role) { // a legacy value written by the previous release
			role = roles.Entrepreneur
		}
		m.Role = &role
	}
	if u.RegionID.Valid {
		m.RegionID = &u.RegionID.Int16
	}
	if u.MarketID.Valid {
		m.MarketID = &u.MarketID.Int16
	}
	if u.PublicName.Valid {
		m.PublicName = &u.PublicName.String
	}
	if len(u.PhoneEnc) > 0 {
		if phone, err := s.sealer.Open(u.PhoneEnc); err == nil && len(phone) > 4 {
			masked := strings.Repeat("•", 5) + " " + phone[len(phone)-4:len(phone)-2] + " " + phone[len(phone)-2:]
			m.PhoneMasked = &masked
		}
	}
	return m
}
