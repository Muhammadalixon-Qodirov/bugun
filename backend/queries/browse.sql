-- Public catalog. Only active, unexpired prices count; hidden, rejected or
-- under-review listings never reach a median. Wholesale and retail are never mixed.
-- Medians and quartiles are weighted by source trust and freshness (spec §10.1,
-- price_weight and weighted_quantile in migration 00012).

-- name: BrowseProducts :many
WITH obs AS (
    SELECT o.id, o.product_id, o.price_som, o.listing_id, o.source, o.observed_at
    FROM price_observations o
    JOIN regions r ON r.id = o.region_id
    WHERE o.status = 'active' AND o.valid_until > now() AND o.trade_type = @trade_type
      AND (sqlc.narg(region_id)::smallint IS NULL OR r.id = sqlc.narg(region_id) OR r.parent_id = sqlc.narg(region_id))
      AND (o.listing_id IS NULL OR EXISTS (SELECT 1 FROM listings l WHERE l.id = o.listing_id AND l.status = 'active'))
), stats AS (
    SELECT product_id,
           count(*)::int AS offers,
           weighted_quantile(array_agg(price_som::float8 ORDER BY id), array_agg(price_weight(source, observed_at) ORDER BY id), 0.5)::float8 AS median,
           min(price_som)::bigint AS min_price,
           max(price_som)::bigint AS max_price
    FROM obs GROUP BY product_id
)
SELECT p.id, p.slug, p.name, p.category, p.base_unit,
       COALESCE(s.offers, 0)::int AS offers,
       COALESCE(s.median, 0)::float8 AS median,
       COALESCE(s.min_price, 0)::bigint AS min_price,
       COALESCE(s.max_price, 0)::bigint AS max_price,
       COALESCE((SELECT ph.storage_key FROM listing_photos ph
                 JOIN obs ON obs.listing_id = ph.listing_id AND obs.product_id = p.id
                 ORDER BY ph.created_at DESC LIMIT 1), '')::text AS cover_key
FROM products p
LEFT JOIN stats s ON s.product_id = p.id
WHERE (sqlc.narg(category)::text IS NULL OR p.category = sqlc.narg(category))
  AND (sqlc.narg(query)::text IS NULL
       OR p.name ILIKE '%' || sqlc.narg(query) || '%'
       OR EXISTS (SELECT 1 FROM unnest(p.synonyms) syn WHERE syn ILIKE '%' || sqlc.narg(query) || '%'))
  AND (NOT @only_offers::bool OR s.offers > 0)
ORDER BY
    CASE WHEN @sort::text = 'price_asc'  THEN COALESCE(s.median, 1e18) END ASC,
    CASE WHEN @sort::text = 'price_desc' THEN COALESCE(s.median, -1) END DESC,
    CASE WHEN @sort::text = 'name' THEN p.name END ASC,
    COALESCE(s.offers, 0) DESC, p.name;


-- name: ProductStats :one
SELECT count(*)::int AS offers,
       COALESCE(weighted_quantile(array_agg(o.price_som::float8 ORDER BY o.id), array_agg(price_weight(o.source, o.observed_at) ORDER BY o.id), 0.5), 0)::float8 AS median,
       COALESCE(weighted_quantile(array_agg(o.price_som::float8 ORDER BY o.id), array_agg(price_weight(o.source, o.observed_at) ORDER BY o.id), 0.25), 0)::float8 AS p25,
       COALESCE(weighted_quantile(array_agg(o.price_som::float8 ORDER BY o.id), array_agg(price_weight(o.source, o.observed_at) ORDER BY o.id), 0.75), 0)::float8 AS p75,
       COALESCE(min(o.price_som), 0)::bigint AS min_price,
       COALESCE(max(o.price_som), 0)::bigint AS max_price,
       COALESCE(min(o.observed_at), now())::timestamptz AS first_seen,
       COALESCE(max(o.observed_at), now())::timestamptz AS last_seen
