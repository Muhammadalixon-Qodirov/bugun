package analyst

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"bugun/internal/insight"
	"bugun/internal/llm"
	"bugun/internal/numcheck"
	"bugun/internal/pricing"
	"bugun/internal/store/db"
)

// The chat assistant (CHATBOT_REJA.md): the analyst in a conversation. It
// remembers the last turns, asks when the product, place or amount is
// unclear (ask_user), says so when it cannot answer (cannot_answer) and logs
// those questions for the admins. Only the functions a message needs are
// offered to the model, which keeps each call small for free-tier limits.

// Answer kinds.
const (
	KindAnswer     = "answer"
	KindClarify    = "clarify"
	KindUnanswered = "unanswered"
)

// ChatRequest is one message of a conversation.
type ChatRequest struct {
	SessionID string // empty or unknown: a new conversation
	Message   string
	Lang      string
	User      *db.User
	Key       string // who asks, for the rate limit and the session owner
}

// ChatAnswer is the reply with the conversation it belongs to.
type ChatAnswer struct {
	SessionID string `json:"session_id" doc:"Send it with the next message to continue the conversation"`
	Answer
}

const (
	historyTurns   = 6   // messages of context
	historyChars   = 600 // each at most
	maxOptions     = 5
	maxOptionChars = 40
)

// Converse answers one message in its conversation.
func (s *Service) Converse(ctx context.Context, req ChatRequest) (ChatAnswer, error) {
	started := time.Now()
	req.Message = strings.TrimSpace(req.Message)
	req.Lang = langOf(req.Lang)
	if req.Message == "" || len([]rune(req.Message)) > MaxQuestionLen {
		return ChatAnswer{}, ErrEmpty
	}
	session, err := s.session(ctx, req)
	if err != nil {
		return ChatAnswer{}, err
	}
	rows, err := s.q.RecentChatMessages(ctx, db.RecentChatMessagesParams{SessionID: session, Limit: historyTurns})
	if err != nil {
		return ChatAnswer{}, err
	}
	history := make([]llm.Message, 0, len(rows))
	var said []string
	for i := len(rows) - 1; i >= 0; i-- {
		c := rows[i].Content
		if r := []rune(c); len(r) > historyChars {
			c = string(r[:historyChars])
		}
		history = append(history, llm.Message{Role: rows[i].Role, Content: c})
		said = append(said, c)
	}
	areq := Request{Question: req.Message, Lang: req.Lang, User: req.User, Key: req.Key, History: said}

	var a Answer
	switch {
	case s.model == nil:
		a, err = s.rules(ctx, areq, "")
	case !s.limit.allow(req.Key, time.Now()):
		a, err = s.rules(ctx, areq, "rate")
	default:
		if req.User != nil {
			ctx = llm.WithUser(ctx, req.User.ID)
		}
		a, err = s.converseModel(llm.WithFeature(ctx, "assistant"), areq, history)
		switch {
		case errors.Is(err, llm.ErrLimit):
			a, err = s.rules(ctx, areq, "limit")
		case errors.Is(err, errNoTools):
			a, err = s.rules(ctx, areq, "no_tools")
		case err != nil:
			var se *llm.StatusError
			why := "error"
			if errors.As(err, &se) && se.Code == 429 {
				why = "limit"
			}
			slog.WarnContext(ctx, "assistant model", "err", err)
			a, err = s.rules(ctx, areq, why)
		}
	}
	if err != nil {
		return ChatAnswer{}, err
	}
	if a.Kind == "" {
		a.Kind = KindAnswer
	}
	if a.Kind == KindUnanswered {
		s.unanswered(ctx, session, req, a.unanswered, a.topic)
	}
	// A product the catalog does not have is a gap the admins should see.
	if a.Kind == KindAnswer && a.Status == StatusNoProduct {
		s.unanswered(ctx, session, req, "no_data", "")
	}
	s.save(ctx, session, req, a, time.Since(started))
	return ChatAnswer{SessionID: session.String(), Answer: a}, nil
}

