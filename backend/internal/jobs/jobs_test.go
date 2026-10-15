package jobs

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"bugun/internal/store"
	"bugun/internal/store/db"
	"bugun/internal/tz"
)

func at(s string) time.Time {
	t, err := time.ParseInLocation("2006-01-02 15:04", s, tz.Tashkent)
	if err != nil {
		panic(err)
	}
	return t
}

func TestWeekly(t *testing.T) {
	monday9 := Weekly(time.Monday, 9, 0)
	cases := map[string]string{
		"2026-09-28 09:00": "2026-09-28 09:00", // Monday, on the dot
		"2026-09-28 10:30": "2026-09-28 09:00",
		"2026-09-28 08:59": "2026-09-21 09:00", // Monday before 9: last week's slot
		"2026-09-25 13:00": "2026-09-21 09:00", // Friday
		"2026-09-27 23:59": "2026-09-21 09:00", // Sunday
	}
	for now, want := range cases {
		if got := monday9(at(now)); !got.Equal(at(want)) {
			t.Errorf("slot at %s = %s, want %s", now, got.In(tz.Tashkent).Format("2006-01-02 15:04"), want)
		}
	}
	// The server clock's zone must not matter.
	utc := at("2026-09-28 09:30").UTC()
	if got := monday9(utc); !got.Equal(at("2026-09-28 09:00")) {
		t.Errorf("slot from a UTC clock = %s", got)
	}
}

func TestDaily(t *testing.T) {
	slot := Daily(Clock{7, 30}, Clock{12, 0}, Clock{16, 30})
	for now, want := range map[string]string{
		"2026-09-25 07:30": "2026-09-25 07:30",
		"2026-09-25 11:59": "2026-09-25 07:30",
		"2026-09-25 12:00": "2026-09-25 12:00",
		"2026-09-25 23:00": "2026-09-25 16:30",
		"2026-09-25 07:29": "2026-09-24 16:30", // before the first slot: yesterday's last
		"2026-10-01 03:00": "2026-09-30 16:30", // across a month end
	} {
		if got := slot(at(now)); !got.Equal(at(want)) {
			t.Errorf("slot at %s = %s, want %s", now, got.In(tz.Tashkent).Format("2006-01-02 15:04"), want)
		}
	}
}

func TestEvery(t *testing.T) {
	slot := Every(15 * time.Minute)
	for now, want := range map[string]string{
		"2026-09-25 09:00": "2026-09-25 09:00",
		"2026-09-25 09:14": "2026-09-25 09:00",
		"2026-09-25 09:15": "2026-09-25 09:15",
		"2026-09-25 23:59": "2026-09-25 23:45",
	} {
		if got := slot(at(now)); !got.Equal(at(want)) {
			t.Errorf("slot at %s = %s, want %s", now, got.In(tz.Tashkent).Format("2006-01-02 15:04"), want)
		}
	}
}

func TestDue(t *testing.T) {
	j := Job{Slot: Weekly(time.Monday, 9, 0), Window: 6 * time.Hour}
	for now, want := range map[string]bool{
		"2026-09-28 09:00": true,
		"2026-09-28 14:59": true,  // restarted late, still inside the window
		"2026-09-28 15:00": false, // too late: wait for next Monday
		"2026-09-29 09:00": false,
	} {
		if _, got := j.Due(at(now)); got != want {
			t.Errorf("Due at %s = %v, want %v", now, got, want)
		}
	}
}

// A slot runs once, even when two schedulers tick at the same time.
func TestSlotRunsOnce(t *testing.T) {
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
	name := "test-" + time.Now().Format("150405.000000")
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM job_runs WHERE job = $1`, name) })

	var runs atomic.Int32
	job := Job{Name: name, Slot: Weekly(time.Monday, 9, 0), Window: 6 * time.Hour,
		Run: func(context.Context) (int, error) { runs.Add(1); return 3, nil }}
	a, b := NewScheduler(db.New(pool), job), NewScheduler(db.New(pool), job)
	now := at("2026-09-28 09:01")
	done := make(chan struct{})
	go func() { a.Tick(ctx, now); close(done) }()
	b.Tick(ctx, now)
	<-done
	a.Tick(ctx, now.Add(time.Minute)) // later tick in the same slot
	if n := runs.Load(); n != 1 {
		t.Errorf("job ran %d times in one slot, want 1", n)
	}
	var items int
	if err := pool.QueryRow(ctx, `SELECT items FROM job_runs WHERE job = $1 AND finished_at IS NOT NULL`, name).Scan(&items); err != nil || items != 3 {
		t.Errorf("job run record: items %d, %v", items, err)
	}
}

// A run a restart cut off is released, so its slot runs again.
func TestCutOffRunRunsAgain(t *testing.T) {
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
	name := "test-cut-" + time.Now().Format("150405.000000")
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM job_runs WHERE job = $1`, name) })
	var runs atomic.Int32
	job := Job{Name: name, Slot: Weekly(time.Monday, 9, 0), Window: 6 * time.Hour,
		Run: func(context.Context) (int, error) { runs.Add(1); return 1, nil }}
	now := at("2026-09-28 09:01")
	// The previous process claimed the slot and died before finishing: its
	// heartbeat stopped ten minutes ago.
	if _, err := pool.Exec(ctx, `INSERT INTO job_runs (job, slot, heartbeat_at) VALUES ($1, $2, now() - interval '10 minutes')`, name, job.Slot(now)); err != nil {
		t.Fatal(err)
	}
	s := NewScheduler(db.New(pool), job)
	s.Tick(ctx, now)
	if runs.Load() != 0 {
		t.Fatal("a claimed slot ran")
	}
	released, err := db.New(pool).ReleaseStaleJobs(ctx, DefaultLeaseTimeout.Seconds())
	if err != nil || !slices.Contains(released, name) {
		t.Fatalf("released %v %v", released, err)
	}
	s.Tick(ctx, now.Add(time.Minute))
	if runs.Load() != 1 {
		t.Errorf("released slot ran %d times, want 1", runs.Load())
	}
}

