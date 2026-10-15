// Command eval measures the AI components on labelled data (spec §16.3):
// the price parser (AI-2), catalog matching (AI-4) and the analyst's routing
// (AI-5). It runs offline on the rule-based paths; a model path runs only
// when its settings (LLM_*, EMBEDDINGS_*) are in the environment. It prints
// the numbers and writes eval/RESULTS.md.
//
//	go run ./cmd/eval [-dir eval] [-out eval/RESULTS.md]
//
// The catalog comes from the database (EVAL_DATABASE_URL, else DATABASE_URL),
// which must be migrated; nothing is written to it.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/joho/godotenv"

	"bugun/internal/analyst"
	"bugun/internal/catalogmatch"
	"bugun/internal/llm"
	"bugun/internal/pricing"
	"bugun/internal/store"
	"bugun/internal/store/db"
)

type expItem struct {
	Product string   `json:"product"`
	Price   int64    `json:"price"`
	Qty     *float64 `json:"qty"`
	Trade   string   `json:"trade"`
}

type parserCase struct {
	ID    string    `json:"id"`
	Text  string    `json:"text"`
	Items []expItem `json:"items"`
	Note  string    `json:"note"`
}

type matchCase struct {
	Word   string `json:"word"`
	Expect string `json:"expect"`
}

type routeCase struct {
	Question string `json:"question"`
	Expect   string `json:"expect"`
}

func main() {
	dir := flag.String("dir", "eval", "directory with the labelled datasets")
	out := flag.String("out", "eval/RESULTS.md", "markdown report to write (empty: none)")
	flag.Parse()
	if err := run(*dir, *out); err != nil {
		fmt.Fprintln(os.Stderr, "eval:", err)
		os.Exit(1)
	}
}

func load(path string, v any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, v)
}

