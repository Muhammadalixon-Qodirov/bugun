// Command seed fills the database with demo sellers and listings for
// presentations, and removes them again with -purge. Demo accounts are marked
// is_demo, use fake phone numbers (+998 90 000 xx xx) and no Telegram username.
//
// Photos are hand-picked CC0 and public-domain photographs (photos.go), cached
// on disk so reruns do not download them again.
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"math"
	"math/rand/v2"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"bugun/internal/config"
	"bugun/internal/insight"
	"bugun/internal/market"
	"bugun/internal/media"
	"bugun/internal/secure"
	"bugun/internal/store"
	"bugun/internal/store/db"
)

func main() {
	var (
		listings = flag.Int("listings", 1000, "number of demo listings to create")
		sellers  = flag.Int("sellers", 150, "number of demo sellers")
		cacheDir = flag.String("cache", "/tmp/bugun-seed-images", "image cache directory")
		purge    = flag.Bool("purge", false, "remove all demo data and exit")
		noImages = flag.Bool("no-images", false, "skip photo download")
		randSeed = flag.Uint64("seed", 2026, "random seed, for repeatable data")
		drop     = flag.String("drop", "", "comma-separated cache images (slug/NN.jpg) to remove from demo listings and the cache, then exit")
		history  = flag.Int("history", 0, "years of demo price history to generate (TAHLIL_REJA.md); 0 = none")
		histOnly = flag.Bool("history-only", false, "only replace the demo history; keep sellers and listings")
		photos   = flag.Bool("photos-only", false, "only replace the photos of demo listings with the hand-picked ones")
	)
	flag.Parse()
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, nil)))
	if err := run(*listings, *sellers, *cacheDir, *purge, *noImages, *randSeed, *drop, *history, *histOnly, *photos); err != nil {
		slog.Error("seed failed", "err", err)
		os.Exit(1)
	}
}

type seeder struct {
	pool   *pgxpool.Pool
	q      *db.Queries
	sealer *secure.Sealer
	store  media.Store
	svc    *market.Service
	rng    *rand.Rand
}

func run(nListings, nSellers int, cacheDir string, purge, noImages bool, seed uint64, drop string, history int, historyOnly, photosOnly bool) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	ctx := context.Background()
	pool, err := store.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	if err := store.Migrate(ctx, pool); err != nil {
		return err
	}
	sealer, err := secure.NewSealer(cfg.DataKey)
	if err != nil {
		return err
	}
	// Demo photos go where the API reads them: MEDIA_DIR or the S3 bucket.
	photos, err := media.NewStore(ctx, cfg.MediaStorage, cfg.MediaDir, cfg.S3())
	if err != nil {
		return err
	}
	s := &seeder{pool: pool, q: db.New(pool), sealer: sealer, store: photos, svc: market.NewService(pool), rng: rand.New(rand.NewPCG(seed, seed^0x9e3779b9))}

	if drop != "" {
		return s.dropImages(ctx, cacheDir, strings.Split(drop, ","))
	}
	if photosOnly {
		products, err := s.products(ctx)
		if err != nil {
			return err
		}
		return s.replacePhotos(ctx, curatedImages(ctx, products, cacheDir))
	}
	if historyOnly {
		products, err := s.products(ctx)
		if err != nil {
			return err
		}
		return s.seedHistory(ctx, max(history, 1), products)
	}
	if err := s.purge(ctx); err != nil {
		return err
	}
	if purge {
		return nil
	}

	products, err := s.products(ctx)
	if err != nil {
		return err
	}
	images := map[string][]media.Photo{}
	if !noImages {
		images = curatedImages(ctx, products, cacheDir)
	}
	users, err := s.createSellers(ctx, nSellers, products)
	if err != nil {
		return err
	}
	if err := s.createListings(ctx, nListings, users, products, images); err != nil {
		return err
	}
	if _, err := insight.NewService(pool).RebuildDaily(ctx, time.Now().AddDate(0, 0, -30), time.Now()); err != nil {
		return err
	}
	if history > 0 {
		return s.seedHistory(ctx, history, products)
	}
	return nil
}

