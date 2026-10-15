package external

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"bugun/internal/geo"
	"bugun/internal/store/db"
)

// Open-Meteo endpoints: the archive lags about five days behind, so the last
// days come from the forecast API's past days.
const (
	OpenMeteoArchive  = "https://archive-api.open-meteo.com/v1/archive"
	OpenMeteoForecast = "https://api.open-meteo.com/v1/forecast"
	archiveLag        = 6
)

// Weather keeps weather_daily filled for every viloyat centre.
type Weather struct {
	Q        *db.Queries
	Client   *http.Client
	Archive  string // OpenMeteoArchive in production
	Forecast string // OpenMeteoForecast in production
	Pause    time.Duration
}

type meteoDaily struct {
	Daily struct {
		Time   []string   `json:"time"`
		Tmin   []*float64 `json:"temperature_2m_min"`
		Tmax   []*float64 `json:"temperature_2m_max"`
		Precip []*float64 `json:"precipitation_sum"`
	} `json:"daily"`
}

func (w *Weather) get(ctx context.Context, base string, p geo.Point, extra url.Values) (*meteoDaily, error) {
	v := url.Values{
		"latitude":  {strconv.FormatFloat(p.Lat, 'f', 4, 64)},
		"longitude": {strconv.FormatFloat(p.Lon, 'f', 4, 64)},
		"daily":     {"temperature_2m_min,temperature_2m_max,precipitation_sum"},
		"timezone":  {"Asia/Tashkent"},
	}
	for k, x := range extra {
		v[k] = x
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"?"+v.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Bugun/1.0 (bozor narxlari tahlili)")
	res, err := w.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("open-meteo: status %d", res.StatusCode)
	}
	var m meteoDaily
	if err := json.NewDecoder(res.Body).Decode(&m); err != nil {
		return nil, fmt.Errorf("open-meteo: %w", err)
	}
	if len(m.Daily.Tmin) != len(m.Daily.Time) || len(m.Daily.Tmax) != len(m.Daily.Time) || len(m.Daily.Precip) != len(m.Daily.Time) {
		return nil, fmt.Errorf("open-meteo: uneven daily arrays")
	}
	return &m, nil
}

func (w *Weather) store(ctx context.Context, province int16, m *meteoDaily) (int, error) {
	n := 0
	for i, d := range m.Daily.Time {
		day, err := time.Parse(time.DateOnly, d)
		if err != nil {
			return n, err
		}
		if err := w.Q.UpsertWeather(ctx, db.UpsertWeatherParams{
			ProvinceID: province, Day: pgtype.Date{Time: day, Valid: true},
			Tmin: opt(m.Daily.Tmin[i]), Tmax: opt(m.Daily.Tmax[i]), Precip: opt(m.Daily.Precip[i]),
		}); err != nil {
			return n, err
		}
		n++
	}
	return n, nil
}

func opt(v *float64) pgtype.Float8 {
	if v == nil {
		return pgtype.Float8{}
	}
	return pgtype.Float8{Float64: *v, Valid: true}
}

// Sync brings every viloyat up to today: the archive from the day after the
// last stored one (or History days back), then the last week from the
// forecast API. A viloyat that fails is logged and retried next run.
func (w *Weather) Sync(ctx context.Context) (int, error) {
	provinces, err := w.Q.ListProvinces(ctx)
	if err != nil {
		return 0, err
	}
	centres, err := geo.Centroids()
	if err != nil {
		return 0, err
	}
	today := tashkentDay(time.Now())
	total, failed := 0, 0
	for i, p := range provinces {
		c, ok := centres[p.Soato.Int64]
		if !ok {
			continue
		}
		if i > 0 && w.Pause > 0 {
			select {
			case <-ctx.Done(): // shutting down: the rest waits for the next run
				return total, ctx.Err()
			case <-time.After(w.Pause):
			}
		}
		n, err := w.syncOne(ctx, p.ID, c, today)
		total += n
		if err != nil {
			failed++
			slog.WarnContext(ctx, "weather sync", "province", p.Name, "err", err)
		}
	}
	if failed == len(provinces) && failed > 0 {
		return total, fmt.Errorf("open-meteo unreachable for every viloyat")
	}
	return total, nil
}

func (w *Weather) syncOne(ctx context.Context, province int16, c geo.Point, today time.Time) (int, error) {
	last, err := w.Q.LastWeatherDay(ctx, province)
	if err != nil {
		return 0, err
	}
	from := today.AddDate(0, 0, -History)
	if last.Valid && last.Time.After(from) {
		from = last.Time.AddDate(0, 0, 1)
	}
	total := 0
	if end := today.AddDate(0, 0, -archiveLag); !from.After(end) {
		m, err := w.get(ctx, w.Archive, c, url.Values{"start_date": {from.Format(time.DateOnly)}, "end_date": {end.Format(time.DateOnly)}})
		if err != nil {
			return 0, err
		}
		if total, err = w.store(ctx, province, m); err != nil {
			return total, err
		}
	}
	m, err := w.get(ctx, w.Forecast, c, url.Values{"past_days": {strconv.Itoa(archiveLag + 1)}, "forecast_days": {"1"}})
	if err != nil {
		return total, err
	}
	n, err := w.store(ctx, province, m)
	return total + n, err
}
