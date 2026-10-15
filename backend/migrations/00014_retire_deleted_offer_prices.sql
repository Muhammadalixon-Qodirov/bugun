-- +goose Up
-- A deleted offer (with its RFQ or the seller's or buyer's account) must leave
-- the median at once, like a deleted listing (00008).
-- +goose StatementBegin
CREATE FUNCTION retire_offer_prices() RETURNS trigger AS $$
BEGIN
    UPDATE price_observations
    SET status = 'expired', valid_until = LEAST(valid_until, now())
    WHERE offer_id = OLD.id AND status IN ('active', 'review');
    RETURN OLD;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER offers_retire_prices BEFORE DELETE ON offers
    FOR EACH ROW EXECUTE FUNCTION retire_offer_prices();

-- Offer prices already orphaned by earlier deletions.
UPDATE price_observations
SET status = 'expired', valid_until = LEAST(valid_until, now())
WHERE source = 'rfq_offer' AND offer_id IS NULL AND status IN ('active', 'review');

-- +goose Down
DROP TRIGGER offers_retire_prices ON offers;
DROP FUNCTION retire_offer_prices();
