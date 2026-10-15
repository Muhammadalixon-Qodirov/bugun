// Package catalogmatch matches a free-text product word to the catalog
// (spec AI-4): "pamidor", "помидор", "tomat" → pomidor. It layers the
// dictionary of names and other names, trigram similarity (pg_trgm, or the
// same measure in Go without a database) and, when configured, embedding
// similarity. A match below the confidence threshold is not taken: the seller
// picks from the candidates.
package catalogmatch

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
	"math"
	"sort"
	"strings"
	"sync"

	"bugun/internal/llm"
	"bugun/internal/pricing"
	"bugun/internal/store/db"
)

// Product is the part of the catalog matching needs.
type Product struct {
	ID      int16
	Slug    string
	Name    string
	Aliases []string
}

// Thresholds: a best score at or above MinConfidence, and clear of the
// runner-up by Margin, is a match; candidates score at least MinCandidate.
type Thresholds struct {
	MinConfidence float64
	MinCandidate  float64
	Margin        float64
}

// DefaultThresholds match config's defaults.
var DefaultThresholds = Thresholds{MinConfidence: 0.6, MinCandidate: 0.3, Margin: 0.05}

const maxCandidates = 5

// Matcher implements pricing.ProductMatcher.
type Matcher struct {
	products []Product
	byID     map[int16]int
	aliases  map[string]int // normalized name or other name → product index
	q        *db.Queries    // pg_trgm; nil = trigrams in Go
	th       Thresholds

	emb     *llm.Client // optional embeddings model
	mu      sync.RWMutex
	vectors map[int16][]float32
}

// New builds a matcher over products. q may be nil (no database: the
// trigram measure runs in Go).
func New(products []Product, q *db.Queries, th Thresholds) *Matcher {
	m := &Matcher{products: products, byID: map[int16]int{}, aliases: map[string]int{}, q: q, th: th}
	for i, p := range products {
		m.byID[p.ID] = i
		for _, a := range append([]string{p.Name, p.Slug}, p.Aliases...) {
			n := norm(a)
			if _, taken := m.aliases[n]; n != "" && !taken {
				m.aliases[n] = i
			}
		}
	}
	return m
}

// Load reads the catalog from the database.
func Load(ctx context.Context, q *db.Queries, th Thresholds) (*Matcher, error) {
	rows, err := q.ListParserProducts(ctx)
	if err != nil {
		return nil, err
	}
	ps := make([]Product, len(rows))
	for i, r := range rows {
		ps[i] = Product{ID: r.ID, Slug: r.Slug, Name: r.Name, Aliases: r.Synonyms}
	}
	return New(ps, q, th), nil
}

// norm is a word in the parser's form: Latin script, lowercase, no
// apostrophes, single spaces; "-" joins as a space ("mol-goshti").
func norm(s string) string {
	return strings.Join(strings.Fields(strings.ReplaceAll(pricing.Normalize(s), "-", " ")), " ")
}

// Result is what a word matched.
type Result struct {
	Query      string                  `json:"query"`
	Match      *pricing.ProductChoice  `json:"match,omitempty" doc:"Omitted when no product is confident enough; pick from candidates"`
	Candidates []pricing.ProductChoice `json:"candidates" doc:"Best first"`
}

// MatchProduct implements pricing.ProductMatcher.
func (m *Matcher) MatchProduct(ctx context.Context, word string) (*pricing.ProductChoice, []pricing.ProductChoice, error) {
	r, err := m.Match(ctx, word)
	return r.Match, r.Candidates, err
}

