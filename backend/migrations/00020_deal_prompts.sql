-- +goose Up
-- Follow-up questions after a contact reveal (QIZIQISH_VA_SAVDO.md step 3):
-- a day later the bot asks the buyer whether they bought. Kept in the
-- database so a restart neither loses nor repeats them.
CREATE TABLE deal_prompts (
    id          BIGSERIAL PRIMARY KEY,
    listing_id  BIGINT NOT NULL REFERENCES listings (id) ON DELETE CASCADE,
    buyer_id    UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    seller_id   UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    due_at      TIMESTAMPTZ NOT NULL,
    sent_at     TIMESTAMPTZ,
    asks        SMALLINT NOT NULL DEFAULT 0, -- times sent; "still talking" asks once more
    answer      TEXT CHECK (answer IN ('yes', 'talking', 'no', 'expired')),
    answered_at TIMESTAMPTZ,
    deal_id     BIGINT REFERENCES deals (id) ON DELETE SET NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT deal_prompts_not_to_oneself CHECK (buyer_id <> seller_id)
);
-- At most one open question per buyer and listing.
CREATE UNIQUE INDEX deal_prompts_open_idx ON deal_prompts (listing_id, buyer_id)
    WHERE answer IS NULL OR answer = 'talking';
CREATE INDEX deal_prompts_due_idx ON deal_prompts (due_at) WHERE sent_at IS NULL;
CREATE INDEX deal_prompts_buyer_idx ON deal_prompts (buyer_id, sent_at);

-- +goose Down
DROP TABLE deal_prompts;
