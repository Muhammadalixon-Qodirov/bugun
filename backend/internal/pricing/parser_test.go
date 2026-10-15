package pricing

import (
	"strings"
	"testing"
)

func testCatalog() *Catalog {
	return NewCatalog([]Product{
		{ID: 1, Slug: "kartoshka", Name: "Kartoshka", BaseUnit: "kg", Aliases: []string{"kartoshka", "kartofel", "картошка"}, Units: map[string]float64{"qop": 50}},
		{ID: 2, Slug: "piyoz", Name: "Piyoz", BaseUnit: "kg", Aliases: []string{"piyoz", "лук"}, Units: map[string]float64{"qop": 40}},
		{ID: 4, Name: "Pomidor", BaseUnit: "kg", Aliases: []string{"pomidor", "pamidor", "tomat", "помидор"}, Units: map[string]float64{"yashik": 20}},
		{ID: 9, Name: "Olma", BaseUnit: "kg", Aliases: []string{"olma"}, Units: map[string]float64{"yashik": 18}},
		{ID: 13, Name: "Mol go'shti", BaseUnit: "kg", Aliases: []string{"mol goshti", "mol go'shti", "говядина"}},
		{ID: 16, Name: "Tuxum", BaseUnit: "dona", Aliases: []string{"tuxum"}, Units: map[string]float64{"karobka": 360, "lotok": 30}},
		{ID: 22, Name: "Paxta yog'i", BaseUnit: "litr", Aliases: []string{"paxta yogi", "yog"}},
	})
}

type want struct {
	product  int16
	price    int64
	qty      float64 // 0 = no quantity
	trade    string
	hasNotes bool
}

func TestParse(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want []want
	}{
		{"spec example", "kartoshka 4500, 2 tonna", []want{{1, 4500, 2000, TradeRetail, false}}},
		{"per kilo spelled out", "kartoshka, kilosi to'rt ming besh yuz, ikki tonna bor", []want{{1, 4500, 2000, TradeRetail, false}}},
		{"half thousand words", "piyoz to'rt yarim ming", []want{{2, 4500, 0, TradeRetail, false}}},
		{"comma decimal thousand", "piyoz 4,5 ming so'm", []want{{2, 4500, 0, TradeRetail, false}}},
		{"ambiguous decimal", "piyoz 4.5", []want{{2, 4500, 0, TradeRetail, true}}},
		{"space thousands", "kartoshka 4 500 so'm", []want{{1, 4500, 0, TradeRetail, false}}},
		{"price per bag", "kartoshka qopi 200 ming", []want{{1, 4000, 0, TradeRetail, true}}},
		{"quantity in bags", "kartoshka 4500 so'mdan 10 qop", []want{{1, 4500, 500, TradeRetail, false}}},
		{"slash unit", "pomidor 120000/yashik", []want{{4, 6000, 0, TradeRetail, true}}},
		{"wholesale", "optom kartoshka 4200 5 tonna", []want{{1, 4200, 5000, TradeWholesale, false}}},
		{"cyrillic", "Картошка 4500 сўм", []want{{1, 4500, 0, TradeRetail, false}}},
		{"typo synonym", "pamidor 9000", []want{{4, 9000, 0, TradeRetail, false}}},
		{"suffix on product", "kartoshkani 4300 dan beraman", []want{{1, 4300, 0, TradeRetail, false}}},
		{"multi-word product", "mol go'shti 115 ming", []want{{13, 115000, 0, TradeRetail, false}}},
		{"per piece", "tuxum donasi 1350", []want{{16, 1350, 0, TradeRetail, false}}},
		{"per box", "tuxum karobkasi 450 ming", []want{{16, 1250, 0, TradeRetail, true}}},
		{"lone number with unit", "kartoshka 4500 kg", []want{{1, 4500, 0, TradeRetail, false}}},
		{"quantity first", "2 tonna kartoshka 4500", []want{{1, 4500, 2000, TradeRetail, false}}},
		{"several products one line", "kartoshka 4500 piyoz 3800 olma 12000", []want{
			{1, 4500, 0, TradeRetail, false}, {2, 3800, 0, TradeRetail, false}, {9, 12000, 0, TradeRetail, false}}},
		{"several lines", "kartoshka 4500\npiyoz 3800 1 tonna", []want{
			{1, 4500, 0, TradeRetail, false}, {2, 3800, 1000, TradeRetail, false}}},
	}
	c := testCatalog()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := c.Parse(tc.in)
			if len(got.Problems) > 0 {
				t.Fatalf("unexpected problems: %+v", got.Problems)
			}
			if len(got.Items) != len(tc.want) {
				t.Fatalf("got %d items %+v, want %d", len(got.Items), got.Items, len(tc.want))
			}
			for i, w := range tc.want {
				it := got.Items[i]
				if it.ProductID != w.product || it.PriceSom != w.price || it.TradeType != w.trade {
					t.Errorf("item %d = {product %d, price %d, trade %s}, want {%d, %d, %s}",
						i, it.ProductID, it.PriceSom, it.TradeType, w.product, w.price, w.trade)
				}
				switch {
				case w.qty == 0 && it.Quantity != nil:
					t.Errorf("item %d quantity = %v, want none", i, *it.Quantity)
				case w.qty != 0 && (it.Quantity == nil || *it.Quantity != w.qty):
					t.Errorf("item %d quantity = %v, want %v", i, it.Quantity, w.qty)
				}
				if (len(it.Notes) > 0) != w.hasNotes {
					t.Errorf("item %d notes = %v, want notes: %v", i, it.Notes, w.hasNotes)
				}
			}
		})
	}
}

