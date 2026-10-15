-- +goose Up
-- Why a seller changed a price (TAHLIL_REJA.md 3.5): after a change of 10%
-- or more the bot asks and the seller picks a ready reason. One answer per
-- listing a day; the analytics page counts them by product and viloyat.
CREATE TABLE price_reasons (
    id          BIGSERIAL PRIMARY KEY,
    listing_id  BIGINT NOT NULL REFERENCES listings (id) ON DELETE CASCADE,
    product_id  SMALLINT NOT NULL REFERENCES products (id),
    province_id SMALLINT NOT NULL REFERENCES regions (id),
    direction   TEXT NOT NULL CHECK (direction IN ('up', 'down')),
    change      DOUBLE PRECISION NOT NULL,
    reason      TEXT NOT NULL CHECK (reason IN ('supply_low', 'demand_up', 'costs_up', 'export', 'supply_high',
                                                'demand_down', 'sell_fast', 'import', 'market', 'other')),
    day         DATE NOT NULL DEFAULT (now() AT TIME ZONE 'Asia/Tashkent')::date,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (listing_id, day)
);
CREATE INDEX price_reasons_product_idx ON price_reasons (product_id, province_id, created_at DESC);

-- +goose Down
DROP TABLE price_reasons;
