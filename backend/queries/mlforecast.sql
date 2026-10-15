-- Data for the boosted-tree forecast (PROGNOZ_ML.md §1). Weeks start on Monday.

-- Weekly price of every product, trade type and viloyat, from the cleaned
-- daily history (suspicious days are already out).
-- name: MLWeeklyPrices :many
SELECT product_id, trade_type, province_id, date_trunc('week', day)::date AS week,
       (sum(median * offers) / sum(offers))::float8 AS price,
       sum(offers)::float8 AS offers,
       (sum(demo_share * offers) / sum(offers))::float8 AS demo_share
FROM price_daily_clean
WHERE day >= sqlc.arg(from_day)::date AND offers > 0
GROUP BY 1, 2, 3, 4
ORDER BY 1, 2, 3, 4;

-- Demand signals per product, viloyat (0 = unknown) and week.
-- name: MLDemandWeekly :many
SELECT product_id, COALESCE(province_id, 0)::smallint AS province_id, date_trunc('week', day)::date AS week, count(*)::float8 AS n
FROM demand_events
WHERE day >= sqlc.arg(from_day)::date
GROUP BY 1, 2, 3;

-- Weather per viloyat and week: frost and heat days, rain and the mean low.
-- name: MLWeatherWeekly :many
SELECT province_id, date_trunc('week', day)::date AS week,
       count(*) FILTER (WHERE tmin < 0)::float8 AS frost_days,
       count(*) FILTER (WHERE tmax > 35)::float8 AS heat_days,
       COALESCE(sum(precip), 0)::float8 AS precip,
       COALESCE(avg(tmin), 0)::float8 AS tmin
FROM weather_daily
WHERE day >= sqlc.arg(from_day)::date
GROUP BY 1, 2;

-- name: MLFxWeekly :many
SELECT date_trunc('week', day)::date AS week, avg(rate)::float8 AS rate
FROM fx_daily
WHERE ccy = 'USD' AND day >= sqlc.arg(from_day)::date
GROUP BY 1
ORDER BY 1;

-- Holidays with the categories they move (the calendar is known ahead).
-- name: MLHolidays :many
SELECT title, category, starts_on, COALESCE(ends_on, starts_on)::date AS ends_on
FROM market_events
WHERE kind = 'holiday' AND category IS NOT NULL
ORDER BY starts_on;

-- name: StartMLRun :one
INSERT INTO forecast_ml_runs (kind) VALUES (sqlc.arg(kind)) RETURNING id;

-- name: FinishMLRun :exec
UPDATE forecast_ml_runs SET finished_at = now(), rows = sqlc.arg(rows), weeks = sqlc.arg(weeks), horizons = sqlc.arg(horizons),
       importance = sqlc.arg(importance), error = sqlc.narg(error)
WHERE id = sqlc.arg(id);

-- name: DeleteForecastML :exec
DELETE FROM forecast_ml;

-- name: InsertForecastML :copyfrom
INSERT INTO forecast_ml (product_id, trade_type, province_id, horizon, origin_week, base, change, drivers, run_id)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9);

-- name: DeleteMLBacktest :exec
DELETE FROM forecast_ml_backtest;

-- name: InsertMLBacktest :copyfrom
INSERT INTO forecast_ml_backtest (product_id, trade_type, province_id, origin_week, horizon, error)
VALUES ($1, $2, $3, $4, $5, $6);

-- The model's forecast of one series.
-- name: ForecastMLFor :many
SELECT horizon, origin_week, base, change, drivers FROM forecast_ml
WHERE product_id = sqlc.arg(product_id) AND trade_type = sqlc.arg(trade_type) AND province_id = sqlc.arg(province_id)
ORDER BY horizon;

-- The model's backtest errors of one series.
-- name: MLBacktestFor :many
SELECT origin_week, horizon, error FROM forecast_ml_backtest
WHERE product_id = sqlc.arg(product_id) AND trade_type = sqlc.arg(trade_type) AND province_id = sqlc.arg(province_id)
ORDER BY origin_week, horizon;

-- name: LatestMLRuns :many
SELECT DISTINCT ON (kind) id, kind, started_at, finished_at, rows, weeks, horizons, importance, error
FROM forecast_ml_runs
WHERE finished_at IS NOT NULL
ORDER BY kind, id DESC;