func keyHash(key string) string {
	h := sha256.Sum256([]byte("chat:" + key))
	return hex.EncodeToString(h[:])
}

// session continues the given conversation when it belongs to the asker,
// else starts one.
func (s *Service) session(ctx context.Context, req ChatRequest) (uuid.UUID, error) {
	hash := keyHash(req.Key)
	if id, err := uuid.Parse(req.SessionID); err == nil {
		if row, err := s.q.GetChatSession(ctx, id); err == nil && row.KeyHash == hash {
			return id, nil
		}
	}
	var user pgtype.UUID
	if req.User != nil {
		user = pgtype.UUID{Bytes: req.User.ID, Valid: true}
	}
	return s.q.CreateChatSession(ctx, db.CreateChatSessionParams{UserID: user, KeyHash: hash})
}

// LatestSession is the user's conversation of the last 30 minutes, for the
// bot, which has no session id to send; "" starts a new one.
func (s *Service) LatestSession(ctx context.Context, user uuid.UUID) string {
	id, err := s.q.LatestChatSession(ctx, db.LatestChatSessionParams{UserID: pgtype.UUID{Bytes: user, Valid: true}, Since: time.Now().Add(-30 * time.Minute)})
	if err != nil {
		return ""
	}
	return id.String()
}

// Options are the buttons last offered in a conversation.
func (s *Service) Options(ctx context.Context, session string) []string {
	id, err := uuid.Parse(session)
	if err != nil {
		return nil
	}
	row, err := s.q.GetChatSession(ctx, id)
	if err != nil {
		return nil
	}
	var out []string
	_ = json.Unmarshal(row.Options, &out)
	return out
}

func (s *Service) save(ctx context.Context, session uuid.UUID, req ChatRequest, a Answer, took time.Duration) {
	ctx = context.WithoutCancel(ctx)
	tools := []string{}
	for _, src := range a.Sources {
		tools = append(tools, src.Tool)
	}
	msgs := []db.InsertChatMessageParams{
		{SessionID: session, Role: "user", Content: req.Message, Tools: []string{}},
		{SessionID: session, Role: "assistant", Content: a.Text, Kind: a.Kind, Status: a.Status, Mode: a.Mode, Fallback: a.Fallback,
			Tools: tools, LatencyMs: int32(took.Milliseconds())},
	}
	for _, m := range msgs {
		if err := s.q.InsertChatMessage(ctx, m); err != nil {
			slog.WarnContext(ctx, "chat message", "err", err)
		}
	}
	opts, _ := json.Marshal(append([]string{}, a.Options...))
	if err := s.q.TouchChatSession(ctx, db.TouchChatSessionParams{ID: session, Options: opts}); err != nil {
		slog.WarnContext(ctx, "chat session", "err", err)
	}
}

// unanswered records a question the assistant could not answer.
func (s *Service) unanswered(ctx context.Context, session uuid.UUID, req ChatRequest, reason, topic string) {
	p := db.InsertChatUnansweredParams{SessionID: pgtype.UUID{Bytes: session, Valid: true}, Question: req.Message,
		Norm: normQuestion(req.Message), Reason: reason, Topic: topic}
	if req.User != nil {
		p.UserID = pgtype.UUID{Bytes: req.User.ID, Valid: true}
	}
	if err := s.q.InsertChatUnanswered(context.WithoutCancel(ctx), p); err != nil {
		slog.WarnContext(ctx, "chat unanswered", "err", err)
	}
}

// normQuestion groups the same question written differently: Latin script,
// no punctuation or numbers, single spaces.
func normQuestion(q string) string {
	var b strings.Builder
	for _, r := range pricing.Normalize(q) {
		switch {
		case r >= 'a' && r <= 'z', r == ' ':
			b.WriteRune(r)
		default:
			b.WriteRune(' ')
		}
	}
	n := strings.Join(strings.Fields(b.String()), " ")
	if r := []rune(n); len(r) > 120 {
		n = string(r[:120])
	}
	return n
}

