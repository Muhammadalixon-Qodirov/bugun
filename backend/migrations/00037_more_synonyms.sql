-- +goose Up
-- Common names the parser missed (MATN_TUSHUNISH.md 2): "sarimsoq piyoz" was
-- read as garlic and onion.
UPDATE products SET synonyms = array_append(synonyms, 'sarimsoq piyoz') WHERE slug = 'sarimsoq' AND NOT 'sarimsoq piyoz' = ANY (synonyms);

-- +goose Down
UPDATE products SET synonyms = array_remove(synonyms, 'sarimsoq piyoz') WHERE slug = 'sarimsoq';
