// Package photoprice reads prices from a photo of a price list (spec AI-3)
// with a vision-capable chat model. The model transcribes the text on the
// photo and suggests items; the rule-based parser reads the transcription
// first, and every item the model suggests is checked like a channel post
// (pricing.Verify): the product must be in the catalog, the unit known and
// the number written in the transcription. The result is a draft: nothing
// becomes a price until the seller confirms it, and the usual anomaly check
// runs then.
package photoprice

import (
	"context"
	"encoding/base64"
	"errors"
	"io"
	"strings"

	"bugun/internal/llm"
	"bugun/internal/media"
	"bugun/internal/pricing"
	"bugun/internal/store/db"
)

// ErrOff: no vision model is configured.
var ErrOff = errors.New("reading prices from photos is off")

// ErrUnreadable: the model's answer could not be used.
var ErrUnreadable = errors.New("the photo could not be read")

type Reader struct {
	vision *llm.Client // nil: off
	prices *pricing.Service
	limits media.Limits
}

// New returns a reader; vision may be nil (the feature is off).
func New(vision *llm.Client, prices *pricing.Service, limits media.Limits) *Reader {
	return &Reader{vision: vision, prices: prices, limits: limits}
}

// Enabled reports whether a vision model is configured.
func (r *Reader) Enabled() bool { return r != nil && r.vision != nil }

const prompt = `Rasmda Oʻzbekiston bozoridagi narxlar roʻyxati (yozuv, doska, chek yoki qogʻoz) bor.
1) "text" ga rasmdagi barcha yozuvni qanday boʻlsa shunday, qator-qator koʻchir. Raqamlarni oʻzgartirma, oʻzingdan hech narsa qoʻshma.
2) "items" ga har bir narxni yoz: product — katalogdagi slug; variety — navi (boʻlsa); unit — narx qaysi birlik uchun yozilgan boʻlsa, rasmdagi soʻz (kg, qop, yashik, dona…), boʻlmasa boʻsh; price — narx raqami rasmdagidek; qty — hajm raqami rasmdagidek, boʻlmasa 0; trade — "ulgurji" (optom) yoki "chakana".
Katalogda yoʻq mahsulotni va oʻqib boʻlmaydigan narxni tashlab ket.
Faqat JSON qaytar: {"text":"...","items":[{"product":"slug","variety":"","unit":"kg","price":0,"qty":0,"trade":"chakana"}]}

Katalog (slug: nomi (boshqa nomlari); birlik):
`

type answer struct {
	Text  string `json:"text"`
	Items []struct {
		pricing.Candidate
		Variety string `json:"variety"`
	} `json:"items"`
}

// Result is what a photo gave: a draft to confirm and the transcription it
// was read from.
type Result struct {
	Draft      pricing.Draft
	Transcript string
}

// Read processes the image (size and pixel limits, re-encoded as JPEG, which
// also drops EXIF and GPS), asks the model and builds a draft. Only the
// processed image leaves the server.
func (r *Reader) Read(ctx context.Context, u db.User, image io.Reader) (Result, error) {
	if !r.Enabled() {
		return Result{}, ErrOff
	}
	if !pricing.CanSubmit(u) {
		return Result{}, pricing.ErrNotSeller
	}
	photo, err := r.limits.Process(image)
	if err != nil {
		return Result{}, err
	}
	ctx = llm.WithUser(llm.WithFeature(ctx, "vision"), u.ID)
	var a answer
	err = r.vision.ChatJSON(ctx, llm.ChatRequest{
		Temperature: 0, MaxTokens: 1500,
		Messages: []llm.Message{
			{Role: "system", Content: prompt + r.prices.CatalogText()},
			{Role: "user", Content: []llm.Part{
				{Type: "text", Text: "Narxlarni oʻqi."},
				{Type: "image_url", ImageURL: &llm.ImageURL{URL: "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(photo.Data)}},
			}},
		},
	}, &a)
	if err != nil {
		if errors.Is(err, llm.ErrLimit) {
			return Result{}, err
		}
		return Result{}, errors.Join(ErrUnreadable, err)
	}
	transcript := strings.TrimSpace(a.Text)
	if transcript == "" {
		return Result{Draft: pricing.Draft{Items: []pricing.Item{}, Problems: []pricing.Problem{}}}, nil
	}
	cands := make([]pricing.Candidate, len(a.Items))
	for i, it := range a.Items {
		cands[i] = it.Candidate
	}
	rules := r.prices.Parse(transcript)
	items := pricing.Merge(rules.Items, r.prices.Verify(transcript, cands))
	// A line the rules could not read but the model could is no problem.
	read := map[string]bool{}
	for _, it := range items {
		read[it.ProductName] = true
	}
	problems := []pricing.Problem{}
	for _, p := range rules.Problems {
		if p.Product == "" || !read[p.Product] {
			problems = append(problems, p)
		}
	}
	d, err := r.prices.CreateDraftItems(ctx, u, transcript, items, problems)
	return Result{Draft: d, Transcript: transcript}, err
}
