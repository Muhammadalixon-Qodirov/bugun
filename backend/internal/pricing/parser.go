// Package pricing turns a seller's free-text message into structured price
// items. This is the rule-based extractor from spec §9 (AI-2 fallback): it is
// deterministic, so every result can be reproduced and tested.
package pricing

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
)

// Product is the part of the catalog the parser needs.
type Product struct {
	ID       int16
	Slug     string
	Name     string
	Category string
	BaseUnit string // kg, litr, dona, ...
	Aliases  []string
	// Units maps a package unit (qop, yashik, ...) to its size in BaseUnit.
	Units map[string]float64
}

type Catalog struct {
	products []Product
	aliases  []alias // sorted: longest first, so "mol goshti" wins over "mol"
}

type alias struct {
	words   []string
	product int
}

func NewCatalog(products []Product) *Catalog {
	c := &Catalog{products: products}
	for i, p := range products {
		seen := map[string]bool{}
		for _, a := range append([]string{p.Name}, p.Aliases...) {
			n := strings.Join(strings.Fields(Normalize(a)), " ")
			if len(n) < 2 || seen[n] {
				continue
			}
			seen[n] = true
			c.aliases = append(c.aliases, alias{words: strings.Fields(n), product: i})
		}
	}
	sort.SliceStable(c.aliases, func(i, j int) bool {
		return len(strings.Join(c.aliases[i].words, " ")) > len(strings.Join(c.aliases[j].words, " "))
	})
	return c
}

const (
	TradeWholesale = "ulgurji"
	TradeRetail    = "chakana"
)

// Item is one confirmed-able price line. Price and quantity are expressed in
// the product's base unit.
type Item struct {
	ProductID   int16    `json:"product_id"`
	ProductName string   `json:"product_name"`
	Category    string   `json:"category"`
	Unit        string   `json:"unit"`
	PriceSom    int64    `json:"price_som"`
	Quantity    *float64 `json:"quantity,omitempty"`
	TradeType   string   `json:"trade_type"`
	// Notes explain interpretations the seller should double-check.
	Notes []string `json:"notes,omitempty"`
	// Hints are the same notes as codes with their numbers, for other languages.
	Hints []Hint `json:"hints,omitempty"`
	Raw   string `json:"raw"`
}

// Hint is one interpretation behind an item, as a code.
type Hint struct {
	Code string `json:"code" enum:"read_as_thousands,converted,by_model,near_spelling,matched"`
	// read_as_thousands: the number as written ("4.5") and the price it became.
	// matched: the word as written that was read as this product.
	Written string `json:"written,omitempty"`
	Price   int64  `json:"price,omitempty"`
	// converted: 1 Unit = PerBase BaseUnit.
	Unit     string  `json:"unit,omitempty"`
	PerBase  float64 `json:"per_base,omitempty"`
	BaseUnit string  `json:"base_unit,omitempty"`
}

// Problem describes a fragment that could not be turned into an item.
type Problem struct {
	Raw     string `json:"raw"`
	Message string `json:"message" doc:"Uzbek text; translate from code instead"`
	Code    string `json:"code" enum:"no_price,unknown_unit,bad_price,bad_quantity,no_product"`
	// The product's catalog name and slug, when one was recognised.
	Product     string `json:"product,omitempty"`
	ProductSlug string `json:"product_slug,omitempty"`
	// unknown_unit: the unit written and the product's base unit.
	Unit     string `json:"unit,omitempty"`
	BaseUnit string `json:"base_unit,omitempty"`
	// bad_price: the price it came to.
	Price int64 `json:"price,omitempty"`
}

// unitError is a unit the product has no size for ("qop" of cement).
type unitError struct {
	product Product
	unit    string
}

func (e unitError) Error() string {
	return fmt.Sprintf("%s: «%s» necha %s ekanini bilmaymiz. Narxni 1 %s uchun yozing.", e.product.Name, e.unit, e.product.BaseUnit, e.product.BaseUnit)
}

// problemFor builds a Problem about product p with its code and Uzbek message.
func problemFor(p Product, raw, code, message string) *Problem {
	return &Problem{Raw: raw, Message: message, Code: code, Product: p.Name, ProductSlug: p.Slug}
}

