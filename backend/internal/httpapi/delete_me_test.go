package httpapi

import (
	"bytes"
	"context"
	"io"
	"maps"
	"slices"
	"sync"
	"testing"

	"bugun/internal/market"
	"bugun/internal/media"
	"bugun/internal/store/db"
)

// memoryStore is a photo store that keeps files in a map.
type memoryStore struct {
	mu    sync.Mutex
	files map[string][]byte
}

func (m *memoryStore) Put(_ context.Context, key string, data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.files[key] = data
	return nil
}

func (m *memoryStore) Open(_ context.Context, key string) (io.ReadCloser, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	data, ok := m.files[key]
	if !ok {
		return nil, media.ErrNotFound
	}
	return io.NopCloser(bytes.NewReader(data)), nil
}

func (m *memoryStore) Delete(_ context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.files, key)
	return nil
}

func (m *memoryStore) keys() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return slices.Sorted(maps.Keys(m.files))
}

func TestDeleteAccountRemovesPhotoFiles(t *testing.T) {
	photos := &memoryStore{files: map[string][]byte{}}
	a := newTestAPIWithMedia(t, photos)
	ctx := context.Background()
	seller, token := a.signIn(900000631, "entrepreneur", 2)
	other, _ := a.signIn(900000632, "entrepreneur", 2)

	addPhoto := func(u db.User, key string) {
		t.Helper()
		l, err := a.listings.Upsert(ctx, u, market.Input{ProductID: 9, PriceSom: 7000, TradeType: "ulgurji"})
		if err != nil {
			t.Fatal(err)
		}
		if err := media.Save(ctx, photos, media.Photo{Key: key, Data: []byte("x"), Thumb: []byte("t")}); err != nil {
			t.Fatal(err)
		}
		if _, err := a.q.AddListingPhoto(ctx, db.AddListingPhotoParams{ListingID: l.ID, StorageKey: key, Width: 1, Height: 1}); err != nil {
			t.Fatal(err)
		}
	}
	gone := "aaaaaaaaaaaaaaaaaaaaaaaa.jpg"
	kept := "bbbbbbbbbbbbbbbbbbbbbbbb.jpg"
	addPhoto(seller, gone)
	addPhoto(other, kept)

	if code := a.call(token, "DELETE", "/v1/me", nil, nil); code/100 != 2 {
		t.Fatalf("delete account: status %d", code)
	}
	if got, want := photos.keys(), []string{kept, media.ThumbKey(kept)}; !slices.Equal(got, want) {
		t.Fatalf("files left %v, want only the other seller's %v", got, want)
	}
}
