package market

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"

	"bugun/internal/store/db"
)

// ContentChecker screens a listing's text (spec AI-7: moderation.Checker)
// and returns a review reason, or "" when it may go live. It must not fail:
// a checker that cannot decide returns "".
type ContentChecker interface {
	Check(ctx context.Context, p db.GetProductRow, variety, description string) string
}

// WithContentChecker screens listing texts on create and update; a flag
// holds the listing for review with the checker's reason.
func WithContentChecker(c ContentChecker) Option { return func(s *Service) { s.content = c } }

// checkContent runs before the transaction (a model may be slow) and
// records the result on in.
func (s *Service) checkContent(ctx context.Context, productID int16, in *Input) {
	in.contentFlag = ""
	if s.content == nil || (in.Variety == "" && in.Description == "") {
		return
	}
	p, err := s.q.GetProduct(ctx, productID)
	if err != nil {
		return // an unknown product fails later with its own error
	}
	in.contentFlag = s.content.Check(ctx, p, in.Variety, in.Description)
}

// contentReview holds a listing whose price passed but whose text was flagged.
func contentReview(in Input) (string, pgtype.Text, bool) {
	if in.contentFlag == "" {
		return "", pgtype.Text{}, false
	}
	return StatusReview, pgtype.Text{String: in.contentFlag, Valid: true}, true
}
