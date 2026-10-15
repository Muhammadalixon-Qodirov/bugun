package llm

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestChainFallsBack(t *testing.T) {
	var first, second atomic.Int32
	busy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		first.Add(1)
		http.Error(w, "slow down", http.StatusTooManyRequests)
	}))
	defer busy.Close()
	ok := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		second.Add(1)
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"salom"}}]}`))
	}))
	defer ok.Close()
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "bad", http.StatusBadRequest)
	}))
	defer bad.Close()

	c := NewChain(New(busy.URL, "a", "k", 10), nil, New(ok.URL, "b", "k", 10))
	msg, err := c.Chat(context.Background(), ChatRequest{})
	if err != nil || msg.Text() != "salom" || first.Load() != 1 || second.Load() != 1 {
		t.Fatalf("chain: %q %v (%d, %d)", msg.Text(), err, first.Load(), second.Load())
	}
	// A request the service rejects is not sent to the next model.
	c = NewChain(New(bad.URL, "a", "k", 10), New(ok.URL, "b", "k", 10))
	var se *StatusError
	if _, err := c.Chat(context.Background(), ChatRequest{}); !errors.As(err, &se) || se.Code != 400 || second.Load() != 1 {
		t.Fatalf("400 retried: %v", err)
	}
	if NewChain(nil, nil) != nil {
		t.Error("an empty chain must be nil")
	}
}
