-- +goose Up
-- Demand signals for the pressure index (spec §10.3): product page views,
-- searches, contact reveals, RFQs and questions. A visitor counts once per
-- product, kind and day (Asia/Tashkent). viewer_key is a keyed hash of the
-- user id or IP together with the day, so no IP is stored and visitors cannot
-- be followed across days.
CREATE TABLE demand_events (
    id          BIGSERIAL PRIMARY KEY,
    kind        TEXT NOT NULL CHECK (kind IN ('view', 'search', 'contact', 'rfq', 'question')),
    product_id  SMALLINT NOT NULL REFERENCES products (id),
    province_id SMALLINT REFERENCES regions (id),
    viewer_key  BYTEA NOT NULL,
    day         DATE NOT NULL DEFAULT (now() AT TIME ZONE 'Asia/Tashkent')::date,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX demand_events_once_idx ON demand_events (kind, product_id, viewer_key, day);
CREATE INDEX demand_events_product_idx ON demand_events (product_id, province_id, day);

-- +goose Down
DROP TABLE demand_events;
