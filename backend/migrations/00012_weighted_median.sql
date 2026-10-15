-- +goose Up
-- Market prices are weighted medians (spec §10.1): each observation counts by
-- how much its source is trusted and how fresh it is, w = s · e^(−0.2·t days).
-- Official benchmarks (UzEx, narxtahlil) weigh 0 and never enter a median.
-- analytics.WeightedQuantile mirrors weighted_quantile in Go.

-- +goose StatementBegin
CREATE FUNCTION price_weight(source text, observed_at timestamptz) RETURNS double precision
LANGUAGE sql STABLE PARALLEL SAFE AS $$
    SELECT CASE source
               WHEN 'rfq_offer' THEN 1.0
               WHEN 'seller' THEN 0.8
               WHEN 'telegram_channel' THEN 0.6
               WHEN 'olx' THEN 0.5
               ELSE 0.0
           END * exp(-0.2 * GREATEST(extract(epoch FROM now() - observed_at), 0) / 86400)
$$;
-- +goose StatementEnd

-- The lowest price at which the running weight reaches q of the total weight
-- (a weighted lower quantile); NULL when nothing has weight.
-- +goose StatementBegin
CREATE FUNCTION weighted_quantile(prices double precision[], weights double precision[], q double precision)
RETURNS double precision
LANGUAGE sql IMMUTABLE PARALLEL SAFE AS $$
    SELECT min(p) FROM (
        SELECT p, sum(w) OVER (ORDER BY p ROWS UNBOUNDED PRECEDING) AS cum, sum(w) OVER () AS total
        FROM unnest(prices, weights) AS t(p, w)
        WHERE w > 0
    ) c
    WHERE cum >= q * total * (1 - 1e-9)
$$;
-- +goose StatementEnd

-- +goose Down
DROP FUNCTION weighted_quantile(double precision[], double precision[], double precision);
DROP FUNCTION price_weight(text, timestamptz);
