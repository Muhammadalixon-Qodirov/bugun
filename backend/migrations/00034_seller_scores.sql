-- +goose Up
-- Unusual sellers (ML_REJA.md 5): an Isolation Forest over each seller's
-- last 60 days, recomputed nightly. A high score is a hint for an admin to
-- look, never an automatic block.
CREATE TABLE seller_scores (
    seller_id   UUID PRIMARY KEY REFERENCES users (id) ON DELETE CASCADE,
    score       DOUBLE PRECISION NOT NULL,
    features    JSONB NOT NULL,
    reasons     TEXT[] NOT NULL DEFAULT '{}',
    computed_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE seller_scores;
