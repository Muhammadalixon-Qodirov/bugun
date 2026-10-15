-- +goose Up
-- Ratings (QIZIQISH_VA_SAVDO.md step 9): only a confirmed deal can be rated,
-- once, by its buyer. A low rating with a comment waits for an admin before
-- the comment is shown; the stars always count.
CREATE TABLE ratings (
    id         BIGSERIAL PRIMARY KEY,
    deal_id    BIGINT NOT NULL UNIQUE REFERENCES deals (id) ON DELETE CASCADE,
    seller_id  UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    buyer_id   UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    stars      SMALLINT NOT NULL CHECK (stars BETWEEN 1 AND 5),
    comment    TEXT NOT NULL DEFAULT '' CHECK (length(comment) <= 300),
    status     TEXT NOT NULL DEFAULT 'published' CHECK (status IN ('published', 'held', 'hidden')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX ratings_seller_idx ON ratings (seller_id, created_at DESC);

-- When the buyer was asked to rate the deal.
ALTER TABLE deals ADD COLUMN rating_asked_at TIMESTAMPTZ;

-- +goose Down
ALTER TABLE deals DROP COLUMN rating_asked_at;
DROP TABLE ratings;
