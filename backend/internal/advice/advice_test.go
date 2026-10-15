package advice

import (
	"context"
	"strings"
	"testing"
	"time"

	"bugun/internal/aitest"
	"bugun/internal/insight"
	"bugun/internal/llm"
)

func sample() insight.Advice {
	w := 0.052
	return insight.AdviceOf(insight.AdviceInput{
		Trend: &insight.Trend{Direction: "up", Weekly: w}, Zone: "surplus",
		Today: time.Date(2026, 9, 26, 0, 0, 0, 0, time.UTC),
	})
}

func TestTemplate(t *testing.T) {
	a := sample()
	if a.Buyer != "buy_now" {
		t.Fatalf("advice %+v", a)
	}
	uz := Template("Piyoz", a, "uz")
	if !strings.HasPrefix(uz, "Xaridorga: hozir olgan maʼqul.") || !strings.Contains(uz, "haftasiga taxminan 5%") {
		t.Errorf("uz: %q", uz)
	}
	if ru := Template("Piyoz", a, "ru"); !strings.Contains(ru, "примерно на 5% в неделю") || !strings.Contains(ru, "удобно покупать") {
		t.Errorf("ru: %q", ru)
	}
	if en := Template("Piyoz", insight.Advice{Buyer: "neutral", Seller: "neutral"}, "en"); en != "No clear signal: the price is usual." {
		t.Errorf("en: %q", en)
	}
}

func TestPhraseWithModel(t *testing.T) {
	fake := aitest.NewFake(t)
	reply := "Piyozni hozir olgan maʼqul: narx haftasiga taxminan 5% oshmoqda, taklif esa koʻp."
	fake.Chat = func(llm.ChatRequest) llm.Message { return llm.Message{Content: reply} }
	p := New(fake.Client("chat"))
	ctx := context.Background()

	got := p.Phrase(ctx, "2|1|ulgurji", "Piyoz", sample(), "uz")
	if got.Mode != "model" || got.Text != reply || len(got.Items) != 2 {
		t.Errorf("reworded: %+v", got)
	}
	// Cached for the same page and language.
	if p.Phrase(ctx, "2|1|ulgurji", "Piyoz", sample(), "uz"); fake.ChatCount() != 1 {
		t.Errorf("calls %d", fake.ChatCount())
	}
	// A new number is not allowed: the template comes back.
	reply = "Piyoz keyingi haftada 7 200 soʻm boʻladi, 15% oshadi."
	got = p.Phrase(ctx, "2|0|ulgurji", "Piyoz", sample(), "uz")
	if got.Mode != "template" || got.Fallback != "unchecked_number" || strings.Contains(got.Text, "7 200") {
		t.Errorf("new number kept: %+v", got)
	}
	fake.Status = 500
	if got := p.Phrase(ctx, "3|0|ulgurji", "Piyoz", sample(), "ru"); got.Mode != "template" || got.Fallback != "error" || !strings.Contains(got.Text, "Покупателю") {
		t.Errorf("model down: %+v", got)
	}
	// Without a model, or without reasons, nothing is called.
	if got := New(nil).Phrase(ctx, "k", "Piyoz", sample(), "en"); got.Mode != "template" || got.Fallback != "" {
		t.Errorf("off: %+v", got)
	}
}
