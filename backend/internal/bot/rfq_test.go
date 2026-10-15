package bot

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"bugun/internal/ask"
	"bugun/internal/rfq"
	"bugun/internal/store/db"
)

func TestParseSom(t *testing.T) {
	cases := map[string]int64{
		"3200":       3200,
		"3 200":      3200,
		"3.200":      3200,
		"3200 so'm":  3200,
		"3,2 ming":   3200,
		"4 ming":     4000,
		"3,5 минг":   3500,
		"45 000 сум": 45000,
	}
	for in, want := range cases {
		if got, ok := parseSom(in); !ok || got != want {
			t.Errorf("parseSom(%q) = %v, %v; want %v", in, got, ok, want)
		}
	}
	for _, bad := range []string{"", "arzon", "-5", "0"} {
		if _, ok := parseSom(bad); ok {
			t.Errorf("parseSom(%q) accepted", bad)
		}
	}
}

func TestQuotePromptFindsTheRequest(t *testing.T) {
	m := quotePrompt.FindStringSubmatch("So'rov #42 uchun narxingizni javob sifatida yozing")
	if m == nil || m[1] != "42" {
		t.Errorf("prompt match = %v, want 42", m)
	}
	if quotePrompt.MatchString("Saqlandi") {
		t.Error("an unrelated message matched the quote prompt")
	}
}

func TestContactLine(t *testing.T) {
	withTG := db.User{DisplayName: "Akmal Karimov", ContactTelegram: true, TelegramUsername: pgtype.Text{String: "akmal", Valid: true}}
	if got := contactLine(withTG); got != "Akmal: @akmal" {
		t.Errorf("with Telegram: %q", got)
	}
	hidden := withTG
	hidden.ContactTelegram = false
	if got := contactLine(hidden); got != "Akmal: aloqa ma'lumoti saytda" {
		t.Errorf("Telegram hidden: %q", got)
	}
	named := withTG
	named.PublicName = pgtype.Text{String: "Barakali <Savdo>", Valid: true}
	if got := contactLine(named); got != "Barakali &lt;Savdo&gt;: @akmal" {
		t.Errorf("public name is escaped: %q", got)
	}
}

func TestRFQErrorText(t *testing.T) {
	if got := rfqErrorText(&rfq.TooManyOpenError{Max: 5}); got != "Bir vaqtda ko'pi bilan 5 ta ochiq so'rov bo'lishi mumkin." {
		t.Errorf("configured limit: %q", got)
	}
	if got := rfqErrorText(rfq.ErrTooManyOpen); got != "Bir vaqtda ko'pi bilan 3 ta ochiq so'rov bo'lishi mumkin." {
		t.Errorf("limit: %q", got)
	}
	ve := &rfq.ValidationError{Field: "quantity", Message: "Miqdor noto'g'ri ko'rinadi."}
	if got := rfqErrorText(ve); got != ve.Message {
		t.Errorf("validation: %q", got)
	}
	if got := rfqErrorText(errors.New("db down")); got != "Xatolik yuz berdi, qayta urinib ko'ring." {
		t.Errorf("unknown error: %q", got)
	}
}

func TestLooksLikeQuestion(t *testing.T) {
	for text, want := range map[string]bool{
		"Namanganda piyoz qancha?":    true,
		"piyoz qancha":                true,
		"qaysi tumanda pomidor arzon": true,
		"почем лук":                   true,
		"piyoz 3500":                  false,
		"kartoshka narxi 4500":        false,
		"kartoshka 200 kg":            false,
		"200 kg piyoz qayerda bor?":   true,
	} {
		if got := looksLikeQuestion(text); got != want {
			t.Errorf("looksLikeQuestion(%q) = %v, want %v", text, got, want)
		}
	}
}

func TestAnswerText(t *testing.T) {
	from := time.Date(2026, 9, 19, 6, 0, 0, 0, time.UTC)
	to := time.Date(2026, 9, 25, 6, 0, 0, 0, time.UTC)
	a := ask.Answer{
		Intent:  ask.IntentAreas,
		Product: db.GetProductRow{Name: "Piyoz", Slug: "piyoz", BaseUnit: "kg"},
		Trade:   "ulgurji",
		Region:  &ask.Region{Name: "Chortoq"},
		Stats:   ask.Stats{Enough: true, Offers: 12, Median: 1950, P25: 1850, P75: 2050, Min: 1700, Max: 2300, From: from, To: to, Scope: "Namangan viloyati"},
		Areas:   []ask.Area{{Name: "Chortoq", Offers: 5, Median: 1850}, {Name: "Uychi", Offers: 3, Median: 2000}},
	}
	got := answerText(a)
	for _, want := range []string{
		"<b>Piyoz</b> — ulgurji, Namangan viloyati",
		"Chortoq bo'yicha narxlar hali kam, kengroq hudud olindi",
		"O'rtacha: <b>1 950 so'm/kg</b>",
		"• Chortoq — 1 850 so'm (5 ta narx)",
		"12 ta narx · 19-sentabr – 25-sentabr · manba: sotuvchilar",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
}
