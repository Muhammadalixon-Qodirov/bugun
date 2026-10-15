package pricing

import (
	"context"
	"errors"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Understanding a price text (MATN_TUSHUNISH.md): the rules first, free and
// instant; a language model only for what they could not read. Both sides
// meet the same checks, and the seller confirms the card either way.

// Extractor asks a language model for price candidates in a text.
type Extractor interface {
	Extract(ctx context.Context, catalog, text string) ([]Candidate, error)
}

// SetExtractor turns the language model on for texts the rules cannot fully read.
func (s *Service) SetExtractor(e Extractor) { s.extractor = e }

// Understood is a text read by the rules and, when needed, a model.
type Understood struct {
	Result
	ByModel bool      // the model added at least one price
	Learn   []Learned // names the model found that the catalog did not know
}

// Learned is a product name worth remembering once the seller confirms it.
type Learned struct {
	Phrase    string `json:"phrase"`
	ProductID int16  `json:"product_id"`
}

// understand reads text with cat, and asks the model when the rules found
// nothing or left lines unread and the text has a number.
func (s *Service) understand(ctx context.Context, cat *Catalog, text string) Understood {
	u := Understood{Result: cat.Parse(text)}
	if s.extractor == nil || !strings.ContainsAny(text, "0123456789") || (len(u.Items) > 0 && len(u.Problems) == 0) {
		return u
	}
	cands, err := s.extractor.Extract(ctx, cat.CatalogText(), text)
	if err != nil {
		slog.WarnContext(ctx, "price text model", "err", err)
		return u
	}
	have := map[string]bool{}
	used := map[float64]bool{} // numbers the rules already gave to a product
	for _, it := range u.Items {
		have[it.TradeType+"|"+it.ProductName] = true
		for _, n := range findNumbers(tokenize(Normalize(it.Raw))) {
			used[n.value] = true
			if n.value < 1000 && !n.multiplied {
				used[n.value*1000] = true // "4.5" read as 4 500
			}
		}
	}
	var added []Verified
	for _, v := range cat.Verify(text, cands) {
		if have[v.TradeType+"|"+v.ProductName] || used[v.Written] {
			continue // the rules already read this product, or this number belongs to another one
		}
		have[v.TradeType+"|"+v.ProductName] = true
		u.Items = append(u.Items, v.Item)
		added = append(added, v)
		if v.Phrase != "" {
			u.Learn = append(u.Learn, Learned{Phrase: v.Phrase, ProductID: v.ProductID})
		}
	}
	if len(added) == 0 {
		return u
	}
	u.ByModel = true
	// Drop the problems the model's prices answer: a line about the same
	// product, or an unknown line that holds the name the model read.
	kept := u.Problems[:0]
	for _, p := range u.Problems {
		solved := false
		for _, v := range added {
			if p.Product == v.ProductName || (v.Phrase != "" && strings.Contains(" "+p.Raw+" ", " "+v.Phrase+" ")) ||
				(p.Code == "no_product" && strings.Contains(p.Raw, trimFloat(float64(v.PriceSom)))) {
				solved = true
				break
			}
		}
		if !solved {
			kept = append(kept, p)
		}
	}
	u.Problems = kept
	return u
}

// UnderstandChannel reads a channel post, logs it and returns what it gave.
func (s *Service) UnderstandChannel(ctx context.Context, text string) Understood {
	u := s.understand(ctx, s.catalogFor(ctx, uuid.Nil), text)
	s.LogChannel(ctx, text, u.Result, u.ByModel)
	return u
}

// With returns a copy of the catalog that also reads the learned names.
func (c *Catalog) With(extra []Learned) *Catalog {
	if len(extra) == 0 {
		return c
	}
	index := make(map[int16]int, len(c.products))
	for i, p := range c.products {
		index[p.ID] = i
	}
	n := &Catalog{products: c.products, aliases: append([]alias(nil), c.aliases...)}
	for _, l := range extra {
		i, ok := index[l.ProductID]
		words := strings.Fields(Normalize(l.Phrase))
		if !ok || len(words) == 0 {
			continue
		}
		n.aliases = append(n.aliases, alias{words: words, product: i})
	}
	sort.SliceStable(n.aliases, func(i, j int) bool {
		return len(strings.Join(n.aliases[i].words, " ")) > len(strings.Join(n.aliases[j].words, " "))
	})
	return n
}

// sharedFor keeps names three sellers confirmed for a while: they change slowly.
const sharedFor = 10 * time.Minute

// catalogFor is the catalog with the names everyone shares and the ones this
// seller taught. A failed lookup falls back to the plain catalog.
func (s *Service) catalogFor(ctx context.Context, user uuid.UUID) *Catalog {
	s.mu.Lock()
	shared, fresh := s.shared, time.Since(s.sharedAt) < sharedFor
	s.mu.Unlock()
	if !fresh {
		rows, err := s.q.SharedLearnedAliases(ctx)
		if err == nil {
			shared = make([]Learned, len(rows))
			for i, r := range rows {
				shared[i] = Learned{Phrase: r.Phrase, ProductID: r.ProductID}
			}
			s.mu.Lock()
			s.shared, s.sharedAt = shared, time.Now()
			s.mu.Unlock()
		}
	}
	extra := append([]Learned(nil), shared...)
	if rows, err := s.q.UserLearnedAliases(ctx, user); err == nil {
		for _, r := range rows {
			extra = append(extra, Learned{Phrase: r.Phrase, ProductID: r.ProductID})
		}
	}
	return s.catalog.With(extra)
}

// Read understands a text with the shared catalog, without logging it (for
// measuring models against a test set).
func (s *Service) Read(ctx context.Context, text string) Understood {
	return s.understand(ctx, s.catalogFor(ctx, uuid.Nil), text)
}

// ToBase converts an amount of a product in unit ("kg", "qop", "tonna",
// "dona"...) to its base unit; unit may be written as a seller would.
func (s *Service) ToBase(productID int16, unit string, amount float64) (float64, error) {
	for _, p := range s.catalog.products {
		if p.ID != productID {
			continue
		}
		u := strings.TrimSpace(Normalize(unit))
		if w := unitWord(u); w != "" {
			u = w
		} else if w := isPerUnit(u); w != "" {
			u = w
		}
		return toBase(p, u, amount)
	}
	return 0, errUnknownProductID
}

var errUnknownProductID = errors.New("unknown product")
