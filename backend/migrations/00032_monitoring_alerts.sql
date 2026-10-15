-- +goose Up
-- System monitoring (ML_REJA.md 3): signals that the platform itself is not
-- working normally, such as a price source going quiet, a failed or stuck job
-- or stale exchange rates. At most one open alert per kind and subject; it
-- is resolved when the next check no longer sees the problem.
CREATE TABLE monitoring_alerts (
    id          BIGSERIAL PRIMARY KEY,
    kind        TEXT NOT NULL,
    subject     TEXT NOT NULL DEFAULT '',
    level       TEXT NOT NULL CHECK (level IN ('medium', 'high', 'critical')),
    value       DOUBLE PRECISION,
    baseline    DOUBLE PRECISION,
    message     TEXT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    notified_at TIMESTAMPTZ,
    resolved_at TIMESTAMPTZ
);
CREATE UNIQUE INDEX monitoring_alerts_open_idx ON monitoring_alerts (kind, subject) WHERE resolved_at IS NULL;

-- +goose Down
DROP TABLE monitoring_alerts;
