-- Unusual sellers (ML_REJA.md 5).

-- Each seller's own prices of the last 60 days against the viloyat's day.
-- name: SellerFeatures :many
WITH obs AS (
    SELECT o.seller_id, o.listing_id, o.status, o.price_som, o.product_id, o.trade_type,
           COALESCE(r.parent_id, r.id) AS province_id, (o.observed_at AT TIME ZONE 'Asia/Tashkent')::date AS day
    FROM price_observations o
    JOIN regions r ON r.id = o.region_id
    WHERE o.seller_id IS NOT NULL AND o.source IN ('seller', 'seller_auto') AND o.observed_at > now() - interval '60 days'
)
SELECT o.seller_id,
       count(*)::int AS prices,
       avg(CASE WHEN o.status IN ('review', 'rejected') THEN 1 ELSE 0 END)::float8 AS review_share,
       COALESCE(percentile_cont(0.5) WITHIN GROUP (ORDER BY abs(ln(o.price_som / d.median))) FILTER (WHERE d.median > 0), 0)::float8 AS deviation,
       COALESCE(avg(CASE WHEN o.price_som < d.p25 THEN 1 ELSE 0 END) FILTER (WHERE d.median > 0), 0)::float8 AS below_share,
       COALESCE(avg(CASE WHEN o.price_som > d.p75 THEN 1 ELSE 0 END) FILTER (WHERE d.median > 0), 0)::float8 AS above_share,
       (count(*)::float8 / GREATEST(count(DISTINCT o.listing_id), 1))::float8 AS per_listing,
       count(DISTINCT o.product_id)::int AS products,
       COALESCE((SELECT max(c) FROM (SELECT count(*) AS c FROM listings l
                                     WHERE l.seller_id = o.seller_id AND l.created_at > now() - interval '60 days'
                                     GROUP BY (l.created_at AT TIME ZONE 'Asia/Tashkent')::date) x), 0)::int AS max_listings_day
FROM obs o
LEFT JOIN price_daily_clean d ON d.product_id = o.product_id AND d.trade_type = o.trade_type
                             AND d.province_id = o.province_id AND d.day = o.day
GROUP BY o.seller_id
HAVING count(*) >= 3
ORDER BY o.seller_id;

-- name: DeleteSellerScores :exec
DELETE FROM seller_scores;

-- name: InsertSellerScore :exec
INSERT INTO seller_scores (seller_id, score, features, reasons) VALUES (sqlc.arg(seller_id), sqlc.arg(score), sqlc.arg(features), sqlc.arg(reasons));

-- name: SuspiciousSellers :many
SELECT s.seller_id, u.display_name, u.telegram_username, u.is_demo, s.score, s.features, s.reasons, s.computed_at,
       (SELECT count(*) FROM listings l WHERE l.seller_id = s.seller_id AND l.status = 'active')::int AS live_listings
FROM seller_scores s
JOIN users u ON u.id = s.seller_id
WHERE s.score >= sqlc.arg(min_score)::float8
ORDER BY s.score DESC
LIMIT 100;

-- name: SellerScoreStats :one
SELECT count(*)::int AS scored, COALESCE(max(computed_at), 'epoch')::timestamptz AS computed_at FROM seller_scores;

-- Public seller page (W-09) of any entrepreneur ('seller', 'buyer': legacy
-- roles). Never select phone, telegram username or id here.
-- name: PublicSeller :one
SELECT u.id,
       COALESCE(NULLIF(u.public_name, ''), split_part(u.display_name, ' ', 1))::text AS name,
       COALESCE(r.name, '')::text AS region_name, COALESCE(pr.name, '')::text AS province_name,
       COALESCE(m.name, '')::text AS market_name, u.created_at,
       (u.contact_telegram AND u.telegram_username IS NOT NULL)::bool AS has_telegram,
       (u.contact_phone AND u.phone_enc IS NOT NULL)::bool AS has_phone,
       COALESCE(live.listings, 0)::int AS listings, COALESCE(live.products, 0)::int AS products,
       COALESCE(NULLIF(live.categories, '{}'), u.categories)::text[] AS categories,
       COALESCE((SELECT max(o.observed_at) FROM price_observations o
                 WHERE o.seller_id = u.id AND o.source IN ('seller', 'seller_auto')), 'epoch')::timestamptz AS last_active
FROM users u
LEFT JOIN regions r ON r.id = u.region_id
LEFT JOIN regions pr ON pr.id = r.parent_id
LEFT JOIN markets m ON m.id = u.market_id
LEFT JOIN LATERAL (
    SELECT count(*) AS listings, count(DISTINCT l.product_id) AS products,
           COALESCE(array_agg(DISTINCT p.category) FILTER (WHERE p.category IS NOT NULL), '{}') AS categories
    FROM listings l JOIN products p ON p.id = l.product_id
    WHERE l.seller_id = u.id AND l.status = 'active' AND l.valid_until > now()
) live ON TRUE
WHERE u.id = $1 AND u.role IN ('entrepreneur', 'seller', 'buyer');

-- Sellers whose public name holds the text, for the assistant: entrepreneurs
-- who have posted a listing ('seller', 'buyer': legacy roles).
-- name: SellersByName :many
SELECT u.id, COALESCE(NULLIF(u.public_name, ''), split_part(u.display_name, ' ', 1))::text AS name
FROM users u
WHERE u.role IN ('entrepreneur', 'seller', 'buyer') AND EXISTS (SELECT 1 FROM listings l WHERE l.seller_id = u.id)
  AND COALESCE(NULLIF(u.public_name, ''), u.display_name) ILIKE '%' || sqlc.arg(name)::text || '%'
ORDER BY u.created_at
LIMIT 5;
