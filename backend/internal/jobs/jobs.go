// Package jobs runs scheduled work on Asia/Tashkent wall-clock time. Each run
// claims its time slot in job_runs first, so a slot runs once even with two
// API instances, and a slot missed during a restart still runs when the
// server comes back within the job's window (CLAUDE §4.2, §4.3).
package jobs

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"bugun/internal/store/db"
	"bugun/internal/tz"
)

// Job is work that runs once per slot.
type Job struct {
	Name string
	// Slot returns the most recent scheduled moment at or before now.
	Slot func(now time.Time) time.Time
	// Window is how late after its slot a run may still start.
	Window time.Duration
	// Run does the work and reports how many items it handled.
	Run func(ctx context.Context) (int, error)
	// MaxGap is the longest normal pause between two runs; monitoring
	// reports the job as overdue after it. Zero means not watched.
	MaxGap time.Duration
}

// Weekly is a slot at a weekday and Tashkent clock time, e.g. Monday 09:00.
func Weekly(day time.Weekday, hour, minute int) func(time.Time) time.Time {
	return func(now time.Time) time.Time {
		t := now.In(tz.Tashkent)
		slot := time.Date(t.Year(), t.Month(), t.Day(), hour, minute, 0, 0, tz.Tashkent)
		slot = slot.AddDate(0, 0, -int((t.Weekday()-day+7)%7))
		if slot.After(t) {
			slot = slot.AddDate(0, 0, -7)
		}
		return slot
	}
}

// Clock is a Tashkent wall-clock time of day.
type Clock struct{ Hour, Minute int }

func (c Clock) String() string { return fmt.Sprintf("%02d:%02d", c.Hour, c.Minute) }

// UnmarshalText reads "HH:MM" (24-hour), so clocks can come from the
// environment; anything else, such as "25:00" or "7.30", is an error.
func (c *Clock) UnmarshalText(b []byte) error {
	h, m, ok := strings.Cut(strings.TrimSpace(string(b)), ":")
	hour, herr := strconv.Atoi(h)
	minute, merr := strconv.Atoi(m)
	if !ok || herr != nil || merr != nil || len(m) != 2 || hour < 0 || hour > 23 || minute < 0 || minute > 59 {
		return fmt.Errorf("clock time %q: want HH:MM between 00:00 and 23:59", b)
	}
	*c = Clock{hour, minute}
	return nil
}

// WeekClock is a weekday and a Tashkent clock time, e.g. "Mon 09:00".
type WeekClock struct {
	Day time.Weekday
	Clock
}

func (w WeekClock) String() string { return w.Day.String()[:3] + " " + w.Clock.String() }

// UnmarshalText reads "Mon 09:00" (the weekday's first three letters).
func (w *WeekClock) UnmarshalText(b []byte) error {
	day, clock, ok := strings.Cut(strings.TrimSpace(string(b)), " ")
	if !ok {
		return fmt.Errorf("weekly time %q: want a weekday and HH:MM, e.g. Mon 09:00", b)
	}
	for d := time.Sunday; d <= time.Saturday; d++ {
		if strings.EqualFold(day, d.String()[:3]) {
			w.Day = d
			return w.Clock.UnmarshalText([]byte(clock))
		}
	}
	return fmt.Errorf("weekly time %q: unknown weekday %q", b, day)
}

// Daily is a slot at each of the given clock times every day, e.g. 07:30,
// 12:00 and 16:30.
func Daily(times ...Clock) func(time.Time) time.Time {
	return func(now time.Time) time.Time {
		t := now.In(tz.Tashkent)
		var last time.Time
		for _, back := range []int{0, -1} {
			for _, c := range times {
				slot := time.Date(t.Year(), t.Month(), t.Day()+back, c.Hour, c.Minute, 0, 0, tz.Tashkent)
				if !slot.After(t) && slot.After(last) {
					last = slot
				}
			}
		}
		return last
	}
}

// Every is a slot every d, aligned to the clock (d divides an hour), e.g.
// every 15 minutes at :00, :15, :30 and :45.
func Every(d time.Duration) func(time.Time) time.Time {
	return func(now time.Time) time.Time { return now.Truncate(d) }
}

// Due reports whether the job's current slot should run at now.
func (j Job) Due(now time.Time) (time.Time, bool) {
	slot := j.Slot(now)
	return slot, !now.Before(slot) && now.Sub(slot) < j.Window
}

// Default lease: a running job refreshes its heartbeat every minute, and an
// unfinished run without one for five minutes counts as cut off.
const (
	DefaultHeartbeat    = time.Minute
	DefaultLeaseTimeout = 5 * time.Minute
)

type Scheduler struct {
	q    *db.Queries
	jobs []Job

	heartbeat, lease time.Duration
	pinger           *Heartbeat // external uptime check; nil when off

	mu      sync.Mutex
	running map[string]bool
	wg      sync.WaitGroup // jobs and pings started by Run
}

