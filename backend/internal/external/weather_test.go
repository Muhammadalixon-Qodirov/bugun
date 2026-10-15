package external

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync/atomic"
	"testing"
	"time"
)

// meteoStub answers like Open-Meteo for a date range or the last N days,
// with a null rain value on one day.
func meteoStub(t *testing.T) (*httptest.Server, *atomic.Int32) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		q := r.URL.Query()
		var from, to time.Time
		if s := q.Get("start_date"); s != "" {
			from, _ = time.Parse(time.DateOnly, s)
			to, _ = time.Parse(time.DateOnly, q.Get("end_date"))
		} else {
			n, _ := strconv.Atoi(q.Get("past_days"))
			to = tashkentDay(time.Now())
			from = to.AddDate(0, 0, -n)
		}
		var m meteoDaily
		for d := from; !d.After(to); d = d.AddDate(0, 0, 1) {
			lo, hi, rain := -2.0, 8.0, 1.5
			m.Daily.Time = append(m.Daily.Time, d.Format(time.DateOnly))
			m.Daily.Tmin = append(m.Daily.Tmin, &lo)
			m.Daily.Tmax = append(m.Daily.Tmax, &hi)
			if d.Day() == 1 {
				m.Daily.Precip = append(m.Daily.Precip, nil)
			} else {
				m.Daily.Precip = append(m.Daily.Precip, &rain)
			}
		}
		_ = json.NewEncoder(w).Encode(m)
	}))
	t.Cleanup(srv.Close)
	return srv, &calls
}

func TestWeatherSync(t *testing.T) {
	q, pool := testQueries(t)
	ctx := context.Background()
	if _, err := pool.Exec(ctx, `DELETE FROM weather_daily`); err != nil {
		t.Fatal(err)
	}
	srv, calls := meteoStub(t)
	w := &Weather{Q: q, Client: srv.Client(), Archive: srv.URL + "/archive", Forecast: srv.URL + "/forecast"}
	if _, err := w.Sync(ctx); err != nil {
		t.Fatal(err)
	}
	// 14 viloyatlar × (3 years + today), each day once.
	var rows, days, nulls int
	if err := pool.QueryRow(ctx, `SELECT count(*), count(DISTINCT day), count(*) FILTER (WHERE precip IS NULL) FROM weather_daily`).Scan(&rows, &days, &nulls); err != nil {
		t.Fatal(err)
	}
	if days != History+1 || rows != 14*(History+1) || nulls == 0 || calls.Load() != 28 {
		t.Errorf("rows %d, days %d, null rain %d, calls %d", rows, days, nulls, calls.Load())
	}
	// Next run: only the last week again, no archive call.
	before := calls.Load()
	if _, err := w.Sync(ctx); err != nil {
		t.Fatal(err)
	}
	if got := calls.Load() - before; got != 14 {
		t.Errorf("second run made %d calls, want 14 (recent days only)", got)
	}
}
