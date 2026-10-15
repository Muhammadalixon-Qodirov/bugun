package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// The OpenAI-compatible endpoints every model feature uses: chat completions
// (with tools, JSON answers and images), audio transcriptions and embeddings.
// Each call is limited per hour and reported to a Recorder (ai_log).

// ErrLimit: the hourly call limit is reached; the caller falls back to rules.
var ErrLimit = errors.New("llm: hourly limit reached")

// Client calls one OpenAI-compatible API with one model.
type Client struct {
	BaseURL    string // e.g. https://api.groq.com/openai/v1
	Model      string
	Key        string
	MaxPerHour int
	HTTP       *http.Client
	// Recorder receives every call (nil: calls are not recorded).
	Recorder Recorder

	mu     sync.Mutex
	window time.Time
	calls  int
}

// New returns a client, or nil when any setting is missing (the feature is off).
func New(baseURL, model, key string, maxPerHour int) *Client {
	if baseURL == "" || model == "" || key == "" {
		return nil
	}
	if maxPerHour <= 0 {
		maxPerHour = 100
	}
	return &Client{BaseURL: strings.TrimRight(baseURL, "/"), Model: model, Key: key, MaxPerHour: maxPerHour,
		HTTP: &http.Client{Timeout: 20 * time.Second}}
}

// WithRecorder sets the recorder and returns the client (nil stays nil).
func (c *Client) WithRecorder(r Recorder) *Client {
	if c != nil {
		c.Recorder = r
	}
	return c
}

// WithTimeout sets the HTTP timeout and returns the client (nil stays nil).
func (c *Client) WithTimeout(d time.Duration) *Client {
	if c != nil && d > 0 {
		c.HTTP = &http.Client{Timeout: d}
	}
	return c
}

func (c *Client) allow(now time.Time) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if now.Sub(c.window) >= time.Hour {
		c.window, c.calls = now, 0
	}
	if c.calls >= c.MaxPerHour {
		return false
	}
	c.calls++
	return true
}

// Call is one model call as ai_log keeps it. Input is only for hashing; a
// Recorder must never store it.
type Call struct {
	Feature      string
	Model        string
	InputTokens  int
	OutputTokens int
	AudioSeconds float64
	Latency      time.Duration
	OK           bool
	ErrorCode    string
	UserID       *uuid.UUID
	Input        []byte
}

// Recorder keeps calls (ailog.Logger).
type Recorder interface {
	Record(ctx context.Context, c Call)
}

type ctxKey int

const (
	featureKey ctxKey = iota
	userKey
)

// WithFeature names what the calls under ctx are for (analyst, stt, vision, …).
func WithFeature(ctx context.Context, feature string) context.Context {
	return context.WithValue(ctx, featureKey, feature)
}

// WithUser ties the calls under ctx to a user.
func WithUser(ctx context.Context, id uuid.UUID) context.Context {
	return context.WithValue(ctx, userKey, id)
}

func featureOf(ctx context.Context, fallback string) string {
	if f, ok := ctx.Value(featureKey).(string); ok && f != "" {
		return f
	}
	return fallback
}

func userOf(ctx context.Context) *uuid.UUID {
	if id, ok := ctx.Value(userKey).(uuid.UUID); ok {
		return &id
	}
	return nil
}

// errorCode is a short, content-free label for a failed call.
func errorCode(err error, status int) string {
	var ne net.Error
	switch {
	case status != 0:
		return fmt.Sprintf("http_%d", status)
	case errors.Is(err, ErrLimit):
		return "limit"
	case errors.Is(err, context.DeadlineExceeded), errors.As(err, &ne) && ne.Timeout():
		return "timeout"
	case errors.Is(err, context.Canceled):
		return "canceled"
	case errors.Is(err, errBadAnswer):
		return "bad_answer"
	case err != nil:
		return "network"
	}
	return ""
}

var errBadAnswer = errors.New("llm: unexpected answer")

// StatusError is an error status from the service (429: rate limited).
type StatusError struct{ Code int }