func NewScheduler(q *db.Queries, jobs ...Job) *Scheduler {
	return &Scheduler{q: q, jobs: jobs, running: map[string]bool{}, heartbeat: DefaultHeartbeat, lease: DefaultLeaseTimeout}
}

// WithLease sets how often a running job refreshes its heartbeat and how old
// a heartbeat may get before the run counts as cut off.
func (s *Scheduler) WithLease(heartbeat, timeout time.Duration) *Scheduler {
	s.heartbeat, s.lease = heartbeat, timeout
	return s
}

// WithHeartbeat pings an external uptime monitor while the scheduler runs.
func (s *Scheduler) WithHeartbeat(h *Heartbeat) *Scheduler {
	s.pinger = h
	return s
}

// Run checks the jobs every minute until ctx is cancelled, then waits for
// the jobs it started to return. Each check first releases runs whose
// heartbeat is older than the lease, so a job cut off by a crash or restart
// runs again instead of waiting for its next slot, while a job another
// instance is still running keeps its claim. Each job runs in its own
// goroutine, so a long one (a three-year backfill) never holds the others
// up; a job still running is not started again.
func (s *Scheduler) Run(ctx context.Context) {
	defer s.wg.Wait()
	tick := time.NewTicker(time.Minute)
	defer tick.Stop()
	for {
		now := time.Now()
		s.releaseStale(ctx)
		for _, j := range s.jobs {
			if _, due := j.Due(now); !due || !s.begin(j.Name) {
				continue
			}
			s.wg.Go(func() {
				defer s.end(j.Name)
				s.runJob(ctx, j, now)
			})
		}
		if s.pinger != nil && s.pinger.due(now) {
			s.wg.Go(func() { s.pinger.Ping(ctx) })
		}
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
	}
}

// releaseStale frees the slots of runs nobody is refreshing any more.
func (s *Scheduler) releaseStale(ctx context.Context) {
	jobs, err := s.q.ReleaseStaleJobs(ctx, s.lease.Seconds())
	if err != nil {
		if ctx.Err() == nil {
			slog.ErrorContext(ctx, "release stale jobs", "err", err)
		}
		return
	}
	if len(jobs) > 0 {
		slog.WarnContext(ctx, "released jobs cut off by a restart or crash", "jobs", jobs)
	}
}

// begin marks a job as running; false when it already is.
func (s *Scheduler) begin(name string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.running[name] {
		return false
	}
	s.running[name] = true
	return true
}

func (s *Scheduler) end(name string) {
	s.mu.Lock()
	delete(s.running, name)
	s.mu.Unlock()
}

// Tick runs every job that is due at now and not yet claimed, one after
// another (tests; Run starts them side by side).
func (s *Scheduler) Tick(ctx context.Context, now time.Time) {
	for _, j := range s.jobs {
		s.runJob(ctx, j, now)
	}
}

// runJob claims the job's slot at now and runs it, once per slot.
func (s *Scheduler) runJob(ctx context.Context, j Job, now time.Time) {
	slot, due := j.Due(now)
	if !due {
		return
	}
	id, err := s.q.ClaimJobSlot(ctx, db.ClaimJobSlotParams{Job: j.Name, Slot: slot})
	if errors.Is(err, pgx.ErrNoRows) {
		return // already run for this slot
	}
	if err != nil {
		slog.ErrorContext(ctx, "claim job slot", "job", j.Name, "err", err)
		return
	}
	stop := s.keepAlive(ctx, j.Name, id)
	items, runErr := j.Run(ctx)
	stop()
	msg := pgtype.Text{}
	if runErr != nil {
		msg = pgtype.Text{String: runErr.Error(), Valid: true}
		slog.ErrorContext(ctx, "job failed", "job", j.Name, "err", runErr)
	} else {
		slog.InfoContext(ctx, "job done", "job", j.Name, "items", items)
	}
	if err := s.q.FinishJobRun(ctx, db.FinishJobRunParams{ID: id, Items: int32(items), Error: msg}); err != nil {
		slog.ErrorContext(ctx, "finish job run", "job", j.Name, "err", err)
	}
}

// keepAlive refreshes the run's heartbeat until the returned stop is called.
func (s *Scheduler) keepAlive(ctx context.Context, job string, id int64) (stop func()) {
	done := make(chan struct{})
	var wg sync.WaitGroup
	wg.Go(func() {
		t := time.NewTicker(s.heartbeat)
		defer t.Stop()
		for {
			select {
			case <-done:
				return
			case <-ctx.Done():
				return
			case <-t.C:
				if err := s.q.HeartbeatJobRun(ctx, id); err != nil && ctx.Err() == nil {
					slog.WarnContext(ctx, "job heartbeat", "job", job, "err", err)
				}
			}
		}
	})
	return func() { close(done); wg.Wait() }
}
