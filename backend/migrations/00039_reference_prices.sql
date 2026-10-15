-- +goose Up
-- Official benchmark prices from outside sources (spec §10.1, §11: UzEx,
-- narxtahlil), one row per product and quote day. They are kept apart from
-- price_observations so they never count as offers or enter a median, a
-- minimum or a maximum; they are a separate reference line and the band of
-- the review check when a product has few sellers (ReferencePrice).
-- region_id NULL is a national price (the exchange).
CREATE TABLE reference_prices (
    id         BIGSERIAL PRIMARY KEY,
    source     TEXT NOT NULL CHECK (source IN ('uzex', 'narxtahlil')),
    product_id SMALLINT NOT NULL REFERENCES products (id) ON DELETE CASCADE,
    trade_type TEXT NOT NULL DEFAULT 'ulgurji' CHECK (trade_type IN ('ulgurji', 'chakana')),
    region_id  SMALLINT REFERENCES regions (id),
    price_som  BIGINT NOT NULL CHECK (price_som > 0), -- per the product's base unit
    quote_name TEXT NOT NULL,                        -- the item as the source names it
    quote_unit TEXT NOT NULL,                        -- the unit the source quotes in, e.g. tonna
    quote_som  NUMERIC(16, 2) NOT NULL CHECK (quote_som > 0),
    quoted_on  DATE NOT NULL,                        -- Tashkent day of the quote
    fetched_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    source_url TEXT
);
-- One quote a day per source, product, trade and region; NULL region counts as one value.
CREATE UNIQUE INDEX reference_prices_day_idx
    ON reference_prices (source, product_id, trade_type, COALESCE(region_id, 0), quoted_on);
CREATE INDEX reference_prices_lookup_idx ON reference_prices (product_id, trade_type, quoted_on DESC);

-- +goose Down
DROP TABLE reference_prices;
