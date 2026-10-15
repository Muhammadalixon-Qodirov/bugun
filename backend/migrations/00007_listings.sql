-- +goose Up
-- Seller listings (marketplace view). Every seller price is a listing, linked to
-- a catalog product so prices stay comparable; price_observations keeps history.

CREATE TABLE listings (
    id          BIGSERIAL PRIMARY KEY,
    seller_id   UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    product_id  SMALLINT NOT NULL REFERENCES products (id),
    variety     TEXT NOT NULL DEFAULT '' CHECK (char_length(variety) <= 60),
    description TEXT NOT NULL DEFAULT '' CHECK (char_length(description) <= 1000),
    price_som   BIGINT NOT NULL CHECK (price_som > 0),
    unit        TEXT NOT NULL,
    min_order   NUMERIC(14, 3) CHECK (min_order > 0),
    stock       NUMERIC(14, 3) CHECK (stock > 0),
    trade_type  TEXT NOT NULL CHECK (trade_type IN ('ulgurji', 'chakana')),
    region_id   SMALLINT NOT NULL REFERENCES regions (id),
    market_id   SMALLINT REFERENCES markets (id),
    status      TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'review', 'hidden', 'rejected')),
    review_note TEXT,
    valid_until TIMESTAMPTZ NOT NULL,
    views       INTEGER NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- One listing per seller, product, variety and trade type.
CREATE UNIQUE INDEX listings_seller_product_idx ON listings (seller_id, product_id, variety, trade_type);
CREATE INDEX listings_browse_idx ON listings (product_id, region_id, price_som) WHERE status = 'active';

CREATE TABLE listing_photos (
    id         BIGSERIAL PRIMARY KEY,
    listing_id BIGINT NOT NULL REFERENCES listings (id) ON DELETE CASCADE,
    storage_key TEXT NOT NULL UNIQUE,
    width      INTEGER NOT NULL,
    height     INTEGER NOT NULL,
    position   SMALLINT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX listing_photos_listing_idx ON listing_photos (listing_id, position);

-- How buyers may reach the seller (spec: seller chooses).
ALTER TABLE users
    ADD COLUMN contact_telegram BOOLEAN NOT NULL DEFAULT TRUE,
    ADD COLUMN contact_phone    BOOLEAN NOT NULL DEFAULT FALSE;

-- Phone reveals are logged so abuse can be spotted.
CREATE TABLE contact_reveals (
    id         BIGSERIAL PRIMARY KEY,
    listing_id BIGINT NOT NULL REFERENCES listings (id) ON DELETE CASCADE,
    viewer_id  UUID REFERENCES users (id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Price history now hangs off listings: one active observation per listing.
ALTER TABLE price_observations ADD COLUMN listing_id BIGINT REFERENCES listings (id) ON DELETE SET NULL;
DROP INDEX price_obs_one_active_idx;
CREATE UNIQUE INDEX price_obs_one_active_idx ON price_observations (listing_id)
    WHERE status = 'active' AND listing_id IS NOT NULL;

-- Carry over prices confirmed before listings existed.
INSERT INTO listings (seller_id, product_id, price_som, unit, stock, trade_type, region_id, market_id, valid_until, created_at, updated_at)
SELECT DISTINCT ON (o.seller_id, o.product_id, o.trade_type)
       o.seller_id, o.product_id, o.price_som, o.unit, o.quantity, o.trade_type, o.region_id, o.market_id,
       o.valid_until, o.observed_at, o.observed_at
FROM price_observations o
WHERE o.seller_id IS NOT NULL AND o.status = 'active' AND o.valid_until > now()
ORDER BY o.seller_id, o.product_id, o.trade_type, o.observed_at DESC;

UPDATE price_observations o SET listing_id = l.id
FROM listings l
WHERE o.status = 'active' AND o.seller_id = l.seller_id AND o.product_id = l.product_id
  AND o.trade_type = l.trade_type AND l.variety = '';

-- +goose Down
DROP INDEX price_obs_one_active_idx;
ALTER TABLE price_observations DROP COLUMN listing_id;
CREATE UNIQUE INDEX price_obs_one_active_idx ON price_observations (seller_id, product_id, trade_type)
    WHERE status = 'active' AND seller_id IS NOT NULL;
DROP TABLE contact_reveals;
ALTER TABLE users DROP COLUMN contact_phone, DROP COLUMN contact_telegram;
DROP TABLE listing_photos;
DROP TABLE listings;
