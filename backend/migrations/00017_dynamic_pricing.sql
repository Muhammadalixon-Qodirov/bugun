-- +goose Up
-- Dynamic pricing (DINAMIK_NARX.md, spec 14.4 decision of 2026-09-25): a seller
-- may let the system move a listing's price within their own min–max range.
ALTER TABLE listings
    ADD COLUMN price_min       BIGINT CHECK (price_min > 0),
    ADD COLUMN price_max       BIGINT CHECK (price_max > 0),
    ADD COLUMN pricing_mode    TEXT NOT NULL DEFAULT 'manual' CHECK (pricing_mode IN ('manual', 'auto')),
    ADD COLUMN pricing_goal    TEXT NOT NULL DEFAULT 'balanced' CHECK (pricing_goal IN ('fast', 'balanced', 'profit')),
    -- When the seller last set the price by hand; auto pricing leaves it alone for 24 hours.
    ADD COLUMN manual_price_at TIMESTAMPTZ,
    ADD CONSTRAINT listings_auto_needs_bounds CHECK (
        pricing_mode <> 'auto' OR (price_min IS NOT NULL AND price_max IS NOT NULL AND price_min <= price_max));

-- Every automatic change with its reasons, so the seller and admins can see why.
CREATE TABLE price_changes (
    id          BIGSERIAL PRIMARY KEY,
    listing_id  BIGINT NOT NULL REFERENCES listings (id) ON DELETE CASCADE,
    old_price   BIGINT NOT NULL,
    new_price   BIGINT NOT NULL,
    status      TEXT NOT NULL DEFAULT 'applied' CHECK (status IN ('applied', 'reverted')),
    reasons     JSONB NOT NULL DEFAULT '[]',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    reverted_at TIMESTAMPTZ
);
CREATE INDEX price_changes_listing_idx ON price_changes (listing_id, created_at DESC);

-- Automatic prices are recorded as seller_auto and weigh half a seller's own
-- price in the median, so the system cannot drive the median it follows.
ALTER TABLE price_observations DROP CONSTRAINT price_observations_source_check;
ALTER TABLE price_observations ADD CONSTRAINT price_observations_source_check CHECK (source IN
    ('seller', 'seller_auto', 'rfq_offer', 'telegram_channel', 'olx', 'uzex', 'narxtahlil'));

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION price_weight(source text, observed_at timestamptz) RETURNS double precision
LANGUAGE sql STABLE PARALLEL SAFE AS $$
    SELECT CASE source
               WHEN 'rfq_offer' THEN 1.0
               WHEN 'seller' THEN 0.8
               WHEN 'telegram_channel' THEN 0.6
               WHEN 'olx' THEN 0.5
               WHEN 'seller_auto' THEN 0.4
               ELSE 0.0
           END * exp(-0.2 * GREATEST(extract(epoch FROM now() - observed_at), 0) / 86400)
$$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION price_weight(source text, observed_at timestamptz) RETURNS double precision
LANGUAGE sql STABLE PARALLEL SAFE AS $$
    SELECT CASE source
               WHEN 'rfq_offer' THEN 1.0
               WHEN 'seller' THEN 0.8
               WHEN 'telegram_channel' THEN 0.6
               WHEN 'olx' THEN 0.5
               ELSE 0.0
           END * exp(-0.2 * GREATEST(extract(epoch FROM now() - observed_at), 0) / 86400)
$$;
-- +goose StatementEnd
UPDATE price_observations SET source = 'seller' WHERE source = 'seller_auto';
ALTER TABLE price_observations DROP CONSTRAINT price_observations_source_check;
ALTER TABLE price_observations ADD CONSTRAINT price_observations_source_check CHECK (source IN
    ('seller', 'rfq_offer', 'telegram_channel', 'olx', 'uzex', 'narxtahlil'));
DROP TABLE price_changes;
ALTER TABLE listings DROP CONSTRAINT listings_auto_needs_bounds,
    DROP COLUMN manual_price_at, DROP COLUMN pricing_goal, DROP COLUMN pricing_mode,
    DROP COLUMN price_max, DROP COLUMN price_min;