// A long job does not hold the others up.
func TestLongJobDoesNotBlockOthers(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	pool, err := store.Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err := store.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	stamp := time.Now().Format("150405.000000")
	slow, fast := "test-slow-"+stamp, "test-fast-"+stamp
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM job_runs WHERE job IN ($1, $2)`, slow, fast)
	})
	release, fastDone := make(chan struct{}), make(chan struct{})
	s := NewScheduler(db.New(pool),
		Job{Name: slow, Slot: Every(time.Hour), Window: time.Hour, Run: func(context.Context) (int, error) { <-release; return 0, nil }},
		Job{Name: fast, Slot: Every(time.Hour), Window: time.Hour, Run: func(context.Context) (int, error) { close(fastDone); return 0, nil }},
	)
	go s.Run(ctx)
	select {
	case <-fastDone:
	case <-time.After(5 * time.Second):
		t.Fatal("the fast job waited for the slow one")
	}
	close(release)
}

func testPool(t *testing.T) *pgxpool.Pool {
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
	return pool
}

// A run another live instance is doing keeps its claim: only a heartbeat
// older than the lease is released.
func TestLiveRunIsNotReleased(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	name := "test-live-" + time.Now().Format("150405.000000")
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM job_runs WHERE job = $1`, name) })
	if _, err := pool.Exec(ctx, `INSERT INTO job_runs (job, slot, heartbeat_at) VALUES ($1, now(), now() - interval '1 minute')`, name); err != nil {
		t.Fatal(err)
	}
	released, err := db.New(pool).ReleaseStaleJobs(ctx, DefaultLeaseTimeout.Seconds())
	if err != nil || slices.Contains(released, name) {
		t.Fatalf("a run with a fresh heartbeat was released: %v %v", released, err)
	}
	if _, err := pool.Exec(ctx, `UPDATE job_runs SET heartbeat_at = now() - interval '6 minutes' WHERE job = $1`, name); err != nil {
		t.Fatal(err)
	}
	if released, err = db.New(pool).ReleaseStaleJobs(ctx, DefaultLeaseTimeout.Seconds()); err != nil || !slices.Contains(released, name) {
		t.Fatalf("a run silent past the lease stayed: %v %v", released, err)
	}
}

// A long job keeps its heartbeat fresh, and Run waits for it on shutdown.
func TestRunningJobRefreshesItsLease(t *testing.T) {
	pool := testPool(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	name := "test-lease-" + time.Now().Format("150405.000000")
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM job_runs WHERE job = $1`, name) })

	started, release := make(chan struct{}), make(chan struct{})
	var finished atomic.Bool
	s := NewScheduler(db.New(pool), Job{Name: name, Slot: Every(time.Hour), Window: time.Hour, Run: func(context.Context) (int, error) {
		close(started)
		<-release
		time.Sleep(50 * time.Millisecond)
		finished.Store(true)
		return 1, nil
	}}).WithLease(20*time.Millisecond, time.Second)
	stopped := make(chan struct{})
	go func() { s.Run(ctx); close(stopped) }()
	<-started
	time.Sleep(150 * time.Millisecond)
	var fresh bool
	if err := pool.QueryRow(ctx, `SELECT heartbeat_at > started_at + interval '50 milliseconds' FROM job_runs WHERE job = $1`, name).Scan(&fresh); err != nil || !fresh {
		t.Errorf("heartbeat not refreshed while running: %v %v", fresh, err)
	}

	cancel()
	close(release)
	select {
	case <-stopped:
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return after cancel")
	}
	if !finished.Load() {
		t.Error("Run returned before its running job finished")
	}
}

func TestHeartbeatPing(t *testing.T) {
	var hits atomic.Int32
	fail := atomic.Bool{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if fail.Load() {
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer srv.Close()
	h := &Heartbeat{URL: srv.URL + "/ping/abc", Every: 5 * time.Minute}
	now := time.Date(2026, 9, 26, 10, 0, 0, 0, time.UTC)
	if !h.due(now) || h.due(now.Add(4*time.Minute)) || !h.due(now.Add(5*time.Minute)) {
		t.Error("heartbeat not due every 5 minutes")
	}
	if err := h.ping(context.Background()); err != nil || hits.Load() != 1 {
		t.Errorf("ping: %v, %d hits", err, hits.Load())
	}
	fail.Store(true)
	if err := h.ping(context.Background()); err == nil {
		t.Error("a 500 from the monitor counted as success")
	}
	h.Ping(context.Background()) // logged, never panics
	down := &Heartbeat{URL: "http://127.0.0.1:1/ping/secret-token", Every: time.Minute}
	if err := down.ping(context.Background()); err == nil || strings.Contains(err.Error(), "secret-token") {
		t.Errorf("unreachable monitor: %v (the URL must stay out of the error)", err)
	}
}

// The scheduler pings the monitor as it ticks.
func TestSchedulerPings(t *testing.T) {
	pool := testPool(t)
	hit := make(chan struct{}, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		select {
		case hit <- struct{}{}:
		default:
		}
	}))
	defer srv.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s := NewScheduler(db.New(pool)).WithHeartbeat(&Heartbeat{URL: srv.URL, Every: time.Minute})
	go s.Run(ctx)
	select {
	case <-hit:
	case <-time.After(5 * time.Second):
		t.Fatal("no heartbeat ping")
	}
}
