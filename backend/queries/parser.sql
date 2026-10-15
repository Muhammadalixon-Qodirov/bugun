-- Parser measurement (MATN_TUSHUNISH.md 1).

-- name: InsertParseLog :one
INSERT INTO parse_log (user_id, via, raw_text, items, problems, items_n, problems_n, by_model, outcome)
VALUES (sqlc.narg(user_id), sqlc.arg(via), sqlc.arg(raw_text), sqlc.arg(items), sqlc.arg(problems),
        sqlc.arg(items_n), sqlc.arg(problems_n), sqlc.arg(by_model), sqlc.arg(outcome))
RETURNING id;

-- name: SetParseOutcome :exec
UPDATE parse_log SET outcome = sqlc.arg(outcome), decided_at = now()
WHERE id = sqlc.arg(id) AND outcome = 'pending';

-- name: ParseStats :one
SELECT count(*)::int AS messages,
       count(*) FILTER (WHERE via = 'web')::int AS web,
       count(*) FILTER (WHERE via = 'bot')::int AS bot,
       count(*) FILTER (WHERE via = 'channel')::int AS channel,
       count(*) FILTER (WHERE items_n > 0 AND problems_n = 0)::int AS full,
       count(*) FILTER (WHERE items_n > 0 AND problems_n > 0)::int AS partial,
       count(*) FILTER (WHERE items_n = 0)::int AS failed,
       count(*) FILTER (WHERE by_model)::int AS by_model,
       count(*) FILTER (WHERE outcome = 'confirmed')::int AS confirmed,
       count(*) FILTER (WHERE outcome = 'discarded')::int AS discarded,
       count(*) FILTER (WHERE outcome = 'pending' AND created_at < now() - interval '1 hour')::int AS expired,
       count(*) FILTER (WHERE outcome = 'pending' AND created_at >= now() - interval '1 hour')::int AS pending
FROM parse_log
WHERE created_at > now() - make_interval(days => sqlc.arg(days)::int);

-- Texts nothing was understood in, and the problems of the rest, for the
-- list of words the parser does not know.
-- name: ParseMisses :many
SELECT raw_text, items_n, problems
FROM parse_log
WHERE created_at > now() - make_interval(days => sqlc.arg(days)::int) AND (items_n = 0 OR problems_n > 0)
ORDER BY id DESC
LIMIT 2000;

-- name: DeleteOldParseLog :execrows
DELETE FROM parse_log WHERE created_at < now() - interval '180 days';

-- name: InsertLearnedAlias :exec
INSERT INTO learned_aliases (phrase, product_id, user_id) VALUES (sqlc.arg(phrase), sqlc.arg(product_id), sqlc.arg(user_id))
ON CONFLICT DO NOTHING;

-- Names confirmed by at least three sellers for the same product, and no
-- other product: everyone's parser reads them.
-- name: SharedLearnedAliases :many
SELECT phrase, min(product_id)::smallint AS product_id
FROM (SELECT phrase, product_id FROM learned_aliases GROUP BY phrase, product_id HAVING count(DISTINCT user_id) >= 3) x
GROUP BY phrase
HAVING count(*) = 1;

-- name: UserLearnedAliases :many
SELECT phrase, product_id FROM learned_aliases WHERE user_id = sqlc.arg(user_id);
