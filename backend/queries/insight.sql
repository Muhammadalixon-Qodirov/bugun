-- Product analytics (TAHLIL_REJA.md).

-- Generated history stays; only rows built from real prices are rebuilt.
-- name: DeletePriceDaily :exec
DELETE FROM price_daily WHERE day BETWEEN sqlc.arg(from_day)::date AND sqlc.arg(to_day)::date AND NOT synthetic;

-- Rebuilds the daily history for days from..to (Tashkent) from real prices.
-- Held and rejected prices and official benchmarks are left out, and a
-- viloyat-day needs at least 3 prices (like any shown median); such a day
-- replaces generated history.
-- name: BuildPriceDaily :execrows
INSERT INTO price_daily (product_id, trade_type, province_id, day, median, p25, p75, min_price, max_price, offers, demo_share)
SELECT o.product_id, o.trade_type, COALESCE(r.parent_id, r.id), d::date,
       weighted_quantile(array_agg(o.price_som::float8 ORDER BY o.id), array_agg(source_trust(o.source) ORDER BY o.id), 0.5),
       weighted_quantile(array_agg(o.price_som::float8 ORDER BY o.id), array_agg(source_trust(o.source) ORDER BY o.id), 0.25),
       weighted_quantile(array_agg(o.price_som::float8 ORDER BY o.id), array_agg(source_trust(o.source) ORDER BY o.id), 0.75),
       min(o.price_som), max(o.price_som), count(*)::int,
       avg(CASE WHEN u.is_demo THEN 1 ELSE 0 END)::real
FROM price_observations o
JOIN regions r ON r.id = o.region_id
LEFT JOIN users u ON u.id = o.seller_id
CROSS JOIN LATERAL generate_series(
    GREATEST((o.observed_at AT TIME ZONE 'Asia/Tashkent')::date, sqlc.arg(from_day)::date),
    LEAST(((o.valid_until - interval '1 second') AT TIME ZONE 'Asia/Tashkent')::date, sqlc.arg(to_day)::date),
    interval '1 day') AS d
WHERE o.status IN ('active', 'expired') AND source_trust(o.source) > 0
  AND o.observed_at < (sqlc.arg(to_day)::date + 1)::timestamp AT TIME ZONE 'Asia/Tashkent'
  AND o.valid_until > sqlc.arg(from_day)::date::timestamp AT TIME ZONE 'Asia/Tashkent'
GROUP BY o.product_id, o.trade_type, COALESCE(r.parent_id, r.id), d::date
HAVING count(*) >= 3
ON CONFLICT (product_id, trade_type, province_id, day) DO UPDATE SET
    median = EXCLUDED.median, p25 = EXCLUDED.p25, p75 = EXCLUDED.p75, min_price = EXCLUDED.min_price,
    max_price = EXCLUDED.max_price, offers = EXCLUDED.offers, demo_share = EXCLUDED.demo_share, synthetic = FALSE;

-- The price series of a product for a viloyat, or the whole country as the
-- offers-weighted mean of viloyat medians, bucketed by day, week or month.
-- name: PriceSeries :many
SELECT date_trunc(sqlc.arg(bucket)::text, d.day)::date AS day,
       (sum(d.median * d.offers) / sum(d.offers))::float8 AS median,
       (sum(d.p25 * d.offers) / sum(d.offers))::float8 AS p25,
       (sum(d.p75 * d.offers) / sum(d.offers))::float8 AS p75,
       min(d.min_price)::bigint AS min_price,
       max(d.max_price)::bigint AS max_price,
       sum(d.offers)::int AS offers,
       (sum(d.demo_share * d.offers) / sum(d.offers))::float8 AS demo_share
FROM price_daily_clean d
WHERE d.product_id = sqlc.arg(product_id) AND d.trade_type = sqlc.arg(trade_type)
  AND d.day BETWEEN sqlc.arg(from_day)::date AND sqlc.arg(to_day)::date
  AND (sqlc.narg(province_id)::smallint IS NULL OR d.province_id = sqlc.narg(province_id))
GROUP BY 1
ORDER BY 1;