// Match scores every product for word and decides.
func (m *Matcher) Match(ctx context.Context, word string) (Result, error) {
	q := norm(word)
	res := Result{Query: q, Candidates: []pricing.ProductChoice{}}
	if len([]rune(q)) < 2 {
		return res, nil
	}
	scores := map[int]pricing.ProductChoice{}
	put := func(i int, score float64, method string) {
		if cur, ok := scores[i]; ok && cur.Score >= score {
			return
		}
		p := m.products[i]
		scores[i] = pricing.ProductChoice{ProductID: p.ID, Slug: p.Slug, Name: p.Name, Score: math.Round(score*1000) / 1000, Method: method}
	}
	if i, ok := m.dictionary(q); ok {
		put(i, 1, "dictionary")
	}
	trgm, err := m.trigram(ctx, q, strings.ToLower(strings.TrimSpace(word)))
	if err != nil {
		return res, err
	}
	for i, s := range trgm {
		put(i, s, "trigram")
	}
	for i, s := range m.embedding(ctx, word) {
		put(i, s, "embedding")
	}

	for _, c := range scores {
		if c.Score >= m.th.MinCandidate {
			res.Candidates = append(res.Candidates, c)
		}
	}
	sort.Slice(res.Candidates, func(i, j int) bool {
		a, b := res.Candidates[i], res.Candidates[j]
		return a.Score > b.Score || (a.Score == b.Score && a.ProductID < b.ProductID)
	})
	if len(res.Candidates) > maxCandidates {
		res.Candidates = res.Candidates[:maxCandidates]
	}
	if len(res.Candidates) > 0 {
		top := res.Candidates[0]
		clear := len(res.Candidates) == 1 || top.Score-res.Candidates[1].Score >= m.th.Margin || top.Method == "dictionary"
		if top.Score >= m.th.MinConfidence && clear {
			res.Match = &top
		}
	}
	return res, nil
}

// dictionary finds the word among names and other names, allowing a case
// ending ("pomidorni", "kartoshkadan") as the parser does.
func (m *Matcher) dictionary(q string) (int, bool) {
	if i, ok := m.aliases[q]; ok {
		return i, true
	}
	best, bestLen := -1, 0
	for a, i := range m.aliases {
		if len(a) >= 4 && len(a) > bestLen && strings.HasPrefix(q, a) && endings[q[len(a):]] {
			best, bestLen = i, len(a)
		}
	}
	return best, best >= 0
}

// endings are the word endings a product name may carry in a message:
// Uzbek case and plural endings, Russian and English plurals. Any other
// tail makes another word ("mash" + "ina" is not mosh).
var endings = map[string]bool{
	"a": true, "i": true, "u": true, "e": true, "y": true, "s": true, "es": true, "ka": true, "ki": true,
	"ni": true, "ga": true, "da": true, "dan": true, "ning": true, "si": true, "lar": true, "lari": true, "larni": true,
	"larga": true, "lardan": true, "dagi": true, "gi": true, "ov": true, "ami": true, "om": true,
}

// trigram scores products by the best trigram similarity of any of their
// names, with pg_trgm when there is a database.
func (m *Matcher) trigram(ctx context.Context, q, raw string) (map[int]float64, error) {
	out := map[int]float64{}
	if m.q != nil {
		rows, err := m.q.MatchProductsTrgm(ctx, db.MatchProductsTrgmParams{Norm: q, Raw: raw, MinScore: m.th.MinCandidate, MaxRows: 10})
		if err != nil {
			return nil, err
		}
		for _, r := range rows {
			if i, ok := m.byID[r.ID]; ok {
				out[i] = r.Score
			}
		}
		return out, nil
	}
	for i, p := range m.products {
		for _, a := range append([]string{p.Name}, p.Aliases...) {
			s := max(Similarity(q, strings.ToLower(a)), Similarity(raw, strings.ToLower(a)))
			if s >= m.th.MinCandidate && s > out[i] {
				out[i] = s
			}
		}
	}
	return out, nil
}

// Similarity is pg_trgm's similarity(): shared trigrams over all trigrams of
// the two strings, each word padded with two spaces in front and one behind.
func Similarity(a, b string) float64 {
	ta, tb := trigrams(a), trigrams(b)
	if len(ta) == 0 || len(tb) == 0 {
		return 0
	}
	shared := 0
	for t := range ta {
		if tb[t] {
			shared++
		}
	}
	return float64(shared) / float64(len(ta)+len(tb)-shared)
}

