// Package aitest holds test helpers for the model features: a fake
// OpenAI-compatible server and a database with a test seller. Tests never
// call a real model.
package aitest

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"bugun/internal/llm"
	"bugun/internal/store"
	"bugun/internal/store/db"
)

// Fake is an OpenAI-compatible server. Chat answers chat completions (the
// returned message is sent as the only choice); Transcribe answers audio
// transcriptions with the text to return.
type Fake struct {
	*httptest.Server
	Chat       func(req llm.ChatRequest) llm.Message
	Transcribe func(audio []byte, fields map[string]string) string
	Status     int // non-zero: every request fails with this status

	mu       sync.Mutex
	Requests []llm.ChatRequest
	Audio    int
}

// NewFake starts a fake server; it stops when the test ends.
func NewFake(t *testing.T) *Fake {
	t.Helper()
	f := &Fake{}
	f.Server = httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(f.Close)
	return f
}

// Client is an llm client for the fake server.
func (f *Fake) Client(model string) *llm.Client { return llm.New(f.URL+"/v1", model, "test-key", 1000) }

// ChatCount is how many chat requests arrived.
func (f *Fake) ChatCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.Requests)
}

func (f *Fake) serve(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Authorization") != "Bearer test-key" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if f.Status != 0 {
		http.Error(w, "failure", f.Status)
		return
	}
	switch r.URL.Path {
	case "/v1/chat/completions":
		var req llm.ChatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad", http.StatusBadRequest)
			return
		}
		f.mu.Lock()
		f.Requests = append(f.Requests, req)
		f.mu.Unlock()
		msg := llm.Message{Role: "assistant", Content: ""}
		if f.Chat != nil {
			msg = f.Chat(req)
		}
		if msg.Role == "" {
			msg.Role = "assistant"
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{"message": msg}},
			"usage":   map[string]int{"prompt_tokens": 120, "completion_tokens": 30},
		})
	case "/v1/audio/transcriptions":
		if err := r.ParseMultipartForm(8 << 20); err != nil {
			http.Error(w, "bad", http.StatusBadRequest)
			return
		}
		file, _, err := r.FormFile("file")
		if err != nil {
			http.Error(w, "no file", http.StatusBadRequest)
			return
		}
		audio, _ := io.ReadAll(file)
		fields := map[string]string{}
		for k, v := range r.MultipartForm.Value {
			fields[k] = v[0]
		}
		f.mu.Lock()
		f.Audio++
		f.mu.Unlock()
		text := ""
		if f.Transcribe != nil {
			text = f.Transcribe(audio, fields)
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"text": text})
	default:
		http.NotFound(w, r)
	}
}

// ToolCall builds a model message asking for one function.
func ToolCall(id, name string, args any) llm.Message {
	b, _ := json.Marshal(args)
	tc := llm.ToolCall{ID: id, Type: "function"}
	tc.Function.Name, tc.Function.Arguments = name, string(b)
	return llm.Message{Role: "assistant", ToolCalls: []llm.ToolCall{tc}}
}

// DB opens the test database (TEST_DATABASE_URL) with migrations applied,
// or skips the test.
func DB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	pool, err := store.Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err := store.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	return pool
}

// User creates a user with a role in Namangan shahri; it is deleted when the
// test ends.
func User(t *testing.T, q *db.Queries, tgID int64, role string) db.User {
	t.Helper()
	ctx := context.Background()
	id, err := q.UpsertTelegramUser(ctx, db.UpsertTelegramUserParams{TelegramID: tgID, DisplayName: "Test " + role})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = q.DeleteUser(context.Background(), id) })
	u, err := q.UpdateProfile(ctx, db.UpdateProfileParams{
		ID: id, Role: pgtype.Text{String: role, Valid: true}, RoleApproved: true,
		RegionID: pgtype.Int2{Int16: 2, Valid: true}, MarketID: pgtype.Int2{Int16: 1, Valid: true},
		Categories: []string{"sabzavot"},
	})
	if err != nil {
		t.Fatal(err)
	}
	return u
}

// Recorder keeps calls in memory, like ai_log.
type Recorder struct {
	mu    sync.Mutex
	Calls []llm.Call
}

func (r *Recorder) Record(_ context.Context, c llm.Call) {
	r.mu.Lock()
	defer r.mu.Unlock()
	c.Input = nil
	r.Calls = append(r.Calls, c)
}

// Features lists the recorded calls' features in order.
func (r *Recorder) Features() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]string, len(r.Calls))
	for i, c := range r.Calls {
		out[i] = c.Feature
	}
	return out
}
