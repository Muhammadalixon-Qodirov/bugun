package external

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"bugun/internal/store"
	"bugun/internal/store/db"
)

func testQueries(t *testing.T) (*db.Queries, *pgxpool.Pool) {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	pool, err := store.Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err := store.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	return db.New(pool), pool
}

// cbuStub answers like cbu.uz: all currencies of a day, rate per Nominal.
func cbuStub(t *testing.T, fail func(day string) bool) (*httptest.Server, *atomic.Int32) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		day := strings.Trim(strings.TrimPrefix(r.URL.Path, "/"), "/")
		if fail != nil && fail(day) {
			http.Error(w, "down", http.StatusBadGateway)
			return
		}
		fmt.Fprintf(w, `[{"Ccy":"USD","Rate":"12650.50","Nominal":"1","Date":"%s"},{"Ccy":"EUR","Rate":"13900","Nominal":"1"},
			{"Ccy":"RUB","Rate":"140.10","Nominal":"1"},{"Ccy":"KZT","Rate":"26.4","Nominal":"1"}]`, day)
	}))
	t.Cleanup(srv.Close)
	return srv, &calls
}

func TestFetch(t *testing.T) {
	srv, _ := cbuStub(t, nil)
	f := &FX{Client: srv.Client(), Base: srv.URL + "/"}
	rates, err := f.Fetch(context.Background(), time.Date(2024, 2, 12, 0, 0, 0, 0, time.UTC))
	if err != nil || rates["USD"] != 12650.5 || rates["RUB"] != 140.1 || rates["KZT"] != 26.4 || len(rates) != 3 {
		t.Errorf("rates %v %v", rates, err)
	}
}

func TestSyncFillsMissingDays(t *testing.T) {
	q, pool := testQueries(t)
	ctx := context.Background()
	if _, err := pool.Exec(ctx, `DELETE FROM fx_daily`); err != nil {
		t.Fatal(err)
	}
	failOnce := map[string]bool{}
	var failed atomic.Int32
	srv, calls := cbuStub(t, func(day string) bool {
		// One day fails the first time only.
		if day == tashkentDay(time.Now()).AddDate(0, 0, -3).Format(time.DateOnly) && !failOnce[day] {
			failOnce[day] = true
			failed.Add(1)
			return true
		}
		return false
	})
	f := &FX{Q: q, Client: srv.Client(), Base: srv.URL + "/"}
	n, err := f.Sync(ctx)
	if err != nil || n != History {
		t.Fatalf("first sync: %d days, %v; want %d (one failed)", n, err, History)
	}
	n, err = f.Sync(ctx)
	if err != nil || n != 1 {
		t.Errorf("second sync: %d days, %v; want only the failed day", n, err)
	}
	before := calls.Load()
	if n, _ := f.Sync(ctx); n != 0 || calls.Load() != before {
		t.Errorf("third sync fetched %d days", n)
	}
}

// A throttled or dead Central Bank stops the run after 5 failures in a row;
// the newest days, fetched first, are already stored.
func TestSyncStopsWhenThrottled(t *testing.T) {
	q, pool := testQueries(t)
	ctx := context.Background()
	if _, err := pool.Exec(ctx, `DELETE FROM fx_daily`); err != nil {
		t.Fatal(err)
	}
	var served atomic.Int32
	srv, calls := cbuStub(t, func(string) bool { return served.Add(1) > 3 }) // 3 days, then only errors
	f := &FX{Q: q, Client: srv.Client(), Base: srv.URL + "/"}
	n, err := f.Sync(ctx)
	if err == nil || n != 3 || calls.Load() != 8 {
		t.Fatalf("throttled sync: %d days, %d calls, %v; want 3 days and a stop after 5 failures", n, calls.Load(), err)
	}
	var newest time.Time
	if err := pool.QueryRow(ctx, `SELECT max(day) FROM fx_daily`).Scan(&newest); err != nil || !newest.Equal(tashkentDay(time.Now())) {
		t.Errorf("newest stored day %v %v, want today", newest, err)
	}
}
