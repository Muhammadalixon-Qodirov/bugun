-- +goose Up
-- A running job refreshes heartbeat_at every minute or so. An unfinished run
-- is released (so its slot can run again) only once its heartbeat is older
-- than the lease: a second API instance never frees a run that another live
-- instance is still doing, while a run cut off by a crash is picked up.
ALTER TABLE job_runs ADD COLUMN heartbeat_at TIMESTAMPTZ;
UPDATE job_runs SET heartbeat_at = COALESCE(finished_at, started_at);
ALTER TABLE job_runs ALTER COLUMN heartbeat_at SET NOT NULL, ALTER COLUMN heartbeat_at SET DEFAULT now();
CREATE INDEX job_runs_unfinished_idx ON job_runs (heartbeat_at) WHERE finished_at IS NULL;

-- +goose Down
DROP INDEX job_runs_unfinished_idx;
ALTER TABLE job_runs DROP COLUMN heartbeat_at;
