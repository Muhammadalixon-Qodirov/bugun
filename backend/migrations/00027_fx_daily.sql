-- +goose Up
-- Daily exchange rates from the Central Bank (cbu.uz), so som against the
-- dollar, rouble and tenge can be read next to prices (TAHLIL_REJA.md 3.1).
-- rate is so'm for one unit of the currency.
CREATE TABLE fx_daily (
    day  DATE NOT NULL,
    ccy  TEXT NOT NULL CHECK (ccy IN ('USD', 'RUB', 'KZT')),
    rate DOUBLE PRECISION NOT NULL CHECK (rate > 0),
    PRIMARY KEY (ccy, day)
);

-- +goose Down
DROP TABLE fx_daily;
