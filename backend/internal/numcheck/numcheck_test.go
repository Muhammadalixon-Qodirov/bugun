package numcheck

import "testing"

func TestNumbersCheck(t *testing.T) {
	a := New()
	a.AddJSON(map[string]any{"median": 4500.0, "share": 0.12, "from": "2026-09-20", "rows": []any{map[string]any{"p": 3800}}})
	for text, want := range map[string]int{
		"Narx 4 500 soʻm, 12% demo, 20-sentabrdan.": 0,
		"4,5 ming soʻm va 3 800 soʻm":               0,
		"4.500 сум, 2026 год":                       0,
		"Narx 5 200 soʻm boʻladi":                   1,
		"Oʻsish 35% va 1 mln":                       2,
	} {
		if got := a.Unchecked(text); len(got) != want {
			t.Errorf("%q: unchecked %v, want %d", text, got, want)
		}
	}
}