const chatPrompt = `Sen "Bugun" platformasining yordamchisisan: Oʻzbekiston bozor narxlari, mahsulotlar, viloyatlar, prognoz, maslahat, savat va retsept narxi.
Qoidalar:
- Maʼlumotni faqat berilgan funksiyalardan ol. Javobdagi har bir raqam (narx, foiz, son, sana) funksiya natijasida yoki foydalanuvchi xabarida aynan boʻlishi shart. Oʻzingdan raqam qoʻshma va hisoblama (jami, farq, foizni oʻzing hisoblama: savat uchun basket_cost dan foydalan).
- Mahsulot, viloyat yoki miqdor noaniq boʻlsa va taxmin notoʻgʻri javobga olib kelsa, ask_user bilan qisqa soʻra, imkon boʻlsa variantlar ber. Viloyat aytilmasa, butun mamlakat boʻyicha javob ber va buni ayt; buning uchun soʻrama.
- Nom bir nechta mahsulotga mos boʻlishi mumkin boʻlsa (masalan "qalampir"), find_product bilan tekshir.
- Retsept yoki "ovqat qilmoqchiman" desa: odatiy masalliqlar va taxminiy miqdorni oʻzing tanla, basket_cost ni chaqir va javobda miqdorlar taxminiy ekanini ayt. Necha kishilik ekani nomaʼlum boʻlsa, avval ask_user bilan soʻra.
- Savol bozor, narx, mahsulot, viloyat yoki platformaga oid boʻlmasa, yoki hech bir funksiyada kerakli maʼlumot boʻlmasa, cannot_answer ni chaqir.
- Funksiya "enough": false qaytarsa, maʼlumot yetarli emasligini ayt va taxmin qilma. Demo maʼlumot ulushi 0 dan katta boʻlsa, buni eslat.
- create_request va watch_price faqat taklif: foydalanuvchi tugmani bosib tasdiqlaydi. Ularni faqat foydalanuvchi soʻrasa chaqir.
- Oldingi xabarlarni hisobga ol: "Andijonda-chi?" oldingi mahsulot haqida.
- Javob qisqa (2–6 gap), oddiy matn, markdownsiz, %s tilida. Soʻm narxlarini birligi bilan yoz.
Bugun: %s.`