func TestParseProblems(t *testing.T) {
	cases := []struct {
		name, in, msg string
		code          Problem // the coded fields; Raw and Message are checked separately
	}{
		{"no price", "kartoshka bor", "narx topilmadi", Problem{Code: "no_price", Product: "Kartoshka", ProductSlug: "kartoshka"}},
		{"unknown package", "piyoz yashigi 100 ming", "necha kg", Problem{Code: "unknown_unit", Product: "Piyoz", ProductSlug: "piyoz", Unit: "yashik", BaseUnit: "kg"}},
		{"absurd price", "kartoshka 900000000", "noto'g'ri", Problem{Code: "bad_price", Product: "Kartoshka", ProductSlug: "kartoshka", Price: 900000000}},
	}
	c := testCatalog()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := c.Parse(tc.in)
			if len(got.Items) != 0 || len(got.Problems) != 1 {
				t.Fatalf("got items %+v problems %+v, want exactly one problem", got.Items, got.Problems)
			}
			if !strings.Contains(got.Problems[0].Message, tc.msg) {
				t.Errorf("message %q does not mention %q", got.Problems[0].Message, tc.msg)
			}
			coded := got.Problems[0]
			coded.Raw, coded.Message = "", ""
			if coded != tc.code {
				t.Errorf("coded problem = %+v, want %+v", coded, tc.code)
			}
		})
	}
}

func TestParseHints(t *testing.T) {
	c := testCatalog()
	got := c.Parse("kartoshka 4.5")
	if len(got.Items) != 1 || len(got.Items[0].Hints) != 1 || got.Items[0].Hints[0] != (Hint{Code: "read_as_thousands", Written: "4.5", Price: 4500}) {
		t.Errorf("thousands hint: %+v", got.Items)
	}
	got = c.Parse("kartoshka qopi 200 ming")
	if len(got.Items) != 1 || len(got.Items[0].Hints) != 1 || got.Items[0].Hints[0] != (Hint{Code: "converted", Unit: "qop", PerBase: 50, BaseUnit: "kg"}) {
		t.Errorf("conversion hint: %+v", got.Items)
	}
	if len(got.Items[0].Notes) != len(got.Items[0].Hints) {
		t.Errorf("every note has a hint: %+v", got.Items[0])
	}
}

func TestParseNothingRecognised(t *testing.T) {
	got := testCatalog().Parse("salom, qalaysiz?")
	if len(got.Items) != 0 || len(got.Problems) != 0 {
		t.Fatalf("got %+v, want empty result", got)
	}
}

func TestNormalize(t *testing.T) {
	for in, want := range map[string]string{
		"O‘zbekiston": "ozbekiston",
		"Ғалла":       "galla",
		"to`rt":       "tort",
		"MOL GO'SHTI": "mol goshti",
	} {
		if got := Normalize(in); got != want {
			t.Errorf("Normalize(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestParseDemand(t *testing.T) {
	c := testCatalog()
	cases := []struct {
		text    string
		product string
		qty     float64 // 0 = no amount
		problem bool
	}{
		{"kartoshka 200 kg", "Kartoshka", 200, false},
		{"200 kg kartoshka kerak", "Kartoshka", 200, false},
		{"piyoz 2 tonna", "Piyoz", 2000, false},
		{"картошка 500 кг", "Kartoshka", 500, false},
		{"kartoshka kerak", "Kartoshka", 0, false},
		{"salom", "", 0, true},
		{"piyoz 3 yashik", "", 0, true},
	}
	for _, tc := range cases {
		d, p := c.ParseDemand(tc.text)
		if (p != nil) != tc.problem {
			t.Errorf("%q: problem = %v, want %v", tc.text, p, tc.problem)
			continue
		}
		if tc.problem {
			if p.Code == "" {
				t.Errorf("%q: problem without a code: %+v", tc.text, p)
			}
			continue
		}
		if d.ProductName != tc.product {
			t.Errorf("%q: product = %q, want %q", tc.text, d.ProductName, tc.product)
		}
		switch {
		case tc.qty == 0 && d.Quantity != nil:
			t.Errorf("%q: quantity = %v, want none", tc.text, *d.Quantity)
		case tc.qty != 0 && (d.Quantity == nil || *d.Quantity != tc.qty):
			t.Errorf("%q: quantity = %v, want %v", tc.text, d.Quantity, tc.qty)
		}
	}
}
