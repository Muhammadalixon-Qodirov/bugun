package httpapi

import (
	"strconv"
	"testing"
)

func TestEventsAdmin(t *testing.T) {
	a := newTestAPI(t)
	_, sellerTok := a.signIn(900000561, "entrepreneur", 2)
	_, adminTok := a.signIn(900000562, "admin", 2)
	body := map[string]any{"title": "Kuchli sovuq", "kind": "frost", "starts_on": "2026-02-10", "ends_on": "2026-02-20", "region_id": 2, "category": "sabzavot", "source_url": "https://example.uz/x"}

	if code := a.call(sellerTok, "POST", "/v1/admin/events", body, nil); code != 403 {
		t.Errorf("seller: status %d, want 403", code)
	}
	var e EventOutput
	if code := a.call(adminTok, "POST", "/v1/admin/events", body, &e.Body); code != 200 {
		t.Fatalf("create: status %d", code)
	}
	if e.Body.ProvinceID == 0 || e.Body.ProvinceID == 2 || e.Body.EndsOn != "2026-02-20" || e.Body.Demo {
		t.Errorf("created %+v (tuman 2 keeps its viloyat)", e.Body)
	}
	for field, value := range map[string]any{"ends_on": "2026-02-01", "source_url": "ftp://x", "product_id": 9999, "region_id": 999} {
		bad := map[string]any{}
		for k, v := range body {
			bad[k] = v
		}
		bad[field] = value
		if code := a.call(adminTok, "POST", "/v1/admin/events", bad, nil); code != 422 {
			t.Errorf("bad %s: status %d, want 422", field, code)
		}
	}
	path := "/v1/admin/events/" + strconv.FormatInt(e.Body.ID, 10)
	body["title"] = "Kuchli sovuq urdi"
	if code := a.call(adminTok, "PUT", path, body, &e.Body); code != 200 || e.Body.Title != "Kuchli sovuq urdi" {
		t.Errorf("update: %d %+v", code, e.Body)
	}
	var list EventsOutput
	if code := a.call(adminTok, "GET", "/v1/admin/events", nil, &list.Body); code != 200 {
		t.Fatalf("list: %d", code)
	}
	found := false
	for _, ev := range list.Body.Events {
		if ev.ID == e.Body.ID {
			found = ev.Province != "" && ev.Title == "Kuchli sovuq urdi"
		}
	}
	if !found {
		t.Errorf("created event missing from the list or without its viloyat (%d events)", len(list.Body.Events))
	}
	if code := a.call(adminTok, "DELETE", path, nil, nil); code != 204 {
		t.Errorf("delete: %d", code)
	}
	if code := a.call(adminTok, "DELETE", path, nil, nil); code != 404 {
		t.Errorf("delete twice: %d", code)
	}
}