// toolGroups are offered together; always is sent with every message.
var (
	alwaysTools = []string{ToolPriceStats, ToolPriceTrend, ToolCheapestOffers, ToolCompareRegions, ToolFindProduct, ToolAskUser, ToolCannotAnswer}
	toolGroups  = []struct {
		tools []string
		words []string
	}{
		{[]string{ToolForecast, ToolAdvice, ToolSeasonality, ToolWeekdays, ToolPressure}, []string{"prognoz", "bashorat", "keyin", "kelasi", "qachon", "olsam", "sotsam",
			"kutay", "kutish", "maslahat", "mavsum", "oyda", "arzonlash", "qimmatlash", "talab", "taklif", "qaysi kun", "hafta kun", "dushanba", "juma", "yakshanba",
			"prognoz", "kogda", "sovet", "sezon", "budet", "forecast", "when", "should i", "season", "weekday", "demand"}},
		{[]string{ToolPriceReasons, ToolMarketEvents, ToolAnomalies, ToolWeeklyMovers}, []string{"nega", "nima uchun", "sabab", "qimmatladi", "arzonladi", "oshdi", "tushdi",
			"voqea", "bayram", "hayit", "ramazon", "navroz", "anomal", "shubhali", "bu hafta", "ozgar", "pochemu", "prichin", "prazdnik", "why", "reason", "holiday", "this week"}},
		{[]string{ToolRegionOverview, ToolDistrictPrices, ToolArbitrage, ToolWholesaleRetail, ToolTopDemand, ToolFindRegion}, []string{"bozor", "tuman", "viloyat",
			"hudud", "shahar", "foyda", "qayerda sot", "olib borib", "ulgurji", "chakana", "optom", "talab", "qidir", "soral", "rayon", "rynok", "bazar", "opt", "roznic",
			"market", "district", "wholesale", "retail", "profit"}},
		{[]string{ToolBasketCost}, []string{"retsept", "ovqat", "osh", "palov", "shorva", "manti", "somsa", "lagmon", "kishilik", "masalliq", "hisobla", "savat", "jami",
			"summa", "qancha ketadi", "kg", "kilo", "recept", "blyud", "korzin", "itogo", "recipe", "cook", "basket", "total"}},
		{[]string{ToolExchangeRate, ToolWeather}, []string{"kurs", "dollar", "valyuta", "rubl", "ob havo", "obhavo", "sovuq", "issiq", "yomgir", "qor", "harorat",
			"pogod", "moroz", "weather", "frost", "rate", "currency"}},
		{[]string{ToolCompareProducts}, []string{"solishtir", "qaysi biri", "farqi", "sravn", "compare", "versus", "vs"}},
		{[]string{ToolSellerInfo, ToolCreateRequest, ToolWatchPrice}, []string{"sotuvchi", "dokon", "soruv", "sorov", "buyurtma", "kuzat", "xabar ber", "ogohlantir",
			"sotib olmoqchi", "kerak", "prodav", "zakaz", "zapros", "sledit", "seller", "order", "request", "alert", "notify", "buy"}},
		{[]string{ToolPlatformHelp}, []string{"qanday", "qanaqa", "joylash", "qoshish", "bot", "buyruq", "royxat", "profil", "kanal", "avtomatik", "ishlaydi",
			"kak", "instruk", "komand", "how", "help", "command", "channel"}},
	}
)

// chatTools picks the functions for a message and the one before it.
func chatTools(text string) []llm.Tool {
	n := " " + strings.Join(strings.Fields(pricing.Normalize(text)), " ") + " "
	want := map[string]bool{}
	for _, t := range alwaysTools {
		want[t] = true
	}
	matched := false
	for _, g := range toolGroups {
		for _, w := range g.words {
			if strings.Contains(n, w) {
				for _, t := range g.tools {
					want[t] = true
				}
				matched = true
				break
			}
		}
	}
	if !matched {
		// Nothing specific: the common analytics, so the model can still
		// answer or say it cannot.
		for _, t := range []string{ToolForecast, ToolAdvice, ToolPriceReasons, ToolRegionOverview, ToolPlatformHelp} {
			want[t] = true
		}
	}
	var out []llm.Tool
	for _, t := range toolDefs() {
		if want[t.Function.Name] {
			out = append(out, t)
		}
	}
	more := moreToolDefs()
	for _, name := range allMoreTools {
		if want[name] {
			out = append(out, more[name])
		}
	}
	return out
}

// allMoreTools orders the further tools (map order is random).
var allMoreTools = []string{ToolFindProduct, ToolFindRegion, ToolAdvice, ToolSeasonality, ToolWeekdays, ToolPriceReasons, ToolDistrictPrices,
	ToolWholesaleRetail, ToolArbitrage, ToolRegionOverview, ToolWeeklyMovers, ToolCompareProducts, ToolBasketCost, ToolExchangeRate, ToolWeather,
	ToolMarketEvents, ToolSellerInfo, ToolPlatformHelp, ToolCreateRequest, ToolWatchPrice, ToolAskUser, ToolCannotAnswer}

var baseTools = map[string]bool{ToolPriceStats: true, ToolPriceTrend: true, ToolCheapestOffers: true, ToolPressure: true,
	ToolCompareRegions: true, ToolForecast: true, ToolAnomalies: true, ToolTopDemand: true}