// purge deletes demo sellers, their listings, price history and photo files.
func (s *seeder) purge(ctx context.Context) error {
	var keys []string
	rows, err := s.pool.Query(ctx, `SELECT ph.storage_key FROM listing_photos ph
		JOIN listings l ON l.id = ph.listing_id JOIN users u ON u.id = l.seller_id WHERE u.is_demo`)
	if err != nil {
		return err
	}
	for rows.Next() {
		var k string
		if err := rows.Scan(&k); err != nil {
			return err
		}
		keys = append(keys, k)
	}
	rows.Close()
	tag, err := s.pool.Exec(ctx, `
		WITH demo AS (SELECT id FROM users WHERE is_demo)
		DELETE FROM price_observations WHERE seller_id IN (SELECT id FROM demo)`)
	if err != nil {
		return err
	}
	users, err := s.pool.Exec(ctx, `DELETE FROM users WHERE is_demo`)
	if err != nil {
		return err
	}
	if _, err := s.pool.Exec(ctx, `DELETE FROM market_events WHERE is_demo`); err != nil {
		return err
	}
	// The demo history and the days demo prices were live go with them.
	if _, err := s.pool.Exec(ctx, `DELETE FROM price_daily WHERE demo_share > 0`); err != nil {
		return err
	}
	// Open anomaly flags were found in that history; the jobs look again.
	if _, err := s.pool.Exec(ctx, `DELETE FROM price_anomalies WHERE status = 'open'`); err != nil {
		return err
	}
	if _, err := insight.NewService(s.pool).RebuildDaily(ctx, time.Now().AddDate(0, 0, -30), time.Now()); err != nil {
		return err
	}
	for _, k := range keys {
		_ = media.Remove(ctx, s.store, k)
	}
	slog.Info("purged demo data", "users", users.RowsAffected(), "prices", tag.RowsAffected(), "photos", len(keys))
	return nil
}

type product struct {
	ID       int16
	Slug     string
	Name     string
	Category string
	Unit     string
	seedProduct
}

func (s *seeder) products(ctx context.Context) ([]product, error) {
	rows, err := s.q.ListProducts(ctx)
	if err != nil {
		return nil, err
	}
	var out []product
	for _, r := range rows {
		sp, ok := seedProducts[r.Slug]
		if !ok {
			slog.Warn("no seed data for product, skipping", "slug", r.Slug)
			continue
		}
		out = append(out, product{ID: r.ID, Slug: r.Slug, Name: r.Name, Category: r.Category, Unit: r.BaseUnit, seedProduct: sp})
	}
	return out, nil
}

type seller struct {
	user       db.User
	categories []string
}

func (s *seeder) createSellers(ctx context.Context, n int, products []product) ([]seller, error) {
	regions, err := s.q.ListRegions(ctx)
	if err != nil {
		return nil, err
	}
	markets, err := s.q.ListMarkets(ctx)
	if err != nil {
		return nil, err
	}
	// Leaf regions, weighted: the Namangan pilot and Tashkent city get more sellers.
	var pool []db.ListRegionsRow
	for _, r := range regions {
		if !r.ParentID.Valid {
			continue
		}
		w := 1
		switch r.ParentID.Int16 {
		case 1:
			w = 6
		case 110:
			w = 3
		}
		for range w {
			pool = append(pool, r)
		}
	}
	cats := map[string]bool{}
	for _, p := range products {
		cats[p.Category] = true
	}
	var catList []string
	for c := range cats {
		catList = append(catList, c)
	}

	out := make([]seller, 0, n)
	for i := range n {
		r := pool[s.rng.IntN(len(pool))]
		name := fmt.Sprintf("%s %s", pick(s.rng, namePrefixes), pick(s.rng, nameSuffixes))
		phone := fmt.Sprintf("+99890000%04d", i+1)
		var market pgtype.Int2
		var local []db.Market
		for _, m := range markets {
			if m.RegionID == r.ID {
				local = append(local, m)
			}
		}
		if len(local) > 0 && s.rng.Float64() < 0.8 {
			market = pgtype.Int2{Int16: local[s.rng.IntN(len(local))].ID, Valid: true}
		}
		c := 1 + s.rng.IntN(3)
		chosen := map[string]bool{}
		for len(chosen) < c {
			chosen[pick(s.rng, catList)] = true
		}
		var sc []string
		for k := range chosen {
			sc = append(sc, k)
		}

		var id pgtype.UUID
		err := s.pool.QueryRow(ctx, `
			INSERT INTO users (telegram_id, display_name, public_name, phone_enc, phone_hash, role, role_approved,
			                   region_id, market_id, categories, consent_at, contact_telegram, contact_phone, bot_enabled,
			                   is_demo, created_at)
			VALUES ($1, $2, $3, $4, $5, 'entrepreneur', true, $6, $7, $8, now(), false, true, false, true, now() - $9::interval)
			RETURNING id`,
			-int64(100000+i), "Demo sotuvchi "+fmt.Sprint(i+1), name, s.sealer.Seal(phone), s.sealer.Hash(phone),
			r.ID, market, sc, fmt.Sprintf("%d days", 5+s.rng.IntN(300)),
		).Scan(&id)
		if err != nil {
			return nil, fmt.Errorf("create seller: %w", err)
		}
		u, err := s.q.GetUser(ctx, id.Bytes)
		if err != nil {
			return nil, err
		}
		out = append(out, seller{user: u, categories: sc})
	}
	slog.Info("created demo sellers", "count", len(out))
	return out, nil
}

