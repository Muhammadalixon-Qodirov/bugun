-- Connected Telegram channels and groups (KANAL_ULASH.md).

-- name: UpsertTelegramChannel :one
INSERT INTO telegram_channels (chat_id, kind, title, username, owner_id, added_by_tg, province_id, status)
VALUES (@chat_id, @kind, @title, sqlc.narg(username), sqlc.narg(owner_id), @added_by_tg, sqlc.narg(province_id), @status)
ON CONFLICT (chat_id) DO UPDATE SET
    kind = EXCLUDED.kind, title = EXCLUDED.title, username = EXCLUDED.username,
    owner_id = EXCLUDED.owner_id, added_by_tg = EXCLUDED.added_by_tg,
    province_id = COALESCE(telegram_channels.province_id, EXCLUDED.province_id),
    status = CASE WHEN EXCLUDED.status = 'active' AND COALESCE(telegram_channels.province_id, EXCLUDED.province_id) IS NULL
                  THEN 'pending' ELSE EXCLUDED.status END,
    updated_at = now()
RETURNING *;

-- name: GetTelegramChannel :one
SELECT * FROM telegram_channels WHERE chat_id = $1;

-- name: DisableTelegramChannel :exec
UPDATE telegram_channels SET status = 'disabled', updated_at = now() WHERE chat_id = $1;

-- Channels a person connected: owned, or added by their Telegram account.
-- name: ListMyChannels :many
SELECT c.*, COALESCE(r.name, '')::text AS province_name
FROM telegram_channels c
LEFT JOIN regions r ON r.id = c.province_id
WHERE (c.owner_id = @owner_id OR c.added_by_tg = @telegram_id) AND c.status <> 'disabled'
ORDER BY c.created_at;

-- Claims a channel for its adder once they have a profile, and sets its viloyat.
-- name: SetChannelProvince :one
UPDATE telegram_channels SET owner_id = @owner_id, province_id = @province_id, status = 'active', updated_at = now()
WHERE id = @id AND (owner_id = @owner_id OR added_by_tg = @telegram_id) AND status <> 'disabled'
RETURNING *;

-- name: SetChannelUpdateListings :one
UPDATE telegram_channels SET update_listings = @update_listings, updated_at = now()
WHERE id = @id AND owner_id = @owner_id AND status <> 'disabled'
RETURNING *;

-- name: DisconnectChannel :one
UPDATE telegram_channels SET status = 'disabled', updated_at = now()
WHERE id = @id AND (owner_id = @owner_id OR added_by_tg = @telegram_id) AND status <> 'disabled'
RETURNING chat_id;

-- name: ChannelPostSeen :one
UPDATE telegram_channels SET posts_seen = posts_seen + 1, prices_taken = prices_taken + @prices::int,
       last_post_at = now(), updated_at = now()
WHERE id = @id
RETURNING posts_seen;

-- A post was edited or replaced: its earlier prices stop counting.
-- name: ExpireTelegramPost :exec
UPDATE price_observations SET status = 'expired', valid_until = LEAST(valid_until, now())
WHERE tg_chat_id = @chat_id AND tg_message_id = @message_id AND status IN ('active', 'review');

-- name: InsertTelegramObservation :exec
-- ai_confidence is set for prices a language model read (0.5), empty for the rules.
INSERT INTO price_observations (product_id, price_som, unit, quantity, trade_type, region_id, source, source_url,
                                seller_id, valid_until, raw_text, status, tg_chat_id, tg_message_id, ai_confidence)
VALUES (@product_id, @price_som, @unit, sqlc.narg(quantity), @trade_type, @region_id, 'telegram_channel', sqlc.narg(source_url),
        sqlc.narg(seller_id), @valid_until, @raw_text, @status, @chat_id, @message_id, sqlc.narg(ai_confidence));

-- The live weighted median of a product in a viloyat and how many prices
-- stand behind it, to screen channel prices.
-- name: ProvinceLiveMedian :one
SELECT count(*)::int AS offers,
       COALESCE(weighted_quantile(array_agg(o.price_som::float8 ORDER BY o.id),
                                  array_agg(price_weight(o.source, o.observed_at) ORDER BY o.id), 0.5), 0)::float8 AS median
FROM price_observations o
JOIN regions r ON r.id = o.region_id
WHERE o.product_id = @product_id AND o.trade_type = @trade_type AND o.status = 'active' AND o.valid_until > now()
  AND COALESCE(r.parent_id, r.id) = @province_id::smallint;