-- The dearest and cheapest day of each year in the range, with where it was;
-- only days with at least 3 prices, so one odd price is not a record.
-- name: PriceRecords :many
WITH ranked AS (
    SELECT d.day, d.median, d.province_id, extract(year FROM d.day)::int AS year,
           row_number() OVER (PARTITION BY extract(year FROM d.day) ORDER BY d.median DESC, d.day) AS hi,
           row_number() OVER (PARTITION BY extract(year FROM d.day) ORDER BY d.median ASC, d.day) AS lo
    FROM price_daily_clean d
    WHERE d.product_id = sqlc.arg(product_id) AND d.trade_type = sqlc.arg(trade_type) AND d.offers >= 3
      AND d.day BETWEEN sqlc.arg(from_day)::date AND sqlc.arg(to_day)::date
      AND (sqlc.narg(province_id)::smallint IS NULL OR d.province_id = sqlc.narg(province_id))
)
SELECT k.year, (k.hi = 1)::bool AS is_max, k.day, k.median::float8 AS median, r.id AS province_id, r.name AS province_name
FROM ranked k
JOIN regions r ON r.id = k.province_id
WHERE k.hi = 1 OR k.lo = 1
ORDER BY k.year, is_max DESC;

-- The live market now: weighted median, quartiles and range of prices valid
-- today (the same rules as the public median).
-- name: LiveStats :one
SELECT count(*)::int AS offers,
       COALESCE(weighted_quantile(array_agg(o.price_som::float8 ORDER BY o.id), array_agg(price_weight(o.source, o.observed_at) ORDER BY o.id), 0.5), 0)::float8 AS median,
       COALESCE(weighted_quantile(array_agg(o.price_som::float8 ORDER BY o.id), array_agg(price_weight(o.source, o.observed_at) ORDER BY o.id), 0.25), 0)::float8 AS p25,
       COALESCE(weighted_quantile(array_agg(o.price_som::float8 ORDER BY o.id), array_agg(price_weight(o.source, o.observed_at) ORDER BY o.id), 0.75), 0)::float8 AS p75,
       COALESCE(min(o.price_som), 0)::bigint AS min_price,
       COALESCE(max(o.price_som), 0)::bigint AS max_price,
       COALESCE(max(o.observed_at), 'epoch'::timestamptz)::timestamptz AS last_price_at
FROM price_observations o
JOIN regions r ON r.id = o.region_id
WHERE o.product_id = sqlc.arg(product_id) AND o.trade_type = sqlc.arg(trade_type) AND o.status = 'active' AND o.valid_until > now()
  AND (sqlc.narg(province_id)::smallint IS NULL OR COALESCE(r.parent_id, r.id) = sqlc.narg(province_id))
  AND (o.listing_id IS NULL OR EXISTS (SELECT 1 FROM listings x WHERE x.id = o.listing_id AND x.status = 'active'));

-- Where the last 30 days of prices came from, and how many were real deals.
-- name: SourceMix :many
SELECT o.source, count(*)::int AS n
FROM price_observations o
JOIN regions r ON r.id = o.region_id
WHERE o.product_id = sqlc.arg(product_id) AND o.trade_type = sqlc.arg(trade_type)
  AND o.status IN ('active', 'expired') AND o.observed_at > now() - interval '30 days'
  AND (sqlc.narg(province_id)::smallint IS NULL OR COALESCE(r.parent_id, r.id) = sqlc.narg(province_id))
GROUP BY o.source
ORDER BY n DESC;

-- name: ConfirmedDeals30d :one
SELECT count(*)::int FROM deals d
JOIN regions r ON r.id = d.region_id
WHERE d.product_id = sqlc.arg(product_id) AND d.status = 'confirmed' AND d.created_at > now() - interval '30 days'
  AND (sqlc.narg(province_id)::smallint IS NULL OR COALESCE(r.parent_id, r.id) = sqlc.narg(province_id));

-- Each viloyat's price over the last 14 days (offers-weighted mean of daily
-- medians) for the ranking and arbitrage.
-- name: ProvincePrices :many
SELECT r.id, r.name, COALESCE(r.soato, 0)::bigint AS soato,
       (sum(d.median * d.offers) / sum(d.offers))::float8 AS median,
       sum(d.offers)::int AS offers,
       (sum(d.demo_share * d.offers) / sum(d.offers))::float8 AS demo_share
FROM price_daily_clean d
JOIN regions r ON r.id = d.province_id
WHERE d.product_id = sqlc.arg(product_id) AND d.trade_type = sqlc.arg(trade_type)
  AND d.day > (now() AT TIME ZONE 'Asia/Tashkent')::date - 14
GROUP BY r.id, r.name, r.soato
ORDER BY median;

-- Live prices of the last 14 days inside a viloyat by tuman.
-- name: DistrictPrices :many
SELECT r.id, r.name, count(*)::int AS offers,
       weighted_quantile(array_agg(o.price_som::float8 ORDER BY o.id), array_agg(source_trust(o.source) ORDER BY o.id), 0.5)::float8 AS median
