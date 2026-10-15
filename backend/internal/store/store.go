// Package store opens the Postgres pool and applies migrations.
package store

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"bugun/migrations"
)

func Open(ctx context.Context, url string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, fmt.Errorf("parse database url: %w", err)
	}
	// All timestamps are stored in UTC; conversion to Asia/Tashkent happens at the edges.
	cfg.ConnConfig.RuntimeParams["timezone"] = "UTC"
	// Queries use optional filters ("$1 IS NULL OR col = $1"). Postgres switches
	// cached statements to a generic plan after five runs, which cannot drop the
	// unused filters and was 20x slower; always plan for the actual arguments.
	cfg.ConnConfig.RuntimeParams["plan_cache_mode"] = "force_custom_plan"
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("open pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}
	return pool, nil
}

// migrationLock is the Postgres advisory lock key that serialises migrations:
// two API instances starting at once (or cmd/migrate beside a starting API)
// apply them one after the other instead of racing.
const migrationLock int64 = 0x6275_6775_6e00_0001 // "bugun" 1

// Migrate applies every pending migration under the advisory lock.
func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	return withMigrationLock(ctx, pool, func(p *goose.Provider) error {
		before, _ := p.GetDBVersion(ctx)
		if _, err := p.Up(ctx); err != nil {
			return fmt.Errorf("migrate: %w", err)
		}
		after, _ := p.GetDBVersion(ctx)
		slog.Info("database migrated", "from", before, "to", after)
		return nil
	})
}

// MigrationStatus lists every migration and whether it is applied.
func MigrationStatus(ctx context.Context, pool *pgxpool.Pool) ([]*goose.MigrationStatus, error) {
	var out []*goose.MigrationStatus
	err := withMigrationLock(ctx, pool, func(p *goose.Provider) error {
		var err error
		out, err = p.Status(ctx)
		return err
	})
	return out, err
}

// withMigrationLock holds the advisory lock on a connection of its own while
// fn runs; other callers wait for it.
func withMigrationLock(ctx context.Context, pool *pgxpool.Pool, fn func(*goose.Provider) error) error {
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("migration lock: %w", err)
	}
	defer conn.Release()
	if _, err := conn.Exec(ctx, "SELECT pg_advisory_lock($1)", migrationLock); err != nil {
		return fmt.Errorf("migration lock: %w", err)
	}
	defer func() {
		// Unlock even when ctx is done; a connection that could not unlock
		// is closed, which ends its session and so the lock.
		uctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		if _, err := conn.Exec(uctx, "SELECT pg_advisory_unlock($1)", migrationLock); err != nil {
			slog.Warn("migration unlock", "err", err)
			_ = conn.Conn().Close(uctx)
		}
	}()

	db := stdlib.OpenDBFromPool(pool)
	defer db.Close()
	p, err := goose.NewProvider(goose.DialectPostgres, db, migrations.FS)
	if err != nil {
		return fmt.Errorf("migrations: %w", err)
	}
	return fn(p)
}
