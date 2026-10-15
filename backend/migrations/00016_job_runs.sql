-- +goose Up
-- Scheduled jobs (weekly reminder and later ones). One row per job and time
-- slot: the unique key makes a slot run once even with two API instances or
-- after a restart, and a slot missed while the server was down still runs
-- when it comes back within the job's window.
CREATE TABLE job_runs (
    id          BIGSERIAL PRIMARY KEY,
    job         TEXT NOT NULL,
    slot        TIMESTAMPTZ NOT NULL,
    started_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    finished_at TIMESTAMPTZ,
    items       INTEGER NOT NULL DEFAULT 0,
    error       TEXT,
    UNIQUE (job, slot)
);

-- +goose Down
DROP TABLE job_runs;
