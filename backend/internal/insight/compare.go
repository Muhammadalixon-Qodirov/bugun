package insight

import (
	"context"
	"math"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"bugun/internal/store/db"
)

// Compare (TAHLIL_REJA.md #40): up to 4 lines on one chart, several products
// in one place or one product in several viloyatlar.

// MaxLines caps a comparison.
const MaxLines = 4

type Line struct {
	Product      ProductRef  `json:"product"`
	ProvinceID   int16       `json:"province_id,omitempty"`
	ProvinceName string      `json:"province_name,omitempty"`
	Now          *int64      `json:"now,omitempty" doc:"Last point"`
	Change       *float64    `json:"change,omitempty" doc:"Last point against the first"`
	Points       []LinePoint `json:"points"`
	DemoShare    float64     `json:"demo_share" doc:"Share of the line's prices that are demo data (demo sellers or generated synthetic history), 0–1; 0 = all live. Show a Demo mark above 0"`
}

type LinePoint struct {
	Date   string  `json:"date"`
	Median int64   `json:"median"`
	Index  float64 `json:"index" doc:"First point of the line = 100, to compare products of different price"`
}

type Comparison struct {
	Trade  string `json:"trade"`
	Period string `json:"period"`
	Bucket string `json:"bucket"`
	Lines  []Line `json:"lines"`
	// DemoShare over every line.
	DemoShare float64 `json:"demo_share" doc:"Share of all the lines' prices that are demo data (demo sellers or generated synthetic history), 0–1; 0 = all live"`
}

// LineSpec is one line to draw.
type LineSpec struct {
	ProductID int16
	Province  int16 // 0 = country
}

// Compare draws each line over the period.
func (s *Service) Compare(ctx context.Context, specs []LineSpec, trade, period string) (*Comparison, error) {
	per, ok := periods[period]
	if !ok {
		return nil, errUnknownPeriod(period)
	}
	products, err := s.products(ctx)
	if err != nil {
		return nil, err
	}
	today := Day(time.Now()).Time
	from := today.AddDate(0, 0, -per.days+1)
	c := &Comparison{Trade: trade, Period: period, Bucket: per.bucket, Lines: []Line{}}
	bucket := per.bucket
	if bucket == "day" && per.days > 90 {
		bucket = "week" // a year of days per line is too dense to compare
		c.Bucket = bucket
	}
	var demoSum, offerSum float64
	for _, sp := range specs {
		p := products[sp.ProductID]
		line := Line{Product: ProductRef{ID: p.id, Slug: p.slug, Name: p.name, Category: p.cat, Unit: p.unt}, Points: []LinePoint{}}
		if sp.Province != 0 {
			r, err := s.q.GetRegion(ctx, sp.Province)
			if err != nil {
				return nil, err
			}
			line.ProvinceID, line.ProvinceName = r.ID, r.Name
		}
		rows, err := s.q.PriceSeries(ctx, db.PriceSeriesParams{
			Bucket: bucket, ProductID: sp.ProductID, TradeType: trade, FromDay: date(from), ToDay: date(today),
			ProvinceID: pgtype.Int2{Int16: sp.Province, Valid: sp.Province != 0},
		})
		if err != nil {
			return nil, err
		}
		var first, lineDemo, lineOffers float64
		for _, r := range rows {
			if r.Day.Time.Before(from) || r.Median <= 0 {
				continue
			}
			lineDemo += r.DemoShare * float64(r.Offers)
			lineOffers += float64(r.Offers)
			if first == 0 {
				first = r.Median
			}
			line.Points = append(line.Points, LinePoint{Date: r.Day.Time.Format(time.DateOnly), Median: rnd(r.Median), Index: math.Round(r.Median/first*1000) / 10})
		}
		if n := len(line.Points); n > 0 {
			last := line.Points[n-1]
			line.Now = &last.Median
			line.Change = diff(float64(last.Median), first)
		}
		if lineOffers > 0 {
			line.DemoShare = math.Round(lineDemo/lineOffers*100) / 100
		}
		demoSum += lineDemo
		offerSum += lineOffers
		c.Lines = append(c.Lines, line)
	}
	if offerSum > 0 {
		c.DemoShare = math.Round(demoSum/offerSum*100) / 100
	}
	return c, nil
}
