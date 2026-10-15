-- Weekly viloyat report (W-10). Organisations get aggregates only: a product
-- is listed with at least 5 prices and no seller is named (spec §14.3).

-- name: ReportSummary :one
SELECT
    (SELECT count(DISTINCT l.seller_id) FROM listings l JOIN regions r ON r.id = l.region_id
     WHERE l.status = 'active' AND l.valid_until > now() AND COALESCE(r.parent_id, r.id) = @province_id::smallint)::int AS sellers,
    (SELECT count(*) FROM listings l JOIN regions r ON r.id = l.region_id
     WHERE l.status = 'active' AND l.valid_until > now() AND COALESCE(r.parent_id, r.id) = @province_id::smallint)::int AS listings,
    (SELECT count(*) FROM rfqs q JOIN regions r ON r.id = q.region_id
     WHERE q.created_at > now() - interval '7 days' AND COALESCE(r.parent_id, r.id) = @province_id::smallint)::int AS rfqs,
    (SELECT count(*) FROM demand_events e
     WHERE e.created_at > now() - interval '7 days' AND e.province_id = @province_id::smallint)::int AS demand,
    (SELECT count(*) FROM deals d JOIN regions r ON r.id = d.region_id
     WHERE d.status = 'confirmed' AND d.created_at > now() - interval '7 days'
       AND COALESCE(r.parent_id, r.id) = @province_id::smallint)::int AS deals;

-- Wholesale weighted medians now and a week ago per product in the viloyat.
-- name: ReportProducts :many
WITH cur AS (
    SELECT o.id, o.product_id, o.price_som, o.source, o.observed_at
    FROM price_observations o JOIN regions r ON r.id = o.region_id
    WHERE o.trade_type = 'ulgurji' AND o.status = 'active' AND o.valid_until > now()
      AND COALESCE(r.parent_id, r.id) = @province_id::smallint
      AND (o.listing_id IS NULL OR EXISTS (SELECT 1 FROM listings l WHERE l.id = o.listing_id AND l.status = 'active'))
), prev AS (
    SELECT o.id, o.product_id, o.price_som, o.source, o.observed_at
    FROM price_observations o JOIN regions r ON r.id = o.region_id
    WHERE o.trade_type = 'ulgurji' AND o.status IN ('active', 'expired') AND o.source IN ('seller', 'seller_auto', 'rfq_offer')
      AND o.observed_at < now() - interval '7 days' AND o.valid_until > now() - interval '14 days'
      AND COALESCE(r.parent_id, r.id) = @province_id::smallint
), cs AS (
    SELECT product_id, count(*)::int AS offers, min(price_som)::bigint AS min_price, max(price_som)::bigint AS max_price,
           weighted_quantile(array_agg(price_som::float8 ORDER BY id), array_agg(price_weight(source, observed_at) ORDER BY id), 0.5)::float8 AS median
    FROM cur GROUP BY product_id HAVING count(*) >= 5
), ps AS (
    SELECT product_id,
           weighted_quantile(array_agg(price_som::float8 ORDER BY id), array_agg(price_weight(source, observed_at) ORDER BY id), 0.5)::float8 AS median
    FROM prev GROUP BY product_id HAVING count(*) >= 5
)
SELECT p.id, p.slug, p.name, p.base_unit, cs.offers, cs.median, cs.min_price, cs.max_price,
       COALESCE(ps.median, 0)::float8 AS prev_median
FROM cs
JOIN products p ON p.id = cs.product_id
LEFT JOIN ps ON ps.product_id = cs.product_id
ORDER BY cs.offers DESC, p.name
LIMIT 30;

-- Confirmed deals of the week per product in the viloyat, only products with
-- at least 5 (QIZIQISH_VA_SAVDO.md step 10).
-- name: ReportDeals :many
SELECT p.name, d.unit, count(*)::int AS deals,
       COALESCE(sum(d.quantity), 0)::float8 AS quantity,
       count(d.price_som)::int AS priced,
       COALESCE(percentile_cont(0.5) WITHIN GROUP (ORDER BY d.price_som), 0)::float8 AS median
FROM deals d
JOIN regions r ON r.id = d.region_id
JOIN products p ON p.id = d.product_id
WHERE d.status = 'confirmed' AND d.created_at > now() - interval '7 days'
  AND COALESCE(r.parent_id, r.id) = @province_id::smallint
GROUP BY p.name, d.unit
HAVING count(*) >= 5
ORDER BY count(*) DESC, p.name;
