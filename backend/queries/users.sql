-- name: UpsertTelegramUser :one
INSERT INTO users (telegram_id, telegram_username, display_name, phone_enc, phone_hash)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (telegram_id) DO UPDATE SET
    telegram_username = EXCLUDED.telegram_username,
    display_name      = EXCLUDED.display_name,
    phone_enc         = COALESCE(EXCLUDED.phone_enc, users.phone_enc),
    phone_hash        = COALESCE(EXCLUDED.phone_hash, users.phone_hash),
    updated_at        = now()
RETURNING id;

-- name: GetUser :one
SELECT * FROM users WHERE id = $1;

-- name: UpdateProfile :one
UPDATE users SET
    role          = $2,
    role_approved = $3,
    region_id     = $4,
    market_id     = $5,
    categories    = $6,
    public_name   = $7,
    consent_at    = COALESCE(consent_at, now()),
    updated_at    = now()
WHERE id = $1
RETURNING *;

-- name: SetBotEnabled :exec
UPDATE users SET bot_enabled = $2, updated_at = now() WHERE id = $1;

-- name: DeleteUser :exec
DELETE FROM users WHERE id = $1;

-- name: GetUserByTelegramID :one
SELECT * FROM users WHERE telegram_id = $1;
