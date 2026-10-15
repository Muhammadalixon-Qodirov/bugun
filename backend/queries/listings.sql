-- name: UpsertListing :one
-- Creates the listing or, if the seller already has one for this product,
-- variety and trade type, updates it. xmax = 0 tells the two cases apart.
INSERT INTO listings (seller_id, product_id, variety, description, price_som, unit, min_order, stock,
                      trade_type, region_id, market_id, status, valid_until, review_reason)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
ON CONFLICT (seller_id, product_id, variety, trade_type) DO UPDATE SET
    description = CASE WHEN EXCLUDED.description <> '' THEN EXCLUDED.description ELSE listings.description END,
    price_som   = EXCLUDED.price_som,
    unit        = EXCLUDED.unit,
    min_order   = COALESCE(EXCLUDED.min_order, listings.min_order),
    stock       = COALESCE(EXCLUDED.stock, listings.stock),
    region_id   = EXCLUDED.region_id,
    market_id   = EXCLUDED.market_id,
    status      = CASE WHEN listings.status = 'rejected' THEN 'rejected' ELSE EXCLUDED.status END,
    review_reason = CASE WHEN listings.status = 'rejected' THEN listings.review_reason ELSE EXCLUDED.review_reason END,
    manual_price_at = CASE WHEN listings.price_som <> EXCLUDED.price_som THEN now() ELSE listings.manual_price_at END,
    valid_until = EXCLUDED.valid_until,
    updated_at  = now()
RETURNING id, (xmax = 0) AS created;

-- name: UpdateListing :one
UPDATE listings SET
    manual_price_at = CASE WHEN price_som <> $5 THEN now() ELSE manual_price_at END,
    variety     = $3,
    description = $4,
    price_som   = $5,
    min_order   = $6,
    stock       = $7,
    trade_type  = $8,
    status      = CASE WHEN status = 'rejected' THEN 'rejected' ELSE $9 END,
    review_reason = CASE WHEN status = 'rejected' THEN review_reason ELSE sqlc.narg(review_reason) END,
    valid_until = $10,
    updated_at  = now()
WHERE id = $1 AND seller_id = $2
RETURNING *;

-- name: GetListing :one
SELECT * FROM listings WHERE id = $1;

-- name: GetListingForUpdate :one
SELECT * FROM listings WHERE id = $1 AND seller_id = $2 FOR UPDATE;

-- name: SetListingStatus :exec
-- Hide or show: only between active and hidden. A listing held for review or
-- rejected by an admin keeps its status, so showing it can't skip the review.
UPDATE listings SET status = $3, updated_at = now() WHERE id = $1 AND seller_id = $2 AND status IN ('active', 'hidden');

-- name: RefreshListing :one
UPDATE listings SET valid_until = $3, updated_at = now()
WHERE id = $1 AND seller_id = $2
RETURNING valid_until;

-- name: RefreshDueListings :many
-- Renews every live listing of the seller that expires before due_before.
UPDATE listings SET valid_until = sqlc.arg(valid_until), updated_at = now()
WHERE seller_id = sqlc.arg(seller_id) AND status = 'active' AND valid_until < sqlc.arg(due_before)
RETURNING id;

-- name: SetListingsObservationValidity :exec
UPDATE price_observations SET valid_until = sqlc.arg(valid_until)
WHERE listing_id = ANY(sqlc.arg(listing_ids)::bigint[]) AND status = 'active';

-- name: DeleteListing :exec
DELETE FROM listings WHERE id = $1 AND seller_id = $2;

-- name: ListSellerListings :many
-- The seller's listings with the market median for the same product and trade
-- type: in the listing's viloyat when it has at least 3 live prices, else
-- nationwide (same rules as the public feed).
WITH live AS (
    SELECT o.id, o.product_id, o.trade_type, o.price_som, o.source, o.observed_at, COALESCE(r.parent_id, r.id) AS province_id
    FROM price_observations o
    JOIN regions r ON r.id = o.region_id
    WHERE o.status = 'active' AND o.valid_until > now()
      AND (o.listing_id IS NULL OR EXISTS (SELECT 1 FROM listings x WHERE x.id = o.listing_id AND x.status = 'active'))
), province_median AS (
    SELECT product_id, trade_type, province_id,
           weighted_quantile(array_agg(price_som::float8 ORDER BY id), array_agg(price_weight(source, observed_at) ORDER BY id), 0.5)::float8 AS median
    FROM live GROUP BY product_id, trade_type, province_id HAVING count(*) >= 3
), country_median AS (
    SELECT product_id, trade_type,
           weighted_quantile(array_agg(price_som::float8 ORDER BY id), array_agg(price_weight(source, observed_at) ORDER BY id), 0.5)::float8 AS median
    FROM live GROUP BY product_id, trade_type HAVING count(*) >= 3
)
SELECT l.*, p.name AS product_name, p.slug AS product_slug, p.category AS product_category,
       COALESCE((SELECT ph.storage_key FROM listing_photos ph WHERE ph.listing_id = l.id ORDER BY ph.position, ph.id LIMIT 1), '')::text AS cover_key,
       COALESCE(pm.median, cm.median, 0)::float8 AS median,
       (pm.median IS NOT NULL)::bool AS median_in_province
FROM listings l
JOIN products p ON p.id = l.product_id
JOIN regions lr ON lr.id = l.region_id
LEFT JOIN province_median pm ON pm.product_id = l.product_id AND pm.trade_type = l.trade_type
                            AND pm.province_id = COALESCE(lr.parent_id, lr.id)
LEFT JOIN country_median cm ON cm.product_id = l.product_id AND cm.trade_type = l.trade_type
WHERE l.seller_id = $1
ORDER BY l.updated_at DESC;