// unitProblem turns a toBase error into a Problem.
func unitProblem(p Product, raw string, err error) *Problem {
	pr := problemFor(p, raw, "unknown_unit", err.Error())
	var ue unitError
	if errors.As(err, &ue) {
		pr.Unit, pr.BaseUnit = ue.unit, p.BaseUnit
	}
	return pr
}

type Result struct {
	Items    []Item    `json:"items"`
	Problems []Problem `json:"problems"`
}

const (
	minPriceSom = 50
	maxPriceSom = 50_000_000
	maxQuantity = 10_000_000
)

// Parse extracts every product mention with its price and optional quantity.
func (c *Catalog) Parse(text string) Result {
	res := Result{Items: []Item{}, Problems: []Problem{}}
	toks := tokenize(Normalize(text))
	wholesale := hasAny(toks, "optom", "opt", "ulgurji", "ulgurjiga", "optomga", "optovoy")

	segs, orphans := c.segments(toks)
	seen := map[string]bool{}
	for _, o := range orphans {
		res.Problems = append(res.Problems, Problem{Raw: o, Code: "no_product", Message: fmt.Sprintf("«%s»: mahsulot tanilmadi.", o)})
	}
	for _, seg := range segs {
		item, problem := c.parseSegment(seg)
		if problem != nil {
			res.Problems = append(res.Problems, *problem)
			continue
		}
		item.TradeType = TradeRetail
		if wholesale {
			item.TradeType = TradeWholesale
		}
		// The same product twice ("oq karam 2000, rangli karam 9000") is
		// usually two products the catalog names alike: keep the first and
		// report the second rather than let it overwrite the first.
		if seen[item.TradeType+"|"+item.ProductName] {
			res.Problems = append(res.Problems, Problem{Raw: item.Raw, Code: "no_product", Product: item.ProductName,
				Message: fmt.Sprintf("«%s»: %s ikki marta yozilgan, bu qaysi mahsulot ekani aniq emas.", item.Raw, item.ProductName)})
			continue
		}
		seen[item.TradeType+"|"+item.ProductName] = true
		res.Items = append(res.Items, item)
	}
	return res
}

type segment struct {
	product int
	toks    []token // tokens after the product name
	raw     string
	written string // the word as written when it was matched by a near spelling
}

// segments cuts the message at every product mention; each product owns the
// tokens up to the next product. A stretch between commas or lines with a
// word nobody knows and its own number ("pomidor 6000, gilos 30000" when
// gilos is not in the catalog) is returned as an orphan instead of being
// read as the previous product's quantity.
func (c *Catalog) segments(toks []token) ([]segment, []string) {
	var segs []segment
	var lead, buf []token // lead: words before the first product, e.g. "2 tonna kartoshka 4500"
	var orphans []string
	cur := func() *segment {
		if len(segs) == 0 {
			return nil
		}
		return &segs[len(segs)-1]
	}
	flush := func() {
		if len(buf) == 0 {
			return
		}
		s := cur()
		if c.orphan(buf) && (s == nil || hasNumber(s.toks)) {
			orphans = append(orphans, joinTokens(buf))
		} else if s == nil {
			lead = append(lead, buf...)
		} else {
			s.toks = append(s.toks, buf...)
		}
		buf = nil
	}
	for i := 0; i < len(toks); {
		t := toks[i]
		if t.kind == tokBreak || t.kind == tokComma {
			flush()
			switch {
			case cur() != nil:
				cur().toks = append(cur().toks, token{kind: tokComma})
			case t.kind == tokBreak:
				lead = nil
			default:
				lead = append(lead, t)
			}
			i++
			continue
		}
		if n := notProduct(toks[i:]); n > 0 {
			buf = append(buf, toks[i:i+n]...) // read as unknown words
			i += n
			continue
		}
		p, n, written := c.matchProduct(toks[i:])
		if p < 0 {
			buf = append(buf, t)
			i++
			continue
		}
		flush()
		segs = append(segs, segment{product: p, written: written})
		if len(segs) == 1 && len(lead) > 0 {
			segs[0].toks = append(append([]token{}, lead...), token{kind: tokComma})
		}
		lead = nil
		i += n
	}
	flush()
	for k := range segs {
		segs[k].raw = strings.TrimSpace(c.products[segs[k].product].Name + " " + joinTokens(segs[k].toks))
	}
	return segs, orphans
}

