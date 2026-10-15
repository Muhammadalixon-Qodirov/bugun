package store

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/pressly/goose/v3"
)

func TestMigrateWaitsForTheLock(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	pool, err := Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err := Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}

	// Another process holds the lock: Migrate waits until it lets go.
	other, err := Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	conn, err := other.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec(ctx, "SELECT pg_advisory_lock($1)", migrationLock); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- Migrate(ctx, pool) }()
	select {
	case err := <-done:
		t.Fatalf("Migrate ran while another process held the lock: %v", err)
	case <-time.After(300 * time.Millisecond):
	}
	if _, err := conn.Exec(ctx, "SELECT pg_advisory_unlock($1)", migrationLock); err != nil {
		t.Fatal(err)
	}
	conn.Release()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Migrate did not continue after the lock was released")
	}

	// Parallel starts: both succeed, one after the other.
	errs := make(chan error, 2)
	for range 2 {
		go func() { errs <- Migrate(ctx, pool) }()
	}
	for range 2 {
		if err := <-errs; err != nil {
			t.Errorf("parallel migrate: %v", err)
		}
	}

	// A cancelled wait gives up instead of hanging.
	conn, err = other.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Release()
	if _, err := conn.Exec(ctx, "SELECT pg_advisory_lock($1)", migrationLock); err != nil {
		t.Fatal(err)
	}
	short, cancel := context.WithTimeout(ctx, 200*time.Millisecond)
	defer cancel()
	if err := Migrate(short, pool); err == nil {
		t.Error("Migrate returned nil while the lock was held and ctx expired")
	}
	if _, err := conn.Exec(ctx, "SELECT pg_advisory_unlock($1)", migrationLock); err != nil {
		t.Fatal(err)
	}

	// Status lists every migration as applied.
	st, err := MigrationStatus(ctx, pool)
	if err != nil {
		t.Fatal(err)
	}
	if len(st) < 35 {
		t.Errorf("%d migrations in the status, want at least 35", len(st))
	}
	for _, m := range st {
		if m.State != goose.StateApplied {
			t.Errorf("migration %d is %s", m.Source.Version, m.State)
		}
	}
}
