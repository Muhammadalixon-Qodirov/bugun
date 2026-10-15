package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"bugun/internal/market"
	"bugun/internal/secure"
	"bugun/internal/store/db"
	"bugun/internal/tz"
)

// Demand signals (spec §10.3) are recorded best-effort: a failure is logged
// and never fails the request that produced it.

type visitorCtxKey struct{}

// withVisitor puts the rate-limit client key (IP, or the site's forwarded
// visitor IP) into the request context for demand logging.
func (s *Server) withVisitor(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if key, err := s.clientKey(r); err == nil {
			r = r.WithContext(context.WithValue(r.Context(), visitorCtxKey{}, key))
		}
		next.ServeHTTP(w, r)
	})
}

// visitor identifies who produced a demand signal: the signed-in user, else
// the client IP. It returns the user when there is one.
func (s *Server) visitor(ctx context.Context, session string) (id string, u *db.User) {
	if session != "" {
		uid, err := s.q.GetSessionUserID(ctx, secure.HashToken(session))
		if err == nil {
			if user, err := s.q.GetUser(ctx, uid); err == nil {
				return "u:" + uid.String(), &user
			}
		} else if !errors.Is(err, pgx.ErrNoRows) {
			slog.WarnContext(ctx, "demand visitor", "err", err)
		}
	}
	key, _ := ctx.Value(visitorCtxKey{}).(string)
	return "ip:" + key, nil
}

// recordDemand stores one signal for a product in a viloyat (0 = unknown).
// The key mixes in the Tashkent day, so a visitor is not linkable across days.
func (s *Server) recordDemand(ctx context.Context, kind string, productID, province int16, visitorID string) {
	if s.sealer == nil || visitorID == "ip:" {
		return
	}
	day := time.Now().In(tz.Tashkent).Format(time.DateOnly)
	err := s.q.RecordDemand(ctx, db.RecordDemandParams{
		Kind: kind, ProductID: productID, ProvinceID: pgtype.Int2{Int16: province, Valid: province != 0},
		ViewerKey: s.sealer.Hash(day + "|" + visitorID),
	})
	if err != nil {
		slog.WarnContext(ctx, "record demand", "kind", kind, "err", err)
	}
}

// recordListingEvent stores one view or contact reveal of a listing, once per
// visitor and Tashkent day. The seller's own visits are not interest.
func (s *Server) recordListingEvent(ctx context.Context, kind string, listingID int64, visitorID string) {
	if s.sealer == nil || visitorID == "ip:" {
		return
	}
	day := time.Now().In(tz.Tashkent).Format(time.DateOnly)
	err := s.q.RecordListingEvent(ctx, db.RecordListingEventParams{
		ListingID: listingID, Kind: kind, ViewerKey: s.sealer.Hash(day + "|" + visitorID),
	})
	if err != nil {
		slog.WarnContext(ctx, "record listing event", "kind", kind, "err", err)
	}
}

// demandProvince is the viloyat a signal belongs to: the region the visitor
// is looking at, else their own, else unknown (0).
func (s *Server) demandProvince(ctx context.Context, region int16, u *db.User) int16 {
	if region == 0 && u != nil && u.RegionID.Valid {
		region = u.RegionID.Int16
	}
	if region == 0 {
		return 0
	}
	p, err := market.ProvinceOf(ctx, s.q, region)
	if err != nil {
		return 0
	}
	return p
}
