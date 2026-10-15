-- +goose Up
-- Two Telegram accounts may share a phone number; login must not fail on that.
ALTER TABLE users DROP CONSTRAINT users_phone_hash_key;
CREATE INDEX users_phone_hash_idx ON users (phone_hash);

-- +goose Down
DROP INDEX users_phone_hash_idx;
ALTER TABLE users ADD CONSTRAINT users_phone_hash_key UNIQUE (phone_hash);
