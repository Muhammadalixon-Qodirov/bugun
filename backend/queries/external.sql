-- External data (TAHLIL_REJA.md §9).

-- Days from..to that have no dollar rate yet, oldest first.
-- name: MissingFxDays :many
SELECT d::date AS day
FROM generate_series(sqlc.arg(from_day)::date, sqlc.arg(to_day)::date, interval '1 day') AS d
WHERE NOT EXISTS (SELECT 1 FROM fx_daily f WHERE f.ccy = 'USD' AND f.day = d::date)
ORDER BY d DESC; -- newest first: recent rates matter most if a run stops early

-- name: UpsertFx :exec
INSERT INTO fx_daily (day, ccy, rate) VALUES ($1, $2, $3)
ON CONFLICT (ccy, day) DO UPDATE SET rate = EXCLUDED.rate;

-- A currency's rate over days from..to, bucketed like the price series.
-- name: FxSeries :many
SELECT date_trunc(sqlc.arg(bucket)::text, f.day)::date AS day, avg(f.rate)::float8 AS rate
FROM fx_daily f
WHERE f.ccy = sqlc.arg(ccy) AND f.day BETWEEN sqlc.arg(from_day)::date AND sqlc.arg(to_day)::date
GROUP BY 1
ORDER BY 1;

-- The last day with weather for a viloyat, or NULL.
-- name: LastWeatherDay :one
SELECT max(day)::date AS day FROM weather_daily WHERE province_id = $1;

-- name: UpsertWeather :exec
INSERT INTO weather_daily (province_id, day, tmin, tmax, precip) VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (province_id, day) DO UPDATE SET tmin = EXCLUDED.tmin, tmax = EXCLUDED.tmax, precip = EXCLUDED.precip;

-- Weather over days from..to for a viloyat, or the mean of all viloyatlar,
-- bucketed: the lowest minimum, highest maximum and total rain.
-- name: WeatherSeries :many
WITH per_day AS (
    SELECT w.day, avg(w.tmin) AS tmin, avg(w.tmax) AS tmax, avg(w.precip) AS precip
    FROM weather_daily w
    WHERE w.day BETWEEN sqlc.arg(from_day)::date AND sqlc.arg(to_day)::date
      AND (sqlc.narg(province_id)::smallint IS NULL OR w.province_id = sqlc.narg(province_id))
    GROUP BY w.day
)
SELECT date_trunc(sqlc.arg(bucket)::text, day)::date AS day,
       COALESCE(min(tmin), 0)::float8 AS tmin, COALESCE(max(tmax), 0)::float8 AS tmax, COALESCE(sum(precip), 0)::float8 AS precip
FROM per_day
GROUP BY 1
ORDER BY 1;

-- Frost and heat days over days from..to: frost at -5°C or below, or 0°C
-- or below in March–May (blossom and seedlings); heat at 40°C or above.
-- name: WeatherExtremes :many
SELECT w.day, r.name AS province_name,
       CASE WHEN w.tmax >= 40 THEN 'heat' ELSE 'frost' END::text AS kind,
       CASE WHEN w.tmax >= 40 THEN w.tmax ELSE w.tmin END::float8 AS temp
FROM weather_daily w
JOIN regions r ON r.id = w.province_id
WHERE w.day BETWEEN sqlc.arg(from_day)::date AND sqlc.arg(to_day)::date
  AND (sqlc.narg(province_id)::smallint IS NULL OR w.province_id = sqlc.narg(province_id))
  AND (w.tmax >= 40 OR w.tmin <= -5 OR (w.tmin <= 0 AND extract(month FROM w.day) BETWEEN 3 AND 5))
ORDER BY w.day, temp;
