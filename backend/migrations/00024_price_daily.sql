-- +goose Up
-- Daily price history per product, viloyat and trade type (TAHLIL_REJA.md
-- 2.1): the analytics page reads years of history from here instead of
-- scanning every observation. A price counts on each Tashkent day it was live.
-- Within a day prices are weighted by source trust only: the freshness decay
-- of the live median would distort history.

-- +goose StatementBegin
CREATE FUNCTION source_trust(source text) RETURNS double precision
LANGUAGE sql IMMUTABLE PARALLEL SAFE AS $$
    SELECT CASE source
               WHEN 'rfq_offer' THEN 1.0
               WHEN 'deal' THEN 1.0
               WHEN 'seller' THEN 0.8
               WHEN 'telegram_channel' THEN 0.6
               WHEN 'olx' THEN 0.5
               WHEN 'seller_auto' THEN 0.4
               ELSE 0.0
           END
$$;
-- +goose StatementEnd

CREATE TABLE price_daily (
    product_id  SMALLINT NOT NULL REFERENCES products (id),
    trade_type  TEXT NOT NULL CHECK (trade_type IN ('ulgurji', 'chakana')),
    province_id SMALLINT NOT NULL REFERENCES regions (id),
    day         DATE NOT NULL,
    median      DOUBLE PRECISION NOT NULL,
    p25         DOUBLE PRECISION NOT NULL,
    p75         DOUBLE PRECISION NOT NULL,
    min_price   BIGINT NOT NULL,
    max_price   BIGINT NOT NULL,
    offers      INTEGER NOT NULL,
    demo_share  REAL NOT NULL DEFAULT 0, -- share of prices from demo sellers
    PRIMARY KEY (product_id, trade_type, province_id, day)
);
CREATE INDEX price_daily_day_idx ON price_daily (day);
CREATE INDEX price_obs_live_idx ON price_observations (observed_at, valid_until) WHERE status IN ('active', 'expired');

-- +goose Down
DROP INDEX price_obs_live_idx;
DROP TABLE price_daily;
DROP FUNCTION source_trust(text);
