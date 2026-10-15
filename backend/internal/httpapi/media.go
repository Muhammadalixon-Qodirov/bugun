package httpapi

import (
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"

	"bugun/internal/market"
	"bugun/internal/media"
)

// serveMedia streams a stored photo. Photos of live listings are public: keys
// are random and content never changes, so they are cached for a year. A
// photo of a listing under review, hidden or rejected is shown only to its
// seller and to admins, and never cached on the way; one whose listing is
// gone is not found.
func (s *Server) serveMedia(w http.ResponseWriter, r *http.Request) {
	key := chi.URLParam(r, "key")
	if !media.ValidKey(key) {
		http.NotFound(w, r)
		return
	}
	base, thumb := strings.CutSuffix(key, "_t.jpg")
	if thumb {
		base += ".jpg"
	}
	owner, err := s.q.PhotoListing(r.Context(), base)
	if errors.Is(err, pgx.ErrNoRows) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		slog.ErrorContext(r.Context(), "photo listing", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	public := owner.Status == market.StatusActive
	if !public {
		c, err := r.Cookie(sessionCookie)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		u, err := s.currentUser(r.Context(), c.Value)
		if err != nil || (u.ID != owner.SellerID && !s.isAdmin(u)) {
			http.NotFound(w, r) // not "forbidden": the photo's existence is private too
			return
		}
	}

	if public && s.redirectToPublic(w, r, key, base, thumb) {
		return
	}

	rc, err := s.media.Open(r.Context(), key)
	// Photos uploaded before thumbnails existed fall back to the full image.
	if thumb && errors.Is(err, media.ErrNotFound) {
		rc, err = s.media.Open(r.Context(), base)
	}
	if errors.Is(err, media.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		slog.ErrorContext(r.Context(), "open media", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	defer rc.Close()
	w.Header().Set("Content-Type", "image/jpeg")
	if public {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	} else {
		w.Header().Set("Cache-Control", "private, no-store")
	}
	w.Header().Set("X-Content-Type-Options", "nosniff")
	_, _ = io.Copy(w, rc)
}

// redirectToPublic sends a live listing's photo to the storage's public
// address when it has one (S3_PUBLIC_BASE_URL), so the bytes do not pass
// through the API. The redirect itself is cached briefly: the listing may
// be hidden later, and then the photo must go back through the checks above.
// A thumbnail missing from storage (photos older than thumbnails) redirects
// to the full image. It reports whether it answered.
func (s *Server) redirectToPublic(w http.ResponseWriter, r *http.Request, key, base string, thumb bool) bool {
	pub, ok := s.media.(media.Public)
	if !ok {
		return false
	}
	if thumb {
		if sz, ok := s.media.(media.Sizer); ok {
			if _, err := sz.Size(r.Context(), key); errors.Is(err, media.ErrNotFound) {
				key = base
			} else if err != nil {
				slog.WarnContext(r.Context(), "media size", "err", err)
				return false // stream it instead; Open reports the error
			}
		}
	}
	u, ok := pub.PublicURL(key)
	if !ok {
		return false
	}
	w.Header().Set("Cache-Control", "public, max-age=3600")
	http.Redirect(w, r, u, http.StatusFound)
	return true
}
