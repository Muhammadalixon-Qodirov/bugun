// Package external brings in free public data that explains prices
// (TAHLIL_REJA.md §9): exchange rates from the Central Bank and weather from
// Open-Meteo. Both need no key and no agreement.
package external

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"bugun/internal/store/db"
	"bugun/internal/tz"
)

// Currencies kept from the Central Bank's list.
var Currencies = []string{"USD", "RUB", "KZT"}

// CBUBase is the Central Bank's rates archive.
const CBUBase = "https://cbu.uz/uz/arkhiv-kursov-valyut/json/all/"

// History is how far back missing days are filled.
const History = 3 * 365

// FX keeps fx_daily filled.
type FX struct {
	Q      *db.Queries
	Client *http.Client
	Base   string        // CBUBase in production
	Pause  time.Duration // between requests, to be polite
}

type cbuRate struct {
	Ccy     string `json:"Ccy"`
	Rate    string `json:"Rate"`
	Nominal string `json:"Nominal"`
}

// Fetch returns the so'm rate of each kept currency on a day.
func (f *FX) Fetch(ctx context.Context, day time.Time) (map[string]float64, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, f.Base+day.Format(time.DateOnly)+"/", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Bugun/1.0 (bozor narxlari tahlili)")
	res, err := f.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("cbu %s: status %d", day.Format(time.DateOnly), res.StatusCode)
	}
	var rows []cbuRate
	if err := json.NewDecoder(res.Body).Decode(&rows); err != nil {
		return nil, fmt.Errorf("cbu %s: %w", day.Format(time.DateOnly), err)
	}
	out := map[string]float64{}
	for _, r := range rows {
		for _, c := range Currencies {
			if r.Ccy != c {
				continue
			}
			rate, err1 := strconv.ParseFloat(strings.TrimSpace(r.Rate), 64)
			nominal, err2 := strconv.ParseFloat(strings.TrimSpace(r.Nominal), 64)
			if err1 == nil && err2 == nil && rate > 0 && nominal > 0 {
				out[c] = rate / nominal
			}
		}
	}
	if _, ok := out["USD"]; !ok {
		return nil, fmt.Errorf("cbu %s: no dollar rate", day.Format(time.DateOnly))
	}
	return out, nil
}

// maxFailsInRow stops a run that only waits for timeouts.
const maxFailsInRow = 5

// Sync fills every missing day of the last History days, newest first, and
// returns how many days it stored. A day that fails is logged and tried
// again on the next run; after maxFailsInRow failures in a row the Central
// Bank is throttling or down, so the run stops and the next one goes on.
func (f *FX) Sync(ctx context.Context) (int, error) {
	today := tashkentDay(time.Now())
	days, err := f.Q.MissingFxDays(ctx, db.MissingFxDaysParams{
		FromDay: pgtype.Date{Time: today.AddDate(0, 0, -History), Valid: true}, ToDay: pgtype.Date{Time: today, Valid: true},
	})
	if err != nil {
		return 0, err
	}
	stored, inRow := 0, 0
	for i, d := range days {
		if i > 0 && f.Pause > 0 {
			select {
			case <-ctx.Done():
				return stored, ctx.Err()
			case <-time.After(f.Pause):
			}
		}
		rates, err := f.Fetch(ctx, d.Time)
		if err != nil {
			inRow++
			slog.WarnContext(ctx, "fx fetch", "err", err)
			if inRow >= maxFailsInRow {
				return stored, fmt.Errorf("cbu: %d failures in a row, %d days stored, stopping: %w", inRow, stored, err)
			}
			continue
		}
		inRow = 0
		for ccy, rate := range rates {
			if err := f.Q.UpsertFx(ctx, db.UpsertFxParams{Day: d, Ccy: ccy, Rate: rate}); err != nil {
				return stored, err
			}
		}
		stored++
	}
	return stored, nil
}

// tashkentDay is the Tashkent calendar day as a UTC midnight.
func tashkentDay(t time.Time) time.Time {
	t = t.In(tz.Tashkent)
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}
