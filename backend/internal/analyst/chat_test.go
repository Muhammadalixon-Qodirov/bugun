package analyst

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"bugun/internal/aitest"
	"bugun/internal/catalogmatch"
	"bugun/internal/insight"
	"bugun/internal/llm"
)

func TestConverse(t *testing.T) {
	pool, q, asker, prices := setup(t)
	fake := aitest.NewFake(t)
	m, err := catalogmatch.Load(context.Background(), q, catalogmatch.DefaultThresholds)
	if err != nil {
		t.Fatal(err)
	}
	s := New(q, asker, prices, insight.NewService(pool), WithModel(fake.Client("chat-test"), 4, 100), WithMatcher(m))
	ctx := context.Background()
	keys := []string{"chat-test:1", "chat-test:2"}
	t.Cleanup(func() {
		for _, k := range keys {
			_, _ = pool.Exec(context.Background(), `DELETE FROM chat_unanswered WHERE session_id IN (SELECT id FROM chat_sessions WHERE key_hash = $1)`, keyHash(k))
			_, _ = pool.Exec(context.Background(), `DELETE FROM chat_sessions WHERE key_hash = $1`, keyHash(k))
		}
	})

	// 1. An unclear product: a question back with buttons.
	fake.Chat = func(req llm.ChatRequest) llm.Message {
		return aitest.ToolCall("c1", ToolAskUser, map[string]any{"question": "Qaysi qalampir: achchiqmi yoki bulgʻor?",
			"options": []string{"Achchiq qalampir", "Bulgʻor qalampiri"}})
	}
	a, err := s.Converse(ctx, ChatRequest{Message: "qalampir qancha?", Key: keys[0]})
	if err != nil {
		t.Fatal(err)
	}
	if a.Kind != KindClarify || len(a.Options) != 2 || a.SessionID == "" || a.Text == "" {
		t.Fatalf("clarify: %+v", a)
	}
	session := a.SessionID
	if opts := s.Options(ctx, session); len(opts) != 2 {
		t.Errorf("stored options %v", opts)
	}

	// 2. A follow-up: the earlier turns go to the model, and a number from
	// them may be repeated.
	var seen llm.ChatRequest
	fake.Chat = func(req llm.ChatRequest) llm.Message {
		seen = req
		if d := toolMessage(req); d != nil {
			return llm.Message{Content: fmt.Sprintf("Namanganda baqlajon %s soʻm/kg (%v ta narx).", som(d["median"].(float64)), d["observations"])}
		}
		return aitest.ToolCall("c2", ToolPriceStats, map[string]string{"product": "baqlajon", "region": "Namangan"})
	}
	a, err = s.Converse(ctx, ChatRequest{SessionID: session, Message: "Unda baqlajon-chi, Namanganda?", Key: keys[0]})
	if err != nil {
		t.Fatal(err)
	}
	if a.Kind != KindAnswer || a.SessionID != session || !strings.Contains(a.Text, "8 400") || a.Mode != "model" {
		t.Fatalf("follow-up: %+v", a)
	}
	if len(seen.Messages) < 4 || seen.Messages[1].Text() != "qalampir qancha?" || seen.Messages[2].Role != "assistant" {
		t.Errorf("history not sent: %+v", seen.Messages)
	}
	names := map[string]bool{}
	for _, tl := range seen.Tools {
		names[tl.Function.Name] = true
	}
	if !names[ToolAskUser] || !names[ToolCannotAnswer] || names[ToolExchangeRate] {
		t.Errorf("tools offered: %v", names)
	}

	// 3. Off topic: said plainly and logged for the admins.
	fake.Chat = func(llm.ChatRequest) llm.Message {
		return aitest.ToolCall("c3", ToolCannotAnswer, map[string]string{"reason": "off_topic", "topic": "futbol"})
	}
	a, err = s.Converse(ctx, ChatRequest{SessionID: session, Message: "Kecha futbol qanday tugadi?", Key: keys[0]})
	if err != nil {
		t.Fatal(err)
	}
	if a.Kind != KindUnanswered || !strings.Contains(a.Text, "bozor narxlari") {
		t.Fatalf("off topic: %+v", a)
	}
	var reason, topic string
	if err := pool.QueryRow(ctx, `SELECT reason, topic FROM chat_unanswered WHERE session_id = $1`, session).Scan(&reason, &topic); err != nil || reason != "off_topic" || topic != "futbol" {
		t.Errorf("unanswered log: %q %q %v", reason, topic, err)
	}

	// 4. A basket: every price and the total come from the function; an item
	// without a price is left out and said so.
	fake.Chat = func(req llm.ChatRequest) llm.Message {
		if d := toolMessage(req); d != nil {
			return llm.Message{Content: fmt.Sprintf("2 kg baqlajon: jami %s soʻm. Velosiped narxi bazada yoʻq.", som(d["total"].(float64)))}
		}
		return aitest.ToolCall("c4", ToolBasketCost, map[string]any{"trade": "ulgurji", "items": []map[string]any{
			{"product": "baqlajon", "quantity": 2, "unit": "kg"}, {"product": "velosiped", "quantity": 1}}})
	}
	a, err = s.Converse(ctx, ChatRequest{Message: "2 kg baqlajon va velosiped, hisoblab ber", Key: keys[1]})
	if err != nil {
		t.Fatal(err)
	}
	if a.Kind != KindAnswer || a.Fallback != "" || !strings.Contains(a.Text, "16 800") || a.Table == nil || len(a.Table.Rows) != 1 {
		t.Fatalf("basket: %+v", a)
	}

	// 5. A proposal comes back as an action to confirm, nothing is created.
	fake.Chat = func(req llm.ChatRequest) llm.Message {
		if toolMessage(req) != nil {
			return llm.Message{Content: "Soʻrovni yuborish uchun tugmani bosing."}
		}
		return aitest.ToolCall("c5", ToolCreateRequest, map[string]any{"product": "baqlajon", "quantity": 100, "region": "Namangan"})
	}
	a, err = s.Converse(ctx, ChatRequest{SessionID: a.SessionID, Message: "100 kg baqlajon sotib olmoqchiman, soʻrov yubor", Key: keys[1]})
	if err != nil {
		t.Fatal(err)
	}
	if len(a.Actions) != 1 || a.Actions[0].Type != ToolCreateRequest || a.Actions[0].Params["quantity"] != float64(100) {
		t.Fatalf("action: %+v", a.Actions)
	}

	// Someone else's session id starts a new conversation.
	fake.Chat = func(llm.ChatRequest) llm.Message {
		return llm.Message{Content: "Salom! Qaysi mahsulot narxini bilmoqchisiz?"}
	}
	other, err := s.Converse(ctx, ChatRequest{SessionID: session, Message: "salom", Key: keys[1]})
	if err != nil || other.SessionID == session {
		t.Fatalf("session taken over: %+v %v", other, err)
	}
}

