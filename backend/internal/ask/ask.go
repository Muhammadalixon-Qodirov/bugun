// Package ask answers price questions such as "Samarqandda piyoz qancha?"
// with rules, not a language model: the product comes from the catalog, the
// place from region names, the intent from a few keywords, and every number
// from the same queries as the site (spec §8.5, §7.4).
package ask

import (
	"context"
	"errors"
	"log/slog"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"bugun/internal/analytics"
	"bugun/internal/pricing"
	"bugun/internal/secure"
	"bugun/internal/store/db"
	"bugun/internal/tz"
)

// Intent is what the question asks for.
type Intent string

const (
	IntentPrice    Intent = "price"    // "piyoz qancha?"
	IntentCheapest Intent = "cheapest" // "eng arzon piyoz kimda?"
	IntentAreas    Intent = "areas"    // "qaysi tumanda piyoz arzon?"
)

var ErrNoProduct = errors.New("no product in the question")

// Region is a place the question names.
type Region struct {
	ID       int16
	Name     string
	Kind     string // viloyat, respublika, shahar, tuman
	Province int16  // its viloyat (itself for a viloyat)
}

// Stats is a market price with the context every number needs (spec §7.4).
type Stats struct {
	Enough           bool
	Offers           int32
	Median, P25, P75 float64
	Min, Max         int64
	From, To         time.Time
	Scope            string // where the numbers come from; "" = whole country
}

type Seller struct {
	Name, Region, Market string
	PriceSom             int64
}

type Area struct {
	Name   string
	Offers int32
	Median float64
}

type Answer struct {
	Intent   Intent
	Product  db.GetProductRow
	Trade    string
	Region   *Region // as asked; nil = none named
	Stats    Stats
	Cheapest []Seller // IntentCheapest
	Areas    []Area   // IntentAreas, cheapest first
}

type Service struct {
	q         *db.Queries
	prices    *pricing.Service
	regions   []alias
	demandKey func(string) []byte
}

// SetDemandKey sets how demand signal keys are hashed, the same keyed hash
// the site uses, so one person's signals on a day share one key.
func (s *Service) SetDemandKey(hash func(string) []byte) { s.demandKey = hash }

// alias is how a region is written in a question: its name without the kind
// word ("namangan" for Namangan viloyati), matched at a word start so that
// case endings still match ("namanganda", "chortoqdagi").
type alias struct {
	re     *regexp.Regexp
	length int
	region Region
}

func NewService(ctx context.Context, q *db.Queries, prices *pricing.Service) (*Service, error) {
	rows, err := q.ListRegions(ctx)
	if err != nil {
		return nil, err
	}
	s := &Service{q: q, prices: prices, demandKey: secure.HashToken}
	for _, r := range rows {
		name := pricing.Normalize(r.Name)
		for _, kind := range []string{" viloyati", " shahri", " tumani", " respublikasi"} {
			name = strings.TrimSuffix(name, kind)
		}
		name = strings.TrimSpace(name)
		if len([]rune(name)) < 3 {
			continue
		}
		prov := r.ID
		if r.ParentID.Valid {
			prov = r.ParentID.Int16
		}
		s.regions = append(s.regions, alias{
			re:     regexp.MustCompile(`(^|[^\p{L}])` + regexp.QuoteMeta(name)),
			length: len(name),
			region: Region{ID: r.ID, Name: r.Name, Kind: r.Kind, Province: prov},
		})
	}
	return s, nil
}

// findRegion picks the place a question names: the longest matching name;
// among equal names the kind the question mentions ("tumanda", "shahrida"),
// else the viloyat — except plain "Toshkent", which usually means the city.
func (s *Service) findRegion(text string) *Region {
	var cands []alias
	for _, a := range s.regions {
		if a.re.MatchString(text) {
			cands = append(cands, a)
		}
	}
	if len(cands) == 0 {
		return nil
	}
	sort.SliceStable(cands, func(i, j int) bool { return cands[i].length > cands[j].length })
	cands = cands[:countLongest(cands)]
	rank := func(r Region) int {
		switch {
		case strings.Contains(text, "tuman") && r.Kind == "tuman",
			strings.Contains(text, "shahr") && r.Kind == "shahar",
			strings.Contains(text, "viloyat") && r.Kind == "viloyat":
			return 0
		case strings.HasPrefix(pricing.Normalize(r.Name), "toshkent") && r.Kind == "shahar":
			return 1
		case r.Kind == "viloyat" || r.Kind == "respublika":
			return 2
		case r.Kind == "shahar":
			return 3
		}
		return 4
	}
	sort.SliceStable(cands, func(i, j int) bool { return rank(cands[i].region) < rank(cands[j].region) })
	r := cands[0].region
	return &r
}

func countLongest(c []alias) int {
	n := 1
	for n < len(c) && c[n].length == c[0].length {
		n++
	}
	return n
}

func intentOf(text string) Intent {
	switch {
	case strings.Contains(text, "qaysi") && (strings.Contains(text, "tuman") || strings.Contains(text, "viloyat") ||
		strings.Contains(text, "hudud") || strings.Contains(text, "joy") || strings.Contains(text, "shahar")):
		return IntentAreas
	case strings.Contains(text, "arzon") || strings.Contains(text, "kimda") || strings.Contains(text, "sotuvchi") ||
		strings.Contains(text, "qayerdan") || strings.Contains(text, "qayerda"):
		return IntentCheapest
	}
	return IntentPrice
}

