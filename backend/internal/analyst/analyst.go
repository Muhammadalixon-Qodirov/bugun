// Package analyst answers price questions (spec AI-5, /savol and POST
// /v1/ask) with a text, the table behind it and its sources. With a chat
// model configured the model picks from fixed functions (tool calling) and
// words the answer; every number comes from those functions, and an answer
// with a number no function returned is replaced by a templated one. Without
// a model, or when it fails or is rate limited, the rule-based answers of
// internal/ask give the same response shape.
package analyst

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"bugun/internal/ask"
	"bugun/internal/insight"
	"bugun/internal/llm"
	"bugun/internal/numcheck"
	"bugun/internal/pricing"
	"bugun/internal/store/db"
)

// Answer statuses.
const (
	StatusOK           = "ok"
	StatusInsufficient = "insufficient_data"
	StatusNoProduct    = "no_product"
)

// Answer is the response of the analyst in every mode.
type Answer struct {
	Text     string   `json:"text" doc:"Plain text in the asked language; every number in it comes from the sources"`
	Status   string   `json:"status" enum:"ok,insufficient_data,no_product" doc:"insufficient_data: fewer than 3 prices (or no history), nothing is guessed"`
	Mode     string   `json:"mode" enum:"model,rules" doc:"model: a language model chose the functions and wrote the text; rules: rule-based routing and templated text"`
	Fallback string   `json:"fallback,omitempty" enum:"limit,rate,error,no_tools,unchecked_number" doc:"Why the model's own text was not used, when a model is configured"`
	Lang     string   `json:"lang" enum:"uz,ru,en"`
	Table    *Table   `json:"table,omitempty" doc:"The rows behind the answer (the last function's)"`
	Sources  []Source `json:"sources" doc:"Every function called, with its parameters, dates and demo share"`
	// The chat assistant's fields (POST /v1/chat).
	Kind              string   `json:"kind,omitempty" enum:"answer,clarify,unanswered" doc:"answer; clarify: a question back, with options as buttons; unanswered: it cannot answer and the admins were told"`
	Options           []string `json:"options,omitempty" doc:"clarify: short answers to offer as buttons; send the chosen one as the next message"`
	Actions           []Action `json:"actions,omitempty" doc:"Proposed changes to confirm with a button (create_request, watch_price)"`
	unanswered, topic string

	// Rules is the rule-based answer behind a rules-mode price question, for
	// the bot's own rendering.
	Rules *ask.Answer `json:"-"`
}

// Matcher resolves product words (catalogmatch.Matcher).
type Matcher = pricing.ProductMatcher

type Service struct {
	q       *db.Queries
	ask     *ask.Service
	prices  *pricing.Service
	insight *insight.Service
	matcher Matcher

	model     llm.Chatter // nil: rules only
	maxRounds int
	limit     *limiter
}

// Option changes a Service's defaults.
type Option func(*Service)

// WithModel turns on the model path: at most rounds function-calling rounds
// per question, perMinute model answers per user or IP. c is a Client or a
// Chain of fallback models.
func WithModel(c llm.Chatter, rounds, perMinute int) Option {
	return func(s *Service) {
		if c == nil {
			return
		}
		if ch, ok := c.(llm.Chain); ok && len(ch) == 0 {
			return
		}
		s.model = c
		if rounds > 0 {
			s.maxRounds = rounds
		}
		if perMinute > 0 {
			s.limit = newLimiter(perMinute, time.Minute)
		}
	}
}

// WithMatcher resolves product words the dictionary does not know.
func WithMatcher(m Matcher) Option { return func(s *Service) { s.matcher = m } }

func New(q *db.Queries, asker *ask.Service, prices *pricing.Service, ins *insight.Service, opts ...Option) *Service {
	s := &Service{q: q, ask: asker, prices: prices, insight: ins, maxRounds: 4, limit: newLimiter(6, time.Minute)}
	for _, o := range opts {
		o(s)
	}
	return s
}

// ModelEnabled reports whether a model answers.
func (s *Service) ModelEnabled() bool { return s.model != nil }

