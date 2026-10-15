package httpapi

import (
	"testing"

	"github.com/jackc/pgx/v5/pgtype"

	"bugun/internal/config"
	"bugun/internal/store/db"
)

func TestIsAdmin(t *testing.T) {
	s := &Server{cfg: config.Config{AdminTelegramIDs: []int64{42}}}
	for _, tc := range []struct {
		name string
		u    db.User
		want bool
	}{
		{"listed telegram id", db.User{TelegramID: 42}, true},
		{"admin role", db.User{TelegramID: 7, Role: pgtype.Text{String: "admin", Valid: true}}, true},
		{"entrepreneur", db.User{TelegramID: 7, Role: pgtype.Text{String: "entrepreneur", Valid: true}}, false},
		{"no role", db.User{TelegramID: 7}, false},
	} {
		if got := s.isAdmin(tc.u); got != tc.want {
			t.Errorf("%s: isAdmin = %v, want %v", tc.name, got, tc.want)
		}
	}
}
