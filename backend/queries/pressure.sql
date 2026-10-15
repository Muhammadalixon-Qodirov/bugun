-- Pressure index inputs (spec §10.3) over five 7-day windows: window 0 is the
-- last 7 days, window 1 the 7 days before, and so on. A province of NULL
-- means the whole country.

-- Demand events per window and kind; weights live in analytics.DemandWeight.
-- name: DemandWindows :many
SELECT w::int AS win, e.kind, count(*)::int AS n
FROM generate_series(0, 4) AS w
JOIN demand_events e
  ON e.product_id = @product_id
 AND (sqlc.narg(province_id)::smallint IS NULL OR e.province_id = sqlc.narg(province_id))
 AND e.created_at >= now() - (w + 1) * interval '7 days'
 AND e.created_at < now() - w * interval '7 days'
GROUP BY w, e.kind;

-- Sellers with a live price (listing or RFQ offer) at any time in the window.
-- name: SupplyWindows :many
SELECT w::int AS win, count(DISTINCT o.seller_id)::int AS sellers
FROM generate_series(0, 4) AS w
JOIN price_observations o
  ON o.product_id = @product_id AND o.trade_type = @trade_type
 AND o.source IN ('seller', 'seller_auto', 'rfq_offer') AND o.status IN ('active', 'expired')
 AND o.observed_at < now() - w * interval '7 days'
 AND o.valid_until > now() - (w + 1) * interval '7 days'
 AND (sqlc.narg(province_id)::smallint IS NULL
      OR o.region_id IN (SELECT id FROM regions WHERE id = sqlc.narg(province_id) OR parent_id = sqlc.narg(province_id)))
GROUP BY w;

-- The 14 top-level regions with their SOATO code, for the map.
-- name: ListProvinces :many
SELECT id, soato, name FROM regions WHERE parent_id IS NULL AND soato IS NOT NULL ORDER BY name;

-- Confirmed deals per window: the strongest demand signal.
-- name: DealWindows :many
SELECT w::int AS win, count(d.id)::int AS n
FROM generate_series(0, 4) AS w
JOIN deals d
  ON d.product_id = @product_id AND d.status = 'confirmed'
 AND d.created_at >= now() - (w + 1) * interval '7 days'
 AND d.created_at < now() - w * interval '7 days'
 AND (sqlc.narg(province_id)::smallint IS NULL
      OR d.region_id IN (SELECT id FROM regions WHERE id = sqlc.narg(province_id) OR parent_id = sqlc.narg(province_id)))
GROUP BY w;
