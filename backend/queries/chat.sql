-- The chat assistant (CHATBOT_REJA.md).

-- name: CreateChatSession :one
INSERT INTO chat_sessions (user_id, key_hash) VALUES (sqlc.narg(user_id), sqlc.arg(key_hash)) RETURNING id;

-- name: GetChatSession :one
SELECT id, user_id, key_hash, options, updated_at FROM chat_sessions WHERE id = $1;

-- The user's latest conversation, when it is recent enough to continue.
-- name: LatestChatSession :one
SELECT id FROM chat_sessions
WHERE user_id = sqlc.arg(user_id) AND updated_at > sqlc.arg(since)::timestamptz
ORDER BY updated_at DESC
LIMIT 1;

-- name: TouchChatSession :exec
UPDATE chat_sessions SET options = sqlc.arg(options), updated_at = now() WHERE id = sqlc.arg(id);

-- name: InsertChatMessage :exec
INSERT INTO chat_messages (session_id, role, content, kind, status, mode, fallback, tools, latency_ms)
VALUES (sqlc.arg(session_id), sqlc.arg(role), sqlc.arg(content), sqlc.arg(kind), sqlc.arg(status), sqlc.arg(mode),
        sqlc.arg(fallback), sqlc.arg(tools), sqlc.arg(latency_ms));

-- The last messages of a conversation, newest first.
-- name: RecentChatMessages :many
SELECT role, content, kind FROM chat_messages WHERE session_id = $1 ORDER BY id DESC LIMIT $2;

-- name: InsertChatUnanswered :exec
INSERT INTO chat_unanswered (session_id, user_id, question, norm, reason, topic)
VALUES (sqlc.narg(session_id), sqlc.narg(user_id), sqlc.arg(question), sqlc.arg(norm), sqlc.arg(reason), sqlc.arg(topic));

-- The same unanswered question grouped, most asked first.
-- name: UnansweredGroups :many
SELECT norm, reason, count(*)::int AS asked, (array_agg(question ORDER BY created_at DESC))[1]::text AS example,
       (array_agg(topic ORDER BY created_at DESC))[1]::text AS topic, max(created_at)::timestamptz AS last_at
FROM chat_unanswered
WHERE created_at > now() - make_interval(days => sqlc.arg(days)::int)
GROUP BY norm, reason
ORDER BY asked DESC, last_at DESC
LIMIT 100;

-- name: ChatStats :one
SELECT count(*)::int AS answers,
       count(*) FILTER (WHERE kind = 'answer')::int AS answered,
       count(*) FILTER (WHERE kind = 'clarify')::int AS clarified,
       count(*) FILTER (WHERE kind = 'unanswered')::int AS unanswered,
       count(*) FILTER (WHERE mode = 'model')::int AS by_model,
       count(*) FILTER (WHERE fallback <> '')::int AS fallbacks,
       count(*) FILTER (WHERE status = 'insufficient_data')::int AS insufficient,
       COALESCE(avg(latency_ms), 0)::int AS avg_latency_ms,
       count(DISTINCT session_id)::int AS conversations
FROM chat_messages
WHERE role = 'assistant' AND created_at > now() - make_interval(days => sqlc.arg(days)::int);

-- name: ChatToolUse :many
SELECT t::text AS tool, count(*)::int AS calls
FROM chat_messages, unnest(tools) AS t
WHERE role = 'assistant' AND created_at > now() - make_interval(days => sqlc.arg(days)::int)
GROUP BY t
ORDER BY calls DESC;

-- name: DeleteOldChats :one
WITH s AS (DELETE FROM chat_sessions WHERE updated_at < now() - interval '30 days' RETURNING 1),
     u AS (DELETE FROM chat_unanswered WHERE created_at < now() - interval '180 days' RETURNING 1)
SELECT ((SELECT count(*) FROM s) + (SELECT count(*) FROM u))::int AS deleted;
