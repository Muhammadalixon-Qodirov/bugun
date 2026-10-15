package bot

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	tg "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"bugun/internal/aitest"
	"bugun/internal/pricing"
	"bugun/internal/store/db"
)

// fakeTelegram answers getFile, serves the file and records sent messages.
type fakeTelegram struct {
	*httptest.Server
	mu   sync.Mutex
	sent []string
}

func newFakeTelegram(t *testing.T, file []byte) *fakeTelegram {
	f := &fakeTelegram{}
	f.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/getFile"):
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "result": map[string]any{"file_id": "f1", "file_path": "voice/file_1.oga"}})
		case strings.HasPrefix(r.URL.Path, "/file/bot"):
			_, _ = w.Write(file)
		case strings.HasSuffix(r.URL.Path, "/sendMessage"):
			_ = r.ParseMultipartForm(1 << 20)
			f.mu.Lock()
			f.sent = append(f.sent, r.FormValue("text"))
			f.mu.Unlock()
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "result": map[string]any{"message_id": 1, "date": 0, "chat": map[string]any{"id": 1, "type": "private"}}})
		default:
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "result": true})
		}
	}))
	t.Cleanup(f.Close)
	return f
}

func (f *fakeTelegram) messages() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.sent...)
}

func TestVoicePrice(t *testing.T) {
	pool := aitest.DB(t)
	ctx := context.Background()
	q := db.New(pool)
	prices, err := pricing.NewService(ctx, pool)
	if err != nil {
		t.Fatal(err)
	}
	seller := aitest.User(t, q, 900800001, "entrepreneur")

	voice := append([]byte("OggS"), make([]byte, 100)...)
	telegram := newFakeTelegram(t, voice)
	api, err := tg.New("123:test", tg.WithServerURL(telegram.URL), tg.WithSkipGetMe())
	if err != nil {
		t.Fatal(err)
	}
	b := &Bot{api: api, q: q, prices: prices, limit: newThrottle(100)}
	upd := &models.Update{Message: &models.Message{
		Chat: models.Chat{ID: 1}, From: &models.User{ID: seller.TelegramID},
		Voice: &models.Voice{FileID: "f1", Duration: 4},
	}}

	// Speech to text off: a polite answer, nothing else.
	b.fallback(ctx, nil, upd)
	if got := telegram.messages(); len(got) != 1 || !strings.Contains(got[0], "Ovozli xabarlar hozircha qabul qilinmaydi") {
		t.Fatalf("off: %q", got)
	}

	stt := aitest.NewFake(t)
	stt.Transcribe = func(audio []byte, fields map[string]string) string {
		if string(audio[:4]) != "OggS" || fields["language"] != "uz" {
			return ""
		}
		return "Pomidor kilosi sakkiz ming"
	}
	b.ai = AI{STT: stt.Client("stt-test"), STTLanguage: "uz", STTMaxSeconds: 60}
	b.fallback(ctx, nil, upd)
	got := telegram.messages()[1:]
	if len(got) != 2 || !strings.Contains(got[0], "Pomidor kilosi sakkiz ming") || !strings.Contains(got[1], "<b>Pomidor</b> — 8 000 so'm/kg") {
		t.Fatalf("voice → card: %q", got)
	}

	// Too long: refused before downloading.
	upd.Message.Text, upd.Message.Voice = "", &models.Voice{FileID: "f1", Duration: 120}
	b.fallback(ctx, nil, upd)
	if got := telegram.messages(); !strings.Contains(got[len(got)-1], "juda uzun") || stt.Audio != 1 {
		t.Errorf("long voice: %q, transcriptions %d", got[len(got)-1], stt.Audio)
	}
}
