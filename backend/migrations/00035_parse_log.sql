-- +goose Up
-- Every attempt to read prices from text (MATN_TUSHUNISH.md 1): what was
-- understood, what was not, whether a language model helped and what the
-- seller did with the card. It measures the parser; rows older than 180
-- days are deleted.
CREATE TABLE parse_log (
    id          BIGSERIAL PRIMARY KEY,
    user_id     UUID REFERENCES users (id) ON DELETE SET NULL,
    via         TEXT NOT NULL CHECK (via IN ('web', 'bot', 'channel')),
    raw_text    TEXT NOT NULL,
    items       JSONB NOT NULL DEFAULT '[]',
    problems    JSONB NOT NULL DEFAULT '[]',
    items_n     INTEGER NOT NULL DEFAULT 0,
    problems_n  INTEGER NOT NULL DEFAULT 0,
    by_model    BOOLEAN NOT NULL DEFAULT FALSE,
    -- pending: a card waits; confirmed, discarded; none: nothing to confirm
    -- (nothing understood, or a channel post). A pending card older than an
    -- hour has expired.
    outcome     TEXT NOT NULL DEFAULT 'none' CHECK (outcome IN ('pending', 'confirmed', 'discarded', 'none')),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    decided_at  TIMESTAMPTZ
);
CREATE INDEX parse_log_created_idx ON parse_log (created_at);

ALTER TABLE price_drafts ADD COLUMN log_id BIGINT REFERENCES parse_log (id) ON DELETE SET NULL;

-- +goose Down
ALTER TABLE price_drafts DROP COLUMN log_id;
DROP TABLE parse_log;
