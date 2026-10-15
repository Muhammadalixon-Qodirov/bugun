package llm_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	"bugun/internal/aitest"
	"bugun/internal/llm"
)

func TestChatRecordsCalls(t *testing.T) {
	fake := aitest.NewFake(t)
	fake.Chat = func(req llm.ChatRequest) llm.Message {
		return aitest.ToolCall("c1", "price_stats", map[string]string{"product": "piyoz"})
	}
	rec := &aitest.Recorder{}
	c := fake.Client("chat-model").WithRecorder(rec)
	c.MaxPerHour = 1
	uid := uuid.New()
	ctx := llm.WithUser(llm.WithFeature(context.Background(), "analyst"), uid)

	msg, err := c.Chat(ctx, llm.ChatRequest{Messages: []llm.Message{{Role: "user", Content: "piyoz?"}},
		Tools: []llm.Tool{{Type: "function", Function: llm.ToolFunction{Name: "price_stats", Parameters: map[string]any{"type": "object"}}}}})
	if err != nil {
		t.Fatal(err)
	}
	if len(msg.ToolCalls) != 1 || msg.ToolCalls[0].Function.Name != "price_stats" || msg.ToolCalls[0].Function.Arguments != `{"product":"piyoz"}` {
		t.Errorf("tool call %+v", msg)
	}
	if fake.Requests[0].Model != "chat-model" || len(fake.Requests[0].Tools) != 1 {
		t.Errorf("request %+v", fake.Requests[0])
	}
	// The hourly cap is recorded too.
	if _, err := c.Chat(ctx, llm.ChatRequest{}); !errors.Is(err, llm.ErrLimit) {
		t.Fatalf("second call: %v", err)
	}
	if len(rec.Calls) != 2 {
		t.Fatalf("calls %+v", rec.Calls)
	}
	ok, limited := rec.Calls[0], rec.Calls[1]
	if !ok.OK || ok.Feature != "analyst" || ok.Model != "chat-model" || ok.InputTokens != 120 || ok.OutputTokens != 30 || ok.UserID == nil || *ok.UserID != uid {
		t.Errorf("ok call %+v", ok)
	}
	if limited.OK || limited.ErrorCode != "limit" {
		t.Errorf("limited call %+v", limited)
	}
}

func TestChatErrors(t *testing.T) {
	fake := aitest.NewFake(t)
	fake.Status = http.StatusTooManyRequests
	rec := &aitest.Recorder{}
	c := fake.Client("m").WithRecorder(rec)
	if _, err := c.Chat(context.Background(), llm.ChatRequest{}); err == nil {
		t.Fatal("want an error")
	}
	if rec.Calls[0].ErrorCode != "http_429" || rec.Calls[0].Feature != "chat" {
		t.Errorf("recorded %+v", rec.Calls[0])
	}

	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
	}))
	defer slow.Close()
	c = llm.New(slow.URL, "m", "k", 10).WithTimeout(50 * time.Millisecond).WithRecorder(rec)
	if _, err := c.Chat(context.Background(), llm.ChatRequest{}); err == nil {
		t.Fatal("want a timeout")
	}
	if got := rec.Calls[1].ErrorCode; got != "timeout" {
		t.Errorf("timeout recorded as %q", got)
	}

	fake.Status = 0
	fake.Chat = func(llm.ChatRequest) llm.Message { return llm.Message{Content: "not json"} }
	var v map[string]any
	if err := fake.Client("m").ChatJSON(context.Background(), llm.ChatRequest{}, &v); err == nil {
		t.Error("ChatJSON accepted text")
	}
}

func TestTranscribe(t *testing.T) {
	fake := aitest.NewFake(t)
	var fields map[string]string
	var got []byte
	fake.Transcribe = func(audio []byte, f map[string]string) string {
		got, fields = audio, f
		return " kartoshka tort yarim ming "
	}
	rec := &aitest.Recorder{}
	c := fake.Client("whisper-test").WithRecorder(rec)
	text, err := c.Transcribe(llm.WithFeature(context.Background(), "stt"), []byte("OggS-audio"), "voice.ogg", "uz", 4)
	if err != nil {
		t.Fatal(err)
	}
	if text != "kartoshka tort yarim ming" || string(got) != "OggS-audio" || fields["model"] != "whisper-test" || fields["language"] != "uz" {
		t.Errorf("text %q audio %q fields %v", text, got, fields)
	}
	if c := rec.Calls[0]; c.Feature != "stt" || c.AudioSeconds != 4 || !c.OK {
		t.Errorf("recorded %+v", c)
	}
}

func TestEmbed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/embeddings" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(`{"data":[{"index":1,"embedding":[0,1]},{"index":0,"embedding":[1,0]}],"usage":{"prompt_tokens":4}}`))
	}))
	defer srv.Close()
	rec := &aitest.Recorder{}
	c := llm.New(srv.URL+"/v1", "emb", "k", 10).WithRecorder(rec)
	vecs, err := c.Embed(context.Background(), []string{"a", "b"})
	if err != nil {
		t.Fatal(err)
	}
	if vecs[0][0] != 1 || vecs[1][1] != 1 {
		t.Errorf("vectors are in input order: %v", vecs)
	}
	if rec.Calls[0].Feature != "embeddings" || rec.Calls[0].InputTokens != 4 {
		t.Errorf("recorded %+v", rec.Calls[0])
	}
	if _, err := c.Embed(context.Background(), []string{"only one"}); err == nil {
		t.Error("a count mismatch must fail")
	}
}
