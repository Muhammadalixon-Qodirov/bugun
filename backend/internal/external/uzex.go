package external

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"bugun/internal/store/db"
	"bugun/internal/tz"
)

// UzEx exchange quotes (spec §11): the Uzbek Republican Commodity Exchange
// publishes the current price of its main exchange items as JSON for the
// ticker on uzex.uz. Each item is {name, summa, difference, percent}: the
// name in Uzbek (Latin, with a few Cyrillic look-alike letters), the price in
// so'm with spaces between thousands, per tonne unless the name says
// otherwise ("(gramm)"). The list has no date: it is the latest price, so a
// fetch is stored under the Tashkent day it was made.
//
// Only items that clearly are a catalog product are kept (uzexProducts); the
// rest (fuel, metals, polymers, cotton) is not traded on the bazaars.

// UzExQuotesURL is the exchange's public quotes list.
const UzExQuotesURL = "https://uzex.uz/Quote/GetQuotes"

// UzExQuote is one item of the list.
type UzExQuote struct {
	Name  string  // as published
	Price float64 // so'm per the item's unit
}

// uzexProduct maps one exchange item to a catalog product.
type uzexProduct struct {
	Name  string  // the exchange's name, compared after normalizeQuoteName
	Slug  string  // catalog product
	Unit  string  // the exchange's unit
	PerBy float64 // exchange units per product base unit: price / PerBy = price per base unit
}

// uzexProducts are the confident matches. Left out on purpose: cotton-seed
// oil (quoted per tonne, sold by the litre: the conversion depends on the
// oil), flour (one grade quoted, the catalog's "un" mixes grades priced
// 30% apart, so the benchmark band would hold honest prices), unbagged
// cement, and everything the bazaars do not sell.
var uzexProducts = []uzexProduct{
	{Name: "Bug'doy 3 sinf", Slug: "bugdoy", Unit: "tonna", PerBy: 1000},
	{Name: "Armatura 12 st 35 GS o'lchamli", Slug: "armatura", Unit: "tonna", PerBy: 1000},
	{Name: "Portlandsement II/V-K 32,5 H (qoplarda)", Slug: "sement", Unit: "tonna", PerBy: 1000},
}

// lookalikes are Cyrillic letters the exchange mixes into Latin names.
var lookalikes = strings.NewReplacer(
	"А", "A", "В", "B", "Е", "E", "К", "K", "М", "M", "Н", "H", "О", "O", "Р", "P", "С", "C", "Т", "T", "Х", "X",
	"а", "a", "е", "e", "о", "o", "р", "p", "с", "c", "у", "y", "х", "x",
	"‘", "'", "’", "'", "ʻ", "'", "ʼ", "'", "`", "'",
)

// normalizeQuoteName makes names comparable: Latin letters for Cyrillic
// look-alikes, one kind of apostrophe, single spaces, lower case.
func normalizeQuoteName(s string) string {
	return strings.ToLower(strings.Join(strings.Fields(lookalikes.Replace(s)), " "))
}

// parseSom reads "3 222 925.54" (spaces, no-break spaces or commas between
// thousands, a dot or a lone comma before the decimals).
func parseSom(s string) (float64, error) {
	s = strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return -1
		}
		return r
	}, s)
	if strings.Contains(s, ".") {
		s = strings.ReplaceAll(s, ",", "")
	} else if strings.Count(s, ",") == 1 && len(s)-strings.Index(s, ",") <= 3 {
		s = strings.Replace(s, ",", ".", 1)
	} else {
		s = strings.ReplaceAll(s, ",", "")
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil || v <= 0 || math.IsInf(v, 0) {
		return 0, fmt.Errorf("price %q", s)
	}
	return v, nil
}

// ParseUzExQuotes reads the quotes list. Items with an unreadable price are
// skipped; a body that is not the list is an error.
func ParseUzExQuotes(body []byte) ([]UzExQuote, error) {
	var rows []struct {
		Name  string `json:"name"`
		Summa string `json:"summa"`
	}
	if err := json.Unmarshal(body, &rows); err != nil {
		return nil, fmt.Errorf("uzex quotes: %w", err)
	}
	out := make([]UzExQuote, 0, len(rows))
	for _, r := range rows {
		p, err := parseSom(r.Summa)
		if err != nil || strings.TrimSpace(r.Name) == "" {
			continue
		}
		out = append(out, UzExQuote{Name: strings.TrimSpace(r.Name), Price: p})
	}
	if len(out) == 0 {
		return nil, errors.New("uzex quotes: no items")
	}
	return out, nil
}

// UzExMatch is a quote matched to a catalog product.
type UzExMatch struct {
	Quote     UzExQuote
	Slug      string
	Unit      string
	BasePrice float64 // so'm per the product's base unit
}

// MatchUzEx keeps the quotes of uzexProducts.
func MatchUzEx(quotes []UzExQuote) []UzExMatch {
	byName := map[string]uzexProduct{}
	for _, p := range uzexProducts {
		byName[normalizeQuoteName(p.Name)] = p
	}
	var out []UzExMatch
	for _, q := range quotes {
		if p, ok := byName[normalizeQuoteName(q.Name)]; ok {
			out = append(out, UzExMatch{Quote: q, Slug: p.Slug, Unit: p.Unit, BasePrice: q.Price / p.PerBy})
		}
	}
	return out
}

// UzEx stores the exchange quotes as official benchmarks (reference_prices).
type UzEx struct {
	Q      *db.Queries
	Client *http.Client
	URL    string           // UzExQuotesURL in production
	Now    func() time.Time // time.Now when nil
}

// Fetch downloads and parses the quotes list.
func (u *UzEx) Fetch(ctx context.Context) ([]UzExQuote, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.URL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Bugun/1.0 (bozor narxlari tahlili)")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	res, err := u.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("uzex: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("uzex: status %d", res.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(res.Body, 2<<20))
	if err != nil {
		return nil, fmt.Errorf("uzex: %w", err)
	}
	return ParseUzExQuotes(body)
}

// Sync fetches the quotes and upserts today's benchmark of every matched
// product; it returns how many it stored. A list with none of the expected
// items fails, so monitoring notices a changed format.
func (u *UzEx) Sync(ctx context.Context) (int, error) {
	quotes, err := u.Fetch(ctx)
	if err != nil {
		return 0, err
	}
	matches := MatchUzEx(quotes)
	if len(matches) == 0 {
		return 0, fmt.Errorf("uzex: none of %d quotes matched the catalog; has the list changed?", len(quotes))
	}
	now := time.Now
	if u.Now != nil {
		now = u.Now
	}
	t := now().In(tz.Tashkent)
	day := pgtype.Date{Time: time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC), Valid: true}
	n := 0
	for _, m := range matches {
		p, err := u.Q.GetProductBySlug(ctx, m.Slug)
		if errors.Is(err, pgx.ErrNoRows) {
			slog.WarnContext(ctx, "uzex: catalog product missing", "slug", m.Slug)
			continue
		}
		if err != nil {
			return n, err
		}
		price := int64(math.Round(m.BasePrice))
		if price <= 0 {
			continue
		}
		if err := u.Q.UpsertReferencePrice(ctx, db.UpsertReferencePriceParams{
			Source: "uzex", ProductID: p.ID, TradeType: "ulgurji", PriceSom: price,
			QuoteName: m.Quote.Name, QuoteUnit: m.Unit, QuoteSom: m.Quote.Price, QuotedOn: day,
			SourceUrl: pgtype.Text{String: "https://uzex.uz/", Valid: true},
		}); err != nil {
			return n, err
		}
		n++
	}
	return n, nil
}
