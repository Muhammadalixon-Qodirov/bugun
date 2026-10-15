package llm

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestExtract(t *testing.T) {
	var got ChatRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" || r.Header.Get("Authorization") != "Bearer test-key" {
			http.Error(w, "bad", http.StatusUnauthorized)
			return
		}
		_ = json.NewDecoder(r.Body).Decode(&got)
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"` + "```json\\n" + `{\"items\":[{\"product\":\"piyoz\",\"price\":4500,\"unit\":\"kg\",\"trade\":\"ulgurji\"}]}` + "\\n```" + `"}}]}`))
	}))
	defer srv.Close()
	c := New(srv.URL+"/v1/", "test-model", "test-key", 2)
	items, err := c.Extract(context.Background(), "piyoz: Piyoz; birlik: kg", "piyoz 4500 optom")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Product != "piyoz" || items[0].Price != 4500 || items[0].Trade != "ulgurji" {
		t.Fatalf("items %+v", items)
	}
	if got.Model != "test-model" || got.Temperature != 0 || !strings.Contains(got.Messages[0].Text(), "piyoz: Piyoz") || got.Messages[1].Text() != "Matn: piyoz 4500 optom" {
		t.Errorf("request %+v", got)
	}
	// The hourly limit stops the third call.
	if _, err := c.Extract(context.Background(), "", "x"); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Extract(context.Background(), "", "x"); !errors.Is(err, ErrLimit) {
		t.Fatalf("third call: %v", err)
	}
	if New("", "m", "k", 0) != nil || New("u", "m", "", 0) != nil {
		t.Error("a missing setting must turn the client off")
	}
}
