-- +goose Up
-- Replaces the unused ai_log of the first schema (it had room for the model's
-- output, which is personal content; nothing ever wrote to it).
DROP TABLE ai_log;
-- Every call to a language, speech, vision or embeddings model: what it was
-- for, which model, how big, how long, what it cost and whether it worked
-- (spec §16). No prompt or answer text is kept: input_hash is a short keyed
-- digest so repeated inputs can be counted without storing them.
CREATE TABLE ai_log (
    id            BIGSERIAL PRIMARY KEY,
    feature       TEXT NOT NULL,
    model         TEXT NOT NULL,
    input_tokens  INT NOT NULL DEFAULT 0 CHECK (input_tokens >= 0),
    output_tokens INT NOT NULL DEFAULT 0 CHECK (output_tokens >= 0),
    audio_seconds REAL CHECK (audio_seconds >= 0),
    latency_ms    INT NOT NULL DEFAULT 0 CHECK (latency_ms >= 0),
    cost_usd      NUMERIC(12, 6) NOT NULL DEFAULT 0,
    ok            BOOLEAN NOT NULL,
    error_code    TEXT,
    user_id       UUID REFERENCES users (id) ON DELETE SET NULL,
    input_hash    TEXT,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX ai_log_created_idx ON ai_log (created_at);
CREATE INDEX ai_log_feature_idx ON ai_log (feature, created_at);

-- +goose Down
DROP TABLE ai_log;
CREATE TABLE ai_log (
    id         BIGSERIAL PRIMARY KEY,
    component  TEXT NOT NULL,
    input_hash BYTEA NOT NULL,
    output     JSONB,
    model      TEXT NOT NULL,
    latency_ms INTEGER NOT NULL,
    cost_usd   NUMERIC(10, 6) NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
