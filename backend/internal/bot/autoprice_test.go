package bot

import (
	"strings"
	"testing"

	"bugun/internal/analytics"
	"bugun/internal/market"
	"bugun/internal/store/db"
)

func TestAutoPriceMessage(t *testing.T) {
	c := market.AutoChange{
		Listing: db.Listing{Unit: "kg"},
		Change:  db.PriceChange{ID: 42, OldPrice: 6000, NewPrice: 6300},
		Reasons: market.Reasons{Median: 6100, Peers: 12, Factors: []analytics.Factor{
			{Code: "pressure", Share: 0.04}, {Code: "trend", Share: 0.01}, {Code: "cap", Share: -0.016},
		}},
	}
	msg := autoPriceMessage([]market.AutoChange{c}, []string{"Pomidor"})
	for _, want := range []string{"Pomidor", "6 000 → <b>6 300</b> so'm/kg", "+5%", "median 6 100 so'm (12 narx)", "talab odatdagidan yuqori", "kunlik 5%"} {
		if !strings.Contains(msg, want) {
			t.Errorf("message lacks %q:\n%s", want, msg)
		}
	}
	kb := undoButtons([]market.AutoChange{c}, []string{"Pomidor"})
	if got := kb.InlineKeyboard[0][0].CallbackData; got != "a:undo:42" {
		t.Errorf("undo callback %q", got)
	}
	// With contributions the message gives each reason's exact share, biggest first.
	c.Reasons.Contrib = []analytics.Factor{{Code: "market", Share: 0.008}, {Code: "pressure", Share: 0.033}, {Code: "trend", Share: 0.009}}
	msg = autoPriceMessage([]market.AutoChange{c}, []string{"Pomidor"})
	if want := "Nega: talab odatdagidan yuqori +3,3%, bozor narxi hafta ichida oshdi +0,9%, viloyat medianiga yaqinlashish +0,8%"; !strings.Contains(msg, want) {
		t.Errorf("message lacks %q:\n%s", want, msg)
	}
	if got := percent(-0.0356); got != "-3,6%" {
		t.Errorf("percent = %q", got)
	}
}
