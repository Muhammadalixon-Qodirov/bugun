package llm

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	"bugun/internal/pricing"
	"bugun/internal/store"
)

// A test set of hard price texts (MATN_TUSHUNISH.md 3) to choose a model:
// Russian and Cyrillic words, slang, spelled-out numbers, emojis, amounts and
// phone numbers that are not prices. It calls the real API, so it only runs
// with LLM_EVAL_KEY set:
//
//	LLM_EVAL_KEY=... LLM_EVAL_MODELS=llama-3.3-70b-versatile,openai/gpt-oss-120b \
//	TEST_DATABASE_URL=... go test -run TestEvalModels -v ./internal/llm
type evalCase struct {
	text string
	want map[string]int64 // slug → price per base unit; trade checked separately
	opt  bool             // wholesale expected
}

var evalCases = []evalCase{
	{"bulba kilosi 4800", map[string]int64{"kartoshka": 4800}, false},
	{"Qizil pamidor 7 lik, bodring 5 lik", map[string]int64{"pomidor": 7000, "bodring": 5000}, false},
	{"🍅 Tomat — 6 500 so'm/kg ✅", map[string]int64{"pomidor": 6500}, false},
	{"Говядина 95 000, баранина 105 000", map[string]int64{"mol-goshti": 95000, "qoy-goshti": 105000}, false},
	{"Yangi hosil! Mandarin Pokiston 18 ming, apelsin Misr 16 ming", map[string]int64{"mandarin": 18000, "apelsin": 16000}, false},
	{"Tuxum 1 dona 1600", map[string]int64{"tuxum": 1600}, false},
	{"Sabzi (sariq) optom 3200", map[string]int64{"sabzi": 3200}, true},
	{"Бугун бозорда пиёз 2800, сабзи 3500", map[string]int64{"piyoz": 2800, "sabzi": 3500}, false},
	{"Guruch Lazer 18 000 dan", map[string]int64{"guruch": 18000}, false},
	{"Paxta yog'i 1 litri 24 ming", map[string]int64{"paxta-yogi": 24000}, false},
	{"Олма семиренко 12000 кг", map[string]int64{"olma": 12000}, false},
	{"Limon 22 ming, banan 19 ming, kivi 25 ming. Hammasi optom", map[string]int64{"limon": 22000, "banan": 19000, "kivi": 25000}, true},
	{"Assalomu alaykum. Bugun bizda tovuq go'shti 32 000, qiyma 85 000", map[string]int64{"tovuq": 32000, "qiyma": 85000}, false},
	{"Ertaga bozor yopiq, 10% chegirma kutib turing", map[string]int64{}, false},
	{"Tel: 90 123 45 67, manzil Chorsu", map[string]int64{}, false},
	{"Kartoshka 4500, gilos 30 000, yangi uzum 14 ming", map[string]int64{"kartoshka": 4500, "gilos": 30000, "uzum": 14000}, false},
	{"sarimsoq piyoz 25 ming", map[string]int64{"sarimsoq": 25000}, false},
	{"Pomidor 6000, 2 tonna bor", map[string]int64{"pomidor": 6000}, false},
	{"Ishonchli sotuvchi: asal 1 kg 120 000", map[string]int64{"asal": 120000}, false},
	{"мол гўшти 98 минг", map[string]int64{"mol-goshti": 98000}, false},
	{"Karam arzon 1500-2000", map[string]int64{"karam": 1500}, false},
	{"Bodom 140 ming, pista 180 ming", map[string]int64{"bodom": 140000, "pista": 180000}, false},
	{"Бодринг 8 мингдан, помидор 9 мингдан", map[string]int64{"bodring": 8000, "pomidor": 9000}, false},
	{"картошка оптом 3800 сум кг", map[string]int64{"kartoshka": 3800}, true},
	{"Shaftoli 12 000 so'mdan beramiz, olxo'ri 10 000", map[string]int64{"shaftoli": 12000, "olxori": 10000}, false},
	// Names the catalog does not hold: only the model can read these.
	{"Oq karam 2000, rangli karam 9000", map[string]int64{"karam": 2000, "gulkaram": 9000}, false},
	{"Ukrop 1000, kinza 1500 bog'i emas kilosi", map[string]int64{"shivit": 1000, "kashnich": 1500}, false},
	{"Semechka yog'i 22 000 litri", map[string]int64{"kungaboqar-yogi": 22000}, false},
	{"Bolgarskiy qalampir 12 000", map[string]int64{"bulgor": 12000}, false},
	{"Yer yong'oq 40 ming", map[string]int64{}, false},
	{"Meva: xurmo 20, anor 25 ming", map[string]int64{"xurmo": 20000, "anor": 25000}, false},
	{"Qo'y go'shti 1 kg - 110.000 so'm", map[string]int64{"qoy-goshti": 110000}, false},
}

func TestEvalModels(t *testing.T) {
	key, url := os.Getenv("LLM_EVAL_KEY"), os.Getenv("TEST_DATABASE_URL")
	if key == "" || url == "" {
		t.Skip("LLM_EVAL_KEY or TEST_DATABASE_URL not set")
	}
	base := os.Getenv("LLM_EVAL_BASE")
	if base == "" {
		base = "https://api.groq.com/openai/v1"
	}
	models := strings.Split(os.Getenv("LLM_EVAL_MODELS"), ",")
	// LLM_EVAL_PAUSE waits between calls to stay under a free tier's tokens per minute.
	pause, _ := time.ParseDuration(os.Getenv("LLM_EVAL_PAUSE"))
	ctx := context.Background()
	pool, err := store.Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	slugs := map[int16]string{}
	rows, err := pool.Query(ctx, `SELECT id, slug FROM products`)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var id int16
		var s string
		_ = rows.Scan(&id, &s)
		slugs[id] = s
	}
	rows.Close()

	run := func(name string, svc *pricing.Service) {
		right, wrong := 0, 0
		for _, c := range evalCases {
			u := svc.Read(ctx, c.text)
			got := map[string]int64{}
			tradeOK := true
			for _, it := range u.Items {
				got[slugs[it.ProductID]] = it.PriceSom
				if (it.TradeType == pricing.TradeWholesale) != c.opt {
					tradeOK = false
				}
			}
			ok := tradeOK && len(got) == len(c.want)
			bad := 0
			for s, p := range got {
				if c.want[s] != p {
					ok, bad = false, bad+1
				}
			}
			if ok {
				right++
			} else {
				t.Logf("%-28s ✗ %q\n      got %v, want %v", name, c.text, sorted(got), sorted(c.want))
			}
			wrong += bad
		}
		t.Logf("%-28s %d/%d texts right, %d wrong prices", name, right, len(evalCases), wrong)
	}
	rules, err := pricing.NewService(ctx, pool)
	if err != nil {
		t.Fatal(err)
	}
	run("rules only", rules)
	for _, m := range models {
		if m = strings.TrimSpace(m); m == "" {
			continue
		}
		svc, err := pricing.NewService(ctx, pool)
		if err != nil {
			t.Fatal(err)
		}
		svc.SetExtractor(paced{New(base, m, key, 1000), pause})
		run(m, svc)
	}
}

// paced waits before every call.
type paced struct {
	c     *Client
	pause time.Duration
}

func (p paced) Extract(ctx context.Context, catalog, text string) ([]pricing.Candidate, error) {
	time.Sleep(p.pause)
	return p.c.Extract(ctx, catalog, text)
}

func sorted(m map[string]int64) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, k := range keys {
		fmt.Fprintf(&b, "%s=%d ", k, m[k])
	}
	return strings.TrimSpace(b.String())
}
