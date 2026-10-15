-- +goose Up
-- Demo sellers created by `seed`; they can be removed in one step and are
-- never mixed up with real accounts.
ALTER TABLE users ADD COLUMN is_demo BOOLEAN NOT NULL DEFAULT FALSE;
CREATE INDEX users_demo_idx ON users (is_demo) WHERE is_demo;

-- +goose Down
DROP INDEX users_demo_idx;
ALTER TABLE users DROP COLUMN is_demo;
