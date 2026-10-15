package httpapi

import (
	"context"
	"testing"
	"time"

	"bugun/internal/secure"
	"bugun/internal/store/db"
)

func TestExpiredSessionsCleanup(t *testing.T) {
	a := newTestAPI(t)
	ctx := context.Background()
	u, live := a.signIn(900000621, "entrepreneur", 2)
	old := secure.Token(24)
	if err := a.q.CreateSession(ctx, db.CreateSessionParams{TokenHash: secure.HashToken(old), UserID: u.ID, ExpiresAt: time.Now().Add(-time.Minute)}); err != nil {
		t.Fatal(err)
	}
	if err := a.q.CreateOAuthState(ctx, db.CreateOAuthStateParams{StateHash: secure.HashToken("stale-state"), CodeVerifier: "v", Nonce: "n", ReturnTo: "/", ExpiresAt: time.Now().Add(-time.Minute)}); err != nil {
		t.Fatal(err)
	}

	if n, err := a.q.DeleteExpiredSessions(ctx); err != nil || n < 1 {
		t.Fatalf("expired sessions: deleted %d, err %v", n, err)
	}
	if n, err := a.q.DeleteExpiredOAuthStates(ctx); err != nil || n < 1 {
		t.Fatalf("expired login states: deleted %d, err %v", n, err)
	}

	var left int
	if err := a.pool.QueryRow(ctx, `SELECT count(*) FROM sessions WHERE token_hash = $1`, secure.HashToken(old)).Scan(&left); err != nil || left != 0 {
		t.Fatalf("expired session still stored: %d, err %v", left, err)
	}
	if code := a.call(live, "GET", "/v1/me", nil, nil); code != 200 {
		t.Fatalf("live session after cleanup: status %d", code)
	}
}