// Request is one question. Key identifies the asker for the rate limit
// (user id, else IP).
type Request struct {
	Question string
	Lang     string
	User     *db.User
	Key      string
	// History are the conversation's earlier messages: numbers from them may
	// be repeated in the answer.
	History []string
}

// MaxQuestionLen bounds a question.
const MaxQuestionLen = 500

// ErrEmpty: the question is empty or too long.
var ErrEmpty = errors.New("empty or too long question")

// Ask answers a question.
func (s *Service) Ask(ctx context.Context, req Request) (Answer, error) {
	req.Question = strings.TrimSpace(req.Question)
	req.Lang = langOf(req.Lang)
	if req.Question == "" || len([]rune(req.Question)) > MaxQuestionLen {
		return Answer{}, ErrEmpty
	}
	if s.model == nil {
		return s.rules(ctx, req, "")
	}
	if !s.limit.allow(req.Key, time.Now()) {
		return s.rules(ctx, req, "rate")
	}
	if req.User != nil {
		ctx = llm.WithUser(ctx, req.User.ID)
	}
	a, err := s.viaModel(llm.WithFeature(ctx, "analyst"), req)
	switch {
	case errors.Is(err, llm.ErrLimit):
		return s.rules(ctx, req, "limit")
	case errors.Is(err, errNoTools):
		return s.rules(ctx, req, "no_tools")
	case err != nil:
		slog.WarnContext(ctx, "analyst model", "err", err)
		return s.rules(ctx, req, "error")
	}
	return a, nil
}

const systemPrompt = `Sen "Bugun" platformasining bozor narxlari tahlilchisisan (Oʻzbekiston).
Qoidalar:
- Maʼlumotni faqat berilgan funksiyalardan ol. Har bir savolga kamida bitta funksiya chaqir.
- Javobdagi har bir raqam (narx, foiz, son, sana) funksiya natijasida aynan boʻlishi shart. Oʻzingdan raqam qoʻshma, hisob-kitob qilma (oʻrtacha, farq yoki foizni oʻzing hisoblama).
- Funksiya "enough": false qaytarsa yoki narxlar 3 tadan kam boʻlsa, maʼlumot yetarli emasligini ayt va taxmin qilma.
- Narx soʻmda, birlik funksiya bergan birlikda. Demo maʼlumot ulushi 0 dan katta boʻlsa, buni eslat.
- Savol narx yoki bozorga oid boʻlmasa, faqat narx savollariga javob berishingni qisqa ayt.
- Javob qisqa (2–5 gap), oddiy matn, markdownsiz, %s tilida.
Bugun: %s.`

var langNames = map[string]string{"uz": "oʻzbek", "ru": "rus", "en": "ingliz"}

// maxCallsPerRound bounds parallel function calls in one round.
const maxCallsPerRound = 4

var errNoTools = errors.New("the model answered with numbers but called no function")

func (s *Service) viaModel(ctx context.Context, req Request) (Answer, error) {
	msgs := []llm.Message{
		{Role: "system", Content: fmt.Sprintf(systemPrompt, langNames[req.Lang], insight.Day(time.Now()).Time.Format(time.DateOnly))},
		{Role: "user", Content: req.Question},
	}
	tools := toolDefs()
	var results []result
	final, done := "", false
	for round := 0; round < s.maxRounds && !done; round++ {
		msg, err := s.model.Chat(ctx, llm.ChatRequest{Messages: msgs, Tools: tools, ToolChoice: "auto", Temperature: 0, MaxTokens: 700})
		if err != nil {
			return Answer{}, err
		}
		if len(msg.ToolCalls) == 0 {
			final, done = strings.TrimSpace(msg.Text()), true
			break
		}
		calls := msg.ToolCalls
		if len(calls) > maxCallsPerRound {
			calls = calls[:maxCallsPerRound]
		}
		msgs = append(msgs, llm.Message{Role: "assistant", Content: msg.Content, ToolCalls: calls})
		for _, tc := range calls {
			res, err := s.run(ctx, tc.Function.Name, tc.Function.Arguments)
			if err != nil {
				return Answer{}, err
			}
			results = append(results, res)
			b, _ := json.Marshal(res.data)
			msgs = append(msgs, llm.Message{Role: "tool", ToolCallID: tc.ID, Content: string(b)})
		}
	}
	if !done {
		// Out of rounds: ask for the answer from what was gathered.
		msg, err := s.model.Chat(ctx, llm.ChatRequest{Messages: msgs, Temperature: 0, MaxTokens: 700})
		if err != nil {
			return Answer{}, err
		}
		final = strings.TrimSpace(msg.Text())
	}
	return s.check(ctx, req, final, results)
}

