package pricing

import (
	"context"
	"encoding/json"
	"sort"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"bugun/internal/store/db"
)

// Measuring the parser (MATN_TUSHUNISH.md 1): every attempt is logged with
// what was understood and what the seller did with the card, so the share
// understood at the first try can be followed week by week.

// Where a text came from.
const (
	ViaWeb     = "web"
	ViaBot     = "bot"
	ViaChannel = "channel"
)

// fillerWords are common words in price messages that name no product.
var fillerWords = map[string]bool{
	"bor": true, "bugun": true, "ertaga": true, "narx": true, "narxi": true, "narxlar": true, "narxda": true,
	"sotiladi": true, "sotaman": true, "sotamiz": true, "sotuvda": true, "beraman": true, "beramiz": true,
	"gacha": true, "har": true, "sifatli": true, "yangi": true, "arzon": true, "toza": true, "zor": true,
	"super": true, "ekstra": true, "extra": true, "kelinglar": true, "keling": true, "murojaat": true, "tel": true,
	"telefon": true, "manzil": true, "bozor": true, "bozorda": true, "bizda": true, "bizdan": true, "va": true,
	"ham": true, "yoki": true, "uchun": true, "faqat": true, "hozir": true, "endi": true, "juda": true,
	"yaxshi": true, "mahsulot": true, "mahsulotlar": true, "sotuv": true, "aksiya": true, "chegirma": true,
	"yetkazib": true, "berish": true, "dostavka": true, "katta": true, "kichik": true, "mayda": true, "orta": true,
	"optom": true, "opt": true, "ulgurji": true, "ulgurjiga": true, "optomga": true, "optovoy": true, "chakana": true,
	"roznitsa": true, "narxlari": true, "kerak": true, "kerakmi": true, "sotib": true, "olaman": true, "olamiz": true, "kurs": true, "the": true, "salom": true, "assalomu": true, "alaykum": true,
}

// unknownWords are the words of a text that name no product, unit, number
// or common filler: the candidates for new aliases.
func (c *Catalog) unknownWords(text string) []string {
	seen := map[string]bool{}
	var out []string
	for _, t := range tokenize(Normalize(text)) {
		w := t.text
		if t.kind != tokWord || len([]rune(w)) < 3 || seen[w] || plainWord(w) {
			continue
		}
		if p, _, _ := c.matchProduct([]token{t}); p >= 0 {
			continue
		}
		seen[w] = true
		out = append(out, w)
	}
	return out
}

// logParse records one attempt; it returns the log id (0 when logging failed,
// which never stops the seller).
func (s *Service) logParse(ctx context.Context, user *uuid.UUID, via, text string, res Result, byModel bool, outcome string) int64 {
	items, _ := json.Marshal(res.Items)
	problems, _ := json.Marshal(res.Problems)
	p := db.InsertParseLogParams{Via: via, RawText: text, Items: items, Problems: problems,
		ItemsN: int32(len(res.Items)), ProblemsN: int32(len(res.Problems)), ByModel: byModel, Outcome: outcome}
	if user != nil {
		p.UserID = pgtype.UUID{Bytes: *user, Valid: true}
	}
	id, err := s.q.InsertParseLog(ctx, p)
	if err != nil {
		return 0
	}
	return id
}

// LogChannel records a channel post; there is no card to confirm.
func (s *Service) LogChannel(ctx context.Context, text string, res Result, byModel bool) {
	s.logParse(ctx, nil, ViaChannel, text, res, byModel, "none")
}

func (s *Service) setOutcome(ctx context.Context, logID pgtype.Int8, outcome string) {
	if logID.Valid {
		_ = s.q.SetParseOutcome(ctx, db.SetParseOutcomeParams{ID: logID.Int64, Outcome: outcome})
	}
}

