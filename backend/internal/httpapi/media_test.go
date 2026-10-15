package httpapi

import (
	"context"
	"io"
	"net/http"
	"testing"

	"bugun/internal/market"
	"bugun/internal/media"
	"bugun/internal/store/db"
)

func TestMediaOfHiddenListingsIsPrivate(t *testing.T) {
	photos := &memoryStore{files: map[string][]byte{}}
	a := newTestAPIWithMedia(t, photos)
	ctx := context.Background()
	seller, sellerToken := a.signIn(900000641, "entrepreneur", 2)
	_, strangerToken := a.signIn(900000642, "entrepreneur", 2)
	_, adminToken := a.signIn(900000643, "admin", 2)

	l, err := a.listings.Upsert(ctx, seller, market.Input{ProductID: 9, PriceSom: 7000, TradeType: "ulgurji"})
	if err != nil {
		t.Fatal(err)
	}
	key := "cccccccccccccccccccccccc.jpg"
	if err := media.Save(ctx, photos, media.Photo{Key: key, Data: []byte("full"), Thumb: []byte("thumb")}); err != nil {
		t.Fatal(err)
	}
	if _, err := a.q.AddListingPhoto(ctx, db.AddListingPhotoParams{ListingID: l.ID, StorageKey: key, Width: 1, Height: 1}); err != nil {
		t.Fatal(err)
	}

	get := func(token, key string) (int, string, string) {
		t.Helper()
		req, _ := http.NewRequest("GET", a.srv.URL+"/media/"+key, nil)
		if token != "" {
			req.AddCookie(&http.Cookie{Name: sessionCookie, Value: token})
		}
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		b, _ := io.ReadAll(res.Body)
		return res.StatusCode, res.Header.Get("Cache-Control"), string(b)
	}

	// A live listing's photo is public and cached for a year.
	if code, cc, body := get("", key); code != 200 || cc != "public, max-age=31536000, immutable" || body != "full" {
		t.Errorf("live photo: %d %q %q", code, cc, body)
	}
	if code, _, body := get("", media.ThumbKey(key)); code != 200 || body != "thumb" {
		t.Errorf("live thumbnail: %d %q", code, body)
	}

	// Hidden: only the seller and admins, never cached.
	if err := a.q.SetListingStatus(ctx, db.SetListingStatusParams{ID: l.ID, SellerID: seller.ID, Status: market.StatusHidden}); err != nil {
		t.Fatal(err)
	}
	for name, token := range map[string]string{"anonymous": "", "another user": strangerToken, "bad session": "nope"} {
		if code, cc, _ := get(token, key); code != http.StatusNotFound || cc == "public, max-age=31536000, immutable" {
			t.Errorf("hidden photo for %s: %d %q, want 404", name, code, cc)
		}
		if code, _, _ := get(token, media.ThumbKey(key)); code != http.StatusNotFound {
			t.Errorf("hidden thumbnail for %s: %d, want 404", name, code)
		}
	}
	for name, token := range map[string]string{"seller": sellerToken, "admin": adminToken} {
		if code, cc, body := get(token, key); code != 200 || cc != "private, no-store" || body != "full" {
			t.Errorf("hidden photo for the %s: %d %q %q", name, code, cc, body)
		}
	}

	// Rejected by an admin: the same.
	if _, err := a.pool.Exec(ctx, `UPDATE listings SET status = 'rejected' WHERE id = $1`, l.ID); err != nil {
		t.Fatal(err)
	}
	if code, _, _ := get("", key); code != http.StatusNotFound {
		t.Errorf("rejected photo for anonymous: %d, want 404", code)
	}
	if code, cc, _ := get(sellerToken, key); code != 200 || cc != "private, no-store" {
		t.Errorf("rejected photo for the seller: %d %q", code, cc)
	}

	// Deleted: the file may linger, but nobody gets it.
	if _, err := a.pool.Exec(ctx, `DELETE FROM listings WHERE id = $1`, l.ID); err != nil {
		t.Fatal(err)
	}
	if code, _, _ := get(sellerToken, key); code != http.StatusNotFound {
		t.Errorf("photo of a deleted listing: %d, want 404", code)
	}
	if code, _, _ := get("", "../../etc/passwd"); code != http.StatusNotFound {
		t.Errorf("invalid key: %d, want 404", code)
	}
}
