-- +goose Up
-- Watches (QIZIQISH_VA_SAVDO.md step 8): a user follows a product in a
-- viloyat (or the whole country) and the bot tells them when a cheaper price
-- appears or the median falls. Adding a watch is also a demand signal.
CREATE TABLE watches (
    id              BIGSERIAL PRIMARY KEY,
    user_id         UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    product_id      SMALLINT NOT NULL REFERENCES products (id),
    province_id     SMALLINT REFERENCES regions (id), -- NULL = whole country
    trade_type      TEXT NOT NULL DEFAULT 'ulgurji' CHECK (trade_type IN ('ulgurji', 'chakana')),
    max_price       BIGINT CHECK (max_price > 0),     -- NULL = 5% under the median
    peak_median     DOUBLE PRECISION,                 -- highest median since the last alert
    checked_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    notified_at     TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX watches_once_idx ON watches (user_id, product_id, trade_type, COALESCE(province_id, 0));

ALTER TABLE demand_events DROP CONSTRAINT demand_events_kind_check;
ALTER TABLE demand_events ADD CONSTRAINT demand_events_kind_check
    CHECK (kind IN ('view', 'search', 'contact', 'rfq', 'question', 'watch'));

-- +goose Down
DELETE FROM demand_events WHERE kind = 'watch';
ALTER TABLE demand_events DROP CONSTRAINT demand_events_kind_check;
ALTER TABLE demand_events ADD CONSTRAINT demand_events_kind_check
    CHECK (kind IN ('view', 'search', 'contact', 'rfq', 'question'));
DROP TABLE watches;
