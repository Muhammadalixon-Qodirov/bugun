package pricing

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
)

// Catalog matching for words the dictionary does not know (AI-4): a word
// like "pamidr" is looked up by a ProductMatcher (trigram similarity and,
// when configured, embeddings). A confident match is read as that product,
// with a note on the card; otherwise the seller picks from the candidates.

// ProductChoice is a catalog product a word may mean.
type ProductChoice struct {
	ProductID int16   `json:"product_id"`
	Slug      string  `json:"slug"`
	Name      string  `json:"name"`
	Score     float64 `json:"score" doc:"0–1; 1 = dictionary"`
	Method    string  `json:"method" enum:"dictionary,trigram,embedding"`
}

// Suggestion is a word that may be a product, with the products it may be.
type Suggestion struct {
	Word       string          `json:"word" doc:"The word as written (Latin script, lowercase)"`
	Candidates []ProductChoice `json:"candidates" doc:"Best first; resend the text with the chosen product's name in place of word"`
}

// ProductMatcher finds catalog products for a word. best is nil when no
// product is confident enough; cands are the ones worth offering.
type ProductMatcher interface {
	MatchProduct(ctx context.Context, word string) (best *ProductChoice, cands []ProductChoice, err error)
}

// SetMatcher turns on matching unknown words to the catalog.
func (s *Service) SetMatcher(m ProductMatcher) { s.matcher = m }

// maxUnknownWords bounds the lookups one message makes.
const maxUnknownWords = 3

// suggestWords are the words of a message that could be a product the
// dictionary missed: not a number, unit, currency or trade word, and not
// already a product.
func (c *Catalog) suggestWords(text string) []string {
	toks := tokenize(Normalize(text))
	var out []string
	seen := map[string]bool{}
	for i, t := range toks {
		if t.kind != tokWord || len([]rune(t.text)) < 3 || seen[t.text] {
			continue
		}
		if isNumberWord(t.text) || unitWord(t.text) != "" || isPerUnit(t.text) != "" || isCurrency(t.text) || stopWords[t.text] {
			continue
		}
		if p, _, _ := c.matchProduct(toks[i:]); p >= 0 {
			continue
		}
		seen[t.text] = true
		out = append(out, t.text)
		if len(out) == maxUnknownWords {
			break
		}
	}
	return out
}

// stopWords are common message words that are never a product.
var stopWords = map[string]bool{
	"optom": true, "opt": true, "ulgurji": true, "ulgurjiga": true, "optomga": true, "optovoy": true, "chakana": true,
	"narx": true, "narxi": true, "narxda": true, "bor": true, "sotiladi": true, "sotaman": true, "sotamiz": true,
	"yangi": true, "sifatli": true, "arzon": true, "bugun": true, "kerak": true, "dona": true, "har": true,
	"bilan": true, "uchun": true, "yoki": true, "ham": true, "va": true, "zor": true, "super": true, "tayyor": true,
	"salom": true, "assalomu": true, "alaykum": true, "rahmat": true,
	"tsena": true, "sena": true, "prodayu": true, "est": true, "nalichii": true, "svejiy": true, "svejaya": true,
}

// ReplaceWord puts a product's name in place of a word the seller picked it
// for; the result is parsed like the seller's own text.
func ReplaceWord(text, word, name string) string { return replaceWord(text, word, name) }

// replaceWord puts name in place of every whole-word occurrence of word in
// the normalized text.
func replaceWord(text, word, name string) string {
	toks := strings.Fields(Normalize(text))
	for i, t := range toks {
		core := strings.TrimFunc(t, func(r rune) bool { return !isLetter(r) })
		if core == word {
			toks[i] = strings.Replace(t, word, Normalize(name), 1)
		}
	}
	return strings.Join(toks, " ")
}

func isLetter(r rune) bool { return (r >= 'a' && r <= 'z') || r > 127 }

// resolveUnknown is called when the rules found no item: words matched with
// confidence are read as their product and the text parsed again; the rest
// come back as suggestions for the seller.
func (s *Service) resolveUnknown(ctx context.Context, text string, res Result) (Result, []Suggestion) {
	// Only a message with a price is worth asking about: "salom" is not a
	// product the seller mistyped.
	if s.matcher == nil || len(findNumbers(tokenize(Normalize(text)))) == 0 {
		return res, nil
	}
	rewritten := text
	type taken struct{ word, name string }
	var matched []taken
	var sugg []Suggestion
	for _, w := range s.catalog.suggestWords(text) {
		best, cands, err := s.matcher.MatchProduct(ctx, w)
		if err != nil {
			slog.WarnContext(ctx, "match product", "err", err)
			continue
		}
		if best != nil {
			rewritten = replaceWord(rewritten, w, best.Name)
			matched = append(matched, taken{w, best.Name})
			continue
		}
		if len(cands) > 0 {
			sugg = append(sugg, Suggestion{Word: w, Candidates: cands})
		}
	}
	if len(matched) == 0 {
		return res, sugg
	}
	again := s.catalog.Parse(rewritten)
	if len(again.Items) == 0 {
		return res, sugg
	}
	for i := range again.Items {
		it := &again.Items[i]
		for _, m := range matched {
			if Normalize(m.name) == Normalize(it.ProductName) {
				it.Notes = append(it.Notes, fmt.Sprintf("«%s» %s deb tushunildi", m.word, it.ProductName))
				it.Hints = append(it.Hints, Hint{Code: "matched", Written: m.word})
			}
		}
	}
	return again, sugg
}
