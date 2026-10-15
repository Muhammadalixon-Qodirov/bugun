package ask

import (
	"context"

	"bugun/internal/pricing"
	"bugun/internal/store/db"
)

// Exported pieces of the rule-based answers, for the analyst (spec AI-5),
// which calls the same functions whether a model or the rules route the
// question.

// FindRegion is the place a text names (any script), or nil.
func (s *Service) FindRegion(text string) *Region { return s.findRegion(pricing.Normalize(text)) }

// Stats is the live market price of a product, widening from the region to
// its viloyat and the country until there are enough prices.
func (s *Service) Stats(ctx context.Context, product int16, trade string, r *Region) (Stats, error) {
	return s.stats(ctx, product, trade, r)
}

// Areas compares tumans of the region's viloyat, or all viloyats for nil.
func (s *Service) Areas(ctx context.Context, product int16, trade string, r *Region) ([]Area, error) {
	return s.areas(ctx, product, trade, r)
}

// Log keeps a question the analyst answered without Ask (question_log and
// demand), with the product and region it was about when known.
func (s *Service) Log(ctx context.Context, u *db.User, question string, product int16, r *Region) {
	s.log(ctx, u, question, product, r)
}

// IntentOf is the rule-based reading of what a question asks.
func IntentOf(question string) Intent { return intentOf(pricing.Normalize(question)) }
