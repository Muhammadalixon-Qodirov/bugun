-- Regional statistics (TAHLIL_REJA.md §8).

-- Every product's price in every viloyat over the last 14 days (offers-weighted
-- mean of daily medians), for the viloyat page and the heatmap.
-- name: ProductProvinceMatrix :many
SELECT d.product_id, d.province_id,
       (sum(d.median * d.offers) / sum(d.offers))::float8 AS median,
       sum(d.offers)::int AS offers,
       (sum(d.demo_share * d.offers) / sum(d.offers))::float8 AS demo_share
FROM price_daily_clean d
WHERE d.trade_type = sqlc.arg(trade_type) AND d.day > (now() AT TIME ZONE 'Asia/Tashkent')::date - 14
GROUP BY d.product_id, d.province_id;

-- Each product's last 7 days against the 7 before, in a viloyat or the country.
-- name: ProductWeekChange :many
SELECT d.product_id,
       COALESCE(sum(d.median * d.offers) FILTER (WHERE d.day > (now() AT TIME ZONE 'Asia/Tashkent')::date - 7)
                / NULLIF(sum(d.offers) FILTER (WHERE d.day > (now() AT TIME ZONE 'Asia/Tashkent')::date - 7), 0), 0)::float8 AS now,
       COALESCE(sum(d.median * d.offers) FILTER (WHERE d.day <= (now() AT TIME ZONE 'Asia/Tashkent')::date - 7)
                / NULLIF(sum(d.offers) FILTER (WHERE d.day <= (now() AT TIME ZONE 'Asia/Tashkent')::date - 7), 0), 0)::float8 AS prev,
       COALESCE(sum(d.offers) FILTER (WHERE d.day > (now() AT TIME ZONE 'Asia/Tashkent')::date - 7), 0)::int AS offers_now,
       COALESCE(sum(d.offers) FILTER (WHERE d.day <= (now() AT TIME ZONE 'Asia/Tashkent')::date - 7), 0)::int AS offers_prev
FROM price_daily_clean d
WHERE d.trade_type = sqlc.arg(trade_type) AND d.day > (now() AT TIME ZONE 'Asia/Tashkent')::date - 14
  AND (sqlc.narg(province_id)::smallint IS NULL OR d.province_id = sqlc.narg(province_id))
GROUP BY d.product_id;

-- Weekly prices of every product over a year, in a viloyat or the country,
-- for the food basket index.
-- name: WeeklyProductPrices :many
SELECT d.product_id, date_trunc('week', d.day)::date AS week,
       (sum(d.median * d.offers) / sum(d.offers))::float8 AS median
FROM price_daily_clean d
WHERE d.trade_type = sqlc.arg(trade_type) AND d.day > (now() AT TIME ZONE 'Asia/Tashkent')::date - 371
  AND (sqlc.narg(province_id)::smallint IS NULL OR d.province_id = sqlc.narg(province_id))
GROUP BY d.product_id, 2
ORDER BY 2;

-- The viloyat's market this week and last week.
-- name: RegionPassport :one
WITH area AS (SELECT id FROM regions WHERE id = sqlc.arg(province_id)::smallint OR parent_id = sqlc.arg(province_id)::smallint)
SELECT
    (SELECT count(DISTINCT l.seller_id) FROM listings l WHERE l.region_id IN (SELECT id FROM area) AND l.status = 'active' AND l.valid_until > now())::int AS sellers,
    (SELECT count(*) FROM listings l WHERE l.region_id IN (SELECT id FROM area) AND l.status = 'active' AND l.valid_until > now())::int AS listings,
    (SELECT count(DISTINCT l.market_id) FROM listings l WHERE l.region_id IN (SELECT id FROM area) AND l.status = 'active' AND l.valid_until > now())::int AS markets,
    (SELECT count(*) FROM rfqs q WHERE q.region_id IN (SELECT id FROM area) AND q.created_at > now() - interval '7 days')::int AS rfqs,
    (SELECT count(*) FROM rfqs q WHERE q.region_id IN (SELECT id FROM area) AND q.created_at BETWEEN now() - interval '14 days' AND now() - interval '7 days')::int AS rfqs_prev,
    (SELECT count(*) FROM deals d WHERE d.region_id IN (SELECT id FROM area) AND d.status = 'confirmed' AND d.created_at > now() - interval '7 days')::int AS deals,
    (SELECT count(*) FROM deals d WHERE d.region_id IN (SELECT id FROM area) AND d.status = 'confirmed' AND d.created_at BETWEEN now() - interval '14 days' AND now() - interval '7 days')::int AS deals_prev,
    (SELECT count(*) FROM demand_events e WHERE e.province_id = sqlc.arg(province_id)::smallint AND e.created_at > now() - interval '7 days')::int AS demand,
    (SELECT count(*) FROM demand_events e WHERE e.province_id = sqlc.arg(province_id)::smallint AND e.created_at BETWEEN now() - interval '14 days' AND now() - interval '7 days')::int AS demand_prev;

-- Live listings and sellers per product in the viloyat.
-- name: RegionProductSellers :many
SELECT l.product_id, count(DISTINCT l.seller_id)::int AS sellers
FROM listings l
JOIN regions r ON r.id = l.region_id
WHERE l.status = 'active' AND l.valid_until > now() AND l.trade_type = sqlc.arg(trade_type)
  AND COALESCE(r.parent_id, r.id) = sqlc.arg(province_id)::smallint
GROUP BY l.product_id;

-- Activity by tuman.
-- name: RegionDistrictActivity :many
SELECT r.id, r.name, count(*)::int AS listings, count(DISTINCT l.seller_id)::int AS sellers
FROM listings l
JOIN regions r ON r.id = l.region_id
WHERE l.status = 'active' AND l.valid_until > now() AND r.parent_id = sqlc.arg(province_id)::smallint
GROUP BY r.id, r.name
ORDER BY listings DESC, r.name;

-- Activity by bazaar.
-- name: RegionMarketActivity :many
SELECT m.id, m.name, count(*)::int AS listings, count(DISTINCT l.seller_id)::int AS sellers
FROM listings l
JOIN markets m ON m.id = l.market_id
JOIN regions r ON r.id = l.region_id
WHERE l.status = 'active' AND l.valid_until > now() AND COALESCE(r.parent_id, r.id) = sqlc.arg(province_id)::smallint
GROUP BY m.id, m.name
ORDER BY listings DESC, m.name;

-- Demand signals per product, kind and 7-day window (0 = last 7 days, 1–4
-- the weeks before), in a viloyat or the country.
-- name: DemandByProduct :many
SELECT e.product_id, e.kind,
       floor(extract(epoch FROM now() - e.created_at) / 604800)::int AS win,
       count(*)::int AS n
FROM demand_events e
WHERE e.created_at > now() - interval '35 days'
  AND (sqlc.narg(province_id)::smallint IS NULL OR e.province_id = sqlc.narg(province_id))
GROUP BY 1, 2, 3;

-- Confirmed deals per product and 7-day window, the same way.
-- name: DealsByProduct :many
SELECT d.product_id, floor(extract(epoch FROM now() - d.created_at) / 604800)::int AS win, count(*)::int AS n
FROM deals d
JOIN regions r ON r.id = d.region_id
WHERE d.status = 'confirmed' AND d.created_at > now() - interval '35 days'
  AND (sqlc.narg(province_id)::smallint IS NULL OR COALESCE(r.parent_id, r.id) = sqlc.narg(province_id))
GROUP BY 1, 2;
