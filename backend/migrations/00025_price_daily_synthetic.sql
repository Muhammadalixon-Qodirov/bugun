-- +goose Up
-- Generated demo history is marked synthetic: rebuilding days from real
-- prices leaves it alone, and a day with enough real prices replaces it.
ALTER TABLE price_daily ADD COLUMN synthetic BOOLEAN NOT NULL DEFAULT FALSE;

-- +goose Down
ALTER TABLE price_daily DROP COLUMN synthetic;
