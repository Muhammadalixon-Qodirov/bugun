package market

import (
	"bytes"
	"context"
	"errors"
	"io"
	"slices"
	"sync"
	"testing"
	"time"

	"bugun/internal/media"
	"bugun/internal/store/db"
)

type mapStore struct {
	mu    sync.Mutex
	files map[string][]byte
}

func (m *mapStore) Put(_ context.Context, key string, data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.files[key] = data
	return nil
}

func (m *mapStore) Open(_ context.Context, key string) (io.ReadCloser, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if d, ok := m.files[key]; ok {
		return io.NopCloser(bytes.NewReader(d)), nil
	}
	return nil, media.ErrNotFound
}

func (m *mapStore) Delete(_ context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.files, key)
	return nil
}

func TestConfiguredPhotoLimit(t *testing.T) {
	_, u, pool := testSeller(t)
	svc := NewService(pool, WithMaxPhotos(1))
	ctx := context.Background()
	l, err := svc.Upsert(ctx, u, Input{ProductID: 1, PriceSom: 3000, TradeType: "ulgurji"})
	if err != nil {
		t.Fatal(err)
	}
	store := &mapStore{files: map[string][]byte{}}
	if _, err := svc.AddPhoto(ctx, store, u, l.ID, media.Photo{Key: "dddddddddddddddddddddddd.jpg", Data: []byte("a")}); err != nil {
		t.Fatal(err)
	}
	_, err = svc.AddPhoto(ctx, store, u, l.ID, media.Photo{Key: "eeeeeeeeeeeeeeeeeeeeeeee.jpg", Data: []byte("b")})
	if !errors.Is(err, ErrTooManyPhotos) {
		t.Errorf("second photo with a limit of 1: %v, want ErrTooManyPhotos", err)
	}
	if _, kept := store.files["eeeeeeeeeeeeeeeeeeeeeeee.jpg"]; kept {
		t.Error("the refused photo's file was left behind")
	}
	if svc.MaxPhotos() != 1 || NewService(pool).MaxPhotos() != DefaultMaxPhotos {
		t.Error("MaxPhotos does not report the limit")
	}
}

// A price set by hand pauses automatic pricing for AUTO_PRICE_MANUAL_PAUSE.
func TestConfiguredManualPause(t *testing.T) {
	_, u, pool := testSeller(t)
	ctx := context.Background()
	l, err := NewService(pool).Upsert(ctx, u, Input{ProductID: 1, PriceSom: 3000, TradeType: "ulgurji"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE listings SET pricing_mode = 'auto', price_min = 2500, price_max = 3500,
		manual_price_at = now() - interval '2 hours' WHERE id = $1`, l.ID); err != nil {
		t.Fatal(err)
	}
	q := db.New(pool)
	due := func(pause time.Duration) bool {
		ids, err := q.AutoPricedListings(ctx, pause.Seconds())
		if err != nil {
			t.Fatal(err)
		}
		return slices.Contains(ids, l.ID)
	}
	if due(DefaultAutoPriceRules.ManualPause) {
		t.Error("repriced 2 hours after a manual price with the 24-hour pause")
	}
	if !due(time.Hour) {
		t.Error("not repriced 2 hours after a manual price with a 1-hour pause")
	}
	short := DefaultAutoPriceRules
	short.ManualPause = time.Hour
	if NewService(pool, WithAutoPrice(short)).ManualPause() != time.Hour {
		t.Error("ManualPause does not report the setting")
	}
}
