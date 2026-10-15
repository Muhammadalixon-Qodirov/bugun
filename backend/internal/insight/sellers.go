package insight

import (
	"context"
	"encoding/json"
	"math"

	"github.com/google/uuid"

	"bugun/internal/ml"
	"bugun/internal/store/db"
)

// Unusual sellers (ML_REJA.md 5, HUDUD's ml/anomaly.py layer 3): an
// Isolation Forest over what each seller did in the last 60 days. It needs
// no labels and finds sellers unlike the rest in some combination of
// features; each one gets the features that stand out as reasons. It is a
// hint for an admin to look, never a block: a big wholesaler can be unusual
// and honest.

const (
	// MinScoredSellers: with fewer the forest has nothing to compare with.
	MinScoredSellers = 30
	// SuspiciousScore: the score from which a seller is listed for admins.
	SuspiciousScore = 0.6
	// reasonZ: a feature this many robust deviations above the others is a reason.
	reasonZ = 2.5
)

// SellerFeatures is what the forest looks at, as stored and shown.
type SellerFeatures struct {
	Prices         int32   `json:"prices" doc:"Own prices in the last 60 days"`
	ReviewShare    float64 `json:"review_share" doc:"Share held for review or rejected"`
	Deviation      float64 `json:"deviation" doc:"Typical distance from the viloyat's daily median, as a log ratio (0.2 ≈ 22%)"`
	BelowShare     float64 `json:"below_share" doc:"Share below the day's lower quartile"`
	AboveShare     float64 `json:"above_share" doc:"Share above the day's upper quartile"`
	PerListing     float64 `json:"per_listing" doc:"Prices per listing: how often prices are changed"`
	Products       int32   `json:"products"`
	MaxListingsDay int32   `json:"max_listings_day" doc:"Most listings created on one day"`
}

func (f SellerFeatures) vector() []float64 {
	return []float64{f.ReviewShare, f.Deviation, f.BelowShare, f.AboveShare, math.Log1p(f.PerListing), math.Log1p(float64(f.MaxListingsDay)), math.Log1p(float64(f.Prices))}
}

// reasonCodes name the vector's features that only count when high.
var reasonCodes = []string{"review_share", "deviation", "below_share", "above_share", "per_listing", "max_listings_day", ""}

// ScoredSeller is one seller's result.
type ScoredSeller struct {
	ID       uuid.UUID
	Features SellerFeatures
	Score    float64
	Reasons  []string
}

// ScoreSellers runs the forest; ok is false with fewer than 30 sellers.
func ScoreSellers(ids []uuid.UUID, fs []SellerFeatures) ([]ScoredSeller, bool) {
	if len(fs) < MinScoredSellers {
		return nil, false
	}
	rows := make([][]float64, len(fs))
	for i, f := range fs {
		rows[i] = f.vector()
	}
	forest := ml.NewForest(rows)
	cols := make([][]float64, len(rows[0]))
	for j := range cols {
		for _, r := range rows {
			cols[j] = append(cols[j], r[j])
		}
	}
	out := make([]ScoredSeller, len(fs))
	for i, f := range fs {
		s := ScoredSeller{ID: ids[i], Features: f, Score: round3(forest.Score(rows[i])), Reasons: []string{}}
		for j, code := range reasonCodes {
			if code == "" {
				continue
			}
			// A feature that is zero for nearly everyone has no spread; any
			// clearly non-zero value then stands out on its own.
			z, ok := ml.RobustZ(rows[i][j], cols[j])
			if (ok && z > reasonZ) || (!ok && rows[i][j] > ml.Median(cols[j])+0.25) {
				s.Reasons = append(s.Reasons, code)
			}
		}
		out[i] = s
	}
	return out, true
}

// RefreshSellerScores recomputes every seller's score. It returns how many
// sellers were scored; with too few, the old scores are cleared.
func (s *Service) RefreshSellerScores(ctx context.Context) (int, error) {
	rows, err := s.q.SellerFeatures(ctx)
	if err != nil {
		return 0, err
	}
	ids := make([]uuid.UUID, 0, len(rows))
	fs := make([]SellerFeatures, 0, len(rows))
	for _, r := range rows {
		if !r.SellerID.Valid {
			continue
		}
		ids = append(ids, r.SellerID.Bytes)
		fs = append(fs, SellerFeatures{Prices: r.Prices, ReviewShare: round3(r.ReviewShare), Deviation: round3(r.Deviation),
			BelowShare: round3(r.BelowShare), AboveShare: round3(r.AboveShare), PerListing: round3(r.PerListing),
			Products: r.Products, MaxListingsDay: r.MaxListingsDay})
	}
	scored, ok := ScoreSellers(ids, fs)
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)
	q := s.q.WithTx(tx)
	if err := q.DeleteSellerScores(ctx); err != nil {
		return 0, err
	}
	if ok {
		for _, sc := range scored {
			f, err := json.Marshal(sc.Features)
			if err != nil {
				return 0, err
			}
			if err := q.InsertSellerScore(ctx, db.InsertSellerScoreParams{SellerID: sc.ID, Score: sc.Score, Features: f, Reasons: sc.Reasons}); err != nil {
				return 0, err
			}
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	return len(scored), nil
}
