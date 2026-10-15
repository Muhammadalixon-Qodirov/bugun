-- name: AdminSummary :one
SELECT
    (SELECT count(*) FROM listings WHERE status = 'review')::int AS review_listings,
    (SELECT count(*) FROM users WHERE role = 'market_admin' AND NOT role_approved)::int AS pending_roles,
    (SELECT count(*) FROM users WHERE role IS NOT NULL)::int AS users,
    -- Sellers: users who have posted at least one listing (any status).
    (SELECT count(DISTINCT seller_id) FROM listings)::int AS sellers,
    (SELECT count(*) FROM listings WHERE status = 'active' AND valid_until > now())::int AS live_listings;

-- name: AdminReviewQueue :many
-- Listings held for review, oldest first, with the median they were compared
-- against (live prices in the same viloyat, same product and trade type).
WITH live AS (
    SELECT o.id, o.product_id, o.trade_type, o.price_som, o.source, o.observed_at, COALESCE(r.parent_id, r.id) AS province_id
    FROM price_observations o
    JOIN regions r ON r.id = o.region_id
    WHERE o.status = 'active' AND o.valid_until > now()
), province_median AS (
    SELECT product_id, trade_type, province_id,
           weighted_quantile(array_agg(price_som::float8 ORDER BY id), array_agg(price_weight(source, observed_at) ORDER BY id), 0.5)::float8 AS median, count(*)::int AS offers
    FROM live GROUP BY product_id, trade_type, province_id
)
SELECT l.id, l.price_som, l.unit, l.trade_type, l.variety, l.description, l.created_at, l.updated_at,
       p.name AS product_name, p.slug AS product_slug, p.category,
       COALESCE(NULLIF(u.public_name, ''), u.display_name)::text AS seller_name,
       r.name AS region_name, COALESCE(pr.name, '')::text AS province_name,
       COALESCE((SELECT ph.storage_key FROM listing_photos ph WHERE ph.listing_id = l.id ORDER BY ph.position, ph.id LIMIT 1), '')::text AS cover_key,
       COALESCE(pm.median, 0)::float8 AS median,
       COALESCE(pm.offers, 0)::int AS offers,
       COALESCE(l.review_reason, '')::text AS review_reason
FROM listings l
JOIN products p ON p.id = l.product_id
JOIN users u ON u.id = l.seller_id
JOIN regions r ON r.id = l.region_id
LEFT JOIN regions pr ON pr.id = r.parent_id
LEFT JOIN province_median pm ON pm.product_id = l.product_id AND pm.trade_type = l.trade_type
                            AND pm.province_id = COALESCE(r.parent_id, r.id)
WHERE l.status = 'review'
ORDER BY l.updated_at
LIMIT 200;

-- name: DecideListingReview :one
UPDATE listings SET status = sqlc.arg(status), review_note = sqlc.narg(note), updated_at = now()
WHERE id = sqlc.arg(id) AND status = 'review'
RETURNING id;

-- name: SetReviewedObservationStatus :exec
UPDATE price_observations SET status = sqlc.arg(status)
WHERE listing_id = sqlc.arg(listing_id) AND status = 'review';

-- name: AdminRoleRequests :many
-- Market staff waiting for an admin to confirm their role.
SELECT u.id, u.display_name, u.role, u.telegram_username, u.created_at, u.updated_at,
       COALESCE(r.name, '')::text AS region_name, COALESCE(pr.name, '')::text AS province_name,
       COALESCE(m.name, '')::text AS market_name
FROM users u
LEFT JOIN regions r ON r.id = u.region_id
LEFT JOIN regions pr ON pr.id = r.parent_id
LEFT JOIN markets m ON m.id = u.market_id
WHERE u.role = 'market_admin' AND NOT u.role_approved
ORDER BY u.updated_at;

-- name: DecideRoleRequest :one
-- Approving keeps the requested role; declining makes the user an
-- entrepreneur, who can still use the whole market.
UPDATE users
SET role = CASE WHEN sqlc.arg(approved)::bool THEN role ELSE 'entrepreneur' END,
    role_approved = true, updated_at = now()
WHERE id = sqlc.arg(id) AND role = 'market_admin' AND NOT role_approved
RETURNING id;
