-- +goose Up
-- RFQ flow (spec §8.4). An offer goes sent -> chosen (buyer picks it) ->
-- confirmed (seller agrees; contacts are exchanged) or rejected (seller
-- refuses). "Menda yo'q" is an offer row with status declined and no price.
ALTER TABLE offers ALTER COLUMN price_som DROP NOT NULL;
ALTER TABLE offers DROP CONSTRAINT offers_status_check;
ALTER TABLE offers ADD CONSTRAINT offers_status_check
    CHECK (status IN ('sent', 'chosen', 'confirmed', 'rejected', 'declined'));
ALTER TABLE offers ADD CONSTRAINT offers_price_given
    CHECK (status = 'declined' OR price_som IS NOT NULL);
ALTER TABLE offers ADD COLUMN updated_at TIMESTAMPTZ NOT NULL DEFAULT now();
CREATE INDEX offers_seller_idx ON offers (seller_id);

-- An offer's price joins the market median as source rfq_offer (weight 1.0,
-- spec §10.1); one live observation per offer.
ALTER TABLE price_observations ADD COLUMN offer_id BIGINT REFERENCES offers (id) ON DELETE SET NULL;
CREATE UNIQUE INDEX price_obs_one_offer_idx ON price_observations (offer_id)
    WHERE status = 'active' AND offer_id IS NOT NULL;

CREATE INDEX rfqs_buyer_idx ON rfqs (buyer_id, created_at DESC);

-- +goose Down
DROP INDEX rfqs_buyer_idx;
DROP INDEX price_obs_one_offer_idx;
ALTER TABLE price_observations DROP COLUMN offer_id;
DROP INDEX offers_seller_idx;
ALTER TABLE offers DROP COLUMN updated_at;
ALTER TABLE offers DROP CONSTRAINT offers_price_given;
DELETE FROM offers WHERE price_som IS NULL;
UPDATE offers SET status = 'chosen' WHERE status = 'confirmed';
ALTER TABLE offers DROP CONSTRAINT offers_status_check;
ALTER TABLE offers ADD CONSTRAINT offers_status_check
    CHECK (status IN ('sent', 'chosen', 'rejected', 'declined'));
ALTER TABLE offers ALTER COLUMN price_som SET NOT NULL;