func (e *StatusError) Error() string { return fmt.Sprintf("llm: status %d", e.Code) }

// record reports a call; limit refusals are recorded too, with no latency.
func (c *Client) record(ctx context.Context, fallback string, input []byte, started time.Time, u usage, audio float64, err error, status int) {
	if c.Recorder == nil {
		return
	}
	call := Call{
		Feature: featureOf(ctx, fallback), Model: c.Model, InputTokens: u.PromptTokens, OutputTokens: u.CompletionTokens,
		AudioSeconds: audio, Latency: time.Since(started), OK: err == nil && status == 0, UserID: userOf(ctx), Input: input,
	}
	if !call.OK {
		call.ErrorCode = errorCode(err, status)
	}
	// Recording must outlive a request that was cut off.
	c.Recorder.Record(context.WithoutCancel(ctx), call)
}

type usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
}

// post sends body to path and returns the answer body; status is non-zero
// when the service answered with an error status.
func (c *Client) post(ctx context.Context, path, contentType string, body []byte) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+path, bytes.NewReader(body))
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Authorization", "Bearer "+c.Key)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, 0, err
	}
	if resp.StatusCode != http.StatusOK {
		// The body may echo the request; keep it out of errors and logs.
		return nil, resp.StatusCode, &StatusError{Code: resp.StatusCode}
	}
	return raw, 0, nil
}