// ParserStats is how well texts were understood over some days.
type ParserStats struct {
	Days      int   `json:"days"`
	Messages  int32 `json:"messages" doc:"Texts read: site, bot and channel posts"`
	Web       int32 `json:"web"`
	Bot       int32 `json:"bot"`
	Channel   int32 `json:"channel"`
	Full      int32 `json:"full" doc:"Everything understood"`
	Partial   int32 `json:"partial" doc:"Some prices understood, some lines not"`
	Failed    int32 `json:"failed" doc:"Nothing understood"`
	ByModel   int32 `json:"by_model" doc:"Texts a language model helped with"`
	Confirmed int32 `json:"confirmed" doc:"Cards the seller confirmed"`
	Discarded int32 `json:"discarded" doc:"Cards the seller threw away (usually misread)"`
	Expired   int32 `json:"expired" doc:"Cards left unanswered for an hour"`
	Pending   int32 `json:"pending"`
	// FirstTry is confirmed cards against every card answered or expired.
	FirstTry *float64    `json:"first_try,omitempty" doc:"Share of cards confirmed as they were, 0–1; omitted without cards"`
	Unknown  []WordCount `json:"unknown" doc:"Words the parser did not know, most frequent first: candidates for new aliases"`
}

// WordCount is an unknown word and how many texts had it.
type WordCount struct {
	Word    string `json:"word"`
	Count   int    `json:"count"`
	Example string `json:"example" doc:"One text it came in, cut to 80 characters"`
}

const topUnknown = 20

// Stats sums up the last days of the parse log.
func (s *Service) Stats(ctx context.Context, days int) (ParserStats, error) {
	r, err := s.q.ParseStats(ctx, int32(days))
	if err != nil {
		return ParserStats{}, err
	}
	st := ParserStats{Days: days, Messages: r.Messages, Web: r.Web, Bot: r.Bot, Channel: r.Channel, Full: r.Full,
		Partial: r.Partial, Failed: r.Failed, ByModel: r.ByModel, Confirmed: r.Confirmed, Discarded: r.Discarded,
		Expired: r.Expired, Pending: r.Pending, Unknown: []WordCount{}}
	if answered := r.Confirmed + r.Discarded + r.Expired; answered > 0 {
		v := float64(r.Confirmed) / float64(answered)
		v = float64(int(v*1000+0.5)) / 1000
		st.FirstTry = &v
	}
	misses, err := s.q.ParseMisses(ctx, int32(days))
	if err != nil {
		return st, err
	}
	counts := map[string]*WordCount{}
	for _, m := range misses {
		texts := []string{}
		if m.ItemsN == 0 {
			texts = append(texts, m.RawText)
		} else {
			var ps []Problem
			if json.Unmarshal(m.Problems, &ps) == nil {
				for _, p := range ps {
					if p.Code == "no_product" {
						texts = append(texts, p.Raw)
					}
				}
			}
		}
		seen := map[string]bool{}
		for _, t := range texts {
			for _, w := range s.catalog.unknownWords(t) {
				if seen[w] {
					continue
				}
				seen[w] = true
				c := counts[w]
				if c == nil {
					ex := []rune(strings.TrimSpace(m.RawText))
					if len(ex) > 80 {
						ex = append(ex[:80], '…')
					}
					c = &WordCount{Word: w, Example: string(ex)}
					counts[w] = c
				}
				c.Count++
			}
		}
	}
	for _, c := range counts {
		st.Unknown = append(st.Unknown, *c)
	}
	sort.Slice(st.Unknown, func(i, j int) bool {
		if st.Unknown[i].Count != st.Unknown[j].Count {
			return st.Unknown[i].Count > st.Unknown[j].Count
		}
		return st.Unknown[i].Word < st.Unknown[j].Word
	})
	if len(st.Unknown) > topUnknown {
		st.Unknown = st.Unknown[:topUnknown]
	}
	return st, nil
}

// CleanParseLog deletes texts older than 180 days.
func (s *Service) CleanParseLog(ctx context.Context) (int, error) {
	n, err := s.q.DeleteOldParseLog(ctx)
	return int(n), err
}
