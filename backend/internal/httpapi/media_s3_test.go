package httpapi

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/johannesboyne/gofakes3"
	"github.com/johannesboyne/gofakes3/backend/s3mem"

	"bugun/internal/market"
	"bugun/internal/media"
	"bugun/internal/store/db"
)

// With S3 storage and a public base URL, live photos are redirected to the
// bucket and all others still stream through the access checks.
func TestMediaFromS3(t *testing.T) {
	backend := s3mem.New()
	if err := backend.CreateBucket("photos"); err != nil {
		t.Fatal(err)
	}
	fake := httptest.NewServer(gofakes3.New(backend).Server())
	defer fake.Close()
	photos, err := media.NewS3Store(media.S3Config{Endpoint: fake.URL, Region: "us-east-1", Bucket: "photos",
		AccessKey: "test", SecretKey: "test", PathStyle: true, PublicBaseURL: "https://cdn.example.com/photos"})
	if err != nil {
		t.Fatal(err)
	}
	a := newTestAPIWithMedia(t, photos)
	ctx := context.Background()
	seller, sellerToken := a.signIn(900000661, "entrepreneur", 2)

	l, err := a.listings.Upsert(ctx, seller, market.Input{ProductID: 9, PriceSom: 7000, TradeType: "ulgurji"})
	if err != nil {
		t.Fatal(err)
	}
	key := "dddddddddddddddddddddddd.jpg"
	// No thumbnail: a photo from before thumbnails existed.
	if err := photos.Put(ctx, key, []byte("full")); err != nil {
		t.Fatal(err)
	}
	if _, err := a.q.AddListingPhoto(ctx, db.AddListingPhotoParams{ListingID: l.ID, StorageKey: key, Width: 1, Height: 1}); err != nil {
		t.Fatal(err)
	}

	noRedirect := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	get := func(token, key string) (int, string, string) {
		t.Helper()
		req, _ := http.NewRequest("GET", a.srv.URL+"/media/"+key, nil)
		if token != "" {
			req.AddCookie(&http.Cookie{Name: sessionCookie, Value: token})
		}
		res, err := noRedirect.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		b, _ := io.ReadAll(res.Body)
		if res.StatusCode == http.StatusFound {
			return res.StatusCode, res.Header.Get("Location"), ""
		}
		return res.StatusCode, res.Header.Get("Cache-Control"), string(b)
	}

	if code, loc, _ := get("", key); code != http.StatusFound || loc != "https://cdn.example.com/photos/"+key {
		t.Errorf("live photo: %d %q, want a redirect to the bucket", code, loc)
	}
	if code, loc, _ := get("", media.ThumbKey(key)); code != http.StatusFound || loc != "https://cdn.example.com/photos/"+key {
		t.Errorf("missing thumbnail: %d %q, want the full photo", code, loc)
	}

	if err := a.q.SetListingStatus(ctx, db.SetListingStatusParams{ID: l.ID, SellerID: seller.ID, Status: market.StatusHidden}); err != nil {
		t.Fatal(err)
	}
	if code, _, _ := get("", key); code != http.StatusNotFound {
		t.Errorf("hidden photo for anonymous: %d, want 404", code)
	}
	if code, cc, body := get(sellerToken, key); code != 200 || cc != "private, no-store" || body != "full" {
		t.Errorf("hidden photo for the seller: %d %q %q, want it streamed from S3", code, cc, body)
	}
}