// Message is one chat message. Content is a string, or parts for images.
type Message struct {
	Role       string     `json:"role"`
	Content    any        `json:"content"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
}

// Text is the message's content when it is plain text.
func (m Message) Text() string {
	s, _ := m.Content.(string)
	return s
}

// Part is one piece of a message with an image.
type Part struct {
	Type     string    `json:"type"` // text or image_url
	Text     string    `json:"text,omitempty"`
	ImageURL *ImageURL `json:"image_url,omitempty"`
}

type ImageURL struct {
	URL string `json:"url"`
}

// Tool is a function the model may call.
type Tool struct {
	Type     string       `json:"type"` // function
	Function ToolFunction `json:"function"`
}

type ToolFunction struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

// ToolCall is the model asking for a function.
type ToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

// ChatRequest is a chat completion request; Model is filled in by the client.
type ChatRequest struct {
	Model          string         `json:"model"`
	Messages       []Message      `json:"messages"`
	Temperature    float64        `json:"temperature"`
	MaxTokens      int            `json:"max_tokens,omitempty"`
	Tools          []Tool         `json:"tools,omitempty"`
	ToolChoice     any            `json:"tool_choice,omitempty"`
	ResponseFormat map[string]any `json:"response_format,omitempty"`
}

// JSONObject asks for a JSON object answer.
var JSONObject = map[string]any{"type": "json_object"}

// Chat sends one chat completion and returns the model's message.
func (c *Client) Chat(ctx context.Context, req ChatRequest) (Message, error) {
	started := time.Now()
	req.Model = c.Model
	body, err := json.Marshal(req)
	if err != nil {
		return Message{}, err
	}
	if !c.allow(started) {
		c.record(ctx, "chat", body, started, usage{}, 0, ErrLimit, 0)
		return Message{}, ErrLimit
	}
	raw, status, err := c.post(ctx, "/chat/completions", "application/json", body)
	if err != nil {
		c.record(ctx, "chat", body, started, usage{}, 0, err, status)
		return Message{}, err
	}
	var out struct {
		Choices []struct {
			Message Message `json:"message"`
		} `json:"choices"`
		Usage usage `json:"usage"`
	}
	if err := json.Unmarshal(raw, &out); err != nil || len(out.Choices) == 0 {
		c.record(ctx, "chat", body, started, out.Usage, 0, errBadAnswer, 0)
		return Message{}, errBadAnswer
	}
	c.record(ctx, "chat", body, started, out.Usage, 0, nil, 0)
	return out.Choices[0].Message, nil
}

// ChatJSON sends a chat completion that must answer with a JSON object and
// decodes it into v, tolerating a code fence around it.
func (c *Client) ChatJSON(ctx context.Context, req ChatRequest, v any) error {
	req.ResponseFormat = JSONObject
	msg, err := c.Chat(ctx, req)
	if err != nil {
		return err
	}
	if err := json.Unmarshal([]byte(StripFence(msg.Text())), v); err != nil {
		return errBadAnswer
	}
	return nil
}

// StripFence removes a ```json fence around a model's answer.
func StripFence(content string) string {
	s := strings.TrimSpace(content)
	s = strings.TrimPrefix(strings.TrimPrefix(s, "```json"), "```")
	return strings.TrimSpace(strings.TrimSuffix(s, "```"))
}

// Transcribe turns speech into text with /audio/transcriptions. seconds is
// the audio's length when known, for the log.
func (c *Client) Transcribe(ctx context.Context, audio []byte, filename, language string, seconds float64) (string, error) {
	started := time.Now()
	if !c.allow(started) {
		c.record(ctx, "stt", audio, started, usage{}, seconds, ErrLimit, 0)
		return "", ErrLimit
	}
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	fw, err := w.CreateFormFile("file", filename)
	if err != nil {
		return "", err
	}
	if _, err := fw.Write(audio); err != nil {
		return "", err
	}
	_ = w.WriteField("model", c.Model)
	_ = w.WriteField("response_format", "json")
	_ = w.WriteField("temperature", "0")
	if language != "" {
		_ = w.WriteField("language", language)
	}
	if err := w.Close(); err != nil {
		return "", err
	}
	raw, status, err := c.post(ctx, "/audio/transcriptions", w.FormDataContentType(), buf.Bytes())
	if err != nil {
		c.record(ctx, "stt", audio, started, usage{}, seconds, err, status)
		return "", err
	}
	var out struct {
		Text     string  `json:"text"`
		Duration float64 `json:"duration"`
		Usage    struct {
			InputTokens  int     `json:"input_tokens"`
			OutputTokens int     `json:"output_tokens"`
			Seconds      float64 `json:"seconds"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		c.record(ctx, "stt", audio, started, usage{}, seconds, errBadAnswer, 0)
		return "", errBadAnswer
	}
	if seconds == 0 {
		seconds = max(out.Duration, out.Usage.Seconds)
	}
	c.record(ctx, "stt", audio, started, usage{PromptTokens: out.Usage.InputTokens, CompletionTokens: out.Usage.OutputTokens}, seconds, nil, 0)
	return strings.TrimSpace(out.Text), nil
}

// Embed returns one vector per input with /embeddings.
func (c *Client) Embed(ctx context.Context, inputs []string) ([][]float32, error) {
	started := time.Now()
	body, err := json.Marshal(map[string]any{"model": c.Model, "input": inputs})
	if err != nil {
		return nil, err
	}
	if !c.allow(started) {
		c.record(ctx, "embeddings", body, started, usage{}, 0, ErrLimit, 0)
		return nil, ErrLimit
	}
	raw, status, err := c.post(ctx, "/embeddings", "application/json", body)
	if err != nil {
		c.record(ctx, "embeddings", body, started, usage{}, 0, err, status)
		return nil, err
	}
	var out struct {
		Data []struct {
			Index     int       `json:"index"`
			Embedding []float32 `json:"embedding"`
		} `json:"data"`
		Usage usage `json:"usage"`
	}
	if err := json.Unmarshal(raw, &out); err != nil || len(out.Data) != len(inputs) {
		c.record(ctx, "embeddings", body, started, out.Usage, 0, errBadAnswer, 0)
		return nil, errBadAnswer
	}
	vecs := make([][]float32, len(inputs))
	for _, d := range out.Data {
		if d.Index < 0 || d.Index >= len(vecs) || len(d.Embedding) == 0 {
			c.record(ctx, "embeddings", body, started, out.Usage, 0, errBadAnswer, 0)
			return nil, errBadAnswer
		}
		vecs[d.Index] = d.Embedding
	}
	c.record(ctx, "embeddings", body, started, out.Usage, 0, nil, 0)
	return vecs, nil
}
