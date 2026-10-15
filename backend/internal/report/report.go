// Package report renders the weekly viloyat report for organisations (W-10)
// as a PDF. It only lays out numbers computed elsewhere.
package report

import (
	"bytes"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/go-pdf/fpdf"
)

type Summary struct {
	Sellers, Listings, RFQs, Demand int
	Deals                           int // confirmed deals in 7 days; shown from 5 up
}

// DealRow is one product's confirmed deals of the week.
type DealRow struct {
	Name     string
	Unit     string
	Deals    int
	Quantity float64
	Median   float64 // agreed price; 0 when fewer than 5 deals gave one
}

type Row struct {
	Name       string
	Unit       string
	Offers     int
	Median     float64
	PrevMedian float64 // 0 when last week had too few prices
	Min, Max   int64
	Zone       string // surplus, balanced, shortage; "" when unknown
}

type Report struct {
	Province    string
	GeneratedAt time.Time // shown in Asia/Tashkent
	Summary     Summary
	Rows        []Row
	Deals       []DealRow // products with at least 5 confirmed deals
}

var months = []string{"yanvar", "fevral", "mart", "aprel", "may", "iyun", "iyul", "avgust", "sentabr", "oktabr", "noyabr", "dekabr"}

func dateUz(t time.Time, loc *time.Location) string {
	t = t.In(loc)
	return fmt.Sprintf("%d-%s %d", t.Day(), months[t.Month()-1], t.Year())
}

// som groups thousands with spaces: 4200 -> "4 200".
func som(v float64) string {
	s := strconv.FormatInt(int64(math.Round(v)), 10)
	var b strings.Builder
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte(' ')
		}
		b.WriteRune(c)
	}
	return b.String()
}

// change is the week-on-week move of the median, e.g. "+5%", or "—".
func change(now, prev float64) string {
	if prev <= 0 {
		return "—"
	}
	d := int(math.Round(100 * (now - prev) / prev))
	switch {
	case d > 0:
		return fmt.Sprintf("+%d%%", d)
	case d < 0:
		return fmt.Sprintf("%d%%", d)
	}
	return "0%"
}

// dealsTile is the summary tile of confirmed deals, shown from 5 up.
const dealsTile = "Savdolar, 7 kun"

var zoneText = map[string]string{
	"surplus":  "taklif ko'p",
	"balanced": "muvozanat",
	"shortage": "talab yuqori",
}

