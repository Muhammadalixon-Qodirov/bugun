-- Deals (QIZIQISH_VA_SAVDO.md).

-- The seller agreed to a picked RFQ offer: both sides have said yes, so the
-- deal is confirmed at once. A repeated call adds nothing.
-- name: RecordRFQDeal :exec
INSERT INTO deals (listing_id, offer_id, product_id, seller_id, buyer_id, source, status,
                   buyer_confirmed_at, seller_confirmed_at, quantity, unit, price_som, region_id)
SELECT (SELECT l.id FROM listings l
        WHERE l.seller_id = o.seller_id AND l.product_id = r.product_id AND l.trade_type = 'ulgurji'
        ORDER BY l.updated_at DESC LIMIT 1),
       o.id, r.product_id, o.seller_id, r.buyer_id, 'rfq', 'confirmed',
       now(), now(), COALESCE(o.quantity, r.quantity), r.unit, o.price_som, r.region_id
FROM offers o
JOIN rfqs r ON r.id = o.rfq_id
WHERE o.id = $1
ON CONFLICT (offer_id) DO NOTHING;

-- A contact reveal schedules a question for the buyer a day later, unless
-- they were asked about this listing in the last 7 days or one is open.
-- name: ScheduleDealPrompt :exec
INSERT INTO deal_prompts (listing_id, buyer_id, seller_id, due_at)
SELECT l.id, @buyer_id, l.seller_id, @due_at
FROM listings l
WHERE l.id = @listing_id AND l.seller_id <> @buyer_id
  AND NOT EXISTS (SELECT 1 FROM deal_prompts p
                  WHERE p.listing_id = l.id AND p.buyer_id = @buyer_id AND p.created_at > now() - interval '7 days')
ON CONFLICT (listing_id, buyer_id) WHERE answer IS NULL OR answer = 'talking' DO NOTHING;

-- Questions whose time has come, locked so a second server skips them.
-- name: LockDueDealPrompts :many
SELECT id, buyer_id FROM deal_prompts
WHERE sent_at IS NULL AND due_at <= now() AND (answer IS NULL OR answer = 'talking')
ORDER BY due_at, id
LIMIT 500
FOR UPDATE SKIP LOCKED;

-- How many questions each buyer got since the Tashkent day began.
-- name: DealPromptsSentToday :many
SELECT buyer_id, count(*)::int AS n FROM deal_prompts
WHERE buyer_id = ANY(@buyer_ids::uuid[])
  AND sent_at >= date_trunc('day', now() AT TIME ZONE 'Asia/Tashkent') AT TIME ZONE 'Asia/Tashkent'
GROUP BY buyer_id;

-- name: MarkDealPromptsSent :exec
UPDATE deal_prompts SET sent_at = now(), asks = asks + 1 WHERE id = ANY(@ids::bigint[]);

-- Questions not sent within 3 days of their time are dropped.
-- name: ExpireDealPrompts :execrows
UPDATE deal_prompts SET answer = 'expired', answered_at = now()
WHERE sent_at IS NULL AND due_at < now() - interval '3 days' AND (answer IS NULL OR answer = 'talking');

-- What the bot needs to ask one question.
-- name: DealPromptView :one
SELECT p.id, p.asks, p.created_at, l.id AS listing_id, l.unit, l.variety, pr.name AS product_name,
       s.display_name AS seller_name, r.name AS region_name, u.telegram_id, u.bot_enabled
FROM deal_prompts p
JOIN listings l ON l.id = p.listing_id
JOIN products pr ON pr.id = l.product_id
JOIN users s ON s.id = p.seller_id
JOIN users u ON u.id = p.buyer_id
JOIN regions r ON r.id = l.region_id
WHERE p.id = $1;

-- name: GetDealPromptForUpdate :one
SELECT * FROM deal_prompts WHERE id = $1 AND buyer_id = $2 FOR UPDATE;

-- name: AnswerDealPrompt :exec
UPDATE deal_prompts SET answer = @answer::text, answered_at = now(), deal_id = sqlc.narg(deal_id),
       sent_at = CASE WHEN @answer::text = 'talking' THEN NULL ELSE sent_at END,
       due_at = CASE WHEN @answer::text = 'talking' THEN now() + interval '48 hours' ELSE due_at END
WHERE id = @id;

-- The buyer's own report of a deal from a contact: one per listing a day.
-- name: RecordBuyerContactDeal :one
INSERT INTO deals (listing_id, product_id, seller_id, buyer_id, source, status, buyer_confirmed_at, unit, region_id)
SELECT l.id, l.product_id, l.seller_id, @buyer_id::uuid, 'contact', 'reported', now(), l.unit, l.region_id
FROM listings l WHERE l.id = @listing_id
ON CONFLICT (listing_id, buyer_id, day) WHERE source = 'contact' DO UPDATE SET updated_at = now()
RETURNING *;