// converseModel is viaModel for a conversation, with the control functions.
func (s *Service) converseModel(ctx context.Context, req Request, history []llm.Message) (Answer, error) {
	msgs := []llm.Message{{Role: "system", Content: fmt.Sprintf(chatPrompt, langNames[req.Lang], insight.Day(time.Now()).Time.Format(time.DateOnly))}}
	msgs = append(msgs, history...)
	msgs = append(msgs, llm.Message{Role: "user", Content: req.Question})
	last := ""
	for i := len(history) - 1; i >= 0; i-- {
		if history[i].Role == "user" {
			last = history[i].Text()
			break
		}
	}
	tools := chatTools(req.Question + " " + last)
	var results []result
	var actions []Action
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
		// A control function ends the turn.
		for _, tc := range calls {
			switch tc.Function.Name {
			case ToolAskUser:
				return s.clarify(req, tc.Function.Arguments, results), nil
			case ToolCannotAnswer:
				var m moreArgs
				_ = json.Unmarshal([]byte(tc.Function.Arguments), &m)
				reason := m.Reason
				if reason != "off_topic" && reason != "no_data" && reason != "no_tool" {
					reason = "no_tool"
				}
				a := Answer{Kind: KindUnanswered, Mode: "model", Lang: req.Lang, Status: StatusOK, Sources: []Source{},
					Text: tr(req.Lang, "cannot_"+reason), unanswered: reason}
				var t struct {
					Topic string `json:"topic"`
				}
				_ = json.Unmarshal([]byte(tc.Function.Arguments), &t)
				a.topic = t.Topic
				return a, nil
			}
		}
		msgs = append(msgs, llm.Message{Role: "assistant", Content: msg.Content, ToolCalls: calls})
		for _, tc := range calls {
			var res result
			var err error
			if baseTools[tc.Function.Name] {
				res, err = s.run(ctx, tc.Function.Name, tc.Function.Arguments)
			} else {
				res, err = s.runMore(ctx, tc.Function.Name, tc.Function.Arguments, req.Lang)
			}
			if err != nil {
				return Answer{}, err
			}
			results = append(results, res)
			if res.action != nil {
				actions = append(actions, *res.action)
			}
			b, _ := json.Marshal(res.data)
			msgs = append(msgs, llm.Message{Role: "tool", ToolCallID: tc.ID, Content: string(b)})
		}
	}
	if !done {
		msg, err := s.model.Chat(ctx, llm.ChatRequest{Messages: msgs, Temperature: 0, MaxTokens: 700})
		if err != nil {
			return Answer{}, err
		}
		final = strings.TrimSpace(msg.Text())
	}
	a, err := s.check(ctx, req, final, results)
	if err != nil {
		return Answer{}, err
	}
	a.Actions = actions
	return a, nil
}

// clarify turns ask_user into a question with buttons. A question with a
// number from nowhere is replaced by a plain one.
func (s *Service) clarify(req Request, raw string, results []result) Answer {
	var m moreArgs
	_ = json.Unmarshal([]byte(raw), &m)
	q := strings.TrimSpace(m.Question)
	allow := numcheck.New()
	allow.AddText(req.Question)
	for _, h := range req.History {
		allow.AddText(h)
	}
	for _, r := range results {
		allow.AddJSON(r.data)
	}
	if q == "" || len(allow.Unchecked(q)) > 0 {
		q = tr(req.Lang, "clarify_generic")
	}
	a := Answer{Kind: KindClarify, Mode: "model", Lang: req.Lang, Status: StatusOK, Text: q, Sources: []Source{}}
	for _, r := range results {
		a.Sources = append(a.Sources, r.source)
	}
	for _, o := range m.Options {
		o = strings.TrimSpace(o)
		if o == "" || len(allow.Unchecked(o)) > 0 {
			continue
		}
		if r := []rune(o); len(r) > maxOptionChars {
			o = string(r[:maxOptionChars])
		}
		a.Options = append(a.Options, o)
		if len(a.Options) == maxOptions {
			break
		}
	}
	return a
}