// Render builds the PDF.
func Render(r Report, loc *time.Location) ([]byte, error) {
	pdf := fpdf.New("P", "mm", "A4", "")
	tr := pdf.UnicodeTranslatorFromDescriptor("") // cp1252: Uzbek Latin with ' apostrophes
	pdf.SetTitle(tr("Bugun — haftalik narx hisoboti"), false)
	pdf.SetMargins(15, 15, 15)
	pdf.SetAutoPageBreak(true, 15)
	pdf.AliasNbPages("")
	pdf.SetFooterFunc(func() {
		pdf.SetY(-12)
		pdf.SetFont("Helvetica", "", 8)
		pdf.SetTextColor(120, 120, 120)
		pdf.CellFormat(0, 5, tr(fmt.Sprintf("Bugun · %s · %d/{nb}", r.Province, pdf.PageNo())), "", 0, "C", false, 0, "")
	})
	pdf.AddPage()

	pdf.SetFont("Helvetica", "B", 16)
	pdf.CellFormat(0, 9, tr("Haftalik narx hisoboti"), "", 1, "L", false, 0, "")
	pdf.SetFont("Helvetica", "", 11)
	pdf.SetTextColor(80, 80, 80)
	week := r.GeneratedAt.AddDate(0, 0, -7)
	pdf.CellFormat(0, 6, tr(fmt.Sprintf("%s · %s – %s", r.Province, dateUz(week, loc), dateUz(r.GeneratedAt, loc))), "", 1, "L", false, 0, "")
	pdf.Ln(4)

	// Summary tiles.
	pdf.SetTextColor(0, 0, 0)
	tiles := []struct {
		label string
		value int
	}{
		{"Faol sotuvchilar", r.Summary.Sellers},
		{"Faol narxlar", r.Summary.Listings},
		{"So'rovlar, 7 kun", r.Summary.RFQs},
		{"Talab signallari, 7 kun", r.Summary.Demand},
		{dealsTile, r.Summary.Deals},
	}
	w := 180.0 / float64(len(tiles))
	for _, t := range tiles {
		x, y := pdf.GetXY()
		pdf.SetFillColor(244, 241, 234)
		pdf.Rect(x, y, w-3, 18, "F")
		pdf.SetXY(x+3, y+2)
		pdf.SetFont("Helvetica", "B", 14)
		value := som(float64(t.value))
		if t.label == dealsTile && t.value < 5 {
			value = "—" // too few to show
		}
		pdf.CellFormat(w-6, 7, tr(value), "", 2, "L", false, 0, "")
		pdf.SetX(x + 3)
		pdf.SetFont("Helvetica", "", 8)
		pdf.CellFormat(w-6, 5, tr(t.label), "", 0, "L", false, 0, "")
		pdf.SetXY(x+w, y)
	}
	pdf.Ln(24)

	pdf.SetFont("Helvetica", "B", 12)
	pdf.CellFormat(0, 7, tr("Ulgurji narxlar (so'm, bir birlik uchun)"), "", 1, "L", false, 0, "")
	cols := []struct {
		title string
		width float64
		align string
	}{
		{"Mahsulot", 44, "L"}, {"Median", 24, "R"}, {"O'tgan haftaga", 26, "R"},
		{"Eng past – eng yuqori", 38, "R"}, {"Narxlar", 18, "R"}, {"Bosim", 30, "L"},
	}
	pdf.SetFont("Helvetica", "B", 9)
	pdf.SetFillColor(30, 90, 70)
	pdf.SetTextColor(255, 255, 255)
	for _, c := range cols {
		pdf.CellFormat(c.width, 7, tr(c.title), "", 0, c.align, true, 0, "")
	}
	pdf.Ln(-1)
	pdf.SetTextColor(0, 0, 0)
	pdf.SetFont("Helvetica", "", 9)
	if len(r.Rows) == 0 {
		pdf.CellFormat(0, 8, tr("Bu hafta viloyatda 5 tadan ko'p narxi bor mahsulot yo'q."), "", 1, "L", false, 0, "")
	}
	for i, row := range r.Rows {
		fill := i%2 == 1
		pdf.SetFillColor(248, 247, 243)
		zone := zoneText[row.Zone]
		if zone == "" {
			zone = "—"
		}
		cells := []string{
			row.Name + " (" + row.Unit + ")",
			som(row.Median),
			change(row.Median, row.PrevMedian),
			som(float64(row.Min)) + " – " + som(float64(row.Max)),
			strconv.Itoa(row.Offers),
			zone,
		}
		for j, c := range cols {
			pdf.CellFormat(c.width, 6.5, tr(cells[j]), "", 0, c.align, fill, 0, "")
		}
		pdf.Ln(-1)
	}

	if len(r.Deals) > 0 {
		pdf.Ln(5)
		pdf.SetFont("Helvetica", "B", 12)
		pdf.CellFormat(0, 7, tr("Tasdiqlangan savdolar (7 kun)"), "", 1, "L", false, 0, "")
		dcols := []struct {
			title string
			width float64
			align string
		}{{"Mahsulot", 60, "L"}, {"Savdolar", 30, "R"}, {"Hajm", 45, "R"}, {"Median narx", 45, "R"}}
		pdf.SetFont("Helvetica", "B", 9)
		pdf.SetFillColor(30, 90, 70)
		pdf.SetTextColor(255, 255, 255)
		for _, c := range dcols {
			pdf.CellFormat(c.width, 7, tr(c.title), "", 0, c.align, true, 0, "")
		}
		pdf.Ln(-1)
		pdf.SetTextColor(0, 0, 0)
		pdf.SetFont("Helvetica", "", 9)
		for i, d := range r.Deals {
			pdf.SetFillColor(248, 247, 243)
			median := "—"
			if d.Median > 0 {
				median = som(d.Median)
			}
			cells := []string{d.Name, strconv.Itoa(d.Deals), som(d.Quantity) + " " + d.Unit, median}
			for j, c := range dcols {
				pdf.CellFormat(c.width, 6.5, tr(cells[j]), "", 0, c.align, i%2 == 1, 0, "")
			}
			pdf.Ln(-1)
		}
	}

	pdf.Ln(5)
	pdf.SetFont("Helvetica", "", 8)
	pdf.SetTextColor(90, 90, 90)
	pdf.MultiCell(0, 4, tr(strings.Join([]string{
		"Median — manba ishonchi va yangiligi bo'yicha vaznli median (sotuvchi narxi 0,8, xaridor so'roviga javob va tasdiqlangan savdo 1,0; har kun vazni e^(-0,2) ga kamayadi).",
		"Faqat faol, tekshiruvdan o'tgan narxlar; kamida 5 ta narxi bor mahsulotlar. Alohida sotuvchi ko'rsatilmaydi.",
		"Bosim: oxirgi 7 kundagi talab va sotuvchilar soni oldingi 4 hafta bilan solishtiriladi; ma'lumot kam bo'lsa \"—\".",
		"Savdolar: faqat ikki tomon tasdiqlagan savdolar (yoki RFQ kelishuvi); kamida 5 ta bo'lsa ko'rsatiladi. Bog'lanish savdo hisoblanmaydi.",
		"Tuzildi: " + r.GeneratedAt.In(loc).Format("2006-01-02 15:04") + " (Toshkent vaqti).",
	}, "\n")), "", "L", false)

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
