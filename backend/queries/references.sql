-- Official benchmark prices from outside sources (migration 00039).

-- One quote a day: fetching the same day again updates it.
-- name: UpsertReferencePrice :exec
INSERT INTO reference_prices (source, product_id, trade_type, region_id, price_som, quote_name, quote_unit, quote_som, quoted_on, fetched_at, source_url)
VALUES (@source, @product_id, @trade_type, sqlc.narg(region_id), @price_som, @quote_name, @quote_unit, sqlc.arg(quote_som)::float8, @quoted_on, now(), sqlc.narg(source_url))
ON CONFLICT (source, product_id, trade_type, COALESCE(region_id, 0), quoted_on) DO UPDATE SET
    price_som = EXCLUDED.price_som, quote_name = EXCLUDED.quote_name, quote_unit = EXCLUDED.quote_unit,
    quote_som = EXCLUDED.quote_som, fetched_at = EXCLUDED.fetched_at, source_url = EXCLUDED.source_url;

-- The latest quote of every source for a product and trade: the viloyat's
-- own when it has one, else the national one.
-- name: LatestReferencePrices :many
SELECT DISTINCT ON (rp.source) rp.source, rp.price_som, rp.quote_name, rp.quote_unit, rp.quote_som::float8 AS quote_som,
       rp.quoted_on, rp.fetched_at, COALESCE(rp.source_url, '')::text AS source_url, rp.region_id
FROM reference_prices rp
WHERE rp.product_id = sqlc.arg(product_id) AND rp.trade_type = sqlc.arg(trade_type)
  AND (rp.region_id IS NULL OR rp.region_id = sqlc.narg(province_id))
ORDER BY rp.source, rp.quoted_on DESC, (rp.region_id IS NULL), rp.fetched_at DESC;

-- When each source last delivered a quote, for monitoring.
-- name: ReferenceFreshness :many
SELECT source, max(fetched_at)::timestamptz AS fetched_at, max(quoted_on)::date AS quoted_on
FROM reference_prices
GROUP BY source;
