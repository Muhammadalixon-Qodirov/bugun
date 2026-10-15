package pricing

import "testing"

// On the real catalog: common words and place names are never taken for a
// product, and usual misspellings are.
func TestNearSpellingRealCatalog(t *testing.T) {
	svc, _ := testService(t)
	c := svc.catalog
	for _, w := range []string{"sotaman", "narxlar", "bozorga", "yetkazamiz", "sifatli", "kelinglar", "murojaat", "telefon",
		"manzil", "toshkent", "samarqand", "namangan", "andijon", "fargona", "buxoro", "xorazm", "bugungi", "ertalab",
		"kechqurun", "hamma", "barcha", "yaxshi", "yangisi", "arzonroq", "qimmat", "dollar", "kilogramm", "mashina"} {
		if p := c.nearProduct(w); p >= 0 {
			t.Errorf("%q taken for %s", w, c.products[p].Name)
		}
	}
	for w, want := range map[string]string{"kartoshga": "kartoshka", "bodrig": "bodring", "piyos": "piyoz", "sabiz": "sabzi", "pomidr": "pomidor", "karam": "karam"} {
		p, _, _ := c.matchProduct([]token{{kind: tokWord, text: w}})
		if p < 0 || c.products[p].Slug != want {
			got := "none"
			if p >= 0 {
				got = c.products[p].Slug
			}
			t.Errorf("%q → %s, want %s", w, got, want)
		}
	}
}