FROM price_observations o
JOIN regions r ON r.id = o.region_id
WHERE o.product_id = sqlc.arg(product_id) AND o.trade_type = sqlc.arg(trade_type)
  AND o.status IN ('active', 'expired') AND source_trust(o.source) > 0 AND o.observed_at > now() - interval '14 days'
  AND COALESCE(r.parent_id, r.id) = sqlc.arg(province_id)::smallint
GROUP BY r.id, r.name
HAVING count(*) >= sqlc.arg(min_offers)::int
ORDER BY median;

-- The same by bazaar.
-- name: MarketPrices :many
SELECT m.id, m.name, count(*)::int AS offers,
       weighted_quantile(array_agg(o.price_som::float8 ORDER BY o.id), array_agg(source_trust(o.source) ORDER BY o.id), 0.5)::float8 AS median
FROM price_observations o
JOIN regions r ON r.id = o.region_id
JOIN markets m ON m.id = o.market_id
WHERE o.product_id = sqlc.arg(product_id) AND o.trade_type = sqlc.arg(trade_type)
  AND o.status IN ('active', 'expired') AND source_trust(o.source) > 0 AND o.observed_at > now() - interval '14 days'
  AND COALESCE(r.parent_id, r.id) = sqlc.arg(province_id)::smallint
GROUP BY m.id, m.name
HAVING count(*) >= sqlc.arg(min_offers)::int
ORDER BY median;

-- Wholesale and retail over the last 30 days, for the retail markup.
-- name: TradeMedians :many
SELECT d.trade_type, (sum(d.median * d.offers) / sum(d.offers))::float8 AS median, sum(d.offers)::int AS offers
FROM price_daily_clean d
WHERE d.product_id = sqlc.arg(product_id) AND d.day > (now() AT TIME ZONE 'Asia/Tashkent')::date - 30
  AND (sqlc.narg(province_id)::smallint IS NULL OR d.province_id = sqlc.narg(province_id))
GROUP BY d.trade_type;

-- Agreed price of each confirmed deal of the last 90 days against the
-- listing's asking price just before the deal.
-- name: DealDiscounts :many
SELECT d.price_som::float8 AS agreed,
       COALESCE((SELECT o.price_som FROM price_observations o
                 WHERE o.listing_id = d.listing_id AND o.observed_at <= d.created_at AND o.source IN ('seller', 'seller_auto')
                 ORDER BY o.observed_at DESC LIMIT 1), 0)::float8 AS asked
FROM deals d
JOIN regions r ON r.id = d.region_id
WHERE d.product_id = sqlc.arg(product_id) AND d.status = 'confirmed' AND d.price_som IS NOT NULL AND d.listing_id IS NOT NULL
  AND d.created_at > now() - interval '90 days'
  AND (sqlc.narg(province_id)::smallint IS NULL OR COALESCE(r.parent_id, r.id) = sqlc.narg(province_id));

-- Live listings, sellers and stock (in the product's base unit) now.
-- name: SupplyNow :one
SELECT count(*)::int AS listings, count(DISTINCT l.seller_id)::int AS sellers,
       COALESCE(sum(l.stock) FILTER (WHERE l.unit = p.base_unit), 0)::float8 AS stock,
       count(l.stock)::int AS stocked
FROM listings l
JOIN products p ON p.id = l.product_id
JOIN regions r ON r.id = l.region_id
WHERE l.product_id = sqlc.arg(product_id) AND l.trade_type = sqlc.arg(trade_type) AND l.status = 'active' AND l.valid_until > now()
  AND (sqlc.narg(province_id)::smallint IS NULL OR COALESCE(r.parent_id, r.id) = sqlc.narg(province_id));

-- Sellers whose price was live a week ago.
-- name: SellersWeekAgo :one
SELECT count(DISTINCT o.seller_id)::int
FROM price_observations o
JOIN regions r ON r.id = o.region_id
WHERE o.product_id = sqlc.arg(product_id) AND o.trade_type = sqlc.arg(trade_type)
  AND o.source IN ('seller', 'seller_auto') AND o.status IN ('active', 'expired')
  AND o.observed_at <= now() - interval '7 days' AND o.valid_until > now() - interval '7 days'
  AND (sqlc.narg(province_id)::smallint IS NULL OR COALESCE(r.parent_id, r.id) = sqlc.narg(province_id));
