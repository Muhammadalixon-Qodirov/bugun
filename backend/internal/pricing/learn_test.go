package pricing

import (
	"context"
	"strings"
	"testing"
)

// Near spellings are read, with a note for the seller; an unknown product
// with its own price is reported, never read as the previous one's amount.
func TestNearSpellingAndOrphans(t *testing.T) {
	c := testCatalog()
	cases := []struct {
		text      string
		items     map[int16]int64 // product → price
		qty       map[int16]float64
		problems  []string // raw of no_product problems
		nearNotes int
	}{
		{text: "kartoshga 4500", items: map[int16]int64{1: 4500}, nearNotes: 1},
		{text: "pomidr 6000, kartofelni 3500", items: map[int16]int64{4: 6000, 1: 3500}, nearNotes: 1},
		{text: "pomidor 6000, gilos 30000", items: map[int16]int64{4: 6000}, problems: []string{"gilos 30000"}},
		{text: "pomidor 6000\ngilos 30000 so'm", items: map[int16]int64{4: 6000}, problems: []string{"gilos 30000 som"}},
		{text: "gilos 30000, piyoz 2500", items: map[int16]int64{2: 2500}, problems: []string{"gilos 30000"}},
		// Words around a product that have no price of their own stay with it.
		{text: "qizil pomidor 6000, 2 tonna bor", items: map[int16]int64{4: 6000}, qty: map[int16]float64{4: 2000}},
		{text: "2 tonna kartoshka 4500", items: map[int16]int64{1: 4500}, qty: map[int16]float64{1: 2000}},
		{text: "pomidor 120000 yashchik", items: map[int16]int64{4: 6000}},
		{text: "tuxum 1500 shtuk", items: map[int16]int64{16: 1500}},
		{text: "Помидор 120 минг яшик", items: map[int16]int64{4: 6000}},
		{text: "kartoshka 250 000 so'm qop", items: map[int16]int64{1: 5000}},
		{text: "Yer yong'oq 40 ming, pomidor 6000", items: map[int16]int64{4: 6000}, problems: []string{"yer yongoq 40 ming"}},
		// A place name is never guessed as a product; the line is reported.
		{text: "Namangan 6000", problems: []string{"namangan 6000"}},
	}
	for _, tc := range cases {
		res := c.Parse(tc.text)
		got := map[int16]int64{}
		notes := 0
		for _, it := range res.Items {
			got[it.ProductID] = it.PriceSom
			for _, h := range it.Hints {
				if h.Code == "near_spelling" {
					notes++
				}
			}
			if want, ok := tc.qty[it.ProductID]; ok && (it.Quantity == nil || *it.Quantity != want) {
				t.Errorf("%q: quantity %v, want %v", tc.text, it.Quantity, want)
			}
			if _, ok := tc.qty[it.ProductID]; !ok && it.Quantity != nil {
				t.Errorf("%q: unexpected quantity %v for %d", tc.text, *it.Quantity, it.ProductID)
			}
		}
		if len(got) != len(tc.items) {
			t.Errorf("%q: items %+v, want %v", tc.text, res.Items, tc.items)
			continue
		}
		for p, price := range tc.items {
			if got[p] != price {
				t.Errorf("%q: product %d price %d, want %d", tc.text, p, got[p], price)
			}
		}
		var orphans []string
		for _, p := range res.Problems {
			if p.Code == "no_product" {
				orphans = append(orphans, p.Raw)
			}
		}
		if strings.Join(orphans, "|") != strings.Join(tc.problems, "|") {
			t.Errorf("%q: orphans %q, want %q", tc.text, orphans, tc.problems)
		}
		if notes != tc.nearNotes {
			t.Errorf("%q: %d near-spelling notes, want %d", tc.text, notes, tc.nearNotes)
		}
	}
	if d := editDistance("kartoshga", "kartoshka"); d != 1 {
		t.Errorf("distance %d", d)
	}
	if d := editDistance("pmoidor", "pomidor"); d != 1 {
		t.Errorf("swap distance %d", d)
	}
}

type fakeExtractor struct {
	cands []Candidate
	calls int
}

func (f *fakeExtractor) Extract(context.Context, string, string) ([]Candidate, error) {
	f.calls++
	return f.cands, nil
}

// The model is asked only about what the rules left; its prices join the
// rules' ones, the problems they answer go, and new names are kept to learn.
func TestUnderstandWithModel(t *testing.T) {
	c := testCatalog()
	model := &fakeExtractor{cands: []Candidate{
		{Product: "kartoshka", Text: "bulba", Price: 4500, Trade: "chakana"},
		{Product: "piyoz", Text: "piyoz", Price: 6000, Trade: "chakana"}, // 6000 belongs to the tomato line
	}}
	s := &Service{catalog: c, extractor: model}
	u := s.understand(context.Background(), c, "pomidor 6000, bulba 4500")
	if !u.ByModel || len(u.Items) != 2 {
		t.Fatalf("understood %+v", u)
	}
	var potato *Item
	for i := range u.Items {
		if u.Items[i].ProductID == 1 {
			potato = &u.Items[i]
		}
	}
	if potato == nil || potato.PriceSom != 4500 || potato.Hints[0].Code != "by_model" {
		t.Fatalf("potato %+v", u.Items)
	}
	if len(u.Problems) != 0 {
		t.Errorf("problems left %+v", u.Problems)
	}
	if len(u.Learn) != 1 || u.Learn[0] != (Learned{Phrase: "bulba", ProductID: 1}) {
		t.Errorf("learn %+v", u.Learn)
	}
	// Fully read texts, or texts without a number, never reach the model.
	model.calls = 0
	s.understand(context.Background(), c, "pomidor 6000, piyoz 3000")
	s.understand(context.Background(), c, "salom, bugun bozor yopiq")
	if model.calls != 0 {
		t.Errorf("model called %d times", model.calls)
	}
}
