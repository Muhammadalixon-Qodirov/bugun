-- Rule-based price questions (/savol).

-- Weighted median per tuman/shahar of one viloyat, at least 3 live prices.
-- name: ProductDistrictMedians :many
SELECT r.id, r.name, count(*)::int AS offers,
       weighted_quantile(array_agg(o.price_som::float8 ORDER BY o.id),
                         array_agg(price_weight(o.source, o.observed_at) ORDER BY o.id), 0.5)::float8 AS median
FROM price_observations o
JOIN regions r ON r.id = o.region_id
WHERE o.product_id = @product_id AND o.trade_type = @trade_type
  AND o.status = 'active' AND o.valid_until > now() AND r.parent_id = @province_id
  AND (o.listing_id IS NULL OR EXISTS (SELECT 1 FROM listings l WHERE l.id = o.listing_id AND l.status = 'active'))
GROUP BY r.id, r.name
HAVING count(*) >= 3
ORDER BY median;

-- name: LogQuestion :exec
INSERT INTO question_log (user_id, question, product_id, region_id) VALUES ($1, $2, $3, $4);
