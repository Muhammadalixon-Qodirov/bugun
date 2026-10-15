package insight

import (
	"context"
	"math"
	"sort"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"bugun/internal/store/db"
)

// JumpThreshold: a week that moved more than this against the week before
// is marked on the chart (TAHLIL_REJA.md #29).
const JumpThreshold = 0.15

// eventWindow: events up to a week around a jump are its likely causes.
const eventWindow = 7

type EventRef struct {
	ID       int64  `json:"id"`
	Title    string `json:"title"`
	Kind     string `json:"kind"`
	StartsOn string `json:"starts_on"`
	EndsOn   string `json:"ends_on,omitempty"`
	Province string `json:"province,omitempty" doc:"Empty when it touched the whole country"`
	Source   string `json:"source_url,omitempty"`
	Demo     bool   `json:"demo"`
}

type Jump struct {
	Week   string     `json:"week" doc:"Monday of the week"`
	Change float64    `json:"change" doc:"Against the week before: 0.31 = +31%"`
	Events []EventRef `json:"events" doc:"Events a week either side: likely causes, not proven ones"`
}

// weekly averages daily points into weeks starting Monday.
func weekly(ps []Point) []Point {
	sums := map[time.Time][2]float64{}
	for _, p := range ps {
		w := p.Day.AddDate(0, 0, -((int(p.Day.Weekday()) + 6) % 7))
		v := sums[w]
		sums[w] = [2]float64{v[0] + p.Median, v[1] + 1}
	}
	out := make([]Point, 0, len(sums))
	for w, v := range sums {
		out = append(out, Point{Day: w, Median: v[0] / v[1]})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Day.Before(out[j].Day) })
	return out
}

// JumpsOf finds weeks that moved more than JumpThreshold against the week
// before (consecutive weeks only).
func JumpsOf(weeks []Point) []Jump {
	out := []Jump{}
	for i := 1; i < len(weeks); i++ {
		prev, cur := weeks[i-1], weeks[i]
		if cur.Day.Sub(prev.Day) != 7*24*time.Hour || prev.Median <= 0 {
			continue
		}
		if c := cur.Median/prev.Median - 1; math.Abs(c) > JumpThreshold {
			out = append(out, Jump{Week: cur.Day.Format(time.DateOnly), Change: math.Round(c*1000) / 1000, Events: []EventRef{}})
		}
	}
	return out
}

// events loads the period's events for the product and links them to jumps.
func (s *Service) events(ctx context.Context, a *Analytics, p Params, prov pgtype.Int2, weeks []Point, from, to time.Time) error {
	prod, err := s.q.GetProduct(ctx, p.ProductID)
	if err != nil {
		return err
	}
	rows, err := s.q.EventsFor(ctx, db.EventsForParams{
		FromDay: date(from.AddDate(0, 0, -eventWindow)), ToDay: date(to), ProductID: p.ProductID, Category: prod.Category, ProvinceID: prov,
	})
	if err != nil {
		return err
	}
	type span struct {
		ref        EventRef
		start, end time.Time
	}
	var spans []span
	a.Events = []EventRef{}
	for _, r := range rows {
		ref := EventRef{ID: r.ID, Title: r.Title, Kind: r.Kind, StartsOn: r.StartsOn.Time.Format(time.DateOnly),
			Province: r.ProvinceName, Source: r.SourceUrl.String, Demo: r.IsDemo}
		end := r.StartsOn.Time
		if r.EndsOn.Valid {
			ref.EndsOn = r.EndsOn.Time.Format(time.DateOnly)
			end = r.EndsOn.Time
		}
		spans = append(spans, span{ref, r.StartsOn.Time, end})
		if !end.Before(from) {
			a.Events = append(a.Events, ref)
		}
	}
	a.Jumps = JumpsOf(weeks)
	for i := range a.Jumps {
		w, _ := time.Parse(time.DateOnly, a.Jumps[i].Week)
		lo, hi := w.AddDate(0, 0, -eventWindow), w.AddDate(0, 0, 6+eventWindow)
		for _, sp := range spans {
			if !sp.start.After(hi) && !sp.end.Before(lo) {
				a.Jumps[i].Events = append(a.Jumps[i].Events, sp.ref)
			}
		}
	}
	return nil
}
