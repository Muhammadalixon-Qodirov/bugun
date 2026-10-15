// Package advice words the buy/sell advice of the analytics page (spec AI-6)
// as a short plain-language text. The advice itself — the verdict and every
// reason with its number — is computed by insight.AdviceOf; by default the
// text is a template over those reasons. With ADVICE_TEXT_ENABLED a chat
// model rewords them, and a text with any number the reasons do not hold is
// replaced by the template.
package advice

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"strings"
	"sync"
	"time"

	"bugun/internal/insight"
	"bugun/internal/llm"
	"bugun/internal/numcheck"
)

// Text is the advice in words.
type Text struct {
	Text     string               `json:"text"`
	Mode     string               `json:"mode" enum:"template,model" doc:"template: fixed sentences over the reason codes; model: reworded by a language model, checked to add no number"`
	Fallback string               `json:"fallback,omitempty" enum:"limit,error,unchecked_number" doc:"Why the model's text was not used, when a model is configured"`
	Lang     string               `json:"lang" enum:"uz,ru,en"`
	Buyer    string               `json:"buyer" enum:"buy_now,wait,neutral"`
	Seller   string               `json:"seller" enum:"sell_now,hold,neutral"`
	Items    []insight.AdviceItem `json:"items" doc:"The reasons as codes with their numbers, as on the analytics page"`
}

// Phraser writes advice texts; with a model it caches them for a while.
type Phraser struct {
	model *llm.Client // nil: templates only

	mu    sync.Mutex
	cache map[string]cached
}

type cached struct {
	at time.Time
	t  Text
}

// cacheFor matches the analytics page's cache.
const cacheFor = 10 * time.Minute

func New(model *llm.Client) *Phraser { return &Phraser{model: model, cache: map[string]cached{}} }

func langOf(l string) string {
	switch l {
	case "ru", "en":
		return l
	}
	return "uz"
}

// Phrase words advice a for a product; key identifies the page (product,
// place, trade) for the cache.
func (p *Phraser) Phrase(ctx context.Context, key, product string, a insight.Advice, lang string) Text {
	lang = langOf(lang)
	t := Text{Mode: "template", Lang: lang, Buyer: a.Buyer, Seller: a.Seller, Items: a.Items, Text: Template(product, a, lang)}
	if t.Items == nil {
		t.Items = []insight.AdviceItem{}
	}
	if p.model == nil || len(a.Items) == 0 {
		return t
	}
	key += "|" + lang
	p.mu.Lock()
	if c, ok := p.cache[key]; ok && time.Since(c.at) < cacheFor {
		p.mu.Unlock()
		return c.t
	}
	p.mu.Unlock()

	text, err := p.reword(llm.WithFeature(ctx, "advice"), product, a, t.Text, lang)
	switch {
	case err == llm.ErrLimit:
		t.Fallback = "limit"
	case err != nil:
		slog.WarnContext(ctx, "advice model", "err", err)
		t.Fallback = "error"
	default:
		allow := numcheck.New()
		allow.AddText(t.Text)
		allow.AddJSON(a.Items)
		if bad := allow.Unchecked(text); len(bad) > 0 || text == "" {
			slog.WarnContext(ctx, "advice text replaced", "numbers_not_from_reasons", len(bad))
			t.Fallback = "unchecked_number"
		} else {
			t.Text, t.Mode = text, "model"
		}
	}
	p.mu.Lock()
	if len(p.cache) > 5000 {
		p.cache = map[string]cached{}
	}
	p.cache[key] = cached{time.Now(), t}
	p.mu.Unlock()
	return t
}

const prompt = `Sen bozor maslahatini oddiy tilga oʻgirasan. Senga mahsulot, xulosa va hisoblangan sabablar beriladi.
Ularni %s tilida 2–3 ta sodda gap bilan qayta yoz: avval xulosa (hozir olish/sotish yoki kutish), keyin asosiy sabablar.
Qoidalar: faqat berilgan sabablardagi raqamlarni ishlat; yangi raqam, foiz, sana yoki bashorat qoʻshma; hech narsani hisoblama.
Faqat matnni qaytar, markdownsiz.`

var langNames = map[string]string{"uz": "oʻzbek", "ru": "rus", "en": "ingliz"}

func (p *Phraser) reword(ctx context.Context, product string, a insight.Advice, template, lang string) (string, error) {
	items, _ := json.Marshal(a.Items)
	user := fmt.Sprintf("Mahsulot: %s\nXaridorga: %s\nSotuvchiga: %s\nSabablar:\n%s\nKodlar: %s",
		product, a.Buyer, a.Seller, template, items)
	msg, err := p.model.Chat(ctx, llm.ChatRequest{Temperature: 0.2, MaxTokens: 300, Messages: []llm.Message{
		{Role: "system", Content: fmt.Sprintf(prompt, langNames[lang])},
		{Role: "user", Content: user},
	}})
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(msg.Text()), nil
}

