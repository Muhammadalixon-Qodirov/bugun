package pricing

import (
	"context"
	"testing"
)

// fakeMatcher knows "pmdr" well and "krtsh" only vaguely.
type fakeMatcher struct{ asked []string }

func (f *fakeMatcher) MatchProduct(_ context.Context, word string) (*ProductChoice, []ProductChoice, error) {
	f.asked = append(f.asked, word)
	switch word {
	case "pmdr":
		c := ProductChoice{ProductID: 4, Name: "Pomidor", Score: 0.8, Method: "trigram"}
		return &c, []ProductChoice{c}, nil
	case "krtsh":
		return nil, []ProductChoice{{ProductID: 1, Slug: "kartoshka", Name: "Kartoshka", Score: 0.4, Method: "trigram"}}, nil
	}
	return nil, nil, nil
}

func TestResolveUnknown(t *testing.T) {
	m := &fakeMatcher{}
	s := &Service{catalog: testCatalog(), matcher: m}

	res := s.catalog.Parse("pmdr 4500 optom")
	got, sugg := s.resolveUnknown(context.Background(), "pmdr 4500 optom", res)
	if len(got.Items) != 1 || got.Items[0].ProductID != 4 || got.Items[0].PriceSom != 4500 || got.Items[0].TradeType != TradeWholesale {
		t.Fatalf("items %+v", got.Items)
	}
	if h := got.Items[0].Hints; len(h) != 1 || h[0].Code != "matched" || h[0].Written != "pmdr" {
		t.Errorf("hints %+v", h)
	}
	if len(sugg) != 0 {
		t.Errorf("suggestions %+v", sugg)
	}
	// Numbers, units and trade words are never looked up.
	for _, w := range m.asked {
		if w != "pmdr" {
			t.Errorf("looked up %q", w)
		}
	}

	got, sugg = s.resolveUnknown(context.Background(), "krtsh 3 ming so'm", s.catalog.Parse("krtsh 3 ming so'm"))
	if len(got.Items) != 0 || len(sugg) != 1 || sugg[0].Word != "krtsh" || sugg[0].Candidates[0].ProductID != 1 {
		t.Errorf("weak match: items %+v suggestions %+v", got.Items, sugg)
	}

	// A message without a price is not looked up.
	m.asked = nil
	if got, sugg := s.resolveUnknown(context.Background(), "salom pmdr", Result{}); len(got.Items) != 0 || sugg != nil || len(m.asked) != 0 {
		t.Errorf("no price: %+v %+v %v", got, sugg, m.asked)
	}

	// Without a matcher nothing changes.
	plain := &Service{catalog: testCatalog()}
	if got, sugg := plain.resolveUnknown(context.Background(), "pmdr 4500", Result{}); len(got.Items) != 0 || sugg != nil {
		t.Error("no matcher must mean no matching")
	}
}

func TestReplaceWord(t *testing.T) {
	if got := replaceWord("Pamidr, 4500 so'm", "pamidr", "Pomidor"); got != "pomidor, 4500 som" {
		t.Errorf("got %q", got)
	}
}
