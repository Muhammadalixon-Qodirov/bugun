package httpapi

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5"

	"bugun/internal/advice"
	"bugun/internal/ailog"
	"bugun/internal/analyst"
	"bugun/internal/catalogmatch"
	"bugun/internal/insight"
	"bugun/internal/llm"
	"bugun/internal/market"
	"bugun/internal/media"
	"bugun/internal/photoprice"
	"bugun/internal/pricing"
	"bugun/internal/speech"
)

// AIDeps are the model features (spec §9). Every field is optional: a nil
// service answers 503 feature_off, and the rest of the API works without it.
type AIDeps struct {
	Analyst       *analyst.Service
	STT           *llm.Client // speech to text; nil = off
	STTLanguage   string
	STTMaxSeconds int
	Photos        *photoprice.Reader
	Matcher       *catalogmatch.Matcher
	Advice        *advice.Phraser
	Log           *ailog.Logger
	// What is on, for the site to show or hide the voice and photo buttons.
	Features AIFeatures
}

// AIFeatures tells clients which model features are on.
type AIFeatures struct {
	Voice          bool `json:"voice" doc:"Speech to text for price messages (POST /v1/prices/transcribe, bot voice messages)"`
	VoiceMaxSecond int  `json:"voice_max_seconds" doc:"Longest recording accepted"`
	Photo          bool `json:"photo" doc:"Prices from a photo of a price list (POST /v1/prices/from-photo)"`
	AnalystModel   bool `json:"analyst_model" doc:"A language model routes and words /v1/ask answers; false: rule-based answers"`
	Embeddings     bool `json:"embeddings" doc:"Catalog matching also uses embeddings"`
	Moderation     bool `json:"moderation_model" doc:"A language model screens listing texts next to the keyword rules"`
	AdviceModel    bool `json:"advice_model" doc:"A language model rewords the buy/sell advice"`
}

func featureOff() error {
	return apiError(http.StatusServiceUnavailable, "feature_off", "Bu imkoniyat hozircha yoqilmagan.")
}

func (s *Server) registerAI(api huma.API) {
	tags := []string{"ai"}
	huma.Register(api, huma.Operation{
		OperationID: "aiFeatures", Method: http.MethodGet, Path: "/v1/ai/features",
		Summary: "Which model features are on", Tags: tags,
	}, s.aiFeatures)
	huma.Register(api, huma.Operation{
		OperationID: "askAnalyst", Method: http.MethodPost, Path: "/v1/ask",
		Summary:     "Answer a price question (AI-5)",
		Description: "Text, the table behind it and the sources (function, parameters, dates, demo share). Every number comes from deterministic functions; with fewer than 3 prices the answer says there is not enough data. Signing in is optional.",
		Tags:        tags,
	}, s.askAnalyst)
	huma.Register(api, huma.Operation{
		OperationID: "transcribePrice", Method: http.MethodPost, Path: "/v1/prices/transcribe",
		Summary:     "Turn a voice recording into text (AI-1)",
		Description: "Multipart field file: OGG, WebM, MP4/M4A, WAV or MP3, at most voice_max_seconds long. Send the text to /v1/prices/drafts for the confirmation card.",
		Tags:        []string{"prices"}, Security: authRequired,
	}, s.transcribePrice)
	huma.Register(api, huma.Operation{
		OperationID: "priceDraftFromPhoto", Method: http.MethodPost, Path: "/v1/prices/from-photo",
		Summary:     "Read prices from a photo of a price list into a draft (AI-3)",
		Description: "Multipart field file: JPEG, PNG or WebP. The draft is confirmed like a text draft; nothing is saved before that.",
		Tags:        []string{"prices"}, Security: authRequired,
	}, s.priceDraftFromPhoto)
	huma.Register(api, huma.Operation{
		OperationID: "matchProduct", Method: http.MethodGet, Path: "/v1/catalog/match",
		Summary: "Match a product word to the catalog (AI-4)", Tags: []string{"catalog"},
	}, s.matchProduct)
	huma.Register(api, huma.Operation{
		OperationID: "productAdviceText", Method: http.MethodGet, Path: "/v1/products/{slug}/advice-text",
		Summary: "The buy/sell advice in plain words (AI-6)", Tags: []string{"analytics"},
	}, s.productAdviceText)
	huma.Register(api, huma.Operation{
		OperationID: "adminAIUsage", Method: http.MethodGet, Path: "/v1/admin/ai-usage",
		Summary: "Model calls by day, feature and model: tokens, audio, latency, cost, errors", Tags: []string{"admin"}, Security: authRequired,
	}, s.adminAIUsage)
}

func (s *Server) aiDeps() AIDeps {
	if s.ai == nil {
		return AIDeps{}
	}
	return *s.ai
}

type AIFeaturesOutput struct {
	Body AIFeatures
}

func (s *Server) aiFeatures(_ context.Context, _ *struct{}) (*AIFeaturesOutput, error) {
	return &AIFeaturesOutput{Body: s.aiDeps().Features}, nil
}

