-- +goose Up
-- The boosted-tree forecast (PROGNOZ_ML.md): the nightly model's forecast of
-- every series 1–4 weeks ahead with its drivers, each run's figures, and the
-- walk-forward backtest errors the analytics page compares with the other
-- methods on the same weeks. province_id 0 is the whole country.
CREATE TABLE forecast_ml_runs (
    id          BIGSERIAL PRIMARY KEY,
    kind        TEXT NOT NULL CHECK (kind IN ('train', 'backtest')),
    started_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    finished_at TIMESTAMPTZ,
    rows        INTEGER NOT NULL DEFAULT 0,
    weeks       INTEGER NOT NULL DEFAULT 0,
    horizons    JSONB NOT NULL DEFAULT '[]', -- per horizon: trees, train and validation rows and errors
    importance  JSONB NOT NULL DEFAULT '{}', -- feature groups and features, summed to 1
    error       TEXT
);

CREATE TABLE forecast_ml (
    product_id  SMALLINT NOT NULL REFERENCES products (id),
    trade_type  TEXT NOT NULL,
    province_id SMALLINT NOT NULL,
    horizon     SMALLINT NOT NULL CHECK (horizon BETWEEN 1 AND 4),
    origin_week DATE NOT NULL, -- Monday of the last week the forecast starts from
    base        DOUBLE PRECISION NOT NULL, -- that week's price
    change      DOUBLE PRECISION NOT NULL, -- forecast log change
    drivers     JSONB NOT NULL, -- driver group -> share of the base price
    run_id      BIGINT NOT NULL REFERENCES forecast_ml_runs (id) ON DELETE CASCADE,
    PRIMARY KEY (product_id, trade_type, province_id, horizon)
);

CREATE TABLE forecast_ml_backtest (
    product_id  SMALLINT NOT NULL REFERENCES products (id),
    trade_type  TEXT NOT NULL,
    province_id SMALLINT NOT NULL,
    origin_week DATE NOT NULL,
    horizon     SMALLINT NOT NULL CHECK (horizon BETWEEN 1 AND 4),
    error       DOUBLE PRECISION NOT NULL, -- |forecast / actual - 1|
    PRIMARY KEY (product_id, trade_type, province_id, origin_week, horizon)
);

-- +goose Down
DROP TABLE forecast_ml_backtest;
DROP TABLE forecast_ml;
DROP TABLE forecast_ml_runs;
