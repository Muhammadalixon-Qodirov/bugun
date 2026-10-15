-- +goose Up
-- Telegram channels and groups whose owner added the bot (KANAL_ULASH.md):
-- prices posted there flow in as telegram_channel observations. Only chats
-- the owner connected are read.
CREATE TABLE telegram_channels (
    id              BIGSERIAL PRIMARY KEY,
    chat_id         BIGINT NOT NULL UNIQUE,
    kind            TEXT NOT NULL CHECK (kind IN ('channel', 'group', 'supergroup')),
    title           TEXT NOT NULL DEFAULT '',
    username        TEXT,
    owner_id        UUID REFERENCES users (id) ON DELETE SET NULL,
    added_by_tg     BIGINT NOT NULL, -- Telegram id of who added the bot, to claim it later
    province_id     SMALLINT REFERENCES regions (id),
    status          TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('active', 'pending', 'disabled')),
    update_listings BOOLEAN NOT NULL DEFAULT FALSE, -- posts also update the owner's listings
    posts_seen      INTEGER NOT NULL DEFAULT 0,
    prices_taken    INTEGER NOT NULL DEFAULT 0,
    last_post_at    TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT telegram_channels_active_needs_owner CHECK (status <> 'active' OR (owner_id IS NOT NULL AND province_id IS NOT NULL))
);
CREATE INDEX telegram_channels_owner_idx ON telegram_channels (owner_id);
CREATE INDEX telegram_channels_adder_idx ON telegram_channels (added_by_tg);

-- Which post a channel price came from, so an edited post replaces its prices.
ALTER TABLE price_observations ADD COLUMN tg_chat_id BIGINT, ADD COLUMN tg_message_id BIGINT;
CREATE INDEX price_obs_tg_idx ON price_observations (tg_chat_id, tg_message_id) WHERE tg_chat_id IS NOT NULL;

-- +goose Down
DROP INDEX price_obs_tg_idx;
ALTER TABLE price_observations DROP COLUMN tg_message_id, DROP COLUMN tg_chat_id;
DROP TABLE telegram_channels;
