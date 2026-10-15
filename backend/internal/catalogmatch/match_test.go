package catalogmatch

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"bugun/internal/llm"
	"bugun/internal/store"
	"bugun/internal/store/db"
)

var testCatalog = []Product{
	{ID: 1, Slug: "kartoshka", Name: "Kartoshka", Aliases: []string{"kartoshka", "kartofel", "kartop", "картошка", "картофель"}},
	{ID: 2, Slug: "piyoz", Name: "Piyoz", Aliases: []string{"piyoz", "luk", "лук", "пиёз"}},
	{ID: 3, Slug: "sabzi", Name: "Sabzi", Aliases: []string{"sabzi", "sabzi sariq", "морковь", "сабзи"}},
	{ID: 4, Slug: "pomidor", Name: "Pomidor", Aliases: []string{"pomidor", "pamidor", "tomat", "помидор", "помидоры"}},
	{ID: 5, Slug: "bodring", Name: "Bodring", Aliases: []string{"bodring", "ogurets", "огурец", "огурцы", "бодринг"}},
	{ID: 6, Slug: "karam", Name: "Karam", Aliases: []string{"karam", "kapusta", "капуста", "карам"}},
	{ID: 9, Slug: "olma", Name: "Olma", Aliases: []string{"olma", "yabloko", "яблоко", "яблоки"}},
	{ID: 13, Slug: "mol-goshti", Name: "Mol goʻshti", Aliases: []string{"mol goshti", "mol go'shti", "govyadina", "говядина"}},
	{ID: 52, Slug: "orik", Name: "Oʻrik", Aliases: []string{"orik", "o'rik", "abrikos", "абрикос"}},
}

func TestMatchMultilingual(t *testing.T) {
	m := New(testCatalog, nil, DefaultThresholds)
	for word, want := range map[string]string{
		"pamidor": "pomidor", "помидор": "pomidor", "tomat": "pomidor", "Pomidorlar": "pomidor",
		"картошки": "kartoshka", "kartoshkaa": "kartoshka", "лук": "piyoz", "Ўрик": "orik", "o‘rik": "orik",
		"морковь": "sabzi", "огурцы": "bodring", "kapusta": "karam", "яблоки": "olma", "mol-goshti": "mol-goshti",
		"govyadina": "mol-goshti",
	} {
		r, err := m.Match(context.Background(), word)
		if err != nil {
			t.Fatal(err)
		}
		if r.Match == nil || r.Match.Slug != want {
			t.Errorf("%q: match %+v, candidates %+v; want %s", word, r.Match, r.Candidates, want)
		}
	}
}

func TestMatchLowConfidenceGivesCandidates(t *testing.T) {
	m := New(testCatalog, nil, DefaultThresholds)
	r, err := m.Match(context.Background(), "pamidr")
	if err != nil {
		t.Fatal(err)
	}
	if r.Match != nil {
		t.Fatalf("a weak match must not be taken: %+v", r.Match)
	}
	if len(r.Candidates) == 0 || r.Candidates[0].Slug != "pomidor" || r.Candidates[0].Method != "trigram" {
		t.Errorf("candidates %+v", r.Candidates)
	}
	if r, _ := m.Match(context.Background(), "velosiped"); r.Match != nil || len(r.Candidates) != 0 {
		t.Errorf("unrelated word: %+v", r)
	}
}

func TestSimilarityMatchesPgTrgm(t *testing.T) {
	// Values from PostgreSQL: SELECT similarity('pamidr', 'pamidor') etc.
	for _, c := range []struct {
		a, b string
		want float64
	}{
		{"pamidr", "pamidor", 0.5},
		{"kartoshki", "kartoshka", 0.6666667},
		{"word", "word", 1},
		{"abc", "xyz", 0},
	} {
		if got := Similarity(c.a, c.b); math.Abs(got-c.want) > 1e-6 {
			t.Errorf("similarity(%q, %q) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}

func TestMatchWithPgTrgmAndEmbeddings(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	pool, err := store.Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err := store.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	q := db.New(pool)
	if _, err := pool.Exec(ctx, "DELETE FROM product_embeddings"); err != nil {
		t.Fatal(err)
	}
	m, err := Load(ctx, q, DefaultThresholds)
	if err != nil {
		t.Fatal(err)
	}
	for word, want := range map[string]string{"pamidor": "pomidor", "помидор": "pomidor", "tomat": "pomidor", "картошки": "kartoshka"} {
		r, err := m.Match(ctx, word)
		if err != nil {
			t.Fatal(err)
		}
		if r.Match == nil || r.Match.Slug != want {
			t.Errorf("pg_trgm %q: %+v %+v", word, r.Match, r.Candidates)
		}
	}
	r, err := m.Match(ctx, "pamidr")
	if err != nil || r.Match != nil || len(r.Candidates) == 0 || r.Candidates[0].Slug != "pomidor" {
		t.Errorf("pg_trgm weak word: %+v %v", r, err)
	}

	// A fake embeddings model: "salad vegetable" points the same way as pomidor.
	embedCalls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Input []string `json:"input"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		embedCalls++
		type item struct {
			Index     int       `json:"index"`
			Embedding []float32 `json:"embedding"`
		}
		out := struct {
			Data []item `json:"data"`
		}{}
		for i, in := range req.Input {
			v := []float32{0, 0, 1}
			if in == "salad vegetable" || (len(in) >= 7 && in[:7] == "Pomidor") {
				v = []float32{1, 0.1, 0}
			}
			out.Data = append(out.Data, item{Index: i, Embedding: v})
		}
		_ = json.NewEncoder(w).Encode(out)
	}))
	defer srv.Close()
	m.SetEmbeddings(llm.New(srv.URL, "emb-test", "key", 100))
	if err := m.SyncEmbeddings(ctx); err != nil {
		t.Fatal(err)
	}
	// A second sync finds every vector cached.
	calls := embedCalls
	if err := m.SyncEmbeddings(ctx); err != nil || embedCalls != calls {
		t.Fatalf("second sync called the model: %v %d→%d", err, calls, embedCalls)
	}
	r, err = m.Match(ctx, "salad vegetable")
	if err != nil || r.Match == nil || r.Match.Slug != "pomidor" || r.Match.Method != "embedding" {
		t.Errorf("embedding match: %+v %+v %v", r.Match, r.Candidates, err)
	}
}
