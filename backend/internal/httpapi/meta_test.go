package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"bugun/internal/config"
	"bugun/internal/market"
	"bugun/internal/rfq"
)

func limitsOf(t *testing.T, h http.Handler) (Limits, string) {
	t.Helper()
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/v1/meta/limits", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body)
	}
	var l Limits
	if err := json.NewDecoder(w.Body).Decode(&l); err != nil {
		t.Fatal(err)
	}
	return l, w.Header().Get("Cache-Control")
}

func TestMetaLimitsDefaults(t *testing.T) {
	l, cc := limitsOf(t, New(config.Config{}, nil, Deps{}))
	if cc != "public, max-age=300" {
		t.Errorf("Cache-Control = %q", cc)
	}
	want := Limits{
		RFQ:                 RFQLimits{MaxOpen: 3, DefaultDays: 3, MinDays: 1, MaxDays: 7},
		AutoPrice:           AutoPriceLimits{MaxRatio: 3, DailyStep: 0.05, ManualPauseHours: 24},
		Photos:              PhotoLimits{MaxPerListing: 5, MaxBytes: 10 << 20, MaxPixels: 40_000_000},
		WatchesMax:          20,
		CompareMaxLines:     4,
		AdminDealsLimit:     100,
		AdminAnomaliesLimit: 300,
	}
	if l != want {
		t.Errorf("limits = %+v\nwant %+v", l, want)
	}
}

func TestMetaLimitsFollowConfig(t *testing.T) {
	cfg := config.Config{MediaMaxUploadBytes: 5 << 20, MediaMaxPixels: 20_000_000}
	auto := market.DefaultAutoPriceRules
	auto.ManualPause, auto.DailyStep = 12*time.Hour, 0.1
	h := New(cfg, nil, Deps{
		Listings: market.NewService(nil, market.WithAutoPrice(auto), market.WithMaxPhotos(8)),
		RFQ:      rfq.NewService(nil, nil, rfq.WithLimits(rfq.Limits{MaxOpen: 5, DefaultDays: 2, MinDays: 1, MaxDays: 14})),
	})
	l, _ := limitsOf(t, h)
	if l.RFQ.MaxOpen != 5 || l.RFQ.MaxDays != 14 || l.AutoPrice.ManualPauseHours != 12 || l.AutoPrice.DailyStep != 0.1 ||
		l.Photos.MaxPerListing != 8 || l.Photos.MaxBytes != 5<<20 || l.Photos.MaxPixels != 20_000_000 {
		t.Errorf("limits = %+v", l)
	}
}
