-- Claims a job slot; no row comes back when another run already took it.
-- name: ClaimJobSlot :one
INSERT INTO job_runs (job, slot) VALUES ($1, $2)
ON CONFLICT (job, slot) DO NOTHING
RETURNING id;

-- name: FinishJobRun :exec
UPDATE job_runs SET finished_at = now(), items = $2, error = $3 WHERE id = $1;

-- Entrepreneurs to ask on Monday whether their prices still hold (spec
-- §8.3): bot linked, with a live listing or one that expired in the last two
-- weeks. 'seller' and 'buyer' are legacy roles (see migration 00045).
-- name: SellersToRemind :many
SELECT DISTINCT u.id
FROM users u
JOIN listings l ON l.seller_id = u.id
WHERE u.role IN ('entrepreneur', 'seller', 'buyer') AND u.bot_enabled
  AND l.status = 'active' AND l.valid_until > now() - interval '14 days';

-- A running job's lease: refreshed while it works.
-- name: HeartbeatJobRun :exec
UPDATE job_runs SET heartbeat_at = now() WHERE id = $1 AND finished_at IS NULL;

-- Runs whose process stopped refreshing them for longer than the lease (a
-- crash, restart or deploy killed it). Deleting them lets their slots run
-- again within the window; runs another live instance is doing keep a fresh
-- heartbeat and stay.
-- name: ReleaseStaleJobs :many
DELETE FROM job_runs
WHERE finished_at IS NULL AND heartbeat_at < now() - make_interval(secs => sqlc.arg(lease_seconds)::float8)
RETURNING job;
