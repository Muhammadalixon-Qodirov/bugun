-- name: ListParserProducts :many
SELECT p.id, p.slug, p.name, p.category, p.base_unit, p.synonyms,
       COALESCE(array_agg(u.unit ORDER BY u.unit) FILTER (WHERE u.unit IS NOT NULL), '{}')::text[] AS units,
       COALESCE(array_agg(u.base_quantity ORDER BY u.unit) FILTER (WHERE u.unit IS NOT NULL), '{}')::float8[] AS unit_sizes
FROM products p
LEFT JOIN unit_conversions u ON u.product_id = p.id
GROUP BY p.id
ORDER BY p.id;

-- name: CreatePriceDraft :one
INSERT INTO price_drafts (user_id, raw_text, items, expires_at, log_id, learn)
VALUES ($1, $2, $3, $4, sqlc.narg(log_id), sqlc.arg(learn))
RETURNING id;

-- Deleting on read makes confirmation single-use: a double tap saves once.
-- name: ConsumePriceDraft :one
DELETE FROM price_drafts
WHERE id = $1 AND user_id = $2 AND expires_at > now()
RETURNING raw_text, items, log_id, learn;

-- name: DeletePriceDraft :one
DELETE FROM price_drafts WHERE id = $1 AND user_id = $2
RETURNING log_id;
