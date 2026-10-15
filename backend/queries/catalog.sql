-- name: ListRegions :many
SELECT id, slug, name, kind, parent_id FROM regions ORDER BY parent_id NULLS FIRST, kind DESC, name;

-- name: ListMarkets :many
SELECT id, slug, name, region_id, kind FROM markets ORDER BY name;

-- name: ListProducts :many
SELECT id, slug, name, category, base_unit FROM products ORDER BY category, name;

-- name: GetProductBySlug :one
SELECT id, slug, name, category, base_unit, synonyms FROM products WHERE slug = $1;

-- name: GetMarket :one
SELECT id, slug, name, region_id, kind FROM markets WHERE id = $1;

-- name: GetRegion :one
SELECT id, slug, name, kind, parent_id FROM regions WHERE id = $1;

-- name: GetProduct :one
SELECT id, slug, name, category, base_unit FROM products WHERE id = $1;
