-- Watches (QIZIQISH_VA_SAVDO.md step 8).

-- name: CountWatches :one
SELECT count(*)::int FROM watches WHERE user_id = $1;

-- Adding the same watch again only changes its price limit.
-- name: UpsertWatch :one
INSERT INTO watches (user_id, product_id, province_id, trade_type, max_price, peak_median)
VALUES (@user_id, @product_id, sqlc.narg(province_id), @trade_type, sqlc.narg(max_price), sqlc.narg(peak_median))
ON CONFLICT (user_id, product_id, trade_type, COALESCE(province_id, 0)) DO UPDATE SET max_price = EXCLUDED.max_price
RETURNING *;

-- name: DeleteWatch :execrows
DELETE FROM watches WHERE id = $1 AND user_id = $2;

-- name: ListWatches :many
SELECT w.*, p.name AS product_name, p.slug AS product_slug, p.base_unit AS unit,
       COALESCE(r.name, '')::text AS province_name
FROM watches w
JOIN products p ON p.id = w.product_id
LEFT JOIN regions r ON r.id = w.province_id
WHERE w.user_id = $1
ORDER BY w.created_at DESC;

-- Watches whose owner has the bot on.
-- name: WatchesToCheck :many
SELECT w.*, u.telegram_id, p.name AS product_name, p.slug AS product_slug, p.base_unit AS unit,
       COALESCE(r.name, '')::text AS province_name
FROM watches w
JOIN users u ON u.id = w.user_id
JOIN products p ON p.id = w.product_id
LEFT JOIN regions r ON r.id = w.province_id
WHERE u.bot_enabled
ORDER BY w.id;

-- The live weighted median of a product in a viloyat (NULL = country).
-- name: WatchMedian :one
SELECT count(*)::int AS offers,
       COALESCE(weighted_quantile(array_agg(o.price_som::float8 ORDER BY o.id),
                                  array_agg(price_weight(o.source, o.observed_at) ORDER BY o.id), 0.5), 0)::float8 AS median
FROM price_observations o
JOIN regions r ON r.id = o.region_id
WHERE o.product_id = @product_id AND o.trade_type = @trade_type AND o.status = 'active' AND o.valid_until > now()
  AND (sqlc.narg(province_id)::smallint IS NULL OR COALESCE(r.parent_id, r.id) = sqlc.narg(province_id))
  AND (o.listing_id IS NULL OR EXISTS (SELECT 1 FROM listings x WHERE x.id = o.listing_id AND x.status = 'active'));

-- The cheapest listing price set since the last check, at or under a limit.
-- name: CheapestNewListing :one
SELECT l.id, l.price_som, rr.name AS region_name
FROM price_observations o
JOIN listings l ON l.id = o.listing_id AND l.status = 'active' AND l.valid_until > now()
JOIN regions rr ON rr.id = l.region_id
WHERE o.product_id = @product_id AND o.trade_type = @trade_type AND o.status = 'active'
  AND o.observed_at > @since AND o.price_som <= @max_price::bigint AND l.seller_id <> @user_id
  AND (sqlc.narg(province_id)::smallint IS NULL OR COALESCE(rr.parent_id, rr.id) = sqlc.narg(province_id))
ORDER BY o.price_som, o.observed_at DESC
LIMIT 1;

-- name: MarkWatchChecked :exec
UPDATE watches SET checked_at = now(), peak_median = sqlc.narg(peak_median),
       notified_at = CASE WHEN @notified::bool THEN now() ELSE notified_at END
WHERE id = @id;
