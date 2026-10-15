package bot

import (
	"strings"
	"testing"
	"time"

	"bugun/internal/pricing"
)

func TestPositionLine(t *testing.T) {
	cases := []struct {
		name  string
		saved pricing.Saved
		want  string
	}{
		{"ranked above median", pricing.Saved{Status: "active", Position: &pricing.MarketPosition{Rank: 9, Total: 12, Median: 4200, DiffPercent: 7}},
			"📊 12 sotuvchi ichida 9-o'rindasiz · o'rtacha 4 200 so'm (+7%)"},
		{"ranked below median", pricing.Saved{Status: "active", Position: &pricing.MarketPosition{Rank: 1, Total: 5, Median: 4200, DiffPercent: -5}},
			"📊 5 sotuvchi ichida 1-o'rindasiz · o'rtacha 4 200 so'm (-5%)"},
		{"at the median", pricing.Saved{Status: "active", Position: &pricing.MarketPosition{Rank: 3, Total: 5, Median: 4200}},
			"📊 5 sotuvchi ichida 3-o'rindasiz · o'rtacha 4 200 so'm"},
		{"too few prices", pricing.Saved{Status: "active"}, "📊 Bu mahsulot bo'yicha viloyatda narxlar hali kam"},
		{"held for review", pricing.Saved{Status: "review"}, "⏳ Narx odatdagidan juda farq qiladi, tekshiruvdan so'ng chiqadi"},
	}
	for _, c := range cases {
		if got := positionLine(c.saved); got != c.want {
			t.Errorf("%s:\n got %q\nwant %q", c.name, got, c.want)
		}
	}
}

func TestMineMessage(t *testing.T) {
	if got := mineMessage(nil); !strings.Contains(got, "faol narx yo'q") {
		t.Errorf("empty: %q", got)
	}
	got := mineMessage([]mineItem{
		{Name: "Piyoz", PriceSom: 4650, Unit: "kg", TradeType: "ulgurji", Status: "active",
			Position: &pricing.MarketPosition{Rank: 9, Total: 12, Median: 4400, DiffPercent: 6}},
		{Name: "Sabzi", PriceSom: 35000, Unit: "kg", TradeType: "ulgurji", Status: "review"},
		{Name: "Karam", PriceSom: 2500, Unit: "kg", TradeType: "chakana", Status: "active", Expired: true},
	})
	for _, want := range []string{
		"1 ta faol",
		"<b>Piyoz</b> — 4 650 so'm/kg · ulgurji",
		"12 sotuvchi ichida 9-o'rindasiz",
		"⏳ Tekshiruvda",
		"⌛ Muddati o'tgan",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
	many := make([]mineItem, mineLimit+3)
	for i := range many {
		many[i] = mineItem{Name: "Olma", PriceSom: 7000, Unit: "kg", TradeType: "ulgurji", Status: "active"}
	}
	if got := mineMessage(many); !strings.Contains(got, "yana 3 ta") {
		t.Errorf("long list not cut: %q", got)
	}
}

func TestReminderMessage(t *testing.T) {
	until := time.Date(2026, 10, 2, 9, 0, 0, 0, time.UTC)
	got := reminderMessage([]mineItem{
		{Name: "Piyoz", PriceSom: 1900, Unit: "kg", Status: "active", ValidUntil: until},
		{Name: "Sabzi <qizil>", PriceSom: 3500, Unit: "kg", Status: "active", Expired: true},
	})
	for _, want := range []string{
		"Narxlaringiz hali shumi?",
		"Piyoz — 1 900 so'm/kg · amal qiladi 2-oktabrgacha",
		"Sabzi &lt;qizil&gt; — 3 500 so'm/kg · ⌛ muddati o'tgan",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
}
