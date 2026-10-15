-- +goose Up
-- Why the automatic check held a listing for review (analytics.Reason), so the
-- admin sees it next to the price. NULL while the listing is not held.
ALTER TABLE listings ADD COLUMN review_reason TEXT CHECK (review_reason IN
    ('jump_from_previous', 'far_from_market', 'far_from_reference'));

-- +goose Down
ALTER TABLE listings DROP COLUMN review_reason;