// check applies the rules to the model's answer: no data means no guess, and
// a number no function returned replaces the text with the templated one.
func (s *Service) check(ctx context.Context, req Request, final string, results []result) (Answer, error) {
	ok := make([]result, 0, len(results))
	for _, r := range results {
		if r.source.Error == "" {
			ok = append(ok, r)
		}
	}
	a := Answer{Mode: "model", Lang: req.Lang, Status: StatusOK, Sources: []Source{}}
	for _, r := range results {
		a.Sources = append(a.Sources, r.source)
	}
	allow := numcheck.New(3) // "fewer than 3 prices" is the rule itself
	allow.AddText(req.Question)
	for _, h := range req.History {
		allow.AddText(h)
	}
	for _, r := range ok {
		allow.AddJSON(r.data)
	}
	if len(ok) == 0 {
		// No data behind it: fine only when the text has no number at all
		// (a refusal, or "which product do you mean?").
		if final == "" || len(numcheck.NumbersIn(final)) > 0 {
			return Answer{}, errNoTools
		}
		a.Text = final
		if len(results) > 0 {
			a.Status = StatusNoProduct
		}
		s.ask.Log(ctx, req.User, req.Question, 0, nil)
		return a, nil
	}
	s.ask.Log(ctx, req.User, req.Question, ok[0].productID, ok[0].region)
	a.Table = lastTable(ok)
	enough := false
	for _, r := range ok {
		enough = enough || r.source.Enough
	}
	switch bad := allow.Unchecked(final); {
	case !enough:
		a.Status, a.Text = StatusInsufficient, compose(ok, req.Lang)
	case final == "" || len(bad) > 0:
		slog.WarnContext(ctx, "analyst answer replaced", "numbers_not_from_tools", len(bad))
		a.Fallback, a.Text = "unchecked_number", compose(ok, req.Lang)
	default:
		a.Text = final
	}
	return a, nil
}

func compose(results []result, lang string) string {
	parts := make([]string, 0, len(results))
	seen := map[string]bool{}
	for _, r := range results {
		if t := r.summary(lang); !seen[t] {
			seen[t] = true
			parts = append(parts, t)
		}
	}
	return strings.Join(parts, "\n\n")
}

func lastTable(results []result) *Table {
	for i := len(results) - 1; i >= 0; i-- {
		if t := results[i].table; t != nil && len(t.Rows) > 0 {
			return t
		}
	}
	return nil
}

// rules answers without a model: rule-based routing to the same functions
// and templated text. Price, cheapest and area questions the /savol rules
// read the same way go through ask.Ask, so the bot can render them as before.
func (s *Service) rules(ctx context.Context, req Request, why string) (Answer, error) {
	a := Answer{Mode: "rules", Fallback: why, Lang: req.Lang, Status: StatusOK, Sources: []Source{}}
	tool := Route(req.Question)
	question := req.Question
	var product db.GetProductRow
	if tool != ToolTopDemand {
		p, rewritten, err := s.productIn(ctx, question)
		if errors.Is(err, errUnknownProduct) {
			s.ask.Log(ctx, req.User, req.Question, 0, nil)
			a.Status, a.Text = StatusNoProduct, tr(req.Lang, "no_product")
			return a, nil
		}
		if err != nil {
			return Answer{}, err
		}
		product, question = p, rewritten
	}
	if askTool(ask.IntentOf(question)) == tool {
		ans, err := s.ask.Ask(ctx, req.User, question)
		if errors.Is(err, ask.ErrNoProduct) {
			a.Status, a.Text = StatusNoProduct, tr(req.Lang, "no_product")
			return a, nil
		}
		if err != nil {
			return Answer{}, err
		}
		return s.fromAsk(ctx, a, ans)
	}
	sc := scope{product: product, trade: tradeOf(tradeWord(question))}
	if r := s.ask.FindRegion(question); r != nil {
		sc.region, sc.province = r, r.Province
	}
	res, err := s.runScoped(ctx, tool, sc)
	if err != nil {
		return Answer{}, err
	}
	s.ask.Log(ctx, req.User, req.Question, product.ID, sc.region)
	return finish(a, []result{res}), nil
}

