-- Model calls (ai_log) and catalog embeddings.

-- name: InsertAILog :exec
INSERT INTO ai_log (feature, model, input_tokens, output_tokens, audio_seconds, latency_ms, cost_usd, ok, error_code, user_id, input_hash)
VALUES (@feature, @model, @input_tokens, @output_tokens, sqlc.narg(audio_seconds), @latency_ms, @cost_usd::float8, @ok, sqlc.narg(error_code), sqlc.narg(user_id), sqlc.narg(input_hash));

-- Per Tashkent day, feature and model since a moment.
-- name: AIUsageDaily :many
SELECT (created_at AT TIME ZONE 'Asia/Tashkent')::date AS day, feature, model,
       count(*)::int AS calls,
       count(*) FILTER (WHERE NOT ok)::int AS errors,
       COALESCE(sum(input_tokens), 0)::bigint AS input_tokens,
       COALESCE(sum(output_tokens), 0)::bigint AS output_tokens,
       COALESCE(sum(audio_seconds), 0)::float8 AS audio_seconds,
       COALESCE(sum(cost_usd), 0)::float8 AS cost_usd,
       COALESCE(avg(latency_ms), 0)::float8 AS avg_latency_ms,
       COALESCE(percentile_cont(0.95) WITHIN GROUP (ORDER BY latency_ms), 0)::float8 AS p95_latency_ms
FROM ai_log
WHERE created_at >= @since
GROUP BY 1, 2, 3
ORDER BY 1 DESC, 2, 3;

-- Error codes since a moment, most frequent first.
-- name: AIUsageErrors :many
SELECT feature, COALESCE(error_code, 'unknown')::text AS error_code, count(*)::int AS calls
FROM ai_log
WHERE created_at >= @since AND NOT ok
GROUP BY 1, 2
ORDER BY 3 DESC, 1, 2;

-- name: DeleteOldAILog :execrows
DELETE FROM ai_log WHERE created_at < @before;

-- name: ListProductEmbeddings :many
SELECT product_id, model, source_hash, embedding FROM product_embeddings;

-- name: UpsertProductEmbedding :exec
INSERT INTO product_embeddings (product_id, model, source_hash, embedding, updated_at)
VALUES (@product_id, @model, @source_hash, @embedding::real[], now())
ON CONFLICT (product_id) DO UPDATE
SET model = EXCLUDED.model, source_hash = EXCLUDED.source_hash, embedding = EXCLUDED.embedding, updated_at = now();

-- Catalog names and other names most like a word (pg_trgm), best first.
-- norm is the word in Latin script, raw as written (lowercase).
-- name: MatchProductsTrgm :many
SELECT p.id, p.slug, p.name,
       max(greatest(similarity(@norm::text, lower(a)), similarity(@raw::text, lower(a))))::float8 AS score
FROM products p, unnest(array_append(p.synonyms, p.name)) AS a
GROUP BY p.id, p.slug, p.name
HAVING max(greatest(similarity(@norm::text, lower(a)), similarity(@raw::text, lower(a)))) >= @min_score::float8
ORDER BY score DESC, p.id
LIMIT @max_rows;