// notProducts are phrases that hold a catalog name but mean something the
// catalog does not have: "yer yong'oq" is peanuts, not walnuts.
var notProducts = [][]string{{"yer", "yongoq"}, {"yeryongoq"}}

// notProduct returns how many tokens a phrase of notProducts spans at toks[0].
func notProduct(toks []token) int {
	for _, ph := range notProducts {
		if len(ph) > len(toks) {
			continue
		}
		ok := true
		for k, w := range ph {
			if toks[k].kind != tokWord || !strings.HasPrefix(toks[k].text, w) {
				ok = false
				break
			}
		}
		if ok {
			return len(ph)
		}
	}
	return 0
}

// plainWord is a word that names no product: a unit, number, currency or
// common filler.
func plainWord(w string) bool {
	return fillerWords[w] || isNumberWord(w) || unitWord(w) != "" || isPerUnit(w) != "" || isCurrency(w)
}

// orphan reports whether a stretch has a number and a word nobody knows.
func (c *Catalog) orphan(toks []token) bool {
	unknown := false
	for _, t := range toks {
		if t.kind == tokWord && len([]rune(t.text)) >= 3 && !plainWord(t.text) {
			unknown = true
		}
	}
	return unknown && hasNumber(toks)
}

func hasNumber(toks []token) bool {
	for _, t := range toks {
		if t.kind == tokNumber || (t.kind == tokWord && isNumberWord(t.text) && t.text != "yarim") {
			return true
		}
	}
	return false
}

// matchProduct reports which product starts at toks[0], how many tokens it
// spans and, for a near spelling, the word as written. A word may carry a
// suffix ("kartoshkani", "piyozdan").
func (c *Catalog) matchProduct(toks []token) (int, int, string) {
	for _, a := range c.aliases {
		if len(a.words) > len(toks) {
			continue
		}
		ok := true
		for k, w := range a.words {
			t := toks[k]
			if t.kind != tokWord {
				ok = false
				break
			}
			last := k == len(a.words)-1
			if !(t.text == w || (last && len(w) >= 4 && strings.HasPrefix(t.text, w) && len(t.text)-len(w) <= 4)) {
				ok = false
				break
			}
		}
		if ok {
			return a.product, len(a.words), ""
		}
	}
	if len(toks) > 0 && toks[0].kind == tokWord {
		if p := c.nearProduct(toks[0].text); p >= 0 {
			return p, 1, toks[0].text
		}
	}
	return -1, 0, ""
}

// Near spellings (MATN_TUSHUNISH.md 2): "kartoshga", "pamidr".
const minFuzzyLen = 5

var fuzzySuffixes = []string{"ning", "dan", "lar", "ni", "ga", "da", "si"}

// nearProduct finds the one product whose single-word name is within one
// edit of w (two for names of 8 letters or more), also after cutting a
// suffix. Two products equally near means no answer.
func (c *Catalog) nearProduct(w string) int {
	if len([]rune(w)) < minFuzzyLen || plainWord(w) {
		return -1
	}
	forms := []string{w}
	for _, suf := range fuzzySuffixes {
		if base := strings.TrimSuffix(w, suf); base != w && len([]rune(base)) >= minFuzzyLen-1 {
			forms = append(forms, base)
		}
	}
	best, bestD, tie := -1, 3, false
	for _, a := range c.aliases {
		if len(a.words) != 1 || len([]rune(a.words[0])) < minFuzzyLen-1 {
			continue
		}
		limit := 1
		if len([]rune(a.words[0])) >= 8 {
			limit = 2
		}
		for _, f := range forms {
			d := editDistance(f, a.words[0])
			if d > limit || d > bestD {
				continue
			}
			switch {
			case d < bestD:
				best, bestD, tie = a.product, d, false
			case a.product != best:
				tie = true
			}
		}
	}
	if tie {
		return -1
	}
	return best
}

// editDistance counts insertions, deletions, substitutions and swaps of
// neighbouring letters (optimal string alignment).
func editDistance(a, b string) int {
	x, y := []rune(a), []rune(b)
	d := make([][]int, len(x)+1)
	for i := range d {
		d[i] = make([]int, len(y)+1)
		d[i][0] = i
	}
	for j := range d[0] {
		d[0][j] = j
	}
	for i := 1; i <= len(x); i++ {
		for j := 1; j <= len(y); j++ {
			cost := 1
			if x[i-1] == y[j-1] {
				cost = 0
			}
			d[i][j] = min(d[i-1][j]+1, d[i][j-1]+1, d[i-1][j-1]+cost)
			if i > 1 && j > 1 && x[i-1] == y[j-2] && x[i-2] == y[j-1] {
				d[i][j] = min(d[i][j], d[i-2][j-2]+1)
			}
		}
	}
	return d[len(x)][len(y)]
}