func TestChatTools(t *testing.T) {
	has := func(text, tool string) bool {
		for _, tl := range chatTools(text) {
			if tl.Function.Name == tool {
				return true
			}
		}
		return false
	}
	for _, c := range []struct {
		text, tool string
		want       bool
	}{
		{"Osh qilmoqchiman, 10 kishilik, masalliqlar qancha?", ToolBasketCost, true},
		{"Dollar kursi qancha?", ToolExchangeRate, true},
		{"Piyoz qachon arzonlashadi?", ToolSeasonality, true},
		{"Nega goʻsht qimmatladi?", ToolPriceReasons, true},
		{"Narxni qanday joylayman?", ToolPlatformHelp, true},
		{"Namanganda piyoz qancha?", ToolBasketCost, false},
		{"salom", ToolAskUser, true},
		{"salom", ToolForecast, true},
	} {
		if got := has(c.text, c.tool); got != c.want {
			t.Errorf("%q: %s offered = %v", c.text, c.tool, got)
		}
	}
	// Every tool a group names exists.
	defs := map[string]bool{}
	for _, tl := range toolDefs() {
		defs[tl.Function.Name] = true
	}
	for name := range moreToolDefs() {
		defs[name] = true
	}
	for _, g := range toolGroups {
		for _, name := range g.tools {
			if !defs[name] {
				t.Errorf("group tool %s has no definition", name)
			}
		}
	}
	if len(moreToolDefs()) != len(allMoreTools) {
		t.Errorf("allMoreTools lists %d of %d", len(allMoreTools), len(moreToolDefs()))
	}
}

// Every further function runs on real data without an error and names
// itself in its source; its summary is not empty in any language.
func TestMoreTools(t *testing.T) {
	pool, q, asker, prices := setup(t)
	s := New(q, asker, prices, insight.NewService(pool))
	ctx := context.Background()
	calls := map[string]string{
		ToolFindProduct:     `{"name":"baqlajon"}`,
		ToolFindRegion:      `{"name":"Chust"}`,
		ToolAdvice:          `{"product":"baqlajon","region":"Namangan"}`,
		ToolSeasonality:     `{"product":"baqlajon"}`,
		ToolWeekdays:        `{"product":"baqlajon"}`,
		ToolPriceReasons:    `{"product":"baqlajon"}`,
		ToolDistrictPrices:  `{"product":"baqlajon","region":"Namangan"}`,
		ToolWholesaleRetail: `{"product":"baqlajon"}`,
		ToolArbitrage:       `{"product":"baqlajon"}`,
		ToolRegionOverview:  `{"region":"Namangan"}`,
		ToolWeeklyMovers:    `{}`,
		ToolCompareProducts: `{"products":["baqlajon","shaftoli"]}`,
		ToolBasketCost:      `{"items":[{"product":"baqlajon","quantity":1.5}],"trade":"ulgurji"}`,
		ToolExchangeRate:    `{"currency":"USD"}`,
		ToolWeather:         `{"region":"Namangan"}`,
		ToolMarketEvents:    `{"product":"baqlajon"}`,
		ToolSellerInfo:      `{"seller":"Test"}`,
		ToolPlatformHelp:    `{"topic":"post_price"}`,
		ToolCreateRequest:   `{"product":"baqlajon","quantity":50}`,
		ToolWatchPrice:      `{"product":"baqlajon","region":"Namangan"}`,
	}
	for name, raw := range calls {
		res, err := s.runMore(ctx, name, raw, "uz")
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		if res.source.Tool != name || res.source.Error != "" {
			t.Errorf("%s: source %+v", name, res.source)
		}
		for _, lang := range Langs {
			if res.summary == nil || strings.TrimSpace(res.summary(lang)) == "" {
				t.Errorf("%s: empty summary in %s", name, lang)
			}
		}
	}
	// Unknown things are told to the model, not errors.
	if res, err := s.runMore(ctx, ToolAdvice, `{"product":"velosiped"}`, "uz"); err != nil || res.source.Error != "unknown_product" {
		t.Errorf("unknown product: %+v %v", res.source, err)
	}
	if res, _ := s.runMore(ctx, ToolDistrictPrices, `{"product":"baqlajon"}`, "uz"); res.source.Error != "unknown_region" {
		t.Errorf("district prices need a viloyat: %+v", res.source)
	}
	res, _ := s.runMore(ctx, ToolBasketCost, `{"items":[{"product":"baqlajon","quantity":1.5}],"trade":"ulgurji"}`, "uz")
	if res.data["total"] != float64(12600) {
		t.Errorf("1.5 kg × 8 400 = 12 600, got %v", res.data["total"])
	}
}
