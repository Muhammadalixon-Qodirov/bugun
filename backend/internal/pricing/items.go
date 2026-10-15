package pricing

import (
	"context"
	"encoding/json"
	"strconv"
	"time"

	"bugun/internal/store/db"
)

// CreateDraftItems stores items read some other way than from the seller's
// own text (a photo of a price list) for confirmation, exactly like
// CreateDraft: nothing is saved as a price until the seller confirms. raw is
// the text the items were read from.
func (s *Service) CreateDraftItems(ctx context.Context, u db.User, raw string, items []Item, problems []Problem) (Draft, error) {
	if !CanSubmit(u) {
		return Draft{}, ErrNotSeller
	}
	if problems == nil {
		problems = []Problem{}
	}
	d := Draft{Items: items, Problems: problems}
	if len(items) == 0 {
		d.Items = []Item{}
		return d, nil
	}
	if r := []rune(raw); len(r) > maxMessageLen {
		raw = string(r[:maxMessageLen])
	}
	b, err := json.Marshal(items)
	if err != nil {
		return d, err
	}
	d.ID, err = s.q.CreatePriceDraft(ctx, db.CreatePriceDraftParams{UserID: u.ID, RawText: raw, Items: b, ExpiresAt: time.Now().Add(draftTTL), Learn: []byte("[]")})
	return d, err
}

// Merge adds to rule items the model's verified items for products the
// rules did not read; the rules win where both read one.
func Merge(rules, model []Item) []Item {
	seen := map[string]bool{}
	out := make([]Item, 0, len(rules)+len(model))
	for _, it := range rules {
		seen[itemKey(it)] = true
		out = append(out, it)
	}
	for _, it := range model {
		if !seen[itemKey(it)] {
			seen[itemKey(it)] = true
			out = append(out, it)
		}
	}
	return out
}

func itemKey(it Item) string { return strconv.Itoa(int(it.ProductID)) }

// Verify checks a model's candidates against the text (Catalog.Verify) and
// returns only the items, for callers that do not learn new names.
func (s *Service) Verify(text string, cands []Candidate) []Item {
	vs := s.catalog.Verify(text, cands)
	out := make([]Item, len(vs))
	for i, v := range vs {
		out[i] = v.Item
	}
	return out
}
