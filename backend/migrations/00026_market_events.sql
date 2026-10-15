-- +goose Up
-- Market events (TAHLIL_REJA.md #30): frosts, export rules, fuel prices,
-- holidays and the like, entered by admins, so price jumps on the analytics
-- page come with their likely causes. An event may target a viloyat, a
-- product or a category; empty means everywhere or everything.
CREATE TABLE market_events (
    id          BIGSERIAL PRIMARY KEY,
    title       TEXT NOT NULL CHECK (length(title) BETWEEN 3 AND 200),
    kind        TEXT NOT NULL CHECK (kind IN ('frost', 'heat', 'rain', 'export', 'import', 'fuel', 'currency',
                                             'holiday', 'harvest', 'roads', 'policy', 'other')),
    starts_on   DATE NOT NULL,
    ends_on     DATE,
    province_id SMALLINT REFERENCES regions (id),
    product_id  SMALLINT REFERENCES products (id),
    category    TEXT,
    source_url  TEXT CHECK (source_url IS NULL OR source_url ~ '^https?://'),
    note        TEXT NOT NULL DEFAULT '' CHECK (length(note) <= 1000),
    is_demo     BOOLEAN NOT NULL DEFAULT FALSE,
    created_by  UUID REFERENCES users (id) ON DELETE SET NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT market_events_dates CHECK (ends_on IS NULL OR ends_on >= starts_on)
);
CREATE INDEX market_events_dates_idx ON market_events (starts_on, ends_on);

-- +goose Down
DROP TABLE market_events;
