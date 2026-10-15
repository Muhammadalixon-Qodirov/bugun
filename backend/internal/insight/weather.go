package insight

import (
	"context"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"bugun/internal/store/db"
)

// Weather beside prices (TAHLIL_REJA.md 3.2).

const (
	maxExtremes     = 20 // listed for the period
	maxJumpWeathers = 3  // linked to one jump
)

type WeatherPoint struct {
	Date   string  `json:"date"`
	Tmin   float64 `json:"tmin" doc:"Lowest minimum in the bucket, °C"`
	Tmax   float64 `json:"tmax"`
	Precip float64 `json:"precip" doc:"Rain and snow in the bucket, mm"`
}

type Extreme struct {
	Date     string  `json:"date"`
	Kind     string  `json:"kind" enum:"frost,heat"`
	Temp     float64 `json:"temp"`
	Province string  `json:"province"`
}

type Weather struct {
	Scope    string         `json:"scope" doc:"The viloyat centre, or the mean of all viloyatlar"`
	Points   []WeatherPoint `json:"points"`
	Extremes []Extreme      `json:"extremes" doc:"Up to 20 harshest frost (≤ -5°C, or ≤ 0°C in March–May) and heat (≥ 40°C) days"`
}

func (s *Service) weather(ctx context.Context, a *Analytics, prov pgtype.Int2, bucket string, from, to time.Time) error {
	rows, err := s.q.WeatherSeries(ctx, db.WeatherSeriesParams{Bucket: bucket, FromDay: date(from), ToDay: date(to), ProvinceID: prov})
	if err != nil || len(rows) == 0 {
		return err
	}
	w := &Weather{Scope: "O'zbekiston (o'rtacha)", Points: make([]WeatherPoint, len(rows)), Extremes: []Extreme{}}
	if a.Scope.ProvinceName != "" {
		w.Scope = a.Scope.ProvinceName
	}
	for i, r := range rows {
		w.Points[i] = WeatherPoint{Date: r.Day.Time.Format(time.DateOnly), Tmin: round1(r.Tmin), Tmax: round1(r.Tmax), Precip: round1(r.Precip)}
	}
	ex, err := s.q.WeatherExtremes(ctx, db.WeatherExtremesParams{FromDay: date(from.AddDate(0, 0, -eventWindow)), ToDay: date(to), ProvinceID: prov})
	if err != nil {
		return err
	}
	all := make([]Extreme, len(ex))
	for i, e := range ex {
		all[i] = Extreme{Date: e.Day.Time.Format(time.DateOnly), Kind: e.Kind, Temp: round1(e.Temp), Province: e.ProvinceName}
	}
	// Link the harshest extremes around each jump as likely causes.
	for i := range a.Jumps {
		wk, _ := time.Parse(time.DateOnly, a.Jumps[i].Week)
		lo, hi := wk.AddDate(0, 0, -eventWindow), wk.AddDate(0, 0, 6+eventWindow)
		var near []Extreme
		for _, e := range all {
			d, _ := time.Parse(time.DateOnly, e.Date)
			if !d.Before(lo) && !d.After(hi) {
				near = append(near, e)
			}
		}
		sort.Slice(near, func(x, y int) bool { return severity(near[x]) > severity(near[y]) })
		for j, e := range near {
			if j == maxJumpWeathers {
				break
			}
			a.Jumps[i].Events = append(a.Jumps[i].Events, EventRef{Title: extremeTitle(e), Kind: e.Kind, StartsOn: e.Date, Province: e.Province})
		}
	}
	var shown []Extreme
	for _, e := range all {
		if d, _ := time.Parse(time.DateOnly, e.Date); !d.Before(from) {
			shown = append(shown, e)
		}
	}
	sort.Slice(shown, func(x, y int) bool { return severity(shown[x]) > severity(shown[y]) })
	if len(shown) > maxExtremes {
		shown = shown[:maxExtremes]
	}
	sort.Slice(shown, func(x, y int) bool { return shown[x].Date < shown[y].Date })
	w.Extremes = append(w.Extremes, shown...)
	a.Weather = w
	return nil
}

// severity: how far past the frost or heat line a day went.
func severity(e Extreme) float64 {
	if e.Kind == "heat" {
		return e.Temp - 40
	}
	return -e.Temp
}

func extremeTitle(e Extreme) string {
	if e.Kind == "heat" {
		return fmt.Sprintf("Jazirama: %+.0f°C (%s)", e.Temp, e.Province)
	}
	return fmt.Sprintf("Sovuq: %+.0f°C (%s)", e.Temp, e.Province)
}

func round1(x float64) float64 { return math.Round(x*10) / 10 }
