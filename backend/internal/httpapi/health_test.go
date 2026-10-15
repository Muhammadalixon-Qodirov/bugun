package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"bugun/internal/config"
	"bugun/internal/store"
)

func healthOf(t *testing.T, h http.Handler) (int, map[string]string) {
	t.Helper()
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/healthz", nil))
	var body map[string]string
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return w.Code, body
}

func TestHealthWithoutDatabaseIs503(t *testing.T) {
	code, body := healthOf(t, New(config.Config{}, nil, Deps{}))
	if code != http.StatusServiceUnavailable || body["database"] != "unavailable" || body["status"] != "ok" {
		t.Errorf("no pool: %d %v, want 503 with database unavailable", code, body)
	}
}

func TestHealthFollowsTheDatabase(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	pool, err := store.Open(context.Background(), url)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	h := New(config.Config{}, pool, Deps{})
	if code, body := healthOf(t, h); code != http.StatusOK || body["database"] != "ok" {
		t.Errorf("database up: %d %v, want 200 ok", code, body)
	}
	pool.Close() // the database "goes away"
	if code, body := healthOf(t, h); code != http.StatusServiceUnavailable || body["database"] != "unavailable" {
		t.Errorf("database down: %d %v, want 503 unavailable", code, body)
	}
}