func finish(a Answer, results []result) Answer {
	enough := false
	for _, r := range results {
		a.Sources = append(a.Sources, r.source)
		enough = enough || r.source.Enough
	}
	if !enough {
		a.Status = StatusInsufficient
	}
	a.Table = lastTable(results)
	a.Text = compose(results, a.Lang)
	return a
}

// fromAsk turns a rule-based /savol answer into an Answer.
func (s *Service) fromAsk(ctx context.Context, a Answer, ans ask.Answer) (Answer, error) {
	sc := scope{product: ans.Product, trade: ans.Trade, region: ans.Region}
	if ans.Region != nil {
		sc.province = ans.Region.Province
	}
	stats, err := s.statsResult(ctx, sc, ans.Stats)
	if err != nil {
		return Answer{}, err
	}
	results := []result{stats}
	if ans.Stats.Enough {
		var extra result
		switch ans.Intent {
		case ask.IntentCheapest:
			extra, err = s.cheapestOffers(ctx, sc, 0)
		case ask.IntentAreas:
			extra, err = s.compareRegions(ctx, sc, nil)
		}
		if err != nil {
			return Answer{}, err
		}
		if extra.summary != nil {
			results = append(results, extra)
		}
	}
	a = finish(a, results)
	a.Rules = &ans
	return a, nil
}

// runScoped runs a tool for an already resolved scope (the rules path).
func (s *Service) runScoped(ctx context.Context, tool string, sc scope) (result, error) {
	var (
		res result
		err error
	)
	switch tool {
	case ToolPriceTrend:
		res, err = s.priceTrend(ctx, sc, 8)
	case ToolCheapestOffers:
		res, err = s.cheapestOffers(ctx, sc, 0)
	case ToolPressure:
		res, err = s.pressure(ctx, sc)
	case ToolCompareRegions:
		res, err = s.compareRegions(ctx, sc, nil)
	case ToolForecast:
		res, err = s.forecast(ctx, sc)
	case ToolAnomalies:
		res, err = s.anomalies(ctx, sc)
	case ToolTopDemand:
		res, err = s.topDemand(ctx, sc)
	default:
		res, err = s.priceStats(ctx, sc)
	}
	res.productID, res.region = sc.product.ID, sc.region
	return res, err
}

// productIn finds the product a question is about: the dictionary first,
// then the catalog matcher word by word. The question comes back with a
// matched word replaced by the product's name, for the rule-based reader.
func (s *Service) productIn(ctx context.Context, question string) (db.GetProductRow, string, error) {
	if d, problem := s.prices.ParseDemand(question); problem == nil || problem.Code != "no_product" {
		if d.ProductID != 0 {
			p, err := s.q.GetProduct(ctx, d.ProductID)
			return p, question, err
		}
	}
	if s.matcher != nil {
		words := strings.Fields(pricing.Normalize(question))
		for i, w := range words {
			w = strings.Trim(w, "?!.,;:«»\"()")
			if len([]rune(w)) < 3 {
				continue
			}
			best, _, err := s.matcher.MatchProduct(ctx, w)
			if err != nil {
				return db.GetProductRow{}, question, err
			}
			if best != nil {
				p, err := s.q.GetProduct(ctx, best.ProductID)
				words[i] = pricing.Normalize(best.Name)
				return p, strings.Join(words, " "), err
			}
		}
	}
	return db.GetProductRow{}, question, errUnknownProduct
}