type AskInput struct {
	SessionInput
	Body struct {
		Question string `json:"question" minLength:"1" maxLength:"500" doc:"e.g. \"Samarqandda piyoz qancha?\""`
		Lang     string `json:"lang,omitempty" enum:"uz,ru,en" default:"uz" doc:"Answer language"`
	}
}

type AskOutput struct {
	Body analyst.Answer
}

func (s *Server) askAnalyst(ctx context.Context, in *AskInput) (*AskOutput, error) {
	an := s.aiDeps().Analyst
	if an == nil {
		return nil, featureOff()
	}
	key, u := s.visitor(ctx, in.Session)
	a, err := an.Ask(ctx, analyst.Request{Question: in.Body.Question, Lang: in.Body.Lang, User: u, Key: key})
	if errors.Is(err, analyst.ErrEmpty) {
		return nil, apiError(http.StatusUnprocessableEntity, "question_invalid", "Savol boʻsh yoki juda uzun.")
	}
	if err != nil {
		return nil, internalError(ctx, "ask analyst", err)
	}
	return &AskOutput{Body: a}, nil
}

type TranscribeInput struct {
	SessionInput
	RawBody huma.MultipartFormFiles[struct {
		File huma.FormFile `form:"file" required:"true" doc:"The recording"`
	}]
}

type TranscribeOutput struct {
	Body struct {
		Text    string  `json:"text" doc:"What was said; empty when nothing was recognised"`
		Seconds float64 `json:"seconds,omitempty" doc:"Recording length when it could be read from the file"`
	}
}

func (s *Server) transcribePrice(ctx context.Context, in *TranscribeInput) (*TranscribeOutput, error) {
	u, err := s.currentUser(ctx, in.Session)
	if err != nil {
		return nil, err
	}
	d := s.aiDeps()
	if d.STT == nil {
		return nil, featureOff()
	}
	f := in.RawBody.Data().File
	defer f.Close()
	maxSeconds := max(d.STTMaxSeconds, 5)
	audio, err := io.ReadAll(io.LimitReader(f, speech.MaxBytes(maxSeconds)+1))
	if err != nil {
		return nil, internalError(ctx, "read audio", err)
	}
	tooLong := apiError(http.StatusRequestEntityTooLarge, "audio_too_long", fmt.Sprintf("Yozuv juda uzun. %d soniyagacha gapiring.", maxSeconds))
	if int64(len(audio)) > speech.MaxBytes(maxSeconds) {
		return nil, tooLong
	}
	format, ok := speech.Detect(audio)
	if !ok {
		return nil, apiError(http.StatusUnsupportedMediaType, "audio_unsupported", "Bu ovoz formatini oʻqib boʻlmadi (OGG, WebM, MP4, WAV yoki MP3 kerak).")
	}
	seconds, known := speech.OggSeconds(audio)
	if !known {
		if v, err := strconv.ParseFloat(strings.TrimSpace(firstValue(in.RawBody.Form.Value["duration"])), 64); err == nil && v > 0 {
			seconds, known = v, true
		}
	}
	if known && seconds > float64(maxSeconds)+0.5 {
		return nil, tooLong
	}
	text, err := d.STT.Transcribe(llm.WithUser(llm.WithFeature(ctx, "stt"), u.ID), audio, "voice."+format.Ext, d.STTLanguage, seconds)
	switch {
	case errors.Is(err, llm.ErrLimit):
		return nil, apiError(http.StatusTooManyRequests, "ai_limit", "Ovozni matnga oʻgirish soatlik chegarasiga yetildi. Matn bilan yozing.")
	case err != nil:
		return nil, apiError(http.StatusBadGateway, "stt_failed", "Ovozni matnga oʻgirib boʻlmadi. Matn bilan yozing.")
	}
	out := &TranscribeOutput{}
	out.Body.Text = text
	if known {
		out.Body.Seconds = seconds
	}
	return out, nil
}

func firstValue(v []string) string {
	if len(v) == 0 {
		return ""
	}
	return v[0]
}

type PhotoPriceInput struct {
	SessionInput
	RawBody huma.MultipartFormFiles[struct {
		File huma.FormFile `form:"file" contentType:"image/jpeg,image/png,image/webp" required:"true"`
	}]
}

type PhotoPriceOutput struct {
	Body struct {
		Draft      pricing.Draft `json:"draft" doc:"Confirm with /v1/prices/drafts/{id}/confirm; id 0 = nothing was read"`
		Transcript string        `json:"transcript" doc:"The text read from the photo; every price in the draft is written in it"`
	}
}