// UnansweredGroup is one question the assistant could not answer, grouped.
type UnansweredGroup struct {
	Question string    `json:"question" doc:"The latest wording"`
	Reason   string    `json:"reason" enum:"off_topic,no_data,no_tool,tool_error,unclear" doc:"off_topic: not about markets; no_data: a product or data the platform does not have; no_tool: no function can answer it"`
	Topic    string    `json:"topic,omitempty" doc:"What it was about, as the model put it"`
	Asked    int32     `json:"asked" doc:"How many times"`
	LastAt   time.Time `json:"last_at"`
}

// Unanswered lists the unanswered questions of the last days, most asked first.
func (s *Service) Unanswered(ctx context.Context, days int) ([]UnansweredGroup, error) {
	rows, err := s.q.UnansweredGroups(ctx, int32(days))
	if err != nil {
		return nil, err
	}
	out := make([]UnansweredGroup, len(rows))
	for i, r := range rows {
		out[i] = UnansweredGroup{Question: r.Example, Reason: r.Reason, Topic: r.Topic, Asked: r.Asked, LastAt: r.LastAt}
	}
	return out, nil
}

// ChatStats are the assistant's figures over some days.
type ChatStats struct {
	Days          int         `json:"days"`
	Conversations int32       `json:"conversations"`
	Answers       int32       `json:"answers" doc:"Replies of every kind"`
	Answered      int32       `json:"answered"`
	Clarified     int32       `json:"clarified" doc:"Questions back to the user"`
	Unanswered    int32       `json:"unanswered"`
	Insufficient  int32       `json:"insufficient" doc:"Answered that there is not enough data"`
	ByModel       int32       `json:"by_model" doc:"Written by the language model (the rest by the rules)"`
	Fallbacks     int32       `json:"fallbacks" doc:"The model was limited, failed or wrote an unchecked number, and the rules answered"`
	AvgLatencyMs  int32       `json:"avg_latency_ms"`
	Tools         []ToolCount `json:"tools" doc:"Functions called, most used first"`
}

type ToolCount struct {
	Tool  string `json:"tool"`
	Calls int32  `json:"calls"`
}

func (s *Service) Stats(ctx context.Context, days int) (ChatStats, error) {
	r, err := s.q.ChatStats(ctx, int32(days))
	if err != nil {
		return ChatStats{}, err
	}
	st := ChatStats{Days: days, Conversations: r.Conversations, Answers: r.Answers, Answered: r.Answered, Clarified: r.Clarified,
		Unanswered: r.Unanswered, Insufficient: r.Insufficient, ByModel: r.ByModel, Fallbacks: r.Fallbacks, AvgLatencyMs: r.AvgLatencyMs, Tools: []ToolCount{}}
	tools, err := s.q.ChatToolUse(ctx, int32(days))
	if err != nil {
		return st, err
	}
	for _, t := range tools {
		st.Tools = append(st.Tools, ToolCount{Tool: t.Tool, Calls: t.Calls})
	}
	return st, nil
}

// CleanChats deletes conversations older than 30 days and unanswered
// questions older than 180.
func (s *Service) CleanChats(ctx context.Context) (int, error) {
	n, err := s.q.DeleteOldChats(ctx)
	return int(n), err
}

// AwaitingReply reports whether the user's latest conversation (10 minutes)
// ended with a question back, so a short plain message answers it.
func (s *Service) AwaitingReply(ctx context.Context, user uuid.UUID) bool {
	id, err := s.q.LatestChatSession(ctx, db.LatestChatSessionParams{UserID: pgtype.UUID{Bytes: user, Valid: true}, Since: time.Now().Add(-10 * time.Minute)})
	if err != nil {
		return false
	}
	rows, err := s.q.RecentChatMessages(ctx, db.RecentChatMessagesParams{SessionID: id, Limit: 1})
	return err == nil && len(rows) == 1 && rows[0].Role == "assistant" && rows[0].Kind == KindClarify
}