FROM price_observations o
JOIN regions r ON r.id = o.region_id
WHERE o.product_id = @product_id AND o.trade_type = @trade_type
  AND o.status = 'active' AND o.valid_until > now()
  AND (sqlc.narg(region_id)::smallint IS NULL OR r.id = sqlc.narg(region_id) OR r.parent_id = sqlc.narg(region_id))
  AND (o.listing_id IS NULL OR EXISTS (SELECT 1 FROM listings l WHERE l.id = o.listing_id AND l.status = 'active'));

-- name: ProductProvinceMedians :many
SELECT COALESCE(r.parent_id, r.id)::smallint AS province_id,
       count(*)::int AS offers,
       weighted_quantile(array_agg(o.price_som::float8 ORDER BY o.id), array_agg(price_weight(o.source, o.observed_at) ORDER BY o.id), 0.5)::float8 AS median
FROM price_observations o
JOIN regions r ON r.id = o.region_id
WHERE o.product_id = @product_id AND o.trade_type = @trade_type
  AND o.status = 'active' AND o.valid_until > now()
  AND (o.listing_id IS NULL OR EXISTS (SELECT 1 FROM listings l WHERE l.id = o.listing_id AND l.status = 'active'))
GROUP BY 1
ORDER BY median;

-- name: ProductListings :many
SELECT l.id, l.seller_id, l.variety, l.description, l.price_som, l.unit, l.min_order, l.stock, l.trade_type,
       l.updated_at, l.valid_until, l.region_id,
       r.name AS region_name,
       COALESCE(pr.name, '')::text AS province_name,
       COALESCE(m.name, '')::text AS market_name,
       COALESCE(NULLIF(u.public_name, ''), split_part(u.display_name, ' ', 1))::text AS seller_name,
       (u.contact_telegram AND u.telegram_username IS NOT NULL)::bool AS has_telegram,
       (u.contact_phone AND u.phone_enc IS NOT NULL)::bool AS has_phone,
       COALESCE((SELECT array_agg(ph.storage_key ORDER BY ph.position, ph.id)
                 FROM listing_photos ph WHERE ph.listing_id = l.id), '{}')::text[] AS photo_keys
FROM listings l
JOIN users u ON u.id = l.seller_id
JOIN regions r ON r.id = l.region_id
LEFT JOIN regions pr ON pr.id = r.parent_id
LEFT JOIN markets m ON m.id = l.market_id
WHERE l.product_id = @product_id AND l.status = 'active' AND l.valid_until > now()
  AND (sqlc.narg(trade_type)::text IS NULL OR l.trade_type = sqlc.narg(trade_type))
  AND (sqlc.narg(region_id)::smallint IS NULL OR r.id = sqlc.narg(region_id) OR r.parent_id = sqlc.narg(region_id))
ORDER BY l.price_som, l.updated_at DESC
LIMIT 100;

-- name: ListingContact :one
SELECT l.id, l.seller_id, l.status, l.product_id, l.region_id, u.telegram_username, u.contact_telegram, u.contact_phone, u.phone_enc
FROM listings l JOIN users u ON u.id = l.seller_id
WHERE l.id = $1;

-- name: FeedListings :many
-- Public listing feed. Median per product and trade type (within the region
-- scope, at least 3 prices) lets cards show "x% below market".
WITH med AS (
    SELECT o.product_id, o.trade_type,
           weighted_quantile(array_agg(o.price_som::float8 ORDER BY o.id), array_agg(price_weight(o.source, o.observed_at) ORDER BY o.id), 0.5)::float8 AS median
    FROM price_observations o
    JOIN regions r ON r.id = o.region_id
    WHERE o.status = 'active' AND o.valid_until > now()
      AND (sqlc.narg(region_id)::smallint IS NULL OR r.id = sqlc.narg(region_id) OR r.parent_id = sqlc.narg(region_id))
      AND (o.listing_id IS NULL OR EXISTS (SELECT 1 FROM listings x WHERE x.id = o.listing_id AND x.status = 'active'))
    GROUP BY o.product_id, o.trade_type
    HAVING count(*) >= 3
)
SELECT l.id, l.seller_id, l.product_id, p.slug AS product_slug, p.name AS product_name, p.category,
       l.variety, l.price_som, l.unit, l.trade_type, l.min_order, l.created_at, l.updated_at, l.views,
       r.name AS region_name, COALESCE(pr.name, '')::text AS province_name,
       COALESCE(NULLIF(u.public_name, ''), split_part(u.display_name, ' ', 1))::text AS seller_name,
       COALESCE(med.median, 0)::float8 AS median,
       COALESCE((SELECT ph.storage_key FROM listing_photos ph WHERE ph.listing_id = l.id ORDER BY ph.position, ph.id LIMIT 1), '')::text AS cover_key,
       (SELECT count(*) FROM listing_photos ph WHERE ph.listing_id = l.id)::int AS photo_count,
       count(*) OVER ()::int AS total