func trigrams(s string) map[string]bool {
	out := map[string]bool{}
	words := strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return !(r >= '0' && r <= '9') && !isLetter(r)
	})
	for _, w := range words {
		rs := []rune("  " + w + " ")
		for i := 0; i+3 <= len(rs); i++ {
			out[string(rs[i:i+3])] = true
		}
	}
	return out
}

func isLetter(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || r > 127 && !strings.ContainsRune("«»‘’ʻʼ`´", r)
}

// SetEmbeddings turns on embedding similarity with an embeddings model.
// SyncEmbeddings must run before it has vectors to compare with.
func (m *Matcher) SetEmbeddings(c *llm.Client) { m.emb = c }

// productText is what is embedded for a product.
func productText(p Product) string {
	return p.Name + "; " + strings.Join(p.Aliases, ", ")
}

func sourceHash(model, text string) string {
	h := sha256.Sum256([]byte(model + "\x00" + text))
	return hex.EncodeToString(h[:8])
}

// SyncEmbeddings loads the cached product vectors and computes the missing
// or outdated ones (the catalog or the model changed), in one request.
func (m *Matcher) SyncEmbeddings(ctx context.Context) error {
	if m.emb == nil || m.q == nil {
		return nil
	}
	ctx = llm.WithFeature(ctx, "embeddings")
	rows, err := m.q.ListProductEmbeddings(ctx)
	if err != nil {
		return err
	}
	cached := map[int16]db.ListProductEmbeddingsRow{}
	for _, r := range rows {
		cached[r.ProductID] = r
	}
	vectors := map[int16][]float32{}
	var stale []Product
	for _, p := range m.products {
		c, ok := cached[p.ID]
		if ok && c.Model == m.emb.Model && c.SourceHash == sourceHash(m.emb.Model, productText(p)) {
			vectors[p.ID] = c.Embedding
			continue
		}
		stale = append(stale, p)
	}
	if len(stale) > 0 {
		texts := make([]string, len(stale))
		for i, p := range stale {
			texts[i] = productText(p)
		}
		vecs, err := m.emb.Embed(ctx, texts)
		if err != nil {
			return err
		}
		for i, p := range stale {
			if err := m.q.UpsertProductEmbedding(ctx, db.UpsertProductEmbeddingParams{
				ProductID: p.ID, Model: m.emb.Model, SourceHash: sourceHash(m.emb.Model, texts[i]), Embedding: vecs[i],
			}); err != nil {
				return err
			}
			vectors[p.ID] = vecs[i]
		}
		slog.InfoContext(ctx, "catalog embeddings updated", "products", len(stale))
	}
	m.mu.Lock()
	m.vectors = vectors
	m.mu.Unlock()
	return nil
}

// embedding scores products by cosine similarity to the word's vector.
// Failures leave the other layers to decide.
func (m *Matcher) embedding(ctx context.Context, word string) map[int]float64 {
	m.mu.RLock()
	vectors := m.vectors
	m.mu.RUnlock()
	if m.emb == nil || len(vectors) == 0 {
		return nil
	}
	vecs, err := m.emb.Embed(llm.WithFeature(ctx, "match"), []string{word})
	if err != nil {
		slog.WarnContext(ctx, "embed word", "err", err)
		return nil
	}
	out := map[int]float64{}
	for id, v := range vectors {
		if i, ok := m.byID[id]; ok {
			if s := Cosine(vecs[0], v); s >= m.th.MinCandidate {
				out[i] = s
			}
		}
	}
	return out
}

// Cosine is the cosine similarity of two vectors (0 when either is empty or
// they differ in length).
func Cosine(a, b []float32) float64 {
	if len(a) == 0 || len(a) != len(b) {
		return 0
	}
	var dot, na, nb float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		na += float64(a[i]) * float64(a[i])
		nb += float64(b[i]) * float64(b[i])
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return dot / math.Sqrt(na*nb)
}
