package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"bugun/internal/media"
	"bugun/internal/secure"
)

// Hand-picked demo photos. Searching Openverse at seed time gave wrong pictures
// (Apple computers for "apples", drawings, dishes), so every product now uses
// photos chosen by eye: photographs only, CC0 or public domain, recorded with
// their source page. Products without picks get no photo rather than a wrong one.

type photoSource struct {
	URL     string // the image itself
	License string // cc0 or pdm
	Page    string // where it came from, for the record
}

// curatedImages downloads the picked photos of each product once into the
// cache and returns them ready to attach.
func curatedImages(ctx context.Context, products []product, dir string) map[string][]media.Photo {
	client := &http.Client{Timeout: 30 * time.Second}
	out := map[string][]media.Photo{}
	for _, p := range products {
		for _, src := range seedPhotos[p.Slug] {
			// Keyed by URL so editing the picks never serves a stale file.
			name := filepath.Join(dir, "curated", p.Slug, fmt.Sprintf("%x.jpg", sha256.Sum256([]byte(src.URL))))
			data, err := os.ReadFile(name)
			if err != nil {
				if data, err = fetch(ctx, client, src.URL); err != nil {
					slog.Warn("curated photo", "product", p.Slug, "url", src.URL, "err", err)
					continue
				}
				if err := os.MkdirAll(filepath.Dir(name), 0o750); err == nil {
					_ = os.WriteFile(name, data, 0o640)
				}
				time.Sleep(300 * time.Millisecond)
			}
			ph, err := media.Process(bytes.NewReader(data))
			if err != nil {
				slog.Warn("curated photo", "product", p.Slug, "url", src.URL, "err", err)
				continue
			}
			out[p.Slug] = append(out[p.Slug], ph)
		}
	}
	return out
}

// replacePhotos gives every demo listing new photos from the picks, keeping
// the listings, prices and history as they are.
func (s *seeder) replacePhotos(ctx context.Context, images map[string][]media.Photo) error {
	rows, err := s.pool.Query(ctx, `
		SELECT l.id, p.slug, l.seller_id FROM listings l
		JOIN users u ON u.id = l.seller_id JOIN products p ON p.id = l.product_id
		WHERE u.is_demo ORDER BY l.id`)
	if err != nil {
		return err
	}
	type listing struct {
		id     int64
		slug   string
		seller [16]byte
	}
	var all []listing
	for rows.Next() {
		var l listing
		if err := rows.Scan(&l.id, &l.slug, &l.seller); err != nil {
			rows.Close()
			return err
		}
		all = append(all, l)
	}
	rows.Close()
	removed, added := 0, 0
	for _, l := range all {
		keys, err := s.pool.Query(ctx, `DELETE FROM listing_photos WHERE listing_id = $1 RETURNING storage_key`, l.id)
		if err != nil {
			return err
		}
		var old []string
		for keys.Next() {
			var k string
			if err := keys.Scan(&k); err != nil {
				keys.Close()
				return err
			}
			old = append(old, k)
		}
		keys.Close()
		for _, k := range old {
			_ = media.Remove(ctx, s.store, k)
			removed++
		}
		pool := images[l.slug]
		if len(pool) == 0 {
			continue
		}
		seller, err := s.q.GetUser(ctx, l.seller)
		if err != nil {
			return err
		}
		k := 1 + s.rng.IntN(min(3, len(pool)))
		for _, idx := range s.rng.Perm(len(pool))[:k] {
			ph := pool[idx]
			ph.Key = secure.Token(24) + ".jpg"
			if _, err := s.svc.AddPhoto(ctx, s.store, seller, l.id, ph); err != nil {
				return fmt.Errorf("attach photo: %w", err)
			}
			added++
		}
	}
	slog.Info("replaced demo photos", "listings", len(all), "removed", removed, "added", added)
	return nil
}
