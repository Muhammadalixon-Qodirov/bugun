-- Market events (TAHLIL_REJA.md #30).

-- name: ListMarketEvents :many
SELECT e.*, COALESCE(r.name, '')::text AS province_name, COALESCE(p.name, '')::text AS product_name
FROM market_events e
LEFT JOIN regions r ON r.id = e.province_id
LEFT JOIN products p ON p.id = e.product_id
ORDER BY e.starts_on DESC, e.id DESC
LIMIT 500;

-- Events touching a product (directly, by category or everything) in a
-- viloyat (or anywhere) that overlap the days from..to.
-- name: EventsFor :many
SELECT e.id, e.title, e.kind, e.starts_on, e.ends_on, e.source_url, e.is_demo,
       COALESCE(r.name, '')::text AS province_name
FROM market_events e
LEFT JOIN regions r ON r.id = e.province_id
WHERE e.starts_on <= sqlc.arg(to_day)::date AND COALESCE(e.ends_on, e.starts_on) >= sqlc.arg(from_day)::date
  AND (e.product_id IS NULL OR e.product_id = sqlc.arg(product_id)::smallint)
  AND (e.category IS NULL OR e.category = sqlc.arg(category)::text)
  AND (sqlc.narg(province_id)::smallint IS NULL OR e.province_id IS NULL OR e.province_id = sqlc.narg(province_id))
ORDER BY e.starts_on;

-- name: CreateMarketEvent :one
INSERT INTO market_events (title, kind, starts_on, ends_on, province_id, product_id, category, source_url, note, is_demo, created_by)
VALUES (@title, @kind, @starts_on, sqlc.narg(ends_on), sqlc.narg(province_id), sqlc.narg(product_id), sqlc.narg(category),
        sqlc.narg(source_url), @note, @is_demo, sqlc.narg(created_by))
RETURNING *;

-- name: UpdateMarketEvent :one
UPDATE market_events SET title = @title, kind = @kind, starts_on = @starts_on, ends_on = sqlc.narg(ends_on),
       province_id = sqlc.narg(province_id), product_id = sqlc.narg(product_id), category = sqlc.narg(category),
       source_url = sqlc.narg(source_url), note = @note
WHERE id = @id
RETURNING *;

-- name: DeleteMarketEvent :execrows
DELETE FROM market_events WHERE id = $1;

-- name: DeleteDemoEvents :exec
DELETE FROM market_events WHERE is_demo;

-- Holidays that touch a category and begin within the next days, or are on now.
-- name: HolidaysAround :many
SELECT title, starts_on, COALESCE(ends_on, starts_on)::date AS ends_on
FROM market_events
WHERE kind = 'holiday' AND category = sqlc.arg(category)::text
  AND starts_on <= sqlc.arg(today)::date + sqlc.arg(days)::int
  AND COALESCE(ends_on, starts_on) >= sqlc.arg(today)::date
ORDER BY starts_on;
