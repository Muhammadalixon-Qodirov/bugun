-- RFQ flow (spec §8.4). An RFQ goes to sellers with a live listing of the same
-- product in the same viloyat; wholesale prices only.

-- Locking the buyer serialises RFQ creation, so the open-RFQ limit holds even
-- for two requests at once.
-- name: LockUser :exec
SELECT id FROM users WHERE id = $1 FOR UPDATE;

-- name: CountOpenRFQs :one
SELECT count(*)::int FROM rfqs WHERE buyer_id = $1 AND status = 'open' AND deadline > now();

-- name: CreateRFQ :one
INSERT INTO rfqs (buyer_id, product_id, quantity, unit, region_id, deadline, max_price_som, note)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: GetRFQ :one
SELECT * FROM rfqs WHERE id = $1;

-- name: CancelRFQ :execrows
UPDATE rfqs SET status = 'cancelled' WHERE id = $1 AND buyer_id = $2 AND status = 'open';

-- Sellers who should hear about an RFQ: a live listing of the product in the
-- RFQ's viloyat, never the buyer.
-- name: RFQRecipients :many
SELECT DISTINCT u.id, u.telegram_id, u.bot_enabled
FROM listings l
JOIN users u ON u.id = l.seller_id
JOIN regions r ON r.id = l.region_id
WHERE l.product_id = @product_id AND l.status = 'active' AND l.valid_until > now()
  AND COALESCE(r.parent_id, r.id) = @province_id::smallint AND u.id <> @buyer_id;

-- name: SellerCanOffer :one
SELECT EXISTS (
    SELECT 1 FROM listings l JOIN regions r ON r.id = l.region_id
    WHERE l.seller_id = @seller_id AND l.product_id = @product_id AND l.status = 'active'
      AND l.valid_until > now() AND COALESCE(r.parent_id, r.id) = @province_id::smallint
)::bool;

-- A seller may change an offer (or a "Menda yo'q") until the buyer picks it.
-- name: UpsertOffer :one
INSERT INTO offers (rfq_id, seller_id, price_som, quantity, status)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (rfq_id, seller_id) DO UPDATE SET
    price_som = EXCLUDED.price_som, quantity = EXCLUDED.quantity,
    status = EXCLUDED.status, updated_at = now()
WHERE offers.status IN ('sent', 'declined')
RETURNING *;

-- name: ExpireOfferObservation :exec
UPDATE price_observations SET status = 'expired', valid_until = LEAST(valid_until, now())
WHERE offer_id = $1 AND status IN ('active', 'review');

-- name: InsertOfferObservation :exec
INSERT INTO price_observations (
    product_id, price_som, unit, quantity, trade_type, region_id, market_id,
    source, seller_id, offer_id, valid_until, status
) VALUES ($1, $2, $3, $4, 'ulgurji', $5, $6, 'rfq_offer', $7, $8, $9, $10);

-- name: GetOfferForUpdate :one
SELECT o.*, r.buyer_id, r.product_id, r.status AS rfq_status, r.deadline
FROM offers o JOIN rfqs r ON r.id = o.rfq_id
WHERE o.id = $1
FOR UPDATE OF o;

-- name: SetOfferStatus :exec
UPDATE offers SET status = $2, updated_at = now() WHERE id = $1;

-- The buyer's RFQs, newest first, with how many sellers answered and the best price.
-- name: BuyerRFQs :many
SELECT q.*, p.name AS product_name, p.slug AS product_slug, reg.name AS region_name,
       count(o.id) FILTER (WHERE o.status <> 'declined')::int AS offers,
       COALESCE(min(o.price_som) FILTER (WHERE o.status <> 'declined'), 0)::bigint AS best_price
FROM rfqs q
JOIN products p ON p.id = q.product_id
JOIN regions reg ON reg.id = q.region_id
LEFT JOIN offers o ON o.rfq_id = q.id
WHERE q.buyer_id = $1
GROUP BY q.id, p.name, p.slug, reg.name
ORDER BY q.created_at DESC
LIMIT 50;

-- Offers on one RFQ, cheapest first. Contact details are only read once both
-- sides agreed (status confirmed).
-- name: RFQOffers :many
SELECT o.id, o.price_som, o.quantity, o.status, o.created_at, o.updated_at,
       COALESCE(NULLIF(u.public_name, ''), split_part(u.display_name, ' ', 1))::text AS seller_name,
       sr.name AS seller_region, COALESCE(m.name, '')::text AS seller_market,
       u.telegram_username, u.contact_telegram, u.contact_phone, u.phone_enc
FROM offers o
JOIN users u ON u.id = o.seller_id
LEFT JOIN regions sr ON sr.id = u.region_id
LEFT JOIN markets m ON m.id = u.market_id
WHERE o.rfq_id = $1 AND o.status <> 'declined'
ORDER BY o.price_som NULLS LAST, o.created_at;

-- Open RFQs a seller can answer, with the seller's own answer if any, plus any
-- RFQ where the buyer picked this seller's offer (chosen or confirmed), even
-- after its deadline, so the seller can still confirm and see the contact.
-- The buyer's name stays hidden until both sides agree.
-- name: SellerInbox :many
SELECT q.id, q.product_id, q.quantity, q.unit, q.deadline, q.max_price_som, q.note, q.created_at, q.status,
       p.name AS product_name, reg.name AS region_name,
       o.id AS offer_id, o.status AS offer_status, o.price_som AS offer_price
FROM rfqs q
JOIN products p ON p.id = q.product_id
JOIN regions reg ON reg.id = q.region_id
LEFT JOIN offers o ON o.rfq_id = q.id AND o.seller_id = @seller_id
WHERE o.status IN ('chosen', 'confirmed')
   OR (q.status = 'open' AND q.deadline > now() AND q.buyer_id <> @seller_id
  AND EXISTS (
      SELECT 1 FROM listings l JOIN regions lr ON lr.id = l.region_id
      WHERE l.seller_id = @seller_id AND l.product_id = q.product_id AND l.status = 'active'
        AND l.valid_until > now() AND COALESCE(lr.parent_id, lr.id) = COALESCE(reg.parent_id, reg.id)))
ORDER BY (o.status = 'chosen') DESC NULLS LAST, q.created_at DESC
LIMIT 50;

-- Both parties of an offer with their contact settings, for the exchange
-- after confirmation.
-- name: OfferParties :one
SELECT o.id, o.status, o.rfq_id, r.buyer_id, o.seller_id,
       b.telegram_username AS buyer_username, b.contact_telegram AS buyer_contact_telegram,
       b.contact_phone AS buyer_contact_phone, b.phone_enc AS buyer_phone_enc,
       COALESCE(NULLIF(b.public_name, ''), b.display_name)::text AS buyer_name,
       s.telegram_username AS seller_username, s.contact_telegram AS seller_contact_telegram,
       s.contact_phone AS seller_contact_phone, s.phone_enc AS seller_phone_enc,
       COALESCE(NULLIF(s.public_name, ''), s.display_name)::text AS seller_name
FROM offers o
JOIN rfqs r ON r.id = o.rfq_id
JOIN users b ON b.id = r.buyer_id
JOIN users s ON s.id = o.seller_id
WHERE o.id = $1;
