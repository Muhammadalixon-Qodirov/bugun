-- +goose Up
-- The chat assistant (CHATBOT_REJA.md): conversations kept 30 days so a
-- follow-up ("Andijonda-chi?") is understood, and the questions it could not
-- answer, kept 180 days for the admins to close the gaps.
CREATE TABLE chat_sessions (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID REFERENCES users (id) ON DELETE CASCADE,
    key_hash   TEXT NOT NULL, -- who may continue it: hash of the user id or visitor key
    options    JSONB NOT NULL DEFAULT '[]', -- the last buttons offered
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX chat_sessions_user_idx ON chat_sessions (user_id, updated_at DESC);
CREATE INDEX chat_sessions_updated_idx ON chat_sessions (updated_at);

CREATE TABLE chat_messages (
    id         BIGSERIAL PRIMARY KEY,
    session_id UUID NOT NULL REFERENCES chat_sessions (id) ON DELETE CASCADE,
    role       TEXT NOT NULL CHECK (role IN ('user', 'assistant')),
    content    TEXT NOT NULL,
    kind       TEXT NOT NULL DEFAULT '' CHECK (kind IN ('', 'answer', 'clarify', 'unanswered')),
    status     TEXT NOT NULL DEFAULT '',
    mode       TEXT NOT NULL DEFAULT '',
    fallback   TEXT NOT NULL DEFAULT '',
    tools      TEXT[] NOT NULL DEFAULT '{}',
    latency_ms INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX chat_messages_session_idx ON chat_messages (session_id, id DESC);
CREATE INDEX chat_messages_created_idx ON chat_messages (created_at) WHERE role = 'assistant';

CREATE TABLE chat_unanswered (
    id         BIGSERIAL PRIMARY KEY,
    session_id UUID REFERENCES chat_sessions (id) ON DELETE SET NULL,
    user_id    UUID REFERENCES users (id) ON DELETE SET NULL,
    question   TEXT NOT NULL,
    norm       TEXT NOT NULL, -- normalized text, to group the same question
    reason     TEXT NOT NULL CHECK (reason IN ('off_topic', 'no_data', 'no_tool', 'tool_error', 'unclear')),
    topic      TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX chat_unanswered_created_idx ON chat_unanswered (created_at);

-- +goose Down
DROP TABLE chat_unanswered;
DROP TABLE chat_messages;
DROP TABLE chat_sessions;
