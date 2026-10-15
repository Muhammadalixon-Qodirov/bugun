-- Automatic pricing (DINAMIK_NARX.md). A listing is repriced only while it is
-- live, in auto mode and its price was not set by hand within the pause
-- (AUTO_PRICE_MANUAL_PAUSE, 24 hours by default).

-- name: AutoPricedListings :many
SELECT id FROM listings
WHERE pricing_mode = 'auto' AND status = 'active' AND valid_until > now()
  AND (manual_price_at IS NULL OR manual_price_at < now() - make_interval(secs => sqlc.arg(pause_seconds)::float8))
ORDER BY id;

-- Locks the listing for one decision; no row when it no longer qualifies.
-- name: LockAutoPricedListing :one
SELECT * FROM listings
WHERE id = sqlc.arg(id) AND pricing_mode = 'auto' AND status = 'active' AND valid_until > now()
  AND (manual_price_at IS NULL OR manual_price_at < now() - make_interval(secs => sqlc.arg(pause_seconds)::float8))
FOR UPDATE;

-- Other sellers' prices in the viloyat as they stood a week ago, for the
-- market trend. Weights share one decay factor, so the median is unaffected.
-- name: PeerPricesWeekAgo :many
SELECT o.price_som::float8 AS price, price_weight(o.source, o.observed_at)::float8 AS weight
FROM price_observations o
JOIN regions r ON r.id = o.region_id
WHERE o.product_id = @product_id AND o.trade_type = @trade_type AND o.status IN ('active', 'expired')
  AND o.observed_at <= now() - interval '7 days' AND o.valid_until > now() - interval '7 days'
  AND COALESCE(r.parent_id, r.id) = @province_id::smallint
  AND o.listing_id IS DISTINCT FROM @listing_id::bigint;

-- The listing's price when the Tashkent day began: the old price of today's
-- first automatic change, 0 when there was none today.
-- name: DayStartPrice :one
SELECT COALESCE((
    SELECT old_price FROM price_changes
    WHERE listing_id = $1
      AND created_at >= date_trunc('day', now() AT TIME ZONE 'Asia/Tashkent') AT TIME ZONE 'Asia/Tashkent'
    ORDER BY created_at, id
    LIMIT 1
), 0)::bigint AS price;

-- The seller's answers to finished RFQs for this product in the last 30 days
-- and how many a buyer picked. Open RFQs are left out: nobody has chosen yet.
-- name: SellerRFQOutcomes :one
SELECT count(*)::int AS offers,
       count(*) FILTER (WHERE f.status IN ('chosen', 'confirmed'))::int AS picked
FROM offers f
JOIN rfqs r ON r.id = f.rfq_id
WHERE f.seller_id = $1 AND r.product_id = $2 AND f.status <> 'declined'
  AND f.created_at > now() - interval '30 days'
  AND (r.status <> 'open' OR r.deadline < now() OR f.status <> 'sent');

-- Median stock of other live listings of the product in the viloyat with the
-- same unit.
-- name: PeerStock :one
SELECT count(*)::int AS n,
       COALESCE(percentile_cont(0.5) WITHIN GROUP (ORDER BY l.stock), 0)::float8 AS median
FROM listings l
JOIN regions r ON r.id = l.region_id
WHERE l.product_id = @product_id AND l.trade_type = @trade_type AND l.unit = @unit
  AND l.status = 'active' AND l.valid_until > now() AND l.stock > 0 AND l.id <> @listing_id
  AND COALESCE(r.parent_id, r.id) = @province_id::smallint;

-- Sets an automatic price; manual_price_at stays as it was.
-- name: SetAutoPrice :one
UPDATE listings SET price_som = $2, updated_at = now() WHERE id = $1 RETURNING *;

-- name: InsertPriceChange :one
INSERT INTO price_changes (listing_id, old_price, new_price, reasons)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- An automatic change of one of the seller's listings, locked for undo.
-- name: GetPriceChangeForUpdate :one
SELECT c.* FROM price_changes c
JOIN listings l ON l.id = c.listing_id
WHERE c.id = $1 AND l.seller_id = $2
FOR UPDATE OF c;

-- Undo puts the old price back as the seller's own, so automatic pricing
-- pauses (AUTO_PRICE_MANUAL_PAUSE).
-- name: RevertAutoPrice :one
UPDATE listings SET price_som = $2, manual_price_at = now(), updated_at = now()
WHERE id = $1
RETURNING *;

-- name: MarkPriceChangeReverted :exec
UPDATE price_changes SET status = 'reverted', reverted_at = now() WHERE id = $1;

-- A listing's automatic changes, newest first.
-- name: ListPriceChanges :many
SELECT * FROM price_changes WHERE listing_id = $1 ORDER BY created_at DESC, id DESC LIMIT 30;

-- Unique viewers and contact reveals in the last 7 days of the listing and,
-- for comparison, of the other listings of the product in the viloyat.
-- name: ResponseCounts :one
WITH peers AS (
    SELECT o.id FROM listings o
    JOIN regions r ON r.id = o.region_id
    WHERE o.product_id = @product_id AND o.trade_type = @trade_type AND o.id <> @listing_id
      AND COALESCE(r.parent_id, r.id) = @province_id::smallint
), recent AS (
    SELECT e.listing_id, e.kind FROM listing_events e
    WHERE e.created_at > now() - interval '7 days'
      AND (e.listing_id = @listing_id OR e.listing_id IN (SELECT id FROM peers))
)
SELECT count(*) FILTER (WHERE listing_id = @listing_id AND kind = 'view')::int AS views,
       count(*) FILTER (WHERE listing_id = @listing_id AND kind = 'contact')::int AS contacts,
       count(*) FILTER (WHERE listing_id <> @listing_id AND kind = 'view')::int AS market_views,
       count(*) FILTER (WHERE listing_id <> @listing_id AND kind = 'contact')::int AS market_contacts
FROM recent;

-- Sales in the last 7 days (reported or confirmed) of the listing and of the
-- other live listings of the product in the viloyat.
-- name: SalesCounts :one
WITH peers AS (
    SELECT o.id FROM listings o
    JOIN regions r ON r.id = o.region_id
    WHERE o.product_id = @product_id AND o.trade_type = @trade_type AND o.id <> @listing_id::bigint
      AND o.status = 'active' AND COALESCE(r.parent_id, r.id) = @province_id::smallint
)
SELECT (SELECT count(*) FROM deals d
        WHERE d.listing_id = @listing_id::bigint AND d.status IN ('reported', 'confirmed') AND d.created_at > now() - interval '7 days')::int AS sales,
       (SELECT count(*) FROM peers)::int AS peers,
       (SELECT count(*) FROM deals d
        WHERE d.listing_id IN (SELECT id FROM peers) AND d.status IN ('reported', 'confirmed') AND d.created_at > now() - interval '7 days')::int AS peer_sales;
