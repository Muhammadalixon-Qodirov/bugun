-- +goose Up
-- Parsed but not yet confirmed prices (spec §8.2: an unconfirmed price is
-- never written to price_observations). Drafts expire after an hour.
CREATE TABLE price_drafts (
    id         BIGSERIAL PRIMARY KEY,
    user_id    UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    raw_text   TEXT NOT NULL,
    items      JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX price_drafts_user_idx ON price_drafts (user_id);

-- A seller has at most one active price per product and trade type.
CREATE UNIQUE INDEX price_obs_one_active_idx ON price_observations (seller_id, product_id, trade_type)
    WHERE status = 'active' AND seller_id IS NOT NULL;

-- +goose Down
DROP INDEX price_obs_one_active_idx;
DROP TABLE price_drafts;
