package httpapi

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"image"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"bugun/internal/advice"
	"bugun/internal/ailog"
	"bugun/internal/aitest"
	"bugun/internal/analyst"
	"bugun/internal/ask"
	"bugun/internal/catalogmatch"
	"bugun/internal/config"
	"bugun/internal/deals"
	"bugun/internal/insight"
	"bugun/internal/llm"
	"bugun/internal/market"
	"bugun/internal/media"
	"bugun/internal/photoprice"
	"bugun/internal/pricing"
	"bugun/internal/secure"
	"bugun/internal/store/db"
	"bugun/internal/watch"
)

// newAITestAPI is the API with the model features built from fake servers;
// build fills AIDeps (nil: every feature off).
func newAITestAPI(t *testing.T, build func(*AIDeps, *pricing.Service, *ask.Service, *insight.Service, *db.Queries)) *testAPI {
	t.Helper()
	pool := aitest.DB(t)
	ctx := context.Background()
	q := db.New(pool)
	sealer, err := secure.NewSealer(base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{7}, 32)))
	if err != nil {
		t.Fatal(err)
	}
	prices, err := pricing.NewService(ctx, pool)
	if err != nil {
		t.Fatal(err)
	}
	asker, err := ask.NewService(ctx, q, prices)
	if err != nil {
		t.Fatal(err)
	}
	insights := insight.NewService(pool)
	var ai *AIDeps
	if build != nil {
		ai = &AIDeps{}
		build(ai, prices, asker, insights, q)
	}
	listings := market.NewService(pool)
	srv := httptest.NewServer(New(config.Config{WebURL: "http://localhost:3000"}, pool, Deps{Sealer: sealer, Prices: prices, Listings: listings,
		Deals: deals.NewService(pool), Watches: watch.NewService(pool), Insight: insights, AI: ai}))
	t.Cleanup(srv.Close)
	return &testAPI{t: t, srv: srv, pool: pool, q: q, listings: listings}
}