func (s *Server) priceDraftFromPhoto(ctx context.Context, in *PhotoPriceInput) (*PhotoPriceOutput, error) {
	u, err := s.currentUser(ctx, in.Session)
	if err != nil {
		return nil, err
	}
	r := s.aiDeps().Photos
	if !r.Enabled() {
		return nil, featureOff()
	}
	f := in.RawBody.Data().File
	defer f.Close()
	res, err := r.Read(ctx, u, f)
	lim := s.mediaLimits()
	switch {
	case errors.Is(err, pricing.ErrNotSeller):
		return nil, apiError(http.StatusForbidden, "sellers_only", "Narx yuborish uchun profilda «Tadbirkor» rolini tanlang va hududingizni kiriting.")
	case errors.Is(err, media.ErrTooLarge):
		return nil, apiError(http.StatusRequestEntityTooLarge, "photo_too_large", fmt.Sprintf("Rasm juda katta. %d MB gacha va %d megapikselgacha rasm yuklang.", lim.MaxBytes>>20, lim.MaxPixels/1_000_000))
	case errors.Is(err, media.ErrUnsupported):
		return nil, apiError(http.StatusUnsupportedMediaType, "photo_unsupported", "Faqat JPEG, PNG yoki WebP rasm yuklash mumkin.")
	case errors.Is(err, llm.ErrLimit):
		return nil, apiError(http.StatusTooManyRequests, "ai_limit", "Rasmdan oʻqish soatlik chegarasiga yetildi. Narxni matn bilan yozing.")
	case errors.Is(err, photoprice.ErrUnreadable):
		return nil, apiError(http.StatusUnprocessableEntity, "photo_unreadable", "Rasmdan narxlarni oʻqib boʻlmadi. Narxni matn bilan yozing.")
	case err != nil:
		return nil, internalError(ctx, "photo prices", err)
	}
	out := &PhotoPriceOutput{}
	out.Body.Draft, out.Body.Transcript = res.Draft, res.Transcript
	return out, nil
}

type MatchInput struct {
	Q string `query:"q" minLength:"1" maxLength:"60" doc:"A product word as written, any script: pamidor, помидор, tomat"`
}

type MatchOutput struct {
	Body catalogmatch.Result
}

func (s *Server) matchProduct(ctx context.Context, in *MatchInput) (*MatchOutput, error) {
	m := s.aiDeps().Matcher
	if m == nil {
		return nil, featureOff()
	}
	r, err := m.Match(ctx, in.Q)
	if err != nil {
		return nil, internalError(ctx, "match product", err)
	}
	return &MatchOutput{Body: r}, nil
}

type AdviceTextInput struct {
	Slug      string `path:"slug" maxLength:"60"`
	Region    int16  `query:"region" minimum:"0" doc:"Viloyat or tuman (its viloyat is used); 0 for the whole country"`
	TradeType string `query:"trade" enum:"ulgurji,chakana" default:"ulgurji"`
	Lang      string `query:"lang" enum:"uz,ru,en" default:"uz"`
}

type AdviceTextOutput struct {
	Body advice.Text
}

func (s *Server) productAdviceText(ctx context.Context, in *AdviceTextInput) (*AdviceTextOutput, error) {
	ph := s.aiDeps().Advice
	if ph == nil {
		ph = advice.New(nil)
	}
	p, err := s.q.GetProductBySlug(ctx, in.Slug)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apiError(http.StatusNotFound, "product_not_found", "Mahsulot topilmadi.")
	}
	if err != nil {
		return nil, internalError(ctx, "get product", err)
	}
	var province int16
	if in.Region > 0 {
		if province, err = market.ProvinceOf(ctx, s.q, in.Region); errors.Is(err, pgx.ErrNoRows) {
			return nil, apiError(http.StatusUnprocessableEntity, "unknown_region", "Bunday hudud topilmadi.")
		} else if err != nil {
			return nil, internalError(ctx, "province", err)
		}
	}
	// The advice does not depend on the period; 90d is the cheapest page.
	a, err := s.insight.Analytics(ctx, insight.Params{ProductID: p.ID, Province: province, Trade: in.TradeType, Period: "90d", Transport: insight.DefaultTransport})
	if err != nil {
		return nil, internalError(ctx, "analytics", err)
	}
	key := fmt.Sprintf("%d|%d|%s", p.ID, province, in.TradeType)
	return &AdviceTextOutput{Body: ph.Phrase(ctx, key, p.Name, a.Advice, in.Lang)}, nil
}

type AIUsageInput struct {
	SessionInput
	Days int `query:"days" minimum:"1" maximum:"365" default:"30" doc:"Tashkent days back, today included"`
}

type AIUsageOutput struct {
	Body ailog.Usage
}

func (s *Server) adminAIUsage(ctx context.Context, in *AIUsageInput) (*AIUsageOutput, error) {
	if _, err := s.currentAdmin(ctx, in.Session); err != nil {
		return nil, err
	}
	l := s.aiDeps().Log
	if l == nil {
		return nil, featureOff()
	}
	u, err := l.Usage(ctx, in.Days)
	if err != nil {
		return nil, internalError(ctx, "ai usage", err)
	}
	return &AIUsageOutput{Body: u}, nil
}
