-- +goose Up
-- Daily weather at each viloyat centre from Open-Meteo (TAHLIL_REJA.md 3.2),
-- so frosts and heat waves can be read beside prices.
CREATE TABLE weather_daily (
    province_id SMALLINT NOT NULL REFERENCES regions (id),
    day         DATE NOT NULL,
    tmin        DOUBLE PRECISION,
    tmax        DOUBLE PRECISION,
    precip      DOUBLE PRECISION,
    PRIMARY KEY (province_id, day)
);

-- +goose Down
DROP TABLE weather_daily;