// number is a numeric phrase inside a segment: "4500", "4,5 ming", "to'rt yarim ming".
type number struct {
	value      float64
	decimal    bool // written with a fractional part, e.g. "4.5"
	multiplied bool // had "ming"/"mln" or was spelled out with a scale word
	start, end int  // token span [start, end)
}

func (c *Catalog) parseSegment(seg segment) (Item, *Problem) {
	p := c.products[seg.product]
	item := Item{ProductID: p.ID, ProductName: p.Name, Category: p.Category, Unit: p.BaseUnit, Raw: seg.raw}
	if seg.written != "" {
		item.Notes = append(item.Notes, fmt.Sprintf("«%s» %s deb tushunildi", seg.written, p.Name))
		item.Hints = append(item.Hints, Hint{Code: "near_spelling", Written: seg.written})
	}
	nums := findNumbers(seg.toks)
	if len(nums) == 0 {
		return item, problemFor(p, seg.raw, "no_price", fmt.Sprintf("%s: narx topilmadi. Masalan: «%s 4500».", p.Name, strings.ToLower(p.Name)))
	}

	var (
		price, qty         *number
		priceUnit, qtyUnit string
	)
	// First pass: numbers with explicit cues.
	for i := range nums {
		n := &nums[i]
		before := wordAt(seg.toks, n.start-1)
		after := wordAt(seg.toks, n.end)
		switch {
		case price == nil && isPerUnit(before) != "":
			price, priceUnit = n, isPerUnit(before)
		case price == nil && (isCurrency(after) || n.multiplied):
			price = n
			if u := slashUnit(seg.toks, n.end); u != "" {
				priceUnit = u
			} else if isCurrency(after) {
				if u := slashUnit(seg.toks, n.end+1); u != "" {
					priceUnit = u
				} else if u := isPerUnit(wordAt(seg.toks, n.end+1)); u != "" {
					priceUnit = u
				}
			}
		case price == nil && slashUnit(seg.toks, n.end) != "":
			price, priceUnit = n, slashUnit(seg.toks, n.end)
		case qty == nil && unitWord(after) != "":
			qty, qtyUnit = n, unitWord(after)
		}
	}
	// Second pass: bare numbers fill whatever is still missing, price first.
	for i := range nums {
		n := &nums[i]
		if n == price || n == qty {
			continue
		}
		if price == nil {
			price = n
		} else if qty == nil {
			qty = n
		}
	}
	// "kartoshka 4500 kg": a lone number with a unit is the price for that unit.
	if price == nil && qty != nil && len(nums) == 1 {
		price, priceUnit, qty, qtyUnit = qty, qtyUnit, nil, ""
	}
	// "pomidor 120 ming yashik", "120 000 so'm qop": a lone price followed by a
	// unit is the price of that unit.
	if price != nil && priceUnit == "" && qty == nil && len(nums) == 1 {
		next := wordAt(seg.toks, price.end)
		if isCurrency(next) {
			next = wordAt(seg.toks, price.end+1)
		}
		priceUnit = unitWord(next)
	}
	if price == nil {
		return item, problemFor(p, seg.raw, "no_price", fmt.Sprintf("%s: narx topilmadi.", p.Name))
	}

	value := price.value
	if value < 1000 && !price.multiplied && (price.decimal || value < 100) {
		// Sellers often say "4.5" meaning 4 500 so'm; show it for confirmation.
		value *= 1000
		item.Notes = append(item.Notes, fmt.Sprintf("«%s» %s so'm deb tushunildi", trimFloat(price.value), groupDigits(int64(value))))
		item.Hints = append(item.Hints, Hint{Code: "read_as_thousands", Written: trimFloat(price.value), Price: int64(value)})
	}
	perBase, err := toBase(p, priceUnit, 1)
	if err != nil {
		return item, unitProblem(p, seg.raw, err)
	}
	item.PriceSom = int64(math.Round(value / perBase))
	if priceUnit != "" && priceUnit != p.BaseUnit {
		item.Notes = append(item.Notes, fmt.Sprintf("1 %s = %s %s bo'yicha hisoblandi", priceUnit, trimFloat(perBase), p.BaseUnit))
		item.Hints = append(item.Hints, Hint{Code: "converted", Unit: priceUnit, PerBase: perBase, BaseUnit: p.BaseUnit})
	}
	if item.PriceSom < minPriceSom || item.PriceSom > maxPriceSom {
		pr := problemFor(p, seg.raw, "bad_price", fmt.Sprintf("%s: narx %s so'm — noto'g'ri ko'rinadi.", p.Name, groupDigits(item.PriceSom)))
		pr.Price = item.PriceSom
		return item, pr
	}

	if qty != nil {
		q, err := toBase(p, qtyUnit, qty.value)
		if err != nil {
			return item, unitProblem(p, seg.raw, err)
		}
		if q <= 0 || q > maxQuantity {
			return item, problemFor(p, seg.raw, "bad_quantity", fmt.Sprintf("%s: hajm noto'g'ri ko'rinadi.", p.Name))
		}
		q = math.Round(q*1000) / 1000
		item.Quantity = &q
	}
	return item, nil
}

