-- +goose Up
-- Suspicious viloyat-days in the daily history (ML_REJA.md 4). An open flag
-- that is settled (four later days were seen) or a flag an admin confirmed
-- keeps the day out of price_daily_clean, which every chart and statistic
-- reads; a dismissed flag puts it back. Provisional flags only mark the day.
CREATE TABLE price_anomalies (
    product_id  SMALLINT NOT NULL REFERENCES products (id),
    trade_type  TEXT NOT NULL,
    province_id SMALLINT NOT NULL REFERENCES regions (id),
    day         DATE NOT NULL,
    layer       TEXT NOT NULL CHECK (layer IN ('temporal', 'cross_section')),
    value       DOUBLE PRECISION NOT NULL,
    reference   DOUBLE PRECISION NOT NULL,
    z           DOUBLE PRECISION NOT NULL,
    settled     BOOLEAN NOT NULL DEFAULT FALSE,
    status      TEXT NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'confirmed', 'dismissed')),
    found_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    decided_at  TIMESTAMPTZ,
    decided_by  UUID REFERENCES users (id) ON DELETE SET NULL,
    PRIMARY KEY (product_id, trade_type, province_id, day, layer)
);
CREATE INDEX price_anomalies_status_idx ON price_anomalies (status, day DESC);

CREATE VIEW price_daily_clean AS
SELECT d.*
FROM price_daily d
WHERE NOT EXISTS (
    SELECT 1 FROM price_anomalies a
    WHERE a.product_id = d.product_id AND a.trade_type = d.trade_type AND a.province_id = d.province_id AND a.day = d.day
      AND (a.status = 'confirmed' OR (a.status = 'open' AND a.settled))
);

-- +goose Down
DROP VIEW price_daily_clean;
DROP TABLE price_anomalies;
