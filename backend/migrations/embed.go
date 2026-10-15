// Package migrations holds goose SQL migrations embedded into the binary.
//
// Policy:
//   - A migration that has been applied in production is never edited; a
//     change to the schema is a new, numbered file.
//   - Migrations hold schema. Some early ones (00003_seed_catalog,
//     00006_all_regions, 00009_more_products) also insert the catalog and
//     the regions; they stay as they are, since rewriting an applied
//     migration would split databases. New reference or demo data goes to
//     cmd/seed, which can be rerun and is kept out of the schema history.
//   - The API applies pending migrations at start (MIGRATE_ON_START=true)
//     under a Postgres advisory lock, so parallel starts wait for each other.
//     A deployment may instead run cmd/migrate ("up", "status") as its own
//     step and start the API with MIGRATE_ON_START=false.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
