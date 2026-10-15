-- Interest and deal funnels (QIZIQISH_VA_SAVDO.md step 6). A window of N days
-- is the last N Tashkent days including today.

-- name: ListingFunnelDays :many
WITH days AS (
    SELECT ((now() AT TIME ZONE 'Asia/Tashkent')::date + 1 - n)::date AS day
    FROM generate_series(1, sqlc.arg(num_days)::int) AS n
)
SELECT days.day,
       count(e.id) FILTER (WHERE e.kind = 'view')::int AS views,
       count(e.id) FILTER (WHERE e.kind = 'contact')::int AS contacts
FROM days
LEFT JOIN listing_events e ON e.listing_id = @listing_id AND e.day = days.day
GROUP BY days.day
ORDER BY days.day;

-- name: ListingDealTotals :one
SELECT count(*) FILTER (WHERE status = 'confirmed')::int AS confirmed,
       count(*) FILTER (WHERE status = 'reported')::int AS reported,
       COALESCE(sum(quantity) FILTER (WHERE status IN ('confirmed', 'reported')), 0)::float8 AS quantity
FROM deals
WHERE listing_id = @listing_id AND day > (now() AT TIME ZONE 'Asia/Tashkent')::date - @num_days::int;

-- Other live listings of the product in the viloyat, for comparison.
-- name: MarketFunnel :one
WITH peers AS (
    SELECT o.id FROM listings o
    JOIN regions r ON r.id = o.region_id
    WHERE o.product_id = @product_id AND o.trade_type = @trade_type AND o.id <> @listing_id
      AND o.status = 'active' AND COALESCE(r.parent_id, r.id) = @province_id::smallint
)
SELECT (SELECT count(*) FROM peers)::int AS listings,
       count(e.id) FILTER (WHERE e.kind = 'view')::int AS views,
       count(e.id) FILTER (WHERE e.kind = 'contact')::int AS contacts
FROM listing_events e
WHERE e.listing_id IN (SELECT id FROM peers)
  AND e.day > (now() AT TIME ZONE 'Asia/Tashkent')::date - @num_days::int;

-- The last 7 days of every listing of a seller, for "Mahsulotlarim".
-- name: SellerFunnels :many
SELECT l.id AS listing_id,
       (SELECT count(*) FROM listing_events e
        WHERE e.listing_id = l.id AND e.kind = 'view' AND e.day > (now() AT TIME ZONE 'Asia/Tashkent')::date - 7)::int AS views,
       (SELECT count(*) FROM listing_events e
        WHERE e.listing_id = l.id AND e.kind = 'contact' AND e.day > (now() AT TIME ZONE 'Asia/Tashkent')::date - 7)::int AS contacts,
       (SELECT count(*) FROM deals d
        WHERE d.listing_id = l.id AND d.status IN ('confirmed', 'reported') AND d.day > (now() AT TIME ZONE 'Asia/Tashkent')::date - 7)::int AS deals
FROM listings l
WHERE l.seller_id = $1;

-- Public interest in a product: distinct visitors today (any demand signal;
-- the visitor key is per day) and confirmed deals in the last 7 days. A
-- province of NULL means the whole country.
-- name: ProductInterest :one
SELECT (SELECT count(DISTINCT e.viewer_key) FROM demand_events e
        WHERE e.product_id = @product_id AND e.day = (now() AT TIME ZONE 'Asia/Tashkent')::date
          AND (sqlc.narg(province_id)::smallint IS NULL OR e.province_id = sqlc.narg(province_id)))::int AS visitors_today,
       count(d.id)::int AS deals,
       count(d.price_som)::int AS priced,
       COALESCE(percentile_cont(0.5) WITHIN GROUP (ORDER BY d.price_som), 0)::float8 AS median
FROM deals d
JOIN regions r ON r.id = d.region_id
WHERE d.product_id = @product_id AND d.status = 'confirmed' AND d.created_at > now() - interval '7 days'
  AND (sqlc.narg(province_id)::smallint IS NULL OR COALESCE(r.parent_id, r.id) = sqlc.narg(province_id));

-- name: AdminDeals :many
SELECT d.id, d.status, d.source, d.quantity, d.unit, d.price_som, d.created_at, d.listing_id,
       (d.buyer_id IS NOT NULL)::bool AS has_buyer, d.buyer_confirmed_at, d.seller_confirmed_at,
       p.name AS product_name, s.display_name AS seller_name, r.name AS region_name
FROM deals d
JOIN products p ON p.id = d.product_id
JOIN users s ON s.id = d.seller_id
JOIN regions r ON r.id = d.region_id
WHERE (sqlc.narg(status)::text IS NULL OR d.status = sqlc.narg(status))
ORDER BY d.created_at DESC, d.id DESC
LIMIT 100;