func tradeWord(q string) string {
	n := pricing.Normalize(q)
	for _, w := range []string{"chakana", "roznisa", "roznich", "retail"} {
		if strings.Contains(n, w) {
			return "chakana"
		}
	}
	return "ulgurji"
}

func askTool(i ask.Intent) string {
	switch i {
	case ask.IntentCheapest:
		return ToolCheapestOffers
	case ask.IntentAreas:
		return ToolCompareRegions
	}
	return ToolPriceStats
}

// routes are keywords (Latin script, as pricing.Normalize writes Uzbek,
// Russian and English) for the rule-based routing, checked in order.
var routes = []struct {
	tool  string
	words []string
}{
	{ToolForecast, []string{"prognoz", "bashorat", "keyingi hafta", "keyingi oy", "kelasi hafta", "qanday boladi", "nima boladi", "sleduyush", "budet stoit", "forecast", "next week", "next month", "will cost", "will be"}},
	{ToolAnomalies, []string{"anomal", "galati", "shubhali", "keskin sakra", "sakrash", "strann", "podozrit", "skachok", "anomaly", "anomalies", "spike", "suspicious", "unusual"}},
	{ToolTopDemand, []string{"eng kop soral", "eng kop qidir", "kop soralgan", "kop qidirilgan", "qaysi mahsulotga talab", "top talab", "populyar", "vostrebovan", "bolshe vsego ishut", "most wanted", "most demand", "most popular", "top demand", "most searched"}},
	{ToolPressure, []string{"talab", "bosim", "taklif kop", "taklif kam", "spros", "predlojen", "defisit", "demand", "pressure", "supply"}},
	{ToolPriceTrend, []string{"dinamika", "ozgar", "haftalar", "tendens", "oshdi", "tushdi", "oshyapti", "oshmoqda", "tushyapti", "tushmoqda", "qimmatlash", "arzonlash", "izmenil", "viros", "podoroj", "podeshev", "trend", "changed", "rising", "falling", "over the last", "weeks"}},
	{ToolCompareRegions, []string{"solishtir", "qaysi viloyat", "qaysi tuman", "qaysi hudud", "qaysi shahar", "hududlar", "viloyatlar", "sravn", "v kakom regione", "v kakoy oblasti", "po regionam", "compare", "which region", "which province", "by region"}},
	{ToolCheapestOffers, []string{"eng arzon", "kimda", "sotuvchi", "qayerdan olsa", "qayerdan", "kim sotadi", "sotadi", "deshevle", "samiy deshev", "samie deshev", "u kogo", "gde kupit", "cheapest", "who sells", "where to buy", "sellers"}},
}

// Route is the function the rules pick for a question.
func Route(question string) string {
	n := " " + strings.Join(strings.Fields(pricing.Normalize(question)), " ") + " "
	for _, r := range routes {
		for _, w := range r.words {
			if strings.Contains(n, w) {
				return r.tool
			}
		}
	}
	return ToolPriceStats
}

// limiter allows n events per window per key.
type limiter struct {
	mu     sync.Mutex
	n      int
	window time.Duration
	seen   map[string][]time.Time
}

func newLimiter(n int, window time.Duration) *limiter {
	return &limiter{n: n, window: window, seen: map[string][]time.Time{}}
}

func (l *limiter) allow(key string, now time.Time) bool {
	if l == nil {
		return true
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.seen) > 10000 { // forget everyone rather than grow without bound
		l.seen = map[string][]time.Time{}
	}
	recent := l.seen[key][:0]
	for _, t := range l.seen[key] {
		if now.Sub(t) < l.window {
			recent = append(recent, t)
		}
	}
	if len(recent) >= l.n {
		l.seen[key] = recent
		return false
	}
	l.seen[key] = append(recent, now)
	return true
}
