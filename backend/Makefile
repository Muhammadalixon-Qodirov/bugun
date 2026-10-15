.PHONY: run build test test-db vet sqlc openapi migrate eval

run:
	go run ./cmd/api

build:
	go build -o bin/api ./cmd/api
	go build -o bin/migrate ./cmd/migrate

# Applies pending migrations (the API does it too unless MIGRATE_ON_START=false).
migrate:
	go run ./cmd/migrate up

test:
	go test ./...

# Also runs database integration tests (needs a local "bugun_test" database).
# Packages share that database and clear tables, so they run one at a time (-p 1).
test-db:
	TEST_DATABASE_URL="postgres://localhost:5432/bugun_test?sslmode=disable" go test -race -p 1 ./...

vet:
	go vet ./...

sqlc:
	go tool sqlc generate

# Writes the OpenAPI spec consumed by the frontend code generator.
openapi:
	go run ./cmd/openapi > openapi.json

# AI evaluation (spec §16.3) on the labelled sets in eval/: prints the numbers
# and writes eval/RESULTS.md. Offline on the rule-based paths; set LLM_* or
# EMBEDDINGS_* to compare a model. Reads the catalog from EVAL_DATABASE_URL
# (else DATABASE_URL), a migrated database.
eval:
	go run ./cmd/eval