func (s *seeder) createListings(ctx context.Context, n int, sellers []seller, products []product, images map[string][]media.Photo) error {
	byCat := map[string][]product{}
	for _, p := range products {
		byCat[p.Category] = append(byCat[p.Category], p)
	}
	// A stable price level per province makes regional differences visible.
	provinceFactor := map[int16]float64{}
	created, skipped := 0, 0
	for attempts := 0; created < n && attempts < n*5; attempts++ {
		sl := sellers[s.rng.IntN(len(sellers))]
		cands := byCat[pick(s.rng, sl.categories)]
		if len(cands) == 0 {
			continue
		}
		p := cands[s.rng.IntN(len(cands))]
		if p.OffSeason && s.rng.Float64() < 0.8 {
			continue
		}
		region, err := s.q.GetRegion(ctx, sl.user.RegionID.Int16)
		if err != nil {
			return err
		}
		pf, ok := provinceFactor[region.ParentID.Int16]
		if !ok {
			pf = 0.9 + s.rng.Float64()*0.22
			provinceFactor[region.ParentID.Int16] = pf
		}

		trade := "ulgurji"
		price := p.Price * pf * (1 + s.rng.NormFloat64()*0.07)
		if s.rng.Float64() < 0.25 {
			trade = "chakana"
			markup := p.Markup
			if markup == 0 {
				markup = categoryMarkup[p.Category]
			}
			price *= 1 + markup*(0.8+s.rng.Float64()*0.4)
		}
		in := market.Input{TradeType: trade, ProductID: p.ID, Description: pick(s.rng, descriptions[p.Category])}
		if len(p.Varieties) > 0 && s.rng.Float64() < 0.7 {
			vr := pick(s.rng, p.Varieties)
			in.Variety, price = vr.Name, price*vr.Factor
		}
		in.PriceSom = roundPrice(price)
		if trade == "ulgurji" && s.rng.Float64() < 0.7 {
			in.MinOrder = ptr(roundQty(p.Price, 200_000+s.rng.Float64()*2_000_000))
		}
		if s.rng.Float64() < 0.6 {
			in.Stock = ptr(roundQty(p.Price, 5_000_000+s.rng.Float64()*80_000_000))
		}
		saved, err := s.svc.Upsert(ctx, sl.user, in)
		if err != nil {
			return fmt.Errorf("listing %s: %w", p.Slug, err)
		}
		if !saved.Created {
			skipped++
			continue
		}
		// Spread listings over the last six days so "new" and expiry look natural.
		age := time.Duration(s.rng.Float64()*6*24) * time.Hour
		if _, err := s.pool.Exec(ctx, `
			WITH l AS (
				UPDATE listings SET created_at = now() - $2::interval, updated_at = now() - $2::interval,
				       valid_until = now() - $2::interval + interval '7 days', views = $3
				WHERE id = $1 RETURNING id, valid_until, updated_at)
			UPDATE price_observations o SET observed_at = l.updated_at, valid_until = l.valid_until
			FROM l WHERE o.listing_id = l.id AND o.status IN ('active', 'review')`,
			saved.ID, fmt.Sprintf("%d seconds", int(age.Seconds())), s.rng.IntN(250)); err != nil {
			return err
		}
		if pool := images[p.Slug]; len(pool) > 0 {
			k := 1 + s.rng.IntN(min(3, len(pool)))
			for _, idx := range s.rng.Perm(len(pool))[:k] {
				ph := pool[idx]
				ph.Key = secure.Token(24) + ".jpg"
				if _, err := s.svc.AddPhoto(ctx, s.store, sl.user, saved.ID, ph); err != nil {
					return fmt.Errorf("attach photo: %w", err)
				}
			}
		}
		created++
		if created%100 == 0 {
			slog.Info("listings", "created", created)
		}
	}
	var review int
	_ = s.pool.QueryRow(ctx, `SELECT count(*) FROM listings l JOIN users u ON u.id = l.seller_id WHERE u.is_demo AND l.status = 'review'`).Scan(&review)
	slog.Info("created demo listings", "count", created, "duplicates_skipped", skipped, "held_for_review", review)
	return nil
}

