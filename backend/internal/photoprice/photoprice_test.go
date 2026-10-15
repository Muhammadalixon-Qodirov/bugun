package photoprice

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"

	"bugun/internal/aitest"
	"bugun/internal/llm"
	"bugun/internal/media"
	"bugun/internal/pricing"
	"bugun/internal/store/db"
)

func pngImage(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 40, 30))
	img.Set(3, 3, color.Black)
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestReadPhoto(t *testing.T) {
	pool := aitest.DB(t)
	ctx := context.Background()
	q := db.New(pool)
	prices, err := pricing.NewService(ctx, pool)
	if err != nil {
		t.Fatal(err)
	}
	seller := aitest.User(t, q, 900400001, "entrepreneur")

	fake := aitest.NewFake(t)
	fake.Chat = func(req llm.ChatRequest) llm.Message {
		return llm.Message{Content: "```json\n" + `{"text":"Pomidor 8 000 so'm\nKartoshka qopi 200 ming\nРепчатый опт 4500 x 300","items":[` +
			`{"product":"pomidor","price":8000,"unit":"kg","trade":"chakana"},` +
			`{"product":"piyoz","variety":"repchatiy","price":4500,"unit":"kg","qty":300,"trade":"ulgurji"},` +
			`{"product":"sabzi","price":9999,"unit":"kg"},` +
			`{"product":"velosiped","price":8000}]}` + "\n```"}
	}
	rec := &aitest.Recorder{}
	r := New(fake.Client("vision-test").WithRecorder(rec), prices, media.DefaultLimits)

	res, err := r.Read(ctx, seller, bytes.NewReader(pngImage(t)))
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]pricing.Item{}
	for _, it := range res.Draft.Items {
		got[it.ProductName] = it
	}
	if len(got) != 3 || res.Draft.ID == 0 {
		t.Fatalf("draft %+v", res.Draft)
	}
	if it := got["Pomidor"]; it.PriceSom != 8000 || len(it.Hints) != 0 {
		t.Errorf("pomidor comes from the rules: %+v", it)
	}
	if it := got["Kartoshka"]; it.PriceSom != 4000 {
		t.Errorf("kartoshka: 200 000 a 50 kg bag is 4 000/kg: %+v", it)
	}
	if it := got["Piyoz"]; it.PriceSom != 4500 || it.TradeType != pricing.TradeWholesale || it.Quantity == nil || *it.Quantity != 300 || it.Hints[0].Code != "by_model" {
		t.Errorf("piyoz comes from the model, checked: %+v", it)
	}
	// 9999 is not on the photo and velosiped is not in the catalog.
	if _, ok := got["Sabzi"]; ok {
		t.Error("a price that is not in the transcription was taken")
	}
	if !strings.Contains(res.Transcript, "Pomidor 8 000") {
		t.Errorf("transcript %q", res.Transcript)
	}
	// The image went out re-encoded as JPEG.
	req := fake.Requests[0]
	parts, _ := req.Messages[1].Content.([]any)
	if len(parts) != 2 || !strings.Contains(parts[1].(map[string]any)["image_url"].(map[string]any)["url"].(string), "data:image/jpeg;base64,") {
		t.Errorf("image part: %+v", req.Messages[1].Content)
	}
	if f := rec.Features(); len(f) != 1 || f[0] != "vision" || rec.Calls[0].UserID == nil || *rec.Calls[0].UserID != seller.ID {
		t.Errorf("ai log: %+v", rec.Calls)
	}
	// Nothing is saved before confirmation.
	if mine, _ := q.ListSellerListings(ctx, seller.ID); len(mine) != 0 {
		t.Errorf("listings before confirm: %d", len(mine))
	}

	// Market staff cannot, and the feature is off without a model.
	staff := aitest.User(t, q, 900400002, "market_admin")
	if _, err := r.Read(ctx, staff, bytes.NewReader(pngImage(t))); !errors.Is(err, pricing.ErrNotSeller) {
		t.Errorf("market staff: %v", err)
	}
	if _, err := New(nil, prices, media.DefaultLimits).Read(ctx, seller, bytes.NewReader(pngImage(t))); !errors.Is(err, ErrOff) {
		t.Errorf("off: %v", err)
	}
	if _, err := r.Read(ctx, seller, strings.NewReader("not an image")); !errors.Is(err, media.ErrUnsupported) {
		t.Errorf("not an image: %v", err)
	}
	fake.Chat = func(llm.ChatRequest) llm.Message { return llm.Message{Content: "sorry"} }
	if _, err := r.Read(ctx, seller, bytes.NewReader(pngImage(t))); !errors.Is(err, ErrUnreadable) {
		t.Errorf("bad answer: %v", err)
	}
}