func run(dir, out string) error {
	_ = godotenv.Load()
	url := os.Getenv("EVAL_DATABASE_URL")
	if url == "" {
		url = os.Getenv("DATABASE_URL")
	}
	if url == "" {
		return fmt.Errorf("set EVAL_DATABASE_URL or DATABASE_URL (a migrated database with the catalog)")
	}
	ctx := context.Background()
	pool, err := store.Open(ctx, url)
	if err != nil {
		return err
	}
	defer pool.Close()
	q := db.New(pool)
	prices, err := pricing.NewService(ctx, pool)
	if err != nil {
		return err
	}
	matcher, err := catalogmatch.Load(ctx, q, catalogmatch.DefaultThresholds)
	if err != nil {
		return err
	}
	slugs := map[int16]string{}
	units := map[string]string{}
	rows, err := q.ListParserProducts(ctx)
	if err != nil {
		return err
	}
	for _, r := range rows {
		slugs[r.ID] = r.Slug
		units[r.Slug] = r.BaseUnit
	}

	chat := llm.New(os.Getenv("LLM_BASE_URL"), os.Getenv("LLM_MODEL"), os.Getenv("LLM_API_KEY"), 1000)
	embBase, embKey := os.Getenv("EMBEDDINGS_BASE_URL"), os.Getenv("EMBEDDINGS_API_KEY")
	if embBase == "" {
		embBase = os.Getenv("LLM_BASE_URL")
	}
	if embKey == "" {
		embKey = os.Getenv("LLM_API_KEY")
	}
	emb := llm.New(embBase, os.Getenv("EMBEDDINGS_MODEL"), embKey, 1000)

	var parser []parserCase
	var matches []matchCase
	var routes []routeCase
	for name, v := range map[string]any{"parser.json": &parser, "match.json": &matches, "routing.json": &routes} {
		if err := load(filepath.Join(dir, name), v); err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
	}

	var md strings.Builder
	fmt.Fprintf(&md, "# AI baholash natijalari\n\n%s · `go run ./cmd/eval` (spec §16.3). Qoidaga asoslangan yoʻllar oflayn ishlaydi; model yoʻllari faqat sozlamalar berilganda.\n\n",
		time.Now().Format("2006-01-02 15:04"))

	pr := evalParser(parser, func(c parserCase) []pricing.Item { return prices.Parse(c.Text).Items }, slugs, units)
	var prm *parserResult
	if chat != nil {
		// Rules first; the model reads only what they could not, and its
		// prices are checked like a channel post's (pricing.Verify).
		r := evalParser(parser, func(c parserCase) []pricing.Item {
			if items := prices.Parse(c.Text).Items; len(items) > 0 {
				return items
			}
			cands, err := chat.Extract(llm.WithFeature(ctx, "eval"), prices.CatalogText(), c.Text)
			if err != nil {
				fmt.Fprintln(os.Stderr, "model:", err)
				return nil
			}
			return prices.Verify(c.Text, cands)
		}, slugs, units)
		prm = &r
	}
	md.WriteString("## AI-2: narx ajratish (parser)\n\n")
	fmt.Fprintf(&md, "%d ta xabar, %d ta kutilgan narx, %d ta narxsiz yoki sotuvchidan soʻralishi kerak boʻlgan xabar.\n\n", len(parser), pr.expected, pr.emptyCases)
	rowsOf := func(r parserResult) []string {
		return []string{pct(r.price, r.expected), pct(r.product, r.expected), pct(r.unit, r.expected), pct(r.trade, r.expected),
			pct(r.correct, r.output), pct(r.exact, len(parser)), pct(r.rejected, r.emptyCases)}
	}
	names := []string{"Narx toʻgʻri ajratildi (±0 soʻm)", "Mahsulot katalogga toʻgʻri moslandi", "Birlik va hajm toʻgʻri (asosiy birlikka oʻtkazish)",
		"Savdo turi (ulgurji/chakana)", "Aniqlik (precision): chiqqan narxlardan toʻgʻrisi", "Xabar toʻliq toʻgʻri", "Narxsiz xabar toʻgʻri rad etildi"}
	targets := []string{"≥ 90%", "≥ 90%", "≥ 85%", "—", "—", "—", "—"}
	rules := rowsOf(pr)
	if prm != nil {
		model := rowsOf(*prm)
		md.WriteString("| Koʻrsatkich | Qoidalar | Qoidalar + model (" + os.Getenv("LLM_MODEL") + ") | Maqsad |\n|---|---|---|---|\n")
		for i := range names {
			fmt.Fprintf(&md, "| %s | %s | %s | %s |\n", names[i], rules[i], model[i], targets[i])
		}
	} else {
		md.WriteString("| Koʻrsatkich | Qoidalar | Maqsad |\n|---|---|---|\n")
		for i := range names {
			fmt.Fprintf(&md, "| %s | %s | %s |\n", names[i], rules[i], targets[i])
		}
	}
	md.WriteString("\n<details><summary>Qoidalar xato qilgan xabarlar</summary>\n\n| # | Xabar | Kutilgan | Chiqdi |\n|---|---|---|---|\n")
	for _, f := range pr.failures {
		fmt.Fprintf(&md, "| %s | %s | %s | %s |\n", f[0], mdEscape(f[1]), f[2], f[3])
	}
	md.WriteString("\n</details>\n\n")

	mr := evalMatch(ctx, matches, matcher)
	md.WriteString("## AI-4: katalogga moslash\n\n")
	fmt.Fprintf(&md, "%d ta soʻz (oʻzbek, rus, ingliz, xato yozilgan; %d tasi katalogda yoʻq). Ishonch chegarasi %.2f; undan past boʻlsa sotuvchi tanlaydi.\n\n",
		len(matches), mr.negatives, catalogmatch.DefaultThresholds.MinConfidence)
	matchTable := [][3]string{
		{"Toʻgʻri qaror (moslandi yoki moslanmadi)", pct(mr.correct, len(matches)), "≥ 90%"},
		{"Katalogdagi soʻz avtomatik toʻgʻri moslandi", pct(mr.autoRight, mr.positives), "—"},
		{"Toʻgʻri mahsulot top-3 nomzodlar ichida", pct(mr.top3, mr.positives), "—"},
		{"Notoʻgʻri avtomatik moslash", fmt.Sprint(mr.autoWrong), "0"},
	}
	var mre *matchResult
	if emb != nil {
		m2, err := catalogmatch.Load(ctx, q, catalogmatch.DefaultThresholds)
		if err != nil {
			return err
		}
		m2.SetEmbeddings(emb)
		if err := m2.SyncEmbeddings(ctx); err != nil {
			fmt.Fprintln(os.Stderr, "embeddings:", err)
		} else {
			r := evalMatch(ctx, matches, m2)
			mre = &r
		}
	}
	if mre != nil {
		md.WriteString("| Koʻrsatkich | Lugʻat + trigram | + embedding (" + os.Getenv("EMBEDDINGS_MODEL") + ") | Maqsad |\n|---|---|---|---|\n")
		e := []string{pct(mre.correct, len(matches)), pct(mre.autoRight, mre.positives), pct(mre.top3, mre.positives), fmt.Sprint(mre.autoWrong)}
		for i, row := range matchTable {
			fmt.Fprintf(&md, "| %s | %s | %s | %s |\n", row[0], row[1], e[i], row[2])
		}
	} else {
		md.WriteString("| Koʻrsatkich | Lugʻat + trigram (pg_trgm) | Maqsad |\n|---|---|---|\n")
		for _, row := range matchTable {
			fmt.Fprintf(&md, "| %s | %s | %s |\n", row[0], row[1], row[2])
		}
	}
	md.WriteString("\n<details><summary>Xato qarorlar</summary>\n\n| Soʻz | Kutilgan | Qaror | Nomzodlar |\n|---|---|---|---|\n")
	for _, f := range mr.failures {
		fmt.Fprintf(&md, "| %s | %s | %s | %s |\n", mdEscape(f[0]), f[1], f[2], f[3])
	}
	md.WriteString("\n</details>\n\n")

	rr := evalRoutes(routes, func(q string) string { return analyst.Route(q) })
	md.WriteString("## AI-5: tahlilchi savolni qaysi funksiyaga yoʻnaltiradi\n\n")
	fmt.Fprintf(&md, "%d ta savol (oʻzbek, rus, ingliz), 8 ta funksiya.\n\n", len(routes))
	if chat != nil {
		m := evalRoutes(routes, func(q string) string {
			tool, err := analyst.FirstTool(ctx, chat, q, "uz")
			if err != nil {
				fmt.Fprintln(os.Stderr, "model:", err)
			}
			return tool
		})
		fmt.Fprintf(&md, "| Koʻrsatkich | Qoidalar | Model (%s) |\n|---|---|---|\n| Toʻgʻri funksiya | %s | %s |\n", os.Getenv("LLM_MODEL"), pct(rr.correct, len(routes)), pct(m.correct, len(routes)))
	} else {
		fmt.Fprintf(&md, "| Koʻrsatkich | Qoidalar |\n|---|---|\n| Toʻgʻri funksiya | %s |\n", pct(rr.correct, len(routes)))
	}
	md.WriteString("\nFunksiya boʻyicha (qoidalar): ")
	var tools []string
	for t := range rr.perTool {
		tools = append(tools, t)
	}
	sort.Strings(tools)
	for i, t := range tools {
		if i > 0 {
			md.WriteString(", ")
		}
		fmt.Fprintf(&md, "%s %d/%d", t, rr.perTool[t][0], rr.perTool[t][1])
	}
	md.WriteString("\n\n<details><summary>Xato yoʻnaltirilgan savollar</summary>\n\n| Savol | Kutilgan | Chiqdi |\n|---|---|---|\n")
	for _, f := range rr.failures {
		fmt.Fprintf(&md, "| %s | %s | %s |\n", mdEscape(f[0]), f[1], f[2])
	}
	md.WriteString("\n</details>\n\n")
	md.WriteString("Raqamlar har doim funksiyadan keladi: model yozgan javobdagi har bir raqam funksiya natijasi bilan solishtiriladi (`internal/numcheck`), mos kelmasa shablon javob qaytadi.\n\n")
	md.WriteString("## Oʻlchanmaganlar\n\n- STT (AI-1) soʻz xatosi darajasi (WER) va ovoz → kartochka vaqti: bozorda yozib olingan ovozlar va ularning qoʻlda yozilgan matni kerak (`eval/` ga qoʻshiladi).\n- Suratdan oʻqish (AI-3): narx roʻyxati suratlari toʻplami kerak.\n")

	report := md.String()
	fmt.Print(report)
	if out != "" {
		return os.WriteFile(out, []byte(report), 0o644)
	}
	return nil
}

