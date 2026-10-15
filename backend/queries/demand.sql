-- name: RecordDemand :exec
INSERT INTO demand_events (kind, product_id, province_id, viewer_key)
VALUES ($1, $2, $3, $4)
ON CONFLICT DO NOTHING;

-- name: RecordListingEvent :exec
INSERT INTO listing_events (listing_id, kind, viewer_key)
VALUES ($1, $2, $3)
ON CONFLICT DO NOTHING;
