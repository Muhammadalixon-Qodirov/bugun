package report

import (
	"bytes"
	"testing"
	"time"
)

func TestSomAndChange(t *testing.T) {
	for in, want := range map[float64]string{0: "0", 950: "950", 4200: "4 200", 112000: "112 000", 3800000: "3 800 000"} {
		if got := som(in); got != want {
			t.Errorf("som(%v) = %q, want %q", in, got, want)
		}
	}
	for _, c := range []struct {
		now, prev float64
		want      string
	}{{4200, 4000, "+5%"}, {3800, 4000, "-5%"}, {4000, 4000, "0%"}, {4000, 0, "—"}} {
		if got := change(c.now, c.prev); got != c.want {
			t.Errorf("change(%v, %v) = %q, want %q", c.now, c.prev, got, c.want)
		}
	}
}

func TestRender(t *testing.T) {
	loc, _ := time.LoadLocation("Asia/Tashkent")
	pdf, err := Render(Report{
		Province: "Farg'ona viloyati", GeneratedAt: time.Date(2026, 9, 25, 9, 0, 0, 0, time.UTC),
		Summary: Summary{Sellers: 41, Listings: 118, RFQs: 9, Demand: 1840, Deals: 23},
		Deals:   []DealRow{{Name: "Kartoshka", Unit: "kg", Deals: 12, Quantity: 4200, Median: 3250}, {Name: "Piyoz", Unit: "kg", Deals: 6, Quantity: 900}},
		Rows: []Row{
			{Name: "Kartoshka", Unit: "kg", Offers: 14, Median: 3300, PrevMedian: 3150, Min: 2900, Max: 3900, Zone: "shortage"},
			{Name: "Piyoz", Unit: "kg", Offers: 9, Median: 1900, Min: 1600, Max: 2300},
		},
	}, loc)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(pdf, []byte("%PDF-")) || len(pdf) < 1500 {
		t.Errorf("not a PDF: %d bytes, starts %q", len(pdf), pdf[:min(8, len(pdf))])
	}
	if _, err := Render(Report{Province: "Namangan viloyati", GeneratedAt: time.Now()}, loc); err != nil {
		t.Errorf("empty report: %v", err)
	}
}
