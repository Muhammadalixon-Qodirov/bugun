-- +goose Up
-- Core schema (spec §12). Money is integer so'm, all times are timestamptz (UTC).

-- hudud
CREATE TABLE regions (
    id        SMALLINT PRIMARY KEY,
    slug      TEXT NOT NULL UNIQUE,
    name      TEXT NOT NULL,
    kind      TEXT NOT NULL CHECK (kind IN ('viloyat', 'tuman', 'shahar')),
    parent_id SMALLINT REFERENCES regions (id),
    geometry  JSONB
);

-- bozor
CREATE TABLE markets (
    id        SMALLINT PRIMARY KEY,
    slug      TEXT NOT NULL UNIQUE,
    name      TEXT NOT NULL,
    region_id SMALLINT NOT NULL REFERENCES regions (id),
    kind      TEXT NOT NULL CHECK (kind IN ('dehqon', 'ulgurji', 'qurilish'))
);
CREATE INDEX markets_region_idx ON markets (region_id);

-- mahsulot
CREATE TABLE products (
    id        SMALLINT PRIMARY KEY,
    slug      TEXT NOT NULL UNIQUE,
    name      TEXT NOT NULL,
    category  TEXT NOT NULL CHECK (category IN
              ('sabzavot', 'meva', 'gosht', 'don', 'oziq_ovqat', 'qurilish', 'xizmat')),
    base_unit TEXT NOT NULL CHECK (base_unit IN ('kg', 'litr', 'dona', 'm2', 'm3', 'xizmat')),
    synonyms  TEXT[] NOT NULL DEFAULT '{}'
);
CREATE INDEX products_synonyms_idx ON products USING GIN (synonyms);
CREATE INDEX products_name_trgm_idx ON products USING GIN (name gin_trgm_ops);

-- birlik_jadvali: qop, yashik, bog' -> base unit
CREATE TABLE unit_conversions (
    product_id    SMALLINT NOT NULL REFERENCES products (id) ON DELETE CASCADE,
    unit          TEXT NOT NULL,
    base_quantity NUMERIC(12, 3) NOT NULL CHECK (base_quantity > 0),
    PRIMARY KEY (product_id, unit)
);

-- foydalanuvchi. Identity comes from Telegram login, so telegram_id is the
-- natural key and also links the bot to the same profile.
CREATE TABLE users (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    telegram_id       BIGINT NOT NULL UNIQUE,
    telegram_username TEXT,
    display_name      TEXT NOT NULL,
    phone_enc         BYTEA,
    phone_hash        BYTEA UNIQUE,
    role              TEXT CHECK (role IN ('seller', 'buyer', 'market_admin', 'org_analyst', 'admin')),
    role_approved     BOOLEAN NOT NULL DEFAULT FALSE,
    region_id         SMALLINT REFERENCES regions (id),
    market_id         SMALLINT REFERENCES markets (id),
    categories        TEXT[] NOT NULL DEFAULT '{}',
    public_name       TEXT,
    consent_at        TIMESTAMPTZ,
    bot_enabled       BOOLEAN NOT NULL DEFAULT TRUE,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Server-side sessions: the cookie carries a random token, only its hash is stored.
CREATE TABLE sessions (
    token_hash BYTEA PRIMARY KEY,
    user_id    UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX sessions_user_idx ON sessions (user_id);

-- Pending OAuth logins (state + PKCE verifier), kept in the database so a
-- restart in the middle of a login does not break it.
CREATE TABLE oauth_states (
    state_hash    BYTEA PRIMARY KEY,
    code_verifier TEXT NOT NULL,
    nonce         TEXT NOT NULL,
    return_to     TEXT NOT NULL DEFAULT '/',
    expires_at    TIMESTAMPTZ NOT NULL
);

-- narx_kuzatuv: every price from the platform and external sources.
CREATE TABLE price_observations (
    id            BIGSERIAL PRIMARY KEY,
    product_id    SMALLINT NOT NULL REFERENCES products (id),
    price_som     BIGINT NOT NULL CHECK (price_som > 0),
    unit          TEXT NOT NULL,
    quantity      NUMERIC(14, 3) CHECK (quantity > 0),
    trade_type    TEXT NOT NULL CHECK (trade_type IN ('ulgurji', 'chakana')),
    region_id     SMALLINT NOT NULL REFERENCES regions (id),
    market_id     SMALLINT REFERENCES markets (id),
    source        TEXT NOT NULL CHECK (source IN
                  ('seller', 'rfq_offer', 'telegram_channel', 'olx', 'uzex', 'narxtahlil')),
    source_url    TEXT,
    seller_id     UUID REFERENCES users (id) ON DELETE SET NULL,
    observed_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    valid_until   TIMESTAMPTZ NOT NULL,
    status        TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'expired', 'review', 'rejected')),
    raw_text      TEXT,
    audio_file_id TEXT,
    ai_confidence REAL CHECK (ai_confidence BETWEEN 0 AND 1),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX price_obs_lookup_idx ON price_observations (product_id, region_id, trade_type, observed_at DESC);
CREATE INDEX price_obs_seller_idx ON price_observations (seller_id, observed_at DESC);

-- sorov_rfq
CREATE TABLE rfqs (
    id            BIGSERIAL PRIMARY KEY,
    buyer_id      UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    product_id    SMALLINT NOT NULL REFERENCES products (id),
    quantity      NUMERIC(14, 3) NOT NULL CHECK (quantity > 0),
    unit          TEXT NOT NULL,
    region_id     SMALLINT NOT NULL REFERENCES regions (id),
    deadline      TIMESTAMPTZ NOT NULL,
    max_price_som BIGINT CHECK (max_price_som > 0),
    note          TEXT,
    status        TEXT NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'closed', 'cancelled')),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX rfqs_open_idx ON rfqs (product_id, region_id) WHERE status = 'open';

-- taklif: one offer per seller per RFQ, enforced by the database.
CREATE TABLE offers (
    id         BIGSERIAL PRIMARY KEY,
    rfq_id     BIGINT NOT NULL REFERENCES rfqs (id) ON DELETE CASCADE,
    seller_id  UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    price_som  BIGINT NOT NULL CHECK (price_som > 0),
    quantity   NUMERIC(14, 3) CHECK (quantity > 0),
    status     TEXT NOT NULL DEFAULT 'sent' CHECK (status IN ('sent', 'chosen', 'rejected', 'declined')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (rfq_id, seller_id)
);

-- savol_log: anonymised demand signal from price questions.
CREATE TABLE question_log (
    id         BIGSERIAL PRIMARY KEY,
    user_id    UUID REFERENCES users (id) ON DELETE SET NULL,
    question   TEXT NOT NULL,
    product_id SMALLINT REFERENCES products (id),
    region_id  SMALLINT REFERENCES regions (id),
    asked_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- manba_ishga_tushish: connector runs.
CREATE TABLE source_runs (
    id          BIGSERIAL PRIMARY KEY,
    source      TEXT NOT NULL,
    started_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    finished_at TIMESTAMPTZ,
    records     INTEGER NOT NULL DEFAULT 0,
    error       TEXT
);

-- ai_log: latency and cost per AI call. Inputs are stored as hashes only.
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

-- +goose Down
DROP TABLE ai_log;
DROP TABLE source_runs;
DROP TABLE question_log;
DROP TABLE offers;
DROP TABLE rfqs;
DROP TABLE price_observations;
DROP TABLE oauth_states;
DROP TABLE sessions;
DROP TABLE users;
DROP TABLE unit_conversions;
DROP TABLE products;
DROP TABLE markets;
DROP TABLE regions;