// roundPrice rounds to steps a seller would quote: 50, 100, 500 or 1000 so'm.
func roundPrice(p float64) int64 {
	step := 50.0
	switch {
	case p >= 1_000_000:
		step = 10_000
	case p >= 100_000:
		step = 1000
	case p >= 20_000:
		step = 500
	case p >= 5000:
		step = 100
	}
	return int64(math.Max(step, math.Round(p/step)*step))
}

// roundQty turns a money volume into a round quantity of the product.
func roundQty(unitPrice, som float64) float64 {
	q := som / unitPrice
	mag := math.Pow(10, math.Floor(math.Log10(math.Max(q, 1))))
	return math.Max(1, math.Round(q/mag)*mag)
}

func pick[T any](r *rand.Rand, xs []T) T { return xs[r.IntN(len(xs))] }
func ptr[T any](v T) *T                  { return &v }

// ---- images ----

func fetch(ctx context.Context, c *http.Client, u string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "bugun-demo-seed/1.0")
	res, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s", res.Status)
	}
	return io.ReadAll(io.LimitReader(res.Body, media.DefaultLimits.MaxBytes))
}

// dropImages removes unsuitable cached images: from the cache (so later seeds
// skip them) and from every demo listing that uses them. Seeded photos are the
// cached image re-encoded deterministically, so identical bytes identify them.
func (s *seeder) dropImages(ctx context.Context, cacheDir string, names []string) error {
	bad := map[[32]byte]string{}
	for _, n := range names {
		n = strings.TrimSpace(n)
		path := filepath.Join(cacheDir, filepath.Clean("/"+n))
		b, err := os.ReadFile(path)
		if err != nil {
			slog.Warn("drop: not found", "image", n)
			continue
		}
		ph, err := media.Process(bytes.NewReader(b))
		if err == nil {
			bad[sha256.Sum256(ph.Data)] = n
		}
		if err := os.Remove(path); err != nil {
			return err
		}
	}
	rows, err := s.pool.Query(ctx, `SELECT ph.id, ph.storage_key FROM listing_photos ph
		JOIN listings l ON l.id = ph.listing_id JOIN users u ON u.id = l.seller_id WHERE u.is_demo`)
	if err != nil {
		return err
	}
	type photo struct {
		id  int64
		key string
	}
	var all []photo
	for rows.Next() {
		var p photo
		if err := rows.Scan(&p.id, &p.key); err != nil {
			return err
		}
		all = append(all, p)
	}
	rows.Close()
	removed := 0
	for _, p := range all {
		rc, err := s.store.Open(ctx, p.key)
		if err != nil {
			continue
		}
		b, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			continue
		}
		if _, ok := bad[sha256.Sum256(b)]; !ok {
			continue
		}
		if _, err := s.pool.Exec(ctx, `DELETE FROM listing_photos WHERE id = $1`, p.id); err != nil {
			return err
		}
		_ = media.Remove(ctx, s.store, p.key)
		removed++
	}
	slog.Info("dropped images", "cache_files", len(bad), "listing_photos", removed)
	return nil
}
