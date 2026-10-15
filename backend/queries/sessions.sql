-- name: CreateSession :exec
INSERT INTO sessions (token_hash, user_id, expires_at) VALUES ($1, $2, $3);

-- name: GetSessionUserID :one
SELECT user_id FROM sessions WHERE token_hash = $1 AND expires_at > now();

-- name: DeleteSession :exec
DELETE FROM sessions WHERE token_hash = $1;

-- name: DeleteExpiredSessions :execrows
DELETE FROM sessions WHERE expires_at <= now();

-- Logins that were started but never finished.
-- name: DeleteExpiredOAuthStates :execrows
DELETE FROM oauth_states WHERE expires_at <= now();

-- name: CreateOAuthState :exec
INSERT INTO oauth_states (state_hash, code_verifier, nonce, return_to, expires_at)
VALUES ($1, $2, $3, $4, $5);

-- Deleting on read makes each state single-use, even under concurrent callbacks.
-- name: ConsumeOAuthState :one
DELETE FROM oauth_states
WHERE state_hash = $1 AND expires_at > now()
RETURNING code_verifier, nonce, return_to;
