-- Daily price anomalies (ML_REJA.md 4).

-- name: DailyForAnomalies :many
SELECT product_id, trade_type, province_id, day, median
FROM price_daily
WHERE product_id = sqlc.arg(product_id) AND day >= sqlc.arg(from_day)::date
ORDER BY trade_type, province_id, day;

-- name: ProductsWithDaily :many
SELECT p.id, p.category FROM products p
WHERE EXISTS (SELECT 1 FROM price_daily d WHERE d.product_id = p.id AND d.day >= sqlc.arg(from_day)::date)
ORDER BY p.id;

-- Events and holidays that can explain a jump: a flagged day near one is
-- left alone.
-- name: EventsSince :many
SELECT starts_on, COALESCE(ends_on, starts_on)::date AS ends_on, province_id, product_id, category
FROM market_events
WHERE COALESCE(ends_on, starts_on) >= sqlc.arg(from_day)::date;

-- name: DeleteOpenAnomalies :exec
DELETE FROM price_anomalies WHERE status = 'open' AND day >= sqlc.arg(from_day)::date;

-- A flag an admin decided on stays as it is.
-- name: InsertAnomaly :exec
INSERT INTO price_anomalies (product_id, trade_type, province_id, day, layer, value, reference, z, settled)
VALUES (sqlc.arg(product_id), sqlc.arg(trade_type), sqlc.arg(province_id), sqlc.arg(day), sqlc.arg(layer),
        sqlc.arg(value), sqlc.arg(reference), sqlc.arg(z), sqlc.arg(settled))
ON CONFLICT DO NOTHING;

-- Flags to mark on a product's chart, with the day's demo share.
-- name: AnomaliesFor :many
SELECT a.day, a.province_id, r.name AS province_name, a.layer, a.value, a.reference, a.settled, a.status,
       COALESCE(d.demo_share, 0)::float8 AS demo_share
FROM price_anomalies a
JOIN regions r ON r.id = a.province_id
LEFT JOIN price_daily d ON d.product_id = a.product_id AND d.trade_type = a.trade_type
                       AND d.province_id = a.province_id AND d.day = a.day
WHERE a.product_id = sqlc.arg(product_id) AND a.trade_type = sqlc.arg(trade_type) AND a.status <> 'dismissed'
  AND a.day BETWEEN sqlc.arg(from_day)::date AND sqlc.arg(to_day)::date
  AND (sqlc.narg(province_id)::smallint IS NULL OR a.province_id = sqlc.narg(province_id))
ORDER BY a.day, a.province_id;

-- name: ListAnomalies :many
SELECT a.product_id, p.slug AS product_slug, p.name AS product_name, a.trade_type, a.province_id, r.name AS province_name,
       a.day, a.layer, a.value, a.reference, a.z, a.settled, a.status, a.found_at, a.decided_at,
       COALESCE(d.demo_share, 0)::float8 AS demo_share
FROM price_anomalies a
JOIN products p ON p.id = a.product_id
JOIN regions r ON r.id = a.province_id
LEFT JOIN price_daily d ON d.product_id = a.product_id AND d.trade_type = a.trade_type
                       AND d.province_id = a.province_id AND d.day = a.day
WHERE (sqlc.narg(status)::text IS NULL OR a.status = sqlc.narg(status))
ORDER BY a.day DESC, abs(a.z) DESC
LIMIT 300;

-- name: DecideAnomaly :execrows
UPDATE price_anomalies SET status = sqlc.arg(status), decided_at = now(), decided_by = sqlc.arg(decided_by)
WHERE product_id = sqlc.arg(product_id) AND trade_type = sqlc.arg(trade_type) AND province_id = sqlc.arg(province_id)
  AND day = sqlc.arg(day) AND layer = sqlc.arg(layer);