var verdicts = map[string][3]string{
	"buy_now":  {"Xaridorga: hozir olgan maʼqul.", "Покупателю: лучше купить сейчас.", "For buyers: better to buy now."},
	"wait":     {"Xaridorga: shoshmay turish mumkin.", "Покупателю: можно подождать.", "For buyers: it can wait."},
	"sell_now": {"Sotuvchiga: hozir sotgan maʼqul.", "Продавцу: лучше продать сейчас.", "For sellers: better to sell now."},
	"hold":     {"Sotuvchiga: shoshmay turish mumkin.", "Продавцу: можно не спешить с продажей.", "For sellers: no need to rush."},
	"neutral":  {"Aniq signal yoʻq: narx odatdagidek.", "Явного сигнала нет: цена обычная.", "No clear signal: the price is usual."},
}

var reasons = map[string][3]string{
	"trend_up":       {"", "За последние 4 недели цена растёт: примерно на %s в неделю.", "Over the last 4 weeks the price has been rising: about %s a week."},
	"trend_down":     {"", "За последние 4 недели цена снижается: примерно на %s в неделю.", "Over the last 4 weeks the price has been falling: about %s a week."},
	"season_dearer":  {"", "Обычно в %s цена на %s выше, чем в %s.", "In %s the price is usually %s higher than in %s."},
	"season_cheaper": {"", "Обычно в %s цена на %s ниже, чем в %s.", "In %s the price is usually %s lower than in %s."},
	"surplus":        {"", "Сейчас предложение выше обычного: удобно покупать.", "Supply is higher than usual now: a good time to buy."},
	"shortage":       {"", "Сейчас спрос выше обычного: цена может вырасти.", "Demand is higher than usual now: the price may rise."},
	"below_year":     {"", "Цена на %s ниже среднегодовой.", "The price is %s below its yearly average."},
	"above_year":     {"", "Цена на %s выше среднегодовой.", "The price is %s above its yearly average."},
	"holiday_soon":   {"", "До «%s» осталось %d дн.: спрос и цена обычно растут.", "%d days until %s: demand and prices usually rise."},
	"holiday_now":    {"", "Сейчас «%s» (до %s): спрос выше обычного.", "Now: %s (until %s), demand is higher than usual."},
}

var months = map[string][12]string{
	"ru": {"январе", "феврале", "марте", "апреле", "мае", "июне", "июле", "августе", "сентябре", "октябре", "ноябре", "декабре"},
	"en": {"January", "February", "March", "April", "May", "June", "July", "August", "September", "October", "November", "December"},
}

func idx(lang string) int {
	switch lang {
	case "ru":
		return 1
	case "en":
		return 2
	}
	return 0
}

func pct(x float64) string { return fmt.Sprintf("%.0f%%", math.Abs(x)*100) }

// Template is the advice as fixed sentences: the verdict, then each reason
// (in Uzbek the reasons insight wrote; in Russian and English from the codes).
func Template(product string, a insight.Advice, lang string) string {
	lang = langOf(lang)
	i := idx(lang)
	var parts []string
	if a.Buyer == "neutral" {
		parts = append(parts, verdicts["neutral"][i])
	} else {
		// Separate lookups: go1.26.2 with -race miscompiled two map-array
		// index expressions in one append (both became the second).
		buyer := verdicts[a.Buyer][i]
		seller := verdicts[a.Seller][i]
		parts = append(parts, buyer, seller)
	}
	if lang == "uz" {
		parts = append(parts, a.Reasons...)
		return strings.Join(parts, " ")
	}
	for _, it := range a.Items {
		f := reasons[it.Code][i]
		m := months[lang]
		month := func(n int) string {
			if n >= 1 && n <= 12 {
				return m[n-1]
			}
			return ""
		}
		switch it.Code {
		case "trend_up", "trend_down", "below_year", "above_year":
			parts = append(parts, fmt.Sprintf(f, pct(it.Share)))
		case "season_dearer", "season_cheaper":
			parts = append(parts, fmt.Sprintf(f, month(it.Month), pct(it.Share), month(it.FromMonth)))
		case "holiday_soon":
			if lang == "ru" {
				parts = append(parts, fmt.Sprintf(f, it.Title, it.Days))
			} else {
				parts = append(parts, fmt.Sprintf(f, it.Days, it.Title))
			}
		case "holiday_now":
			parts = append(parts, fmt.Sprintf(f, it.Title, it.Until))
		case "surplus", "shortage":
			parts = append(parts, f)
		}
	}
	return strings.Join(parts, " ")
}
