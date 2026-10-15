-- +goose Up
-- Deals (QIZIQISH_VA_SAVDO.md): a sale is recorded only when someone says it
-- happened. A contact reveal is interest, never a deal. A deal is confirmed
-- when both sides say so; an RFQ offer the seller agreed to is confirmed at once.
CREATE TABLE deals (
    id                  BIGSERIAL PRIMARY KEY,
    listing_id          BIGINT REFERENCES listings (id) ON DELETE SET NULL,
    offer_id            BIGINT UNIQUE REFERENCES offers (id) ON DELETE SET NULL,
    product_id          SMALLINT NOT NULL REFERENCES products (id),
    seller_id           UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    buyer_id            UUID REFERENCES users (id) ON DELETE SET NULL,
    source              TEXT NOT NULL CHECK (source IN ('rfq', 'contact', 'seller_marked')),
    status              TEXT NOT NULL DEFAULT 'reported' CHECK (status IN ('reported', 'confirmed', 'disputed')),
    buyer_confirmed_at  TIMESTAMPTZ,
    seller_confirmed_at TIMESTAMPTZ,
    quantity            NUMERIC(14, 3) CHECK (quantity > 0),
    unit                TEXT NOT NULL,
    price_som           BIGINT CHECK (price_som > 0),
    region_id           SMALLINT NOT NULL REFERENCES regions (id),
    day                 DATE NOT NULL DEFAULT (now() AT TIME ZONE 'Asia/Tashkent')::date,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT deals_not_with_oneself CHECK (buyer_id IS DISTINCT FROM seller_id),
    CONSTRAINT deals_confirmed_by_both CHECK (
        status <> 'confirmed' OR (buyer_confirmed_at IS NOT NULL AND seller_confirmed_at IS NOT NULL))
);
-- One deal per buyer, listing and Tashkent day from contact follow-ups.
CREATE UNIQUE INDEX deals_contact_once_a_day_idx ON deals (listing_id, buyer_id, day) WHERE source = 'contact';
CREATE INDEX deals_market_idx ON deals (product_id, region_id, created_at DESC) WHERE status = 'confirmed';
CREATE INDEX deals_seller_idx ON deals (seller_id, created_at DESC);

-- Offers sellers already agreed to.
INSERT INTO deals (listing_id, offer_id, product_id, seller_id, buyer_id, source, status,
                   buyer_confirmed_at, seller_confirmed_at, quantity, unit, price_som, region_id, day, created_at, updated_at)
SELECT (SELECT l.id FROM listings l
        WHERE l.seller_id = o.seller_id AND l.product_id = r.product_id AND l.trade_type = 'ulgurji'
        ORDER BY l.updated_at DESC LIMIT 1),
       o.id, r.product_id, o.seller_id, r.buyer_id, 'rfq', 'confirmed',
       o.updated_at, o.updated_at, COALESCE(o.quantity, r.quantity), r.unit, o.price_som, r.region_id,
       (o.updated_at AT TIME ZONE 'Asia/Tashkent')::date, o.updated_at, o.updated_at
FROM offers o
JOIN rfqs r ON r.id = o.rfq_id
WHERE o.status = 'confirmed' AND r.buyer_id <> o.seller_id;

-- +goose Down
DROP TABLE deals;
