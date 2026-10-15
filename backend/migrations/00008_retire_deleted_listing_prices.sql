-- +goose Up
-- A deleted listing (directly or with its seller's account) must leave the
-- median at once, not after its 7-day validity runs out.
-- +goose StatementBegin
CREATE FUNCTION retire_listing_prices() RETURNS trigger AS $$
BEGIN
    UPDATE price_observations
    SET status = 'expired', valid_until = LEAST(valid_until, now())
    WHERE listing_id = OLD.id AND status IN ('active', 'review');
    RETURN OLD;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER listings_retire_prices BEFORE DELETE ON listings
    FOR EACH ROW EXECUTE FUNCTION retire_listing_prices();

-- Seller prices already orphaned by earlier deletions.
UPDATE price_observations
SET status = 'expired', valid_until = LEAST(valid_until, now())
WHERE source = 'seller' AND seller_id IS NULL AND listing_id IS NULL AND status IN ('active', 'review');

-- +goose Down
DROP TRIGGER listings_retire_prices ON listings;
DROP FUNCTION retire_listing_prices();
