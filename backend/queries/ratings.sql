-- Ratings (QIZIQISH_VA_SAVDO.md step 9).

-- Confirmed deals with a buyer, confirmed 3 to 30 days ago, not yet asked
-- about; claimed by setting rating_asked_at, so each is asked once.
-- name: ClaimDealsToRate :many
UPDATE deals SET rating_asked_at = now()
WHERE id IN (
    SELECT d.id FROM deals d
    JOIN users u ON u.id = d.buyer_id
    WHERE d.status = 'confirmed' AND d.buyer_id IS NOT NULL AND d.rating_asked_at IS NULL AND u.bot_enabled
      AND GREATEST(d.buyer_confirmed_at, d.seller_confirmed_at) BETWEEN now() - interval '30 days' AND now() - interval '3 days'
      AND NOT EXISTS (SELECT 1 FROM ratings r WHERE r.deal_id = d.id)
    ORDER BY d.id
    LIMIT 200
    FOR UPDATE OF d SKIP LOCKED
)
RETURNING id;

-- What the bot needs to ask for a rating.
-- name: RatingPromptView :one
SELECT d.id, u.telegram_id, p.name AS product_name, s.display_name AS seller_name, r.name AS region_name
FROM deals d
JOIN users u ON u.id = d.buyer_id
JOIN users s ON s.id = d.seller_id
JOIN products p ON p.id = d.product_id
JOIN regions r ON r.id = d.region_id
WHERE d.id = $1;

-- name: RateDeal :one
INSERT INTO ratings (deal_id, seller_id, buyer_id, stars)
SELECT d.id, d.seller_id, d.buyer_id, @stars
FROM deals d
WHERE d.id = @deal_id AND d.buyer_id = @buyer_id::uuid AND d.status = 'confirmed'
ON CONFLICT (deal_id) DO UPDATE SET stars = EXCLUDED.stars, updated_at = now(),
    status = CASE WHEN ratings.status = 'hidden' THEN 'hidden'
                  WHEN EXCLUDED.stars <= 2 AND ratings.comment <> '' THEN 'held' ELSE 'published' END
RETURNING *;

-- A low rating's comment waits for an admin; others are shown at once.
-- name: CommentRating :one
UPDATE ratings SET comment = @comment,
       status = CASE WHEN status = 'hidden' THEN 'hidden'
                     WHEN stars <= 2 AND @comment <> '' THEN 'held' ELSE 'published' END,
       updated_at = now()
WHERE deal_id = @deal_id AND buyer_id = @buyer_id
RETURNING *;

-- The seller's average over every rating, and the latest shown comments.
-- name: SellerRating :one
SELECT count(*)::int AS ratings, COALESCE(avg(stars), 0)::float8 AS average
FROM ratings WHERE seller_id = $1;

-- name: SellerRatingComments :many
SELECT stars, comment, created_at FROM ratings
WHERE seller_id = $1 AND status = 'published' AND comment <> ''
ORDER BY created_at DESC
LIMIT 3;

-- name: HeldRatings :many
SELECT r.id, r.stars, r.comment, r.created_at, s.display_name AS seller_name, p.name AS product_name
FROM ratings r
JOIN users s ON s.id = r.seller_id
JOIN deals d ON d.id = r.deal_id
JOIN products p ON p.id = d.product_id
WHERE r.status = 'held'
ORDER BY r.created_at
LIMIT 100;

-- name: DecideRating :execrows
UPDATE ratings SET status = @status, updated_at = now() WHERE id = @id AND status = 'held';
