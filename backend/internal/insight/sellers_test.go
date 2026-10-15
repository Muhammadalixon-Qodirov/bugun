package insight

import (
	"math/rand/v2"
	"slices"
	"testing"

	"github.com/google/uuid"
)

func TestScoreSellers(t *testing.T) {
	r := rand.New(rand.NewPCG(5, 6))
	var ids []uuid.UUID
	var fs []SellerFeatures
	for i := 0; i < 60; i++ {
		ids = append(ids, uuid.New())
		fs = append(fs, SellerFeatures{
			Prices: int32(10 + r.IntN(20)), ReviewShare: 0.02 * r.Float64(), Deviation: 0.05 + 0.05*r.Float64(),
			BelowShare: 0.2 + 0.1*r.Float64(), AboveShare: 0.2 + 0.1*r.Float64(), PerListing: 2 + r.Float64(),
			Products: int32(1 + r.IntN(4)), MaxListingsDay: int32(1 + r.IntN(3)),
		})
	}
	// A dumper: always far below the market, many listings in a day, often held.
	odd := uuid.New()
	ids = append(ids, odd)
	fs = append(fs, SellerFeatures{Prices: 40, ReviewShare: 0.4, Deviation: 0.6, BelowShare: 0.95, AboveShare: 0, PerListing: 2, Products: 2, MaxListingsDay: 30})

	scored, ok := ScoreSellers(ids, fs)
	if !ok {
		t.Fatal("60 sellers must be scored")
	}
	var dumper ScoredSeller
	highest := 0.0
	for _, s := range scored {
		if s.ID == odd {
			dumper = s
		} else if s.Score > highest {
			highest = s.Score
		}
	}
	if dumper.Score < SuspiciousScore || dumper.Score <= highest {
		t.Fatalf("dumper %.3f, highest ordinary %.3f", dumper.Score, highest)
	}
	for _, want := range []string{"review_share", "deviation", "below_share", "max_listings_day"} {
		if !slices.Contains(dumper.Reasons, want) {
			t.Errorf("reasons %v lack %s", dumper.Reasons, want)
		}
	}
	if slices.Contains(dumper.Reasons, "above_share") {
		t.Errorf("a low value is never a reason: %v", dumper.Reasons)
	}
	if _, ok := ScoreSellers(ids[:10], fs[:10]); ok {
		t.Error("10 sellers are too few")
	}
}
