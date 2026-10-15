package bot

import (
	"strings"
	"testing"

	"bugun/internal/analyst"
)

func TestAnalystText(t *testing.T) {
	got := analystText(analyst.Answer{
		Text: "Piyoz <arzon>: 4 500 soʻm.",
		Sources: []analyst.Source{
			{Tool: analyst.ToolPriceStats, Observations: 12, DataFrom: "2026-09-19", DataTo: "2026-09-26", DemoShare: 0.25},
			{Tool: analyst.ToolForecast, Error: "unknown_product"},
		},
	})
	for _, want := range []string{"Piyoz &lt;arzon&gt;: 4 500 soʻm.", "12 ta kuzatuv · 2026-09-19 – 2026-09-26", "demo ulushi 25%"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in %q", want, got)
		}
	}
	if got := analystText(analyst.Answer{Text: "Faqat narx savollariga javob beraman."}); strings.Contains(got, "kuzatuv") {
		t.Errorf("no sources, no footer: %q", got)
	}
}

func TestSuggestionPrompt(t *testing.T) {
	// What Telegram gives back as the message text (entities decoded).
	text := "🤔 «pamidr» — qaysi mahsulot?\n\nXabaringiz: «Pamidr 8 ming, <optom>»"
	m := suggestionPrompt.FindStringSubmatch(text)
	if m == nil || m[1] != "pamidr" || m[2] != "Pamidr 8 ming, <optom>" {
		t.Fatalf("match %q", m)
	}
}