-- name: ExpireListingObservation :exec
UPDATE price_observations
SET status = 'expired', valid_until = LEAST(valid_until, now())
WHERE listing_id = $1 AND status = 'active';

-- name: InsertListingObservation :exec
INSERT INTO price_observations (
    product_id, price_som, unit, quantity, trade_type, region_id, market_id,
    source, seller_id, listing_id, valid_until, raw_text, status
) VALUES ($1, $2, $3, $4, $5, $6, $7, @source, $8, $9, $10, $11, $12);

-- name: SetListingObservationValidity :exec
UPDATE price_observations SET valid_until = $2 WHERE listing_id = $1 AND status = 'active';

-- name: CountListingPhotos :one
SELECT count(*) FROM listing_photos WHERE listing_id = $1;

-- name: AddListingPhoto :one
INSERT INTO listing_photos (listing_id, storage_key, width, height, position)
VALUES ($1, $2, $3, $4, (SELECT COALESCE(max(position) + 1, 0) FROM listing_photos WHERE listing_id = $1))
RETURNING *;

-- name: ListListingPhotos :many
SELECT * FROM listing_photos WHERE listing_id = $1 ORDER BY position, id;

-- Every photo of the seller's listings, to remove the files with the account.
-- name: ListSellerPhotoKeys :many
SELECT ph.storage_key FROM listing_photos ph
JOIN listings l ON l.id = ph.listing_id
WHERE l.seller_id = $1;

-- name: DeleteListingPhoto :one
DELETE FROM listing_photos ph
USING listings l
WHERE ph.id = $1 AND ph.listing_id = l.id AND l.seller_id = $2
RETURNING ph.storage_key;

-- name: SetContactPrefs :exec
UPDATE users SET contact_telegram = $2, contact_phone = $3, updated_at = now() WHERE id = $1;

-- name: LogContactReveal :exec
INSERT INTO contact_reveals (listing_id, viewer_id) VALUES ($1, $2);

-- Other sellers' current prices for the same product in the same province with
-- their median weight (spec §10.1): the price screen uses the prices, the
-- market position their weighted median. Like the public medians, a hidden
-- listing's price does not count.
-- name: PeerPrices :many
SELECT o.price_som::float8 AS price, price_weight(o.source, o.observed_at)::float8 AS weight
FROM price_observations o
JOIN regions r ON r.id = o.region_id
WHERE o.product_id = $1 AND o.trade_type = $2 AND o.status = 'active' AND o.valid_until > now()
  AND COALESCE(r.parent_id, r.id) = $3
  AND (o.listing_id IS NULL OR EXISTS (SELECT 1 FROM listings x WHERE x.id = o.listing_id AND x.status = 'active'))
  AND (sqlc.narg(exclude_listing)::bigint IS NULL OR o.listing_id IS DISTINCT FROM sqlc.narg(exclude_listing));

-- The seller's existing listing for this product, variety and trade type, so a
-- new bot price is compared with their last one and not with itself.
-- name: SellerListingID :one
SELECT id FROM listings
WHERE seller_id = $1 AND product_id = $2 AND variety = $3 AND trade_type = $4;

-- The listing's last price that passed the checks (live or since expired), 0
-- when there is none. A held typo is skipped, so correcting it is not held too.
-- name: LastAcceptedPrice :one
SELECT COALESCE((
    SELECT price_som FROM price_observations
    WHERE listing_id = $1 AND status IN ('active', 'expired')
    ORDER BY observed_at DESC, id DESC
    LIMIT 1
), 0)::bigint AS price;

-- Official benchmark (UzEx, narxtahlil) from the last two weeks: the viloyat's
-- median when it has one, otherwise the national median, 0 when there is none.
-- Benchmarks come from reference_prices (the connectors; region NULL is
-- national) and from benchmark rows in price_observations.
-- name: ReferencePrice :one
WITH ref AS (
    SELECT o.price_som::float8 AS price, COALESCE(r.parent_id, r.id) AS province_id
    FROM price_observations o
    JOIN regions r ON r.id = o.region_id
    WHERE o.product_id = sqlc.arg(product_id) AND o.trade_type = sqlc.arg(trade_type)
      AND o.source IN ('uzex', 'narxtahlil') AND o.status <> 'rejected'
      AND o.observed_at > now() - interval '14 days'
    UNION ALL
    SELECT rp.price_som::float8, COALESCE(r.parent_id, r.id)
    FROM reference_prices rp
    LEFT JOIN regions r ON r.id = rp.region_id
    WHERE rp.product_id = sqlc.arg(product_id) AND rp.trade_type = sqlc.arg(trade_type)
      AND rp.quoted_on > (now() AT TIME ZONE 'Asia/Tashkent')::date - 14
)
SELECT COALESCE(
           percentile_cont(0.5) WITHIN GROUP (ORDER BY price) FILTER (WHERE province_id = sqlc.arg(province_id)::smallint),
           percentile_cont(0.5) WITHIN GROUP (ORDER BY price),
           0)::float8 AS price
FROM ref;

-- Pricing settings of one of the seller's listings (DINAMIK_NARX.md).
-- name: SetListingPricing :one
UPDATE listings SET pricing_mode = @mode, pricing_goal = @goal,
       price_min = sqlc.narg(price_min), price_max = sqlc.narg(price_max), updated_at = now()
WHERE id = @id AND seller_id = @seller_id
RETURNING *;

-- The listing a stored photo belongs to, to decide who may see it.
-- name: PhotoListing :one
SELECT l.id, l.seller_id, l.status
FROM listing_photos p
JOIN listings l ON l.id = p.listing_id
WHERE p.storage_key = $1;
