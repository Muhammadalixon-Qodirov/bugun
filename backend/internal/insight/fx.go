package insight

import (
	"context"
	"math"
	"time"

	"bugun/internal/store/db"
)

// Exchange rates beside prices (TAHLIL_REJA.md 3.1).

type FxRate struct {
	Ccy    string    `json:"ccy" enum:"USD,RUB,KZT"`
	Now    float64   `json:"now" doc:"So'm for one unit, latest"`
	Change *float64  `json:"change,omitempty" doc:"Over the period"`
	Points []FxPoint `json:"points,omitempty" doc:"Only for USD, bucketed like the price series"`
}

type FxPoint struct {
	Date string  `json:"date"`
	Rate float64 `json:"rate"`
}

type Fx struct {
	Rates []FxRate `json:"rates"`
	// Correlation of the product's price with the dollar over the period.
	Correlation *float64 `json:"correlation_usd,omitempty" doc:"Pearson r of price and dollar rate, -1..1; omitted under 20 points. Moving together is not proof of cause"`
}

func (s *Service) fx(ctx context.Context, a *Analytics, bucket string, from, to time.Time, prices map[string]float64) error {
	f := &Fx{Rates: []FxRate{}}
	for _, ccy := range []string{"USD", "RUB", "KZT"} {
		rows, err := s.q.FxSeries(ctx, db.FxSeriesParams{Bucket: bucket, Ccy: ccy, FromDay: date(from), ToDay: date(to)})
		if err != nil {
			return err
		}
		if len(rows) == 0 {
			continue
		}
		r := FxRate{Ccy: ccy, Now: math.Round(rows[len(rows)-1].Rate*100) / 100, Change: diff(rows[len(rows)-1].Rate, rows[0].Rate)}
		if ccy == "USD" {
			var xs, ys []float64
			for _, row := range rows {
				d := row.Day.Time.Format(time.DateOnly)
				r.Points = append(r.Points, FxPoint{Date: d, Rate: math.Round(row.Rate*100) / 100})
				if p, ok := prices[d]; ok {
					xs, ys = append(xs, p), append(ys, row.Rate)
				}
			}
			if c, ok := Correlation(xs, ys); ok {
				f.Correlation = &c
			}
		}
		f.Rates = append(f.Rates, r)
	}
	if len(f.Rates) > 0 {
		a.Fx = f
	}
	return nil
}
