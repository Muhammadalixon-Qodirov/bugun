package pricing

import (
	"fmt"
	"math"
	"strings"
)

// Prices a language model read from a post the rules could not (ML_REJA.md
// 8). The model only suggests; every suggestion is checked here like a
// seller's own line: the product must be in the catalog, the unit one the
// product has, and the price a number actually written in the post, so a
// made-up number can never become a price.

// Candidate is one price a model suggests.
type Candidate struct {
	Product string  `json:"product"` // catalog slug
	Text    string  `json:"text"`    // the words that name the product in the text, without the price
	Price   float64 `json:"price"`   // the number as written in the text
	Unit    string  `json:"unit"`    // the unit word written for the price, or empty
	Trade   string  `json:"trade"`   // ulgurji or chakana
	// Qty is the amount offered, in the price's unit, as written; 0 = none.
	Qty float64 `json:"qty,omitempty"`
}

// Verified is a candidate that passed every check, with the phrase the text
// used for the product when it is worth learning.
type Verified struct {
	Item
	Phrase  string  // normalized words naming the product; empty when not a new name
	Written float64 // the price as written in the text
}

// NoteByModel marks items a language model read.
const NoteByModel = "Sunʼiy intellekt oʻqidi; raqam postda bor"

// Verify keeps the candidates that pass every check. At most one per
// product and trade type.
func (c *Catalog) Verify(text string, cands []Candidate) []Verified {
	bySlug := map[string]int{}
	for i, p := range c.products {
		bySlug[p.Slug] = i
	}
	written := findNumbers(tokenize(Normalize(text)))
	isWritten := func(v float64) (float64, bool) {
		for _, n := range written {
			if n.value == v {
				// "4.5" for 4 500 so'm, as the rules read it too.
				if v < 1000 && !n.multiplied && (n.decimal || v < 100) {
					return v * 1000, true
				}
				return v, true
			}
			if v >= 1000 && !n.multiplied && n.value < 1000 && n.value*1000 == v {
				return v, true // the model wrote 4500 for "4.5"
			}
		}
		return 0, false
	}
	normText := " " + strings.Join(strings.Fields(Normalize(text)), " ") + " "
	seen := map[string]bool{}
	var out []Verified
	for _, cd := range cands {
		i, ok := bySlug[strings.TrimSpace(cd.Product)]
		if !ok || cd.Price <= 0 {
			continue
		}
		value, ok := isWritten(cd.Price)
		if !ok {
			continue
		}
		p := c.products[i]
		unit := ""
		if u := strings.TrimSpace(Normalize(cd.Unit)); u != "" {
			if unit = unitWord(u); unit == "" {
				if unit = isPerUnit(u); unit == "" {
					continue // a unit we do not know: skip rather than guess
				}
			}
		}
		perBase, err := toBase(p, unit, 1)
		if err != nil {
			continue
		}
		price := int64(math.Round(value / perBase))
		if price < minPriceSom || price > maxPriceSom {
			continue
		}
		trade := TradeRetail
		if cd.Trade == TradeWholesale {
			trade = TradeWholesale
		}
		key := p.Slug + "|" + trade
		if seen[key] {
			continue
		}
		seen[key] = true
		it := Item{
			ProductID: p.ID, ProductName: p.Name, Category: p.Category, Unit: p.BaseUnit, PriceSom: price, TradeType: trade,
			Notes: []string{NoteByModel}, Hints: []Hint{{Code: "by_model"}}, Raw: fmt.Sprintf("%s %s", p.Name, trimFloat(cd.Price)),
		}
		// An amount is kept only when it too is written in the text.
		if cd.Qty > 0 && writtenExactly(written, cd.Qty) {
			if q := cd.Qty * perBase; q > 0 && q <= maxQuantity {
				q = math.Round(q*1000) / 1000
				it.Quantity = &q
			}
		}
		out = append(out, Verified{Item: it, Phrase: c.learnable(normText, cd.Text, i), Written: cd.Price})
	}
	return out
}

// learnable returns the model's name for product i, normalized, when it is
// a short phrase really written in the text that the catalog does not
// already read as that product; otherwise "".
func (c *Catalog) learnable(normText, phrase string, i int) string {
	n := strings.Join(strings.Fields(Normalize(phrase)), " ")
	words := strings.Fields(n)
	if len(words) == 0 || len(words) > 3 || len([]rune(n)) < 3 || len([]rune(n)) > 30 || strings.ContainsAny(n, "0123456789") {
		return ""
	}
	if !strings.Contains(normText, " "+n+" ") {
		return ""
	}
	for _, w := range words {
		if plainWord(w) {
			return ""
		}
	}
	toks := tokenize(n)
	if p, k, near := c.matchProduct(toks); p == i && k == len(toks) && near == "" {
		return "" // already a name of that product
	}
	return n
}

func writtenExactly(written []number, v float64) bool {
	for _, n := range written {
		if n.value == v {
			return true
		}
	}
	return false
}

// CatalogText lists the catalog for a model, one product per line: slug,
// name and up to two other Latin names. It is kept short: the free tier of
// a model API counts every token, and Cyrillic names are read by the rules.
func (c *Catalog) CatalogText() string {
	lines := make([]string, 0, len(c.products))
	for _, p := range c.products {
		var other []string
		for _, a := range p.Aliases {
			n := strings.TrimSpace(a)
			if len(other) == 2 || strings.EqualFold(n, p.Name) || strings.EqualFold(n, p.Slug) || !latin(n) {
				continue
			}
			other = append(other, n)
		}
		line := p.Slug + ": " + p.Name
		if len(other) > 0 {
			line += " (" + strings.Join(other, ", ") + ")"
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

func latin(s string) bool {
	for _, r := range s {
		if r > 0x2FF && r != 'ʻ' && r != 'ʼ' {
			return false
		}
	}
	return true
}

// CatalogText is the catalog as a model reads it.
func (s *Service) CatalogText() string { return s.catalog.CatalogText() }