// upload posts one multipart file and decodes a successful answer.
func (a *testAPI) upload(token, path, name string, data []byte, fields map[string]string, out any) int {
	a.t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	h := make(map[string][]string)
	h["Content-Disposition"] = []string{`form-data; name="file"; filename="` + name + `"`}
	ct := "application/octet-stream"
	if strings.HasSuffix(name, ".png") {
		ct = "image/png"
	}
	h["Content-Type"] = []string{ct}
	fw, _ := w.CreatePart(h)
	_, _ = fw.Write(data)
	for k, v := range fields {
		_ = w.WriteField(k, v)
	}
	_ = w.Close()
	req, _ := http.NewRequest(http.MethodPost, a.srv.URL+path, &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	if token != "" {
		req.AddCookie(&http.Cookie{Name: sessionCookie, Value: token})
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		a.t.Fatal(err)
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	if out != nil && res.StatusCode < 300 {
		if err := json.Unmarshal(body, out); err != nil {
			a.t.Fatalf("decode %s: %v", body, err)
		}
	}
	return res.StatusCode
}

// oggVoice is an Ogg Opus stream header and a last page at seconds.
func oggVoice(seconds float64) []byte {
	page := func(granule uint64, payload string) []byte {
		b := make([]byte, 27)
		copy(b, "OggS")
		binary.LittleEndian.PutUint64(b[6:14], granule)
		return append(b, payload...)
	}
	return append(page(0, "OpusHead"), page(uint64(seconds*48000), "audio")...)
}

func TestAIFeaturesOff(t *testing.T) {
	a := newAITestAPI(t, nil)
	_, token := a.signIn(900900001, "entrepreneur", 2)
	var f AIFeatures
	if code := a.call("", "GET", "/v1/ai/features", nil, &f); code != 200 || f.Voice || f.Photo || f.AnalystModel {
		t.Errorf("features: %d %+v", code, f)
	}
	for _, c := range []struct {
		code int
		do   func() int
	}{
		{503, func() int { return a.call("", "POST", "/v1/ask", map[string]string{"question": "piyoz qancha?"}, nil) }},
		{503, func() int { return a.call("", "POST", "/v1/chat", map[string]string{"message": "piyoz qancha?"}, nil) }},
		{503, func() int { return a.upload(token, "/v1/prices/transcribe", "v.ogg", oggVoice(3), nil, nil) }},
		{401, func() int { return a.upload("", "/v1/prices/transcribe", "v.ogg", oggVoice(3), nil, nil) }},
		{503, func() int { return a.call("", "GET", "/v1/catalog/match?q=pamidor", nil, nil) }},
	} {
		if got := c.do(); got != c.code {
			t.Errorf("got %d, want %d", got, c.code)
		}
	}
	// The advice text works without a model: the template.
	var adv advice.Text
	if code := a.call("", "GET", "/v1/products/piyoz/advice-text?lang=ru", nil, &adv); code != 200 || adv.Mode != "template" || adv.Lang != "ru" || adv.Text == "" {
		t.Errorf("advice text: %d %+v", code, adv)
	}
}

func TestAIEndpoints(t *testing.T) {
	stt := aitest.NewFake(t)
	stt.Transcribe = func([]byte, map[string]string) string { return "kartoshka 4500 optom" }
	vision := aitest.NewFake(t)
	vision.Chat = func(llm.ChatRequest) llm.Message {
		return llm.Message{Content: `{"text":"Sabzi 6 000","items":[{"product":"sabzi","price":6000,"unit":"kg"}]}`}
	}
	var logger *ailog.Logger
	a := newAITestAPI(t, func(d *AIDeps, prices *pricing.Service, asker *ask.Service, ins *insight.Service, q *db.Queries) {
		logger = ailog.New(q, secure.HashToken, nil)
		m, err := catalogmatch.Load(context.Background(), q, catalogmatch.DefaultThresholds)
		if err != nil {
			t.Fatal(err)
		}
		prices.SetMatcher(m)
		d.Matcher = m
		d.Analyst = analyst.New(q, asker, prices, ins, analyst.WithMatcher(m))
		d.STT, d.STTLanguage, d.STTMaxSeconds = stt.Client("stt").WithRecorder(logger), "uz", 60
		d.Photos = photoprice.New(vision.Client("vision").WithRecorder(logger), prices, media.DefaultLimits)
		d.Log = logger
		d.Features = AIFeatures{Voice: true, VoiceMaxSecond: 60, Photo: true}
	})
	if _, err := a.pool.Exec(context.Background(), `DELETE FROM ai_log`); err != nil {
		t.Fatal(err)
	}
	_, seller := a.signIn(900900011, "entrepreneur", 2)
	_, staff := a.signIn(900900012, "market_admin", 2)
	_, admin := a.signIn(900900013, "admin", 2)

	var ans analyst.Answer
	if code := a.call("", "POST", "/v1/ask", map[string]string{"question": "Namanganda kartoshka qancha?", "lang": "en"}, &ans); code != 200 ||
		ans.Mode != "rules" || ans.Lang != "en" || len(ans.Sources) == 0 || ans.Sources[0].Tool != analyst.ToolPriceStats {
		t.Errorf("ask: %d %+v", code, ans)
	}
	if code := a.call("", "POST", "/v1/ask", map[string]string{"question": ""}, nil); code != 422 {
		t.Errorf("empty question: %d", code)
	}

	// The assistant keeps the conversation; the rules answer without a model.
	var chat analyst.ChatAnswer
	if code := a.call("", "POST", "/v1/chat", map[string]string{"message": "Namanganda kartoshka qancha?"}, &chat); code != 200 ||
		chat.SessionID == "" || chat.Kind != analyst.KindAnswer || len(chat.Sources) == 0 {
		t.Errorf("chat: %d %+v", code, chat)
	}
	var next analyst.ChatAnswer
	if code := a.call("", "POST", "/v1/chat", map[string]string{"message": "velosiped qancha?", "session_id": chat.SessionID}, &next); code != 200 ||
		next.SessionID != chat.SessionID || next.Status != analyst.StatusNoProduct {
		t.Errorf("chat follow-up: %d %+v", code, next)
	}
	var unanswered ChatUnansweredOutput
	if code := a.call(admin, "GET", "/v1/admin/chat/unanswered?days=1", nil, &unanswered.Body); code != 200 || len(unanswered.Body.Questions) == 0 {
		t.Errorf("admin unanswered: %d %+v", code, unanswered.Body)
	}
	var cs analyst.ChatStats
	if code := a.call(admin, "GET", "/v1/admin/chat/stats?days=1", nil, &cs); code != 200 || cs.Answers < 2 {
		t.Errorf("admin chat stats: %d %+v", code, cs)
	}
	if code := a.call(staff, "GET", "/v1/admin/chat/stats", nil, nil); code != 403 {
		t.Errorf("chat stats for market staff: %d", code)
	}

	var tr TranscribeOutput
	if code := a.upload(seller, "/v1/prices/transcribe", "voice.ogg", oggVoice(3), nil, &tr.Body); code != 200 || tr.Body.Text != "kartoshka 4500 optom" || tr.Body.Seconds != 3 {
		t.Errorf("transcribe: %d %+v", code, tr.Body)
	}
	if code := a.upload(seller, "/v1/prices/transcribe", "voice.ogg", oggVoice(75), nil, nil); code != 413 {
		t.Errorf("75 s voice: %d", code)
	}
	if code := a.upload(seller, "/v1/prices/transcribe", "voice.webm", []byte{0x1A, 0x45, 0xDF, 0xA3, 0, 0}, map[string]string{"duration": "90"}, nil); code != 413 {
		t.Errorf("webm said to be 90 s: %d", code)
	}
	if code := a.upload(seller, "/v1/prices/transcribe", "notes.txt", []byte("hello there"), nil, nil); code != 415 {
		t.Errorf("not audio: %d", code)
	}

	img := image.NewRGBA(image.Rect(0, 0, 20, 20))
	var pngBuf bytes.Buffer
	_ = png.Encode(&pngBuf, img)
	var ph PhotoPriceOutput
	if code := a.upload(seller, "/v1/prices/from-photo", "list.png", pngBuf.Bytes(), nil, &ph.Body); code != 200 ||
		ph.Body.Draft.ID == 0 || len(ph.Body.Draft.Items) != 1 || ph.Body.Draft.Items[0].PriceSom != 6000 || ph.Body.Transcript != "Sabzi 6 000" {
		t.Errorf("from photo: %d %+v", code, ph.Body)
	}
	if code := a.upload(staff, "/v1/prices/from-photo", "list.png", pngBuf.Bytes(), nil, nil); code != 403 {
		t.Errorf("market staff photo: %d", code)
	}

	var m catalogmatch.Result
	if code := a.call("", "GET", "/v1/catalog/match?q=%D0%BF%D0%BE%D0%BC%D0%B8%D0%B4%D0%BE%D1%80", nil, &m); code != 200 || m.Match == nil || m.Match.Slug != "pomidor" {
		t.Errorf("match помидор: %d %+v", code, m)
	}
	// A typo in a price message is read by the rules' near spelling; words
	// the rules cannot read at all go to the matcher (pricing tests).
	var d pricing.Draft
	if code := a.call(seller, "POST", "/v1/prices/drafts", map[string]string{"text": "pamidr 8000"}, &d); code != 200 || len(d.Items) != 1 ||
		d.Items[0].ProductName != "Pomidor" || len(d.Suggestions) != 0 {
		t.Errorf("typo draft: %d %+v", code, d)
	}

	if code := a.call(staff, "GET", "/v1/admin/ai-usage", nil, nil); code != 403 {
		t.Errorf("usage for market staff: %d", code)
	}
	var us ailog.Usage
	if code := a.call(admin, "GET", "/v1/admin/ai-usage?days=7", nil, &us); code != 200 || us.Total.Calls != 2 || len(us.ByFeature) != 2 {
		t.Errorf("usage: %d %+v", code, us)
	}
}