-- name: GetDealForUpdate :one
SELECT * FROM deals WHERE id = $1 FOR UPDATE;

-- name: SetDealDetails :one
UPDATE deals SET quantity = sqlc.narg(quantity), price_som = sqlc.narg(price_som), updated_at = now()
WHERE id = @id
RETURNING *;

-- name: GetDeal :one
SELECT * FROM deals WHERE id = $1;

-- Today's contact reveals per listing for the sellers' evening question.
-- name: TodaysContactsBySeller :many
SELECT l.seller_id, u.telegram_id, l.id AS listing_id, p.name AS product_name, l.variety, count(*)::int AS contacts
FROM listing_events e
JOIN listings l ON l.id = e.listing_id
JOIN products p ON p.id = l.product_id
JOIN users u ON u.id = l.seller_id
WHERE e.kind = 'contact' AND e.day = (now() AT TIME ZONE 'Asia/Tashkent')::date AND u.bot_enabled
GROUP BY l.seller_id, u.telegram_id, l.id, p.name, l.variety
ORDER BY l.seller_id, contacts DESC, l.id;

-- The oldest deal on the listing a buyer reported in the last week and the
-- seller has not confirmed yet.
-- name: LockBuyerReportedDeal :one
SELECT * FROM deals
WHERE listing_id = $1 AND status = 'reported' AND buyer_confirmed_at IS NOT NULL AND seller_confirmed_at IS NULL
  AND created_at > now() - interval '7 days'
ORDER BY created_at, id
LIMIT 1
FOR UPDATE;

-- The oldest sale on the listing the seller reported in the last week (the
-- evening question or the "sold" button) with no buyer yet.
-- name: LockSellerReportedDeal :one
SELECT * FROM deals
WHERE listing_id = $1 AND source IN ('contact', 'seller_marked') AND status = 'reported' AND buyer_id IS NULL AND seller_confirmed_at IS NOT NULL
  AND created_at > now() - interval '7 days'
ORDER BY created_at, id
LIMIT 1
FOR UPDATE;

-- name: ConfirmDealBySeller :one
UPDATE deals SET seller_confirmed_at = now(), status = 'confirmed', updated_at = now()
WHERE id = $1
RETURNING *;

-- name: ConfirmDealByBuyer :one
UPDATE deals SET buyer_id = @buyer_id::uuid, buyer_confirmed_at = now(), status = 'confirmed', updated_at = now()
WHERE id = @id
RETURNING *;

-- A sale the seller reports from a contact, before or without the buyer.
-- name: RecordSellerContactDeal :one
INSERT INTO deals (listing_id, product_id, seller_id, source, status, seller_confirmed_at, unit, region_id)
SELECT l.id, l.product_id, l.seller_id, 'contact', 'reported', now(), l.unit, l.region_id
FROM listings l WHERE l.id = $1
RETURNING *;

-- The seller's "sold" button on a listing.
-- name: RecordSellerMarkedDeal :one
INSERT INTO deals (listing_id, product_id, seller_id, source, status, seller_confirmed_at, quantity, unit, price_som, region_id)
SELECT l.id, l.product_id, l.seller_id, 'seller_marked', 'reported', now(), @quantity, l.unit, sqlc.narg(price_som), l.region_id
FROM listings l WHERE l.id = @listing_id
RETURNING *;

-- name: SetListingStockAndStatus :one
UPDATE listings SET stock = sqlc.narg(stock), status = @status, updated_at = now()
WHERE id = @id
RETURNING *;

-- A deal's price in the median (QIZIQISH_VA_SAVDO.md step 7): the old row is
-- retired, and a confirmed, priced, non-RFQ deal gets a fresh one valid for
-- 7 days. A price outside 0.5–2 × the listing's price is held for review
-- (a typo must not move the median).
-- name: ExpireDealObservation :exec
UPDATE price_observations SET status = 'expired', valid_until = LEAST(valid_until, now())
WHERE deal_id = $1 AND status IN ('active', 'review');

-- name: InsertDealObservation :exec
INSERT INTO price_observations (product_id, price_som, unit, quantity, trade_type, region_id,
                                source, seller_id, deal_id, valid_until, status)
SELECT d.product_id, d.price_som, d.unit, d.quantity, COALESCE(l.trade_type, 'ulgurji'), d.region_id,
       'deal', d.seller_id, d.id, now() + interval '7 days',
       CASE WHEN l.id IS NULL OR d.price_som BETWEEN l.price_som / 2 AND l.price_som * 2 THEN 'active' ELSE 'review' END
FROM deals d
LEFT JOIN listings l ON l.id = d.listing_id
WHERE d.id = $1 AND d.status = 'confirmed' AND d.price_som IS NOT NULL AND d.source <> 'rfq';
