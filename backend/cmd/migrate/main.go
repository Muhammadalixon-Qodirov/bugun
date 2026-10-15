// Command migrate applies the database migrations as a step of its own, for
// deployments that start the API with MIGRATE_ON_START=false.
//
//	migrate up      apply every pending migration (the default)
//	migrate status  list the migrations and whether each is applied
//
// It reads DATABASE_URL from the environment (or .env in development) and
// takes the same advisory lock as the API, so it never races a starting API.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"

	"bugun/internal/store"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		slog.Error("migrate", "err", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	cmd := "up"
	if len(args) > 0 {
		cmd = args[0]
	}
	if len(args) > 1 || (cmd != "up" && cmd != "status") {
		return errors.New("usage: migrate [up|status]")
	}
	_ = godotenv.Load()
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		return errors.New("DATABASE_URL is not set")
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	pool, err := store.Open(ctx, url)
	if err != nil {
		return err
	}
	defer pool.Close()

	if cmd == "up" {
		return store.Migrate(ctx, pool)
	}
	st, err := store.MigrationStatus(ctx, pool)
	if err != nil {
		return err
	}
	pending := 0
	for _, m := range st {
		applied := "pending"
		if !m.AppliedAt.IsZero() {
			applied = m.AppliedAt.UTC().Format(time.DateTime) + " UTC"
		} else {
			pending++
		}
		fmt.Printf("%05d  %-45s  %s\n", m.Source.Version, m.Source.Path, applied)
	}
	fmt.Printf("%d migrations, %d pending\n", len(st), pending)
	return nil
}
