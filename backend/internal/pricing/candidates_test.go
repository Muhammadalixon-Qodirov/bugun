package pricing

import (
	"strings"
	"testing"
)

// A model's suggestions become prices only when the product is in the
// catalog, the unit is known and the number is written in the post.
func TestVerifyCandidates(t *testing.T) {
	c := testCatalog()
	post := "Bugun bozorda: kartoshkaning qopi 250 ming, piyoz ham arzonlashdi 4.5 dan beryapmiz. Optom."
	items := c.Verify(post, []Candidate{
		{Product: "kartoshka", Price: 250000, Unit: "qopi", Trade: "ulgurji"},  // 250 000 / 50 kg
		{Product: "piyoz", Price: 4500, Trade: "ulgurji"},                      // "4.5" read as 4 500
		{Product: "piyoz", Price: 4500, Trade: "ulgurji"},                      // the same twice
		{Product: "kartoshka", Price: 3100, Trade: "chakana"},                  // 3100 is not in the post
		{Product: "banan", Price: 250000},                                      // not in the catalog
		{Product: "kartoshka", Price: 250000, Unit: "vagon", Trade: "chakana"}, // unknown unit
	})
	if len(items) != 2 {
		t.Fatalf("items %+v", items)
	}
	if items[0].ProductID != 1 || items[0].PriceSom != 5000 || items[0].TradeType != TradeWholesale {
		t.Errorf("kartoshka %+v", items[0])
	}
	if items[1].ProductID != 2 || items[1].PriceSom != 4500 || items[1].Hints[0].Code != "by_model" {
		t.Errorf("piyoz %+v", items[1])
	}
	if !strings.Contains(c.CatalogText(), "kartoshka: Kartoshka (kartofel)\npiyoz: Piyoz\n") {
		t.Errorf("catalog text:\n%s", c.CatalogText())
	}
}
