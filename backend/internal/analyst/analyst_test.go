package analyst

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"bugun/internal/aitest"
	"bugun/internal/ask"
	"bugun/internal/catalogmatch"
	"bugun/internal/insight"
	"bugun/internal/llm"
	"bugun/internal/market"
	"bugun/internal/pricing"
	"bugun/internal/store/db"
)

const (
	eggplant = 34 // baqlajon: three live prices in Namangan
	peach    = 53 // shaftoli: no prices
)

// setup gives a rule-only analyst over a database with three eggplant prices.
func setup(t *testing.T) (*pgxpool.Pool, *db.Queries, *ask.Service, *pricing.Service) {
	t.Helper()
	pool := aitest.DB(t)
	ctx := context.Background()
	clean := func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM price_observations WHERE product_id IN ($1, $2)`, eggplant, peach)
	}
	clean()
	t.Cleanup(clean)
	q := db.New(pool)
	listings := market.NewService(pool)
	for i, price := range []int64{8000, 8400, 9000} {
		u := aitest.User(t, q, int64(900500001+i), "entrepreneur")
		if _, err := listings.Upsert(ctx, u, market.Input{ProductID: eggplant, PriceSom: price, TradeType: "ulgurji"}); err != nil {
			t.Fatal(err)
		}
	}
	prices, err := pricing.NewService(ctx, pool)
	if err != nil {
		t.Fatal(err)
	}
	asker, err := ask.NewService(ctx, q, prices)
	if err != nil {
		t.Fatal(err)
	}
	return pool, q, asker, prices
}

func TestRulesAnswer(t *testing.T) {
	pool, q, asker, prices := setup(t)
	s := New(q, asker, prices, insight.NewService(pool))
	ctx := context.Background()

	a, err := s.Ask(ctx, Request{Question: "Namanganda baqlajon qancha?", Key: "ip:1"})
	if err != nil {
		t.Fatal(err)
	}
	if a.Mode != "rules" || a.Status != StatusOK || a.Rules == nil || len(a.Sources) != 1 || a.Sources[0].Tool != ToolPriceStats {
		t.Fatalf("answer %+v", a)
	}
	src := a.Sources[0]
	if src.Observations != 3 || !src.Enough || src.Params["product"] != "baqlajon" || src.DataFrom == "" {
		t.Errorf("source %+v", src)
	}
	if !strings.Contains(a.Text, "8 400") || a.Table == nil || a.Table.Rows[0][1] != float64(8400) {
		t.Errorf("median 8 400 in text and table: %q %+v", a.Text, a.Table)
	}

	// Russian, routed to the cheapest offers without the Uzbek /savol reading.
	a, err = s.Ask(ctx, Request{Question: "где дешевле баклажан?", Lang: "ru"})
	if err != nil {
		t.Fatal(err)
	}
	if a.Sources[0].Tool != ToolCheapestOffers || a.Lang != "ru" || !strings.Contains(a.Text, "8 000 сум") || len(a.Table.Rows) != 3 {
		t.Errorf("cheapest: %+v", a)
	}

	a, _ = s.Ask(ctx, Request{Question: "shaftoli qancha?"})
	if a.Status != StatusInsufficient || !strings.Contains(a.Text, "yetarli emas") {
		t.Errorf("no prices must say so: %+v", a)
	}
	a, _ = s.Ask(ctx, Request{Question: "velosiped qancha?", Lang: "en"})
	if a.Status != StatusNoProduct || !strings.Contains(a.Text, "No product") {
		t.Errorf("unknown product: %+v", a)
	}
	if _, err := s.Ask(ctx, Request{Question: "  "}); err != ErrEmpty {
		t.Errorf("empty: %v", err)
	}
}

// toolMessage returns the data of the last function result in a request.
func toolMessage(req llm.ChatRequest) map[string]any {
	for i := len(req.Messages) - 1; i >= 0; i-- {
		if m := req.Messages[i]; m.Role == "tool" {
			var v map[string]any
			_ = json.Unmarshal([]byte(m.Text()), &v)
			return v
		}
	}
	return nil
}

func TestModelAnswer(t *testing.T) {
	pool, q, asker, prices := setup(t)
	fake := aitest.NewFake(t)
	rec := &aitest.Recorder{}
	model := fake.Client("chat-test").WithRecorder(rec)
	m, err := catalogmatch.Load(context.Background(), q, catalogmatch.DefaultThresholds)
	if err != nil {
		t.Fatal(err)
	}
	s := New(q, asker, prices, insight.NewService(pool), WithModel(model, 3, 100), WithMatcher(m))
	ctx := context.Background()

	var final func(data map[string]any) string
	fake.Chat = func(req llm.ChatRequest) llm.Message {
		if data := toolMessage(req); data != nil {
			return llm.Message{Content: final(data)}
		}
		return aitest.ToolCall("call_1", ToolPriceStats, map[string]string{"product": "баклажан", "region": "Namangan"})
	}

	// A good answer: its numbers are the function's.
	final = func(d map[string]any) string {
		return fmt.Sprintf("Namanganda baqlajonning oʻrtacha ulgurji narxi %s soʻm/kg, %v ta narx boʻyicha.", som(d["median"].(float64)), d["observations"])
	}
	a, err := s.Ask(ctx, Request{Question: "Namanganda baqlajon qancha turibdi?", Key: "u:1"})
	if err != nil {
		t.Fatal(err)
	}
	if a.Mode != "model" || a.Fallback != "" || a.Text != "Namanganda baqlajonning oʻrtacha ulgurji narxi 8 400 soʻm/kg, 3 ta narx boʻyicha." {
		t.Fatalf("answer %+v", a)
	}
	if len(a.Sources) != 1 || a.Sources[0].Params["region"] != "Namangan viloyati" || a.Table == nil {
		t.Errorf("sources %+v table %+v", a.Sources, a.Table)
	}
	req := fake.Requests[0]
	if len(req.Tools) != len(ToolNames) || req.Messages[1].Text() != "Namanganda baqlajon qancha turibdi?" {
		t.Errorf("first request %+v", req)
	}
	if f := rec.Features(); len(f) != 2 || f[0] != "analyst" {
		t.Errorf("ai log %v", f)
	}

	// A made-up number: the text is replaced by the templated one.
	final = func(d map[string]any) string {
		return "Baqlajon oʻrtacha 9 950 soʻm, ertaga 12 000 soʻm boʻladi."
	}
	a, err = s.Ask(ctx, Request{Question: "Namanganda baqlajon qancha?", Key: "u:1"})
	if err != nil {
		t.Fatal(err)
	}
	if a.Mode != "model" || a.Fallback != "unchecked_number" || strings.Contains(a.Text, "9 950") || !strings.Contains(a.Text, "8 400") {
		t.Errorf("hallucinated number kept: %+v", a)
	}

	// No data: the model's guess is not shown.
	fake.Chat = func(req llm.ChatRequest) llm.Message {
		if toolMessage(req) != nil {
			return llm.Message{Content: "Shaftoli taxminan 15 000 soʻm."}
		}
		return aitest.ToolCall("call_2", ToolPriceStats, map[string]string{"product": "shaftoli"})
	}
	a, _ = s.Ask(ctx, Request{Question: "shaftoli necha pul?", Key: "u:1"})
	if a.Status != StatusInsufficient || strings.Contains(a.Text, "15 000") || !strings.Contains(a.Text, "yetarli emas") {
		t.Errorf("insufficient: %+v", a)
	}

	// Numbers without calling a function: the rules answer instead.
	fake.Chat = func(llm.ChatRequest) llm.Message { return llm.Message{Content: "Baqlajon 7 000 soʻm."} }
	a, _ = s.Ask(ctx, Request{Question: "Namanganda baqlajon qancha?", Key: "u:1"})
	if a.Mode != "rules" || a.Fallback != "no_tools" || !strings.Contains(a.Text, "8 400") {
		t.Errorf("no tools: %+v", a)
	}

	// Per-user rate limit and the model failing both fall back to the rules.
	limited := New(q, asker, prices, insight.NewService(pool), WithModel(model, 3, 1))
	fake.Chat = func(llm.ChatRequest) llm.Message {
		return llm.Message{Content: "Faqat narx savollariga javob beraman."}
	}
	if a, _ := limited.Ask(ctx, Request{Question: "salom", Key: "u:2"}); a.Mode != "model" || a.Text != "Faqat narx savollariga javob beraman." {
		t.Errorf("a numberless answer without data is fine: %+v", a)
	}
	if a, _ := limited.Ask(ctx, Request{Question: "baqlajon qancha?", Key: "u:2"}); a.Mode != "rules" || a.Fallback != "rate" {
		t.Errorf("rate limit: %+v", a)
	}
	fake.Status = 500
	if a, _ := s.Ask(ctx, Request{Question: "baqlajon qancha?", Key: "u:3"}); a.Mode != "rules" || a.Fallback != "error" || a.Status != StatusOK {
		t.Errorf("model error: %+v", a)
	}
}

func TestRoute(t *testing.T) {
	for q, want := range map[string]string{
		"Samarqandda piyoz qancha?":                 ToolPriceStats,
		"Eng arzon kartoshka kimda?":                ToolCheapestOffers,
		"Qaysi viloyatda pomidor arzon?":            ToolCompareRegions,
		"Piyoz narxi keyingi hafta qanday boʻladi?": ToolForecast,
		"Сколько стоит лук?":                        ToolPriceStats,
		"Прогноз цены на картошку":                  ToolForecast,
		"Which products are most wanted?":           ToolTopDemand,
		"Piyozga talab qanday?":                     ToolPressure,
		"Oxirgi haftalarda olma narxi oshdimi?":     ToolPriceTrend,
	} {
		if got := Route(q); got != want {
			t.Errorf("%q → %s, want %s", q, got, want)
		}
	}
}
