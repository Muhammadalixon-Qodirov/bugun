package moderation

import (
	"context"
	"strings"
	"testing"
	"time"

	"bugun/internal/aitest"
	"bugun/internal/llm"
	"bugun/internal/market"
	"bugun/internal/store/db"
)

func TestRules(t *testing.T) {
	for text, want := range map[string]string{
		"Qizil, shirin, Chust pomidori":                  "",
		"Нашатырный спирт yo'q, faqat pomidor":           "",
		"Montaj pistoleti uchun ko'pik, patron 220V":     "",
		"Batafsil: https://example.com/aksiya":           ReasonSpam,
		"Kanalga obuna bo'ling t.me/arzon":               ReasonSpam,
		"Nasha bor, arzon":                               ReasonProhibited,
		"Продаю оружие":                                  ReasonProhibited,
		"Sotuvchi jalab":                                 ReasonAbuse,
		"Qozon kazinosi — yutuqli o'yin":                 ReasonSpam,
		"kartoshka 4.5 ming so'm, 2 tonna, ertaga bozor": "",
	} {
		if got := Rules(text); got != want {
			t.Errorf("%q: %q, want %q", text, got, want)
		}
	}
}

var tomato = db.GetProductRow{ID: 4, Slug: "pomidor", Name: "Pomidor", Category: "sabzavot", BaseUnit: "kg"}

func TestModelCheck(t *testing.T) {
	fake := aitest.NewFake(t)
	answer := `{"category":"off_category","confidence":0.9}`
	fake.Chat = func(req llm.ChatRequest) llm.Message { return llm.Message{Content: answer} }
	rec := &aitest.Recorder{}
	c := New(fake.Client("mod").WithRecorder(rec), time.Second)
	ctx := context.Background()

	if got := c.Check(ctx, tomato, "", "iPhone 13 sotiladi, holati zo'r"); got != ReasonOffCategory {
		t.Errorf("off category: %q", got)
	}
	if u := fake.Requests[0].Messages[1].Text(); !strings.Contains(u, "Mahsulot: Pomidor (sabzavot)") {
		t.Errorf("prompt %q", u)
	}
	if f := rec.Features(); len(f) != 1 || f[0] != "moderation" {
		t.Errorf("ai log %v", f)
	}
	answer = `{"category":"spam","confidence":0.4}`
	if got := c.Check(ctx, tomato, "", "Eng yaxshi pomidor"); got != "" {
		t.Errorf("an unsure flag holds nothing: %q", got)
	}
	// The rules decide without a call; empty text is not checked.
	n := fake.ChatCount()
	if got := c.Check(ctx, tomato, "", "t.me/kanal"); got != ReasonSpam || fake.ChatCount() != n {
		t.Errorf("rules first: %q, calls %d→%d", got, n, fake.ChatCount())
	}
	if got := c.Check(ctx, tomato, "", ""); got != "" || fake.ChatCount() != n {
		t.Error("empty text")
	}
	// A failing or slow model never holds a listing.
	fake.Status = 500
	if got := c.Check(ctx, tomato, "", "Eng yaxshi pomidor"); got != "" {
		t.Errorf("model error: %q", got)
	}
	fake.Status = 0
	fake.Chat = func(llm.ChatRequest) llm.Message {
		time.Sleep(300 * time.Millisecond)
		return llm.Message{Content: `{"category":"abuse","confidence":1}`}
	}
	start := time.Now()
	if got := New(fake.Client("mod"), 50*time.Millisecond).Check(ctx, tomato, "", "Eng yaxshi pomidor"); got != "" || time.Since(start) > 250*time.Millisecond {
		t.Errorf("timeout: %q after %s", got, time.Since(start))
	}
}

func TestListingHeldForReview(t *testing.T) {
	pool := aitest.DB(t)
	ctx := context.Background()
	q := db.New(pool)
	seller := aitest.User(t, q, 900700001, "entrepreneur")
	fake := aitest.NewFake(t)
	fake.Chat = func(req llm.ChatRequest) llm.Message {
		if strings.Contains(req.Messages[1].Text(), "telefon") {
			return llm.Message{Content: `{"category":"off_category","confidence":0.95}`}
		}
		return llm.Message{Content: `{"category":"ok","confidence":0.99}`}
	}
	listings := market.NewService(pool, market.WithContentChecker(New(fake.Client("mod"), time.Second)))

	ok, err := listings.Upsert(ctx, seller, market.Input{ProductID: 36, PriceSom: 3000, TradeType: "ulgurji", Description: "Shirin qovoq, Chustdan"})
	if err != nil || ok.Status != market.StatusActive {
		t.Fatalf("clean text: %+v %v", ok, err)
	}
	held, err := listings.Upsert(ctx, seller, market.Input{ProductID: 36, Variety: "katta", PriceSom: 3000, TradeType: "ulgurji", Description: "Arzon telefon sotaman"})
	if err != nil || held.Status != market.StatusReview {
		t.Fatalf("off-category text: %+v %v", held, err)
	}
	l, _ := q.GetListing(ctx, held.ID)
	if l.ReviewReason.String != ReasonOffCategory {
		t.Errorf("reason %q", l.ReviewReason.String)
	}
	// Editing the text clean again lets it go live; a failing model never holds it.
	fake.Status = 503
	upd, err := listings.Update(ctx, seller, ok.ID, market.Input{PriceSom: 3100, TradeType: "ulgurji", Description: "Yana telefon"})
	if err != nil || upd.Status != market.StatusActive {
		t.Errorf("model down: %+v %v", upd, err)
	}
	upd, err = listings.Update(ctx, seller, ok.ID, market.Input{PriceSom: 3100, TradeType: "ulgurji", Description: "https://spam.example"})
	if err != nil || upd.Status != market.StatusReview {
		t.Errorf("rules still hold it: %+v %v", upd, err)
	}
}
