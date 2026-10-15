// Package moderation screens listing texts (spec AI-7): spam, abuse,
// prohibited goods and text about something other than the listing's
// product. Keyword rules always run; a chat model can also classify the text
// (MODERATION_ENABLED). A flag only holds the listing for an admin's review
// with a reason code, never rejects it, and a model failure or timeout never
// holds anything.
package moderation

import (
	"context"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"bugun/internal/llm"
	"bugun/internal/pricing"
	"bugun/internal/store/db"
)

// Review reasons (listings.review_reason).
const (
	ReasonSpam        = "content_spam"
	ReasonAbuse       = "content_abuse"
	ReasonProhibited  = "content_prohibited"
	ReasonOffCategory = "content_off_category"
)

// Checker implements market.ContentChecker.
type Checker struct {
	model   *llm.Client // nil: rules only
	timeout time.Duration
}

// New returns a checker; model may be nil (keyword rules only).
func New(model *llm.Client, timeout time.Duration) *Checker {
	if timeout <= 0 {
		timeout = 4 * time.Second
	}
	return &Checker{model: model, timeout: timeout}
}

var (
	linkRe = regexp.MustCompile(`(?i)(https?://|www\.|t\.me/|telegram\.me/|\b[a-z0-9-]+\.(com|uz|ru|net|org|info|me)\b)`)
	// Word stems in the parser's Latin form (Cyrillic is transliterated); a
	// stem ending in "=" must be the whole word ("nasha" but not "nashatir").
	// Tool words that name weapons elsewhere (montaj pistoleti, patron for a
	// lamp) are left out: the model judges those.
	prohibitedStems = []string{"narkotik", "nasha=", "geroin", "kokain", "marixuana", "marihuana", "gashish", "spays", "tramadol",
		"qurolyarog", "oruji", "vzrivchat", "portlovchi", "granata", "kontrabanda"}
	abuseStems = []string{"jalab", "qanjiq", "haromi", "dalbayob", "suka=", "blyad", "blyat", "xuy=", "pizd", "ebat", "eblan", "mudak", "pidor"}
	spamStems  = []string{"obuna", "podpish", "reklama", "viigrish", "kazino", "kripto", "bitkoin", "stavki=", "zarabotok"}
)

// Rules is the keyword check: "" or a reason.
func Rules(text string) string {
	if linkRe.MatchString(text) {
		return ReasonSpam
	}
	words := strings.FieldsFunc(pricing.Normalize(text), func(r rune) bool {
		return !(r >= 'a' && r <= 'z') && !(r >= '0' && r <= '9') && r <= 127
	})
	has := func(stems []string) bool {
		for _, w := range words {
			for _, s := range stems {
				if whole, ok := strings.CutSuffix(s, "="); ok && w == whole || !ok && strings.HasPrefix(w, s) {
					return true
				}
			}
		}
		return false
	}
	switch {
	case has(prohibitedStems):
		return ReasonProhibited
	case has(abuseStems):
		return ReasonAbuse
	case has(spamStems):
		return ReasonSpam
	}
	return ""
}

const prompt = `Sen Oʻzbekiston bozor eʼlonlari moderatorisan. Eʼlon matnini baholaysan: mahsulot, navi va tavsif.
Toifalar:
- ok — oddiy eʼlon;
- spam — reklama, havola, boshqa kanal yoki xizmatga chaqiruv, bir xil matn takrori;
- abuse — haqorat, soʻkinish, kamsitish;
- prohibited — taqiqlangan tovar (giyohvand modda, qurol, oʻq-dori, retseptli dori, spirt kontrabandasi va h.k.);
- off_category — matn eʼlondagi mahsulotga umuman aloqasi yoʻq narsani sotadi.
Shubhali boʻlsang "ok" deb qoʻy. Faqat JSON qaytar: {"category":"ok","confidence":0.0}`

// minConfidence: a model's flag below this is ignored.
const minConfidence = 0.7

// Check screens a listing's variety and description for product p and
// returns a review reason, or "" when the text may go live.
func (c *Checker) Check(ctx context.Context, p db.GetProductRow, variety, description string) string {
	text := strings.TrimSpace(variety + "\n" + description)
	if text == "" {
		return ""
	}
	if r := Rules(text); r != "" {
		return r
	}
	if c.model == nil {
		return ""
	}
	ctx, cancel := context.WithTimeout(llm.WithFeature(ctx, "moderation"), c.timeout)
	defer cancel()
	var v struct {
		Category   string  `json:"category"`
		Confidence float64 `json:"confidence"`
	}
	err := c.model.ChatJSON(ctx, llm.ChatRequest{Temperature: 0, MaxTokens: 60, Messages: []llm.Message{
		{Role: "system", Content: prompt},
		{Role: "user", Content: "Mahsulot: " + p.Name + " (" + p.Category + ")\nNavi: " + variety + "\nTavsif: " + description},
	}}, &v)
	if err != nil {
		slog.WarnContext(ctx, "moderation model", "err", err) // never blocks publishing
		return ""
	}
	if v.Confidence < minConfidence {
		return ""
	}
	switch strings.ToLower(strings.TrimSpace(v.Category)) {
	case "spam":
		return ReasonSpam
	case "abuse":
		return ReasonAbuse
	case "prohibited":
		return ReasonProhibited
	case "off_category":
		return ReasonOffCategory
	}
	return ""
}
