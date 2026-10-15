-- +goose Up
-- Product names learned from sellers (MATN_TUSHUNISH.md 2): a language model
-- read a word as a product and the seller confirmed the card. The word is
-- read by the rules for that seller at once, and for everyone once three
-- sellers confirmed it.
CREATE TABLE learned_aliases (
    phrase     TEXT NOT NULL CHECK (length(phrase) BETWEEN 3 AND 30),
    product_id SMALLINT NOT NULL REFERENCES products (id),
    user_id    UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (phrase, product_id, user_id)
);

-- The names a draft would teach when confirmed.
ALTER TABLE price_drafts ADD COLUMN learn JSONB NOT NULL DEFAULT '[]';

-- +goose Down
ALTER TABLE price_drafts DROP COLUMN learn;
DROP TABLE learned_aliases;