type parserResult struct {
	expected, emptyCases                         int
	product, price, unit, trade, correct, output int
	exact, rejected                              int
	failures                                     [][4]string
}

func evalParser(cases []parserCase, parse func(parserCase) []pricing.Item, slugs map[int16]string, units map[string]string) parserResult {
	var r parserResult
	for _, c := range cases {
		items := parse(c)
		r.output += len(items)
		got := map[string]pricing.Item{}
		for _, it := range items {
			got[slugs[it.ProductID]] = it
		}
		if len(c.Items) == 0 {
			r.emptyCases++
			if len(items) == 0 {
				r.rejected++
				r.exact++
			} else {
				r.failures = append(r.failures, [4]string{c.ID, c.Text, "—", describe(items, slugs)})
			}
			continue
		}
		ok := len(items) == len(c.Items)
		for _, e := range c.Items {
			r.expected++
			it, found := got[e.Product]
			if !found {
				ok = false
				continue
			}
			r.product++
			qtyOK := (e.Qty == nil && it.Quantity == nil) || (e.Qty != nil && it.Quantity != nil && math.Abs(*e.Qty-*it.Quantity) < 1e-6)
			if it.PriceSom == e.Price {
				r.price++
				r.correct++
			} else {
				ok = false
			}
			if qtyOK && it.Unit == units[e.Product] {
				r.unit++
			} else {
				ok = false
			}
			if it.TradeType == e.Trade {
				r.trade++
			} else {
				ok = false
			}
		}
		if ok {
			r.exact++
		} else {
			var want []string
			for _, e := range c.Items {
				s := fmt.Sprintf("%s %d", e.Product, e.Price)
				if e.Qty != nil {
					s += fmt.Sprintf(" ×%g", *e.Qty)
				}
				want = append(want, s+" "+e.Trade)
			}
			r.failures = append(r.failures, [4]string{c.ID, c.Text, strings.Join(want, "; "), describe(items, slugs)})
		}
	}
	return r
}