// toBase converts amount in unit to the product's base unit.
func toBase(p Product, unit string, amount float64) (float64, error) {
	switch unit {
	case "", p.BaseUnit:
		return amount, nil
	case "tonna":
		if p.BaseUnit == "kg" {
			return amount * 1000, nil
		}
	case "gramm":
		if p.BaseUnit == "kg" {
			return amount / 1000, nil
		}
	}
	if size, ok := p.Units[unit]; ok {
		return amount * size, nil
	}
	return 0, unitError{product: p, unit: unit}
}

var (
	digitWords = map[string]float64{
		"bir": 1, "ikki": 2, "uch": 3, "tort": 4, "besh": 5, "olti": 6, "yetti": 7, "sakkiz": 8, "toqqiz": 9,
	}
	tenWords = map[string]float64{
		"on": 10, "yigirma": 20, "ottiz": 30, "qirq": 40, "ellik": 50, "oltmish": 60, "yetmish": 70, "sakson": 80, "toqson": 90,
	}
	scaleWords = map[string]float64{"ming": 1000, "mln": 1_000_000, "million": 1_000_000, "mlyon": 1_000_000}
)

func isNumberWord(w string) bool {
	_, d := digitWords[w]
	_, t := tenWords[w]
	_, s := scaleWords[w]
	return d || t || s || w == "yuz" || w == "yarim"
}

// findNumbers groups digits and spelled-out number words into values.
func findNumbers(toks []token) []number {
	var out []number
	for i := 0; i < len(toks); {
		t := toks[i]
		if t.kind != tokNumber && !(t.kind == tokWord && isNumberWord(t.text) && t.text != "yarim") {
			i++
			continue
		}
		start := i
		var total, current float64
		var decimal, multiplied, any bool
		for ; i < len(toks); i++ {
			t := toks[i]
			if t.kind == tokNumber {
				if any && current != 0 {
					break // a second literal number starts a new value
				}
				v, err := strconv.ParseFloat(strings.ReplaceAll(t.text, ",", "."), 64)
				if err != nil {
					break
				}
				decimal = decimal || strings.ContainsAny(t.text, ".,")
				current += v
				any = true
				continue
			}
			if t.kind != tokWord || !isNumberWord(t.text) {
				break
			}
			switch {
			case digitWords[t.text] > 0:
				current += digitWords[t.text]
			case tenWords[t.text] > 0:
				current += tenWords[t.text]
			case t.text == "yuz":
				if current == 0 {
					current = 1
				}
				current *= 100
			case t.text == "yarim":
				current += 0.5
			default: // ming, mln
				if current == 0 {
					current = 1
				}
				total += current * scaleWords[t.text]
				current = 0
				multiplied = true
			}
			any = true
		}
		if any {
			out = append(out, number{value: total + current, decimal: decimal, multiplied: multiplied, start: start, end: i})
		}
		if i == start {
			i++
		}
	}
	return out
}

func wordAt(toks []token, i int) string {
	if i < 0 || i >= len(toks) || toks[i].kind != tokWord {
		return ""
	}
	return toks[i].text
}

