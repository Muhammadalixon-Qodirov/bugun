-- +goose Up
-- Catalog products as embedding vectors for matching free text to the
-- catalog (AI-4). source_hash is a digest of the text that was embedded
-- (name and other names), so a catalog change or another model recomputes it.
CREATE TABLE product_embeddings (
    product_id  SMALLINT PRIMARY KEY REFERENCES products (id) ON DELETE CASCADE,
    model       TEXT NOT NULL,
    source_hash TEXT NOT NULL,
    embedding   REAL[] NOT NULL,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE product_embeddings;
