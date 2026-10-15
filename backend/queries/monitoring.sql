-- Prices per source in each of the last 15 whole days back from now, by when
-- they were recorded; demo sellers left out. days_ago 0 is the last 24 hours.
-- name: SourceDailyCounts :many
SELECT o.source,
       floor(extract(epoch FROM (now() - o.created_at)) / 86400)::int AS days_ago,
       count(*)::int AS n,
       count(*) FILTER (WHERE o.status = 'review')::int AS review
FROM price_observations o
LEFT JOIN users u ON u.id = o.seller_id
WHERE o.created_at > now() - interval '15 days' AND NOT COALESCE(u.is_demo, FALSE)
GROUP BY 1, 2;

-- The latest run of every job.
-- name: LatestJobRuns :many
SELECT DISTINCT ON (job) job, started_at, finished_at, error
FROM job_runs
ORDER BY job, started_at DESC;

-- name: DataFreshness :one
SELECT (SELECT max(day) FROM fx_daily)::date AS fx,
       (SELECT max(day) FROM weather_daily)::date AS weather;

-- name: UpsertAlert :one
INSERT INTO monitoring_alerts (kind, subject, level, value, baseline, message)
VALUES (sqlc.arg(kind), sqlc.arg(subject), sqlc.arg(level), sqlc.narg(value), sqlc.narg(baseline), sqlc.arg(message))
ON CONFLICT (kind, subject) WHERE resolved_at IS NULL DO UPDATE
SET level = EXCLUDED.level, value = EXCLUDED.value, baseline = EXCLUDED.baseline, message = EXCLUDED.message,
    updated_at = now(),
    -- a worse level is news again
    notified_at = CASE WHEN array_position(ARRAY['medium', 'high', 'critical'], EXCLUDED.level)
                            > array_position(ARRAY['medium', 'high', 'critical'], monitoring_alerts.level)
                       THEN NULL ELSE monitoring_alerts.notified_at END
RETURNING *;

-- Resolve the open alerts whose kind|subject is not in keys.
-- name: ResolveAlertsExcept :execrows
UPDATE monitoring_alerts SET resolved_at = now()
WHERE resolved_at IS NULL AND NOT ((kind || '|' || subject) = ANY (sqlc.arg(keys)::text[]));

-- name: AlertsToNotify :many
SELECT * FROM monitoring_alerts
WHERE resolved_at IS NULL AND notified_at IS NULL AND level IN ('high', 'critical')
ORDER BY id;

-- name: MarkAlertsNotified :exec
UPDATE monitoring_alerts SET notified_at = now() WHERE id = ANY (sqlc.arg(ids)::bigint[]);

-- Open alerts, worst first, then those resolved in the last 7 days.
-- name: ListAlerts :many
SELECT * FROM monitoring_alerts
WHERE resolved_at IS NULL OR resolved_at > now() - interval '7 days'
ORDER BY resolved_at IS NOT NULL, array_position(ARRAY['medium', 'high', 'critical'], level) DESC, updated_at DESC
LIMIT 200;

-- name: AdminTelegramIDs :many
SELECT telegram_id FROM users WHERE role = 'admin';

-- The assistant's replies of the last 24 hours and how many could not answer.
-- name: ChatHealth :one
SELECT count(*)::int AS answers, count(*) FILTER (WHERE kind = 'unanswered')::int AS unanswered
FROM chat_messages WHERE role = 'assistant' AND created_at > now() - interval '24 hours';