func describe(items []pricing.Item, slugs map[int16]string) string {
	if len(items) == 0 {
		return "—"
	}
	var parts []string
	for _, it := range items {
		s := fmt.Sprintf("%s %d", slugs[it.ProductID], it.PriceSom)
		if it.Quantity != nil {
			s += fmt.Sprintf(" ×%g", *it.Quantity)
		}
		parts = append(parts, s+" "+it.TradeType)
	}
	return strings.Join(parts, "; ")
}

type matchResult struct {
	positives, negatives                int
	correct, autoRight, autoWrong, top3 int
	failures                            [][4]string
}

func evalMatch(ctx context.Context, cases []matchCase, m *catalogmatch.Matcher) matchResult {
	var r matchResult
	for _, c := range cases {
		res, err := m.Match(ctx, c.Word)
		if err != nil {
			fmt.Fprintln(os.Stderr, "match:", err)
			continue
		}
		decided := ""
		if res.Match != nil {
			decided = res.Match.Slug
		}
		var cands []string
		for i, cd := range res.Candidates {
			if i < 3 {
				cands = append(cands, fmt.Sprintf("%s %.2f", cd.Slug, cd.Score))
				if c.Expect != "" && cd.Slug == c.Expect {
					r.top3++
				}
			}
		}
		if c.Expect == "" {
			r.negatives++
		} else {
			r.positives++
			if decided == c.Expect {
				r.autoRight++
			}
		}
		if decided != "" && decided != c.Expect {
			r.autoWrong++
		}
		if decided == c.Expect {
			r.correct++
		} else {
			d := decided
			if d == "" {
				d = "—"
			}
			e := c.Expect
			if e == "" {
				e = "—"
			}
			r.failures = append(r.failures, [4]string{c.Word, e, d, strings.Join(cands, ", ")})
		}
	}
	return r
}

type routeResult struct {
	correct  int
	perTool  map[string][2]int
	failures [][3]string
}

func evalRoutes(cases []routeCase, route func(string) string) routeResult {
	r := routeResult{perTool: map[string][2]int{}}
	for _, c := range cases {
		got := route(c.Question)
		t := r.perTool[c.Expect]
		t[1]++
		if got == c.Expect {
			r.correct++
			t[0]++
		} else {
			r.failures = append(r.failures, [3]string{c.Question, c.Expect, got})
		}
		r.perTool[c.Expect] = t
	}
	return r
}

func pct(n, d int) string {
	if d == 0 {
		return "—"
	}
	return fmt.Sprintf("%.1f%% (%d/%d)", 100*float64(n)/float64(d), n, d)
}

func mdEscape(s string) string {
	return strings.NewReplacer("|", "\\|", "\n", " ⏎ ").Replace(s)
}
