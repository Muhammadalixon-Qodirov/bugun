-- +goose Up
-- Interest in one listing (QIZIQISH_VA_SAVDO.md step 2): page views and
-- contact reveals, once per visitor, listing, kind and Tashkent day, with the
-- same keyed day-bound visitor hash as demand_events. The seller's own visits
-- are not recorded. listings.views stays as the raw counter the site shows.
CREATE TABLE listing_events (
    id         BIGSERIAL PRIMARY KEY,
    listing_id BIGINT NOT NULL REFERENCES listings (id) ON DELETE CASCADE,
    kind       TEXT NOT NULL CHECK (kind IN ('view', 'contact')),
    viewer_key BYTEA NOT NULL,
    day        DATE NOT NULL DEFAULT (now() AT TIME ZONE 'Asia/Tashkent')::date,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX listing_events_once_idx ON listing_events (listing_id, kind, viewer_key, day);
CREATE INDEX listing_events_listing_idx ON listing_events (listing_id, day);

-- +goose Down
DROP TABLE listing_events;
