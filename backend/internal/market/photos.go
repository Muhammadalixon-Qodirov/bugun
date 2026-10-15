package market

import (
	"context"
	"errors"
	"log/slog"

	"github.com/jackc/pgx/v5"

	"bugun/internal/media"
	"bugun/internal/store/db"
)

// DefaultMaxPhotos per listing (config: MAX_PHOTOS_PER_LISTING).
const DefaultMaxPhotos = 5

// MaxPhotos is how many photos a listing may have.
func (s *Service) MaxPhotos() int { return s.maxPhotos }

var ErrTooManyPhotos = errors.New("too many photos")

// AddPhoto stores the image and attaches it to the seller's listing. The
// listing row is locked, so concurrent uploads cannot exceed the limit.
func (s *Service) AddPhoto(ctx context.Context, store media.Store, u db.User, listingID int64, p media.Photo) (db.ListingPhoto, error) {
	if err := media.Save(ctx, store, p); err != nil {
		return db.ListingPhoto{}, err
	}
	var row db.ListingPhoto
	err := s.inTx(ctx, func(q *db.Queries) error {
		if _, err := q.GetListingForUpdate(ctx, db.GetListingForUpdateParams{ID: listingID, SellerID: u.ID}); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrNotFound
			}
			return err
		}
		n, err := q.CountListingPhotos(ctx, listingID)
		if err != nil {
			return err
		}
		if n >= int64(s.maxPhotos) {
			return ErrTooManyPhotos
		}
		row, err = q.AddListingPhoto(ctx, db.AddListingPhotoParams{
			ListingID: listingID, StorageKey: p.Key, Width: int32(p.Width), Height: int32(p.Height),
		})
		return err
	})
	if err != nil {
		if derr := media.Remove(context.WithoutCancel(ctx), store, p.Key); derr != nil {
			slog.WarnContext(ctx, "remove orphan photo", "err", derr)
		}
		return db.ListingPhoto{}, err
	}
	return row, nil
}

// DeletePhoto removes one of the seller's photos from the listing and storage.
func (s *Service) DeletePhoto(ctx context.Context, store media.Store, u db.User, photoID int64) error {
	key, err := s.q.DeleteListingPhoto(ctx, db.DeleteListingPhotoParams{ID: photoID, SellerID: u.ID})
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if err := media.Remove(ctx, store, key); err != nil {
		slog.WarnContext(ctx, "delete photo file", "err", err)
	}
	return nil
}

// DeleteListing removes the listing and its photo files.
func (s *Service) DeleteListing(ctx context.Context, store media.Store, u db.User, id int64) error {
	photos, err := s.q.ListListingPhotos(ctx, id)
	if err != nil {
		return err
	}
	l, err := s.q.GetListing(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && l.SellerID != u.ID) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if err := s.q.DeleteListing(ctx, db.DeleteListingParams{ID: id, SellerID: u.ID}); err != nil {
		return err
	}
	for _, p := range photos {
		if err := media.Remove(ctx, store, p.StorageKey); err != nil {
			slog.WarnContext(ctx, "delete photo file", "err", err)
		}
	}
	return nil
}