// Ask answers one question. u may be nil (not signed in); the question is
// logged either way as demand (spec §10.3).
func (s *Service) Ask(ctx context.Context, u *db.User, question string) (Answer, error) {
	text := pricing.Normalize(question)
	d, problem := s.prices.ParseDemand(question)
	if problem != nil {
		s.log(ctx, u, question, 0, nil)
		return Answer{}, ErrNoProduct
	}
	p, err := s.q.GetProduct(ctx, d.ProductID)
	if err != nil {
		return Answer{}, err
	}
	a := Answer{Intent: intentOf(text), Product: p, Trade: "ulgurji", Region: s.findRegion(text)}
	if strings.Contains(text, "chakana") {
		a.Trade = "chakana"
	}
	if a.Stats, err = s.stats(ctx, p.ID, a.Trade, a.Region); err != nil {
		return Answer{}, err
	}
	switch a.Intent {
	case IntentCheapest:
		if a.Cheapest, err = s.cheapest(ctx, p.ID, a.Trade, a.Region); err != nil {
			return Answer{}, err
		}
	case IntentAreas:
		if a.Areas, err = s.areas(ctx, p.ID, a.Trade, a.Region); err != nil {
			return Answer{}, err
		}
	}
	s.log(ctx, u, question, p.ID, a.Region)
	return a, nil
}

// stats widens the area until there are enough prices: the named place, its
// viloyat, then the whole country — the same steps as the product page.
func (s *Service) stats(ctx context.Context, product int16, trade string, r *Region) (Stats, error) {
	type scope struct {
		id   int16
		name string
	}
	var scopes []scope
	if r != nil {
		scopes = append(scopes, scope{r.ID, r.Name})
		if r.Province != r.ID {
			if pv, err := s.q.GetRegion(ctx, r.Province); err == nil {
				scopes = append(scopes, scope{pv.ID, pv.Name})
			}
		}
	}
	scopes = append(scopes, scope{0, ""})
	var st Stats
	for _, sc := range scopes {
		row, err := s.q.ProductStats(ctx, db.ProductStatsParams{
			ProductID: product, TradeType: trade, RegionID: pgtype.Int2{Int16: sc.id, Valid: sc.id != 0},
		})
		if err != nil {
			return Stats{}, err
		}
		st = Stats{Offers: row.Offers, Scope: sc.name}
		if row.Offers < analytics.MinObservations {
			continue
		}
		return Stats{Enough: true, Offers: row.Offers, Median: row.Median, P25: row.P25, P75: row.P75,
			Min: row.MinPrice, Max: row.MaxPrice, From: row.FirstSeen, To: row.LastSeen, Scope: sc.name}, nil
	}
	return st, nil
}

func (s *Service) cheapest(ctx context.Context, product int16, trade string, r *Region) ([]Seller, error) {
	region := pgtype.Int2{}
	if r != nil {
		region = pgtype.Int2{Int16: r.ID, Valid: true}
	}
	rows, err := s.q.ProductListings(ctx, db.ProductListingsParams{
		ProductID: product, TradeType: pgtype.Text{String: trade, Valid: true}, RegionID: region,
	})
	if err != nil {
		return nil, err
	}
	out := []Seller{}
	for _, l := range rows {
		if len(out) == 3 {
			break
		}
		out = append(out, Seller{Name: l.SellerName, Region: l.RegionName, Market: l.MarketName, PriceSom: l.PriceSom})
	}
	return out, nil
}

// areas compares tumans of the named viloyat (or of the named tuman's
// viloyat), or all viloyats when no place is named.
func (s *Service) areas(ctx context.Context, product int16, trade string, r *Region) ([]Area, error) {
	out := []Area{}
	if r == nil {
		rows, err := s.q.ProductProvinceMedians(ctx, db.ProductProvinceMediansParams{ProductID: product, TradeType: trade})
		if err != nil {
			return nil, err
		}
		for _, row := range rows {
			if row.Offers < analytics.MinObservations {
				continue
			}
			reg, err := s.q.GetRegion(ctx, row.ProvinceID)
			if err != nil {
				return nil, err
			}
			out = append(out, Area{Name: reg.Name, Offers: row.Offers, Median: row.Median})
		}
		return out, nil
	}
	rows, err := s.q.ProductDistrictMedians(ctx, db.ProductDistrictMediansParams{ProductID: product, TradeType: trade, ProvinceID: pgtype.Int2{Int16: r.Province, Valid: true}})
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		out = append(out, Area{Name: row.Name, Offers: row.Offers, Median: row.Median})
	}
	return out, nil
}

// log keeps the question (question_log) and counts it as demand for the
// product once per user and day. Failures are logged, never returned.
func (s *Service) log(ctx context.Context, u *db.User, question string, product int16, r *Region) {
	var uid pgtype.UUID
	if u != nil {
		uid = pgtype.UUID{Bytes: u.ID, Valid: true}
	}
	var region pgtype.Int2
	if r != nil {
		region = pgtype.Int2{Int16: r.ID, Valid: true}
	}
	if err := s.q.LogQuestion(ctx, db.LogQuestionParams{
		UserID: uid, Question: question, ProductID: pgtype.Int2{Int16: product, Valid: product != 0}, RegionID: region,
	}); err != nil {
		slog.WarnContext(ctx, "log question", "err", err)
	}
	if product == 0 || u == nil {
		return
	}
	var province pgtype.Int2
	if r != nil {
		province = pgtype.Int2{Int16: r.Province, Valid: true}
	}
	day := time.Now().In(tz.Tashkent).Format(time.DateOnly)
	if err := s.q.RecordDemand(ctx, db.RecordDemandParams{
		Kind: "question", ProductID: product, ProvinceID: province,
		ViewerKey: s.demandKey(day + "|u:" + u.ID.String()),
	}); err != nil {
		slog.WarnContext(ctx, "question demand", "err", err)
	}
}
