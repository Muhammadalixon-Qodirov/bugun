package httpapi

import (
	"context"
	"testing"
)

// Two roles can be chosen: an entrepreneur is active at once and needs no
// categories; market staff need a market and an admin's approval, and a
// declined request leaves an entrepreneur.
func TestProfileRoles(t *testing.T) {
	a := newTestAPI(t)
	ctx := context.Background()
	u, tok := a.signIn(900000951, "entrepreneur", 2)
	_, adminTok := a.signIn(900000952, "admin", 2)

	var me Me
	if code := a.call(tok, "PUT", "/v1/me/profile", map[string]any{"role": "entrepreneur", "region_id": 2, "consent": true}, &me); code != 200 {
		t.Fatalf("entrepreneur without categories: status %d", code)
	}
	if me.Role == nil || *me.Role != "entrepreneur" || !me.RoleApproved || len(me.Categories) != 0 {
		t.Errorf("entrepreneur profile: %+v", me)
	}
	if code := a.call(tok, "PUT", "/v1/me/profile", map[string]any{"role": "entrepreneur", "region_id": 2, "consent": true, "categories": []string{"nope"}}, nil); code != 422 {
		t.Errorf("unknown category: status %d, want 422", code)
	}
	for _, role := range []string{"seller", "buyer", "org_analyst", "admin"} {
		if code := a.call(tok, "PUT", "/v1/me/profile", map[string]any{"role": role, "region_id": 2, "consent": true}, nil); code != 422 {
			t.Errorf("role %s: status %d, want 422", role, code)
		}
	}

	if code := a.call(tok, "PUT", "/v1/me/profile", map[string]any{"role": "market_admin", "region_id": 2, "consent": true}, nil); code != 422 {
		t.Errorf("market staff without a market: status %d, want 422", code)
	}
	if code := a.call(tok, "PUT", "/v1/me/profile", map[string]any{"role": "market_admin", "region_id": 2, "market_id": 1, "consent": true}, &me); code != 200 {
		t.Fatalf("market staff: status %d", code)
	}
	if *me.Role != "market_admin" || me.RoleApproved {
		t.Errorf("market staff waits for approval: %+v", me)
	}

	var requests RoleRequestsOutput
	if code := a.call(adminTok, "GET", "/v1/admin/role-requests", nil, &requests.Body); code != 200 {
		t.Fatalf("role requests: status %d", code)
	}
	found := false
	for _, r := range requests.Body.Requests {
		found = found || (r.ID == u.ID && r.Role == "market_admin")
	}
	if !found {
		t.Errorf("request missing: %+v", requests.Body.Requests)
	}
	if code := a.call(adminTok, "POST", "/v1/admin/role-requests/"+u.ID.String()+"/decision", map[string]any{"approve": false}, nil); code != 204 {
		t.Fatalf("decline: status %d", code)
	}
	if code := a.call(tok, "GET", "/v1/me", nil, &me); code != 200 || *me.Role != "entrepreneur" || !me.RoleApproved {
		t.Errorf("after a declined request: %d %+v, want an approved entrepreneur", code, me)
	}

	// A legacy value written by the previous release reads as entrepreneur.
	if _, err := a.pool.Exec(ctx, `UPDATE users SET role = 'seller' WHERE id = $1`, u.ID); err != nil {
		t.Fatal(err)
	}
	if code := a.call(tok, "GET", "/v1/me", nil, &me); code != 200 || *me.Role != "entrepreneur" {
		t.Errorf("legacy seller: %d %v", code, me.Role)
	}
	// The database refuses a role nobody knows.
	if _, err := a.pool.Exec(ctx, `UPDATE users SET role = 'owner' WHERE id = $1`, u.ID); err == nil {
		t.Error("unknown role accepted")
	}
}