// slashUnit detects "4500/kg" style price units.
func slashUnit(toks []token, i int) string {
	if i < len(toks) && toks[i].kind == tokSlash {
		return unitWord(wordAt(toks, i+1))
	}
	return ""
}

var units = map[string]string{
	"kg": "kg", "kilo": "kg", "kila": "kg", "kilogramm": "kg", "kilogram": "kg", "kgdan": "kg",
	"tonna": "tonna", "t": "tonna", "tn": "tonna", "tonnadan": "tonna", "tonn": "tonna", "tonni": "tonna",
	"gramm": "gramm", "gr": "gramm",
	"qop": "qop", "meshok": "qop", "qopdan": "qop",
	"yashik": "yashik", "yashikdan": "yashik", "yashchik": "yashik",
	"karobka": "karobka", "korobka": "karobka", "karobkadan": "karobka", "lotok": "lotok",
	"litr": "litr", "l": "litr", "litrdan": "litr",
	"dona": "dona", "ta": "dona", "donadan": "dona", "shtuk": "dona", "shtuka": "dona", "sht": "dona",
}

var perUnit = map[string]string{
	"kilosi": "kg", "kilasi": "kg", "kgsi": "kg", "kilogrami": "kg", "kgi": "kg", "kilogramgi": "kg",
	"tonnasi": "tonna",
	"qopi":    "qop", "meshogi": "qop",
	"yashigi": "yashik", "yashiki": "yashik",
	"karobkasi": "karobka", "lotogi": "lotok",
	"litri":  "litr",
	"donasi": "dona",
}

func unitWord(w string) string { return units[w] }

// isPerUnit recognises possessive forms like "kilosi" ("its kilo") that mark a price unit.
func isPerUnit(w string) string { return perUnit[w] }

func isCurrency(w string) bool {
	switch w {
	case "som", "sum", "somdan", "sumdan", "dan", "sumga", "somga":
		return true
	}
	return false
}

func hasAny(toks []token, words ...string) bool {
	for _, t := range toks {
		for _, w := range words {
			if t.kind == tokWord && t.text == w {
				return true
			}
		}
	}
	return false
}

func joinTokens(toks []token) string {
	parts := make([]string, 0, len(toks))
	for _, t := range toks {
		if t.kind == tokWord || t.kind == tokNumber || t.kind == tokSlash {
			parts = append(parts, t.text)
		}
	}
	return strings.Join(parts, " ")
}

func trimFloat(f float64) string {
	return strconv.FormatFloat(f, 'f', -1, 64)
}

// groupDigits formats 4500 as "4 500".
func groupDigits(n int64) string {
	s := strconv.FormatInt(n, 10)
	var b strings.Builder
	for i, r := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte(' ')
		}
		b.WriteRune(r)
	}
	return b.String()
}

// Demand is what a buyer asks for in an RFQ: a product and, when given, an
// amount in the product's base unit.
type Demand struct {
	ProductID   int16    `json:"product_id"`
	ProductName string   `json:"product_name"`
	Unit        string   `json:"unit"`
	Quantity    *float64 `json:"quantity,omitempty"`
}

// ParseDemand reads a buyer's request such as "kartoshka 200 kg" or
// "2 tonna piyoz kerak": the first product mentioned and its first amount.
// Unlike Parse there is no price; every number is an amount.
func (c *Catalog) ParseDemand(text string) (Demand, *Problem) {
	segs, _ := c.segments(tokenize(Normalize(text)))
	if len(segs) == 0 {
		return Demand{}, &Problem{Raw: text, Message: "Mahsulot tanilmadi. Masalan: «kartoshka 200 kg».", Code: "no_product"}
	}
	seg := segs[0]
	p := c.products[seg.product]
	d := Demand{ProductID: p.ID, ProductName: p.Name, Unit: p.BaseUnit}
	nums := findNumbers(seg.toks)
	if len(nums) == 0 {
		return d, nil
	}
	n := nums[0]
	unit := unitWord(wordAt(seg.toks, n.end))
	q, err := toBase(p, unit, n.value)
	if err != nil {
		return d, unitProblem(p, seg.raw, err)
	}
	if q <= 0 || q > maxQuantity {
		return d, problemFor(p, seg.raw, "bad_quantity", fmt.Sprintf("%s: hajm noto'g'ri ko'rinadi.", p.Name))
	}
	q = math.Round(q*1000) / 1000
	d.Quantity = &q
	return d, nil
}