FROM listings l
JOIN products p ON p.id = l.product_id
JOIN users u ON u.id = l.seller_id
JOIN regions r ON r.id = l.region_id
LEFT JOIN regions pr ON pr.id = r.parent_id
LEFT JOIN med ON med.product_id = l.product_id AND med.trade_type = l.trade_type
WHERE l.status = 'active' AND l.valid_until > now()
  AND (sqlc.narg(region_id)::smallint IS NULL OR r.id = sqlc.narg(region_id) OR r.parent_id = sqlc.narg(region_id))
  AND (sqlc.narg(category)::text IS NULL OR p.category = sqlc.narg(category))
  AND (sqlc.narg(product)::text IS NULL OR p.slug = sqlc.narg(product))
  AND (sqlc.narg(trade_type)::text IS NULL OR l.trade_type = sqlc.narg(trade_type))
  AND (sqlc.narg(min_price)::bigint IS NULL OR l.price_som >= sqlc.narg(min_price))
  AND (sqlc.narg(max_price)::bigint IS NULL OR l.price_som <= sqlc.narg(max_price))
  AND (sqlc.narg(seller_id)::uuid IS NULL OR l.seller_id = sqlc.narg(seller_id))
  AND (sqlc.narg(exclude_id)::bigint IS NULL OR l.id <> sqlc.narg(exclude_id))
  AND (sqlc.narg(query)::text IS NULL
       OR p.name ILIKE '%' || sqlc.narg(query) || '%'
       OR l.variety ILIKE '%' || sqlc.narg(query) || '%'
       OR EXISTS (SELECT 1 FROM unnest(p.synonyms) syn WHERE syn ILIKE '%' || sqlc.narg(query) || '%'))
ORDER BY
    CASE WHEN @sort::text = 'price_asc' THEN l.price_som END ASC,
    CASE WHEN @sort::text = 'price_desc' THEN l.price_som END DESC,
    CASE WHEN @sort::text = 'popular' THEN l.views END DESC,
    CASE WHEN @sort::text = 'deal' THEN COALESCE(l.price_som / NULLIF(med.median, 0), 9) END ASC,
    l.created_at DESC, l.id DESC
LIMIT @lim OFFSET @off;

-- name: PublicListing :one
SELECT l.*, p.slug AS product_slug, p.name AS product_name, p.category, p.base_unit,
       r.name AS region_name, COALESCE(pr.name, '')::text AS province_name, COALESCE(m.name, '')::text AS market_name,
       COALESCE(NULLIF(u.public_name, ''), split_part(u.display_name, ' ', 1))::text AS seller_name,
       u.created_at AS seller_since,
       (u.contact_telegram AND u.telegram_username IS NOT NULL)::bool AS has_telegram,
       (u.contact_phone AND u.phone_enc IS NOT NULL)::bool AS has_phone,
       (SELECT count(*) FROM listings x WHERE x.seller_id = l.seller_id AND x.status = 'active' AND x.valid_until > now())::int AS seller_listings
FROM listings l
JOIN products p ON p.id = l.product_id
JOIN users u ON u.id = l.seller_id
JOIN regions r ON r.id = l.region_id
LEFT JOIN regions pr ON pr.id = r.parent_id
LEFT JOIN markets m ON m.id = l.market_id
WHERE l.id = $1;

-- name: IncrementListingView :exec
UPDATE listings SET views = views + 1 WHERE id = $1;
