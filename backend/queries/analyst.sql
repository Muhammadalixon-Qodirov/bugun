-- Functions of the analyst (spec AI-5) that the site's queries do not cover.

-- Share of the live prices behind ProductStats that come from demo sellers.
-- name: AnalystLiveDemoShare :one
SELECT COALESCE(avg(CASE WHEN u.is_demo THEN 1 ELSE 0 END), 0)::float8 AS demo_share
FROM price_observations o
JOIN regions r ON r.id = o.region_id
LEFT JOIN users u ON u.id = o.seller_id
WHERE o.product_id = @product_id AND o.trade_type = @trade_type
  AND o.status = 'active' AND o.valid_until > now()
  AND (sqlc.narg(region_id)::smallint IS NULL OR r.id = sqlc.narg(region_id) OR r.parent_id = sqlc.narg(region_id))
  AND (o.listing_id IS NULL OR EXISTS (SELECT 1 FROM listings l WHERE l.id = o.listing_id AND l.status = 'active'));

-- Live offers, cheapest first, that can sell the amount asked (a listing
-- without stock or minimum order qualifies).
-- name: AnalystOffers :many
SELECT l.price_som, l.unit, l.min_order, l.stock, l.updated_at,
       r.name AS region_name, COALESCE(m.name, '')::text AS market_name,
       COALESCE(NULLIF(u.public_name, ''), split_part(u.display_name, ' ', 1))::text AS seller_name,
       u.is_demo
FROM listings l
JOIN users u ON u.id = l.seller_id
JOIN regions r ON r.id = l.region_id
LEFT JOIN markets m ON m.id = l.market_id
WHERE l.product_id = @product_id AND l.status = 'active' AND l.valid_until > now() AND l.trade_type = @trade_type
  AND (sqlc.narg(region_id)::smallint IS NULL OR r.id = sqlc.narg(region_id) OR r.parent_id = sqlc.narg(region_id))
  AND (sqlc.narg(quantity)::numeric IS NULL
       OR ((l.stock IS NULL OR l.stock >= sqlc.narg(quantity)) AND (l.min_order IS NULL OR l.min_order <= sqlc.narg(quantity))))
ORDER BY l.price_som, l.updated_at DESC
LIMIT @max_rows;
