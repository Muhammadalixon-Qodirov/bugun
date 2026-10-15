-- +goose Up
-- Agreed prices of confirmed deals join the median (QIZIQISH_VA_SAVDO.md
-- step 7) as source 'deal' with the highest trust, 1.0, like RFQ offers.
-- RFQ deals are already there as their rfq_offer price. A deal's price row
-- goes with the deal.
ALTER TABLE price_observations ADD COLUMN deal_id BIGINT REFERENCES deals (id) ON DELETE CASCADE;
CREATE UNIQUE INDEX price_obs_one_deal_idx ON price_observations (deal_id)
    WHERE status IN ('active', 'review') AND deal_id IS NOT NULL;

ALTER TABLE price_observations DROP CONSTRAINT price_observations_source_check;
ALTER TABLE price_observations ADD CONSTRAINT price_observations_source_check CHECK (source IN
    ('seller', 'seller_auto', 'rfq_offer', 'deal', 'telegram_channel', 'olx', 'uzex', 'narxtahlil'));

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION price_weight(source text, observed_at timestamptz) RETURNS double precision
LANGUAGE sql STABLE PARALLEL SAFE AS $$
    SELECT CASE source
               WHEN 'rfq_offer' THEN 1.0
               WHEN 'deal' THEN 1.0
               WHEN 'seller' THEN 0.8
               WHEN 'telegram_channel' THEN 0.6
               WHEN 'olx' THEN 0.5
               WHEN 'seller_auto' THEN 0.4
               ELSE 0.0
           END * exp(-0.2 * GREATEST(extract(epoch FROM now() - observed_at), 0) / 86400)
$$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION price_weight(source text, observed_at timestamptz) RETURNS double precision
LANGUAGE sql STABLE PARALLEL SAFE AS $$
    SELECT CASE source
               WHEN 'rfq_offer' THEN 1.0
               WHEN 'seller' THEN 0.8
               WHEN 'telegram_channel' THEN 0.6
               WHEN 'olx' THEN 0.5
               WHEN 'seller_auto' THEN 0.4
               ELSE 0.0
           END * exp(-0.2 * GREATEST(extract(epoch FROM now() - observed_at), 0) / 86400)
$$;
-- +goose StatementEnd
DELETE FROM price_observations WHERE source = 'deal';
ALTER TABLE price_observations DROP CONSTRAINT price_observations_source_check;
ALTER TABLE price_observations ADD CONSTRAINT price_observations_source_check CHECK (source IN
    ('seller', 'seller_auto', 'rfq_offer', 'telegram_channel', 'olx', 'uzex', 'narxtahlil'));
DROP INDEX price_obs_one_deal_idx;
ALTER TABLE price_observations DROP COLUMN deal_id;
