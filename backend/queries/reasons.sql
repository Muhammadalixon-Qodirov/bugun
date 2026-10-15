-- Price change reasons (TAHLIL_REJA.md 3.5).

-- name: RecordPriceReason :exec
INSERT INTO price_reasons (listing_id, product_id, province_id, direction, change, reason)
SELECT l.id, l.product_id, COALESCE(r.parent_id, r.id), @direction::text, @change::float8, @reason::text
FROM listings l JOIN regions r ON r.id = l.region_id
WHERE l.id = @listing_id AND l.seller_id = @seller_id
ON CONFLICT (listing_id, day) DO UPDATE SET reason = EXCLUDED.reason, direction = EXCLUDED.direction, change = EXCLUDED.change;

-- Reasons for a product over the last 30 days, in a viloyat or the country.
-- name: ReasonCounts :many
SELECT reason, direction, count(*)::int AS n
FROM price_reasons
WHERE product_id = sqlc.arg(product_id) AND created_at > now() - interval '30 days'
  AND (sqlc.narg(province_id)::smallint IS NULL OR province_id = sqlc.narg(province_id))
GROUP BY reason, direction
ORDER BY n DESC;
