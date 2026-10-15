-- +goose Up
-- A listing's text can also hold it for review (AI-7): spam, abuse,
-- prohibited goods, or text about something other than its product.
ALTER TABLE listings DROP CONSTRAINT IF EXISTS listings_review_reason_check;
ALTER TABLE listings ADD CONSTRAINT listings_review_reason_check CHECK (review_reason IN
    ('jump_from_previous', 'far_from_market', 'far_from_reference',
     'content_spam', 'content_abuse', 'content_prohibited', 'content_off_category'));

-- +goose Down
UPDATE listings SET review_reason = NULL WHERE review_reason LIKE 'content_%';
ALTER TABLE listings DROP CONSTRAINT IF EXISTS listings_review_reason_check;
ALTER TABLE listings ADD CONSTRAINT listings_review_reason_check CHECK (review_reason IN
    ('jump_from_previous', 'far_from_market', 'far_from_reference'));
