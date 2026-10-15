package bot

import (
	"context"
	"errors"
	"fmt"
	"html"
	"math"
	"strings"

	tg "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"bugun/internal/ask"
	"bugun/internal/pricing"
)

// savol answers /savol "Samarqandda piyoz qancha?" (spec §8.5).
func (b *Bot) savol(ctx context.Context, _ *tg.Bot, upd *models.Update) {
	msg := upd.Message
	u, ok := b.requireUser(ctx, msg.Chat.ID, msg.From.ID)
	if !ok {
		return
	}
	text := strings.TrimSpace(strings.TrimPrefix(msg.Text, "/savol"))
	if text == "" {
		b.send(ctx, msg.Chat.ID, "Narx haqida so'rang, masalan:\n<code>Samarqandda piyoz qancha?</code>\n<code>Urgutda eng arzon kartoshka kimda?</code>\n<code>Qaysi tumanda pomidor arzon?</code>", nil)
		return
	}
	if b.ai.Analyst != nil { // the analyst (AI-5): a model when configured, else these rules
		b.analystAnswer(ctx, msg.Chat.ID, u, text)
		return
	}
	a, err := b.ask.Ask(ctx, u, text)
	if errors.Is(err, ask.ErrNoProduct) {
		b.send(ctx, msg.Chat.ID, "Savolda mahsulot tanilmadi. Masalan: <code>Samarqandda piyoz qancha?</code>", nil)
		return
	}
	if err != nil {
		b.fail(ctx, msg.Chat.ID, "savol", err)
		return
	}
	b.sendAnswer(ctx, msg.Chat.ID, a)
}

// sendAnswer sends a rule-based answer with its watch and details buttons.
func (b *Bot) sendAnswer(ctx context.Context, chatID int64, a ask.Answer) {
	var province int16
	if a.Region != nil {
		province = a.Region.Province
	}
	b.send(ctx, chatID, answerText(a), &models.InlineKeyboardMarkup{InlineKeyboard: [][]models.InlineKeyboardButton{
		{watchButton(a.Product.ID, province, a.Trade)},
		{{Text: "Batafsil", URL: b.webURL + "/mahsulot/" + a.Product.Slug}},
	}})
}

// looksLikeQuestion tells a price question apart from a price or a request
// written as plain text: "piyoz qancha?" vs "piyoz 3500" vs "piyoz 200 kg".
func looksLikeQuestion(text string) bool {
	t := strings.ToLower(text)
	if strings.Contains(t, "?") {
		return true
	}
	// "kartoshka narxi 4500" is a seller's price, not a question.
	if strings.ContainsAny(t, "0123456789") {
		return false
	}
	for _, w := range []string{"qancha", "narxi", "narx qancha", "arzon", "qaysi", "kimda", "qayerda", "сколько", "почем"} {
		if strings.Contains(t, w) {
			return true
		}
	}
	return false
}

// answerText renders an answer with the context every number needs (spec
// §7.4): how many prices, the date range, where from.
func answerText(a ask.Answer) string {
	var sb strings.Builder
	place := "butun respublika"
	if a.Stats.Scope != "" {
		place = a.Stats.Scope
	}
	unit := a.Product.BaseUnit
	fmt.Fprintf(&sb, "<b>%s</b> — %s, %s\n", html.EscapeString(a.Product.Name), a.Trade, html.EscapeString(place))
	if a.Region != nil && a.Stats.Scope != a.Region.Name {
		fmt.Fprintf(&sb, "<i>%s bo'yicha narxlar hali kam, kengroq hudud olindi.</i>\n", html.EscapeString(a.Region.Name))
	}
	if !a.Stats.Enough {
		sb.WriteString("\nBu mahsulot bo'yicha narxlar hali kam (3 tadan kam). Ma'lumot yig'ilmoqda.")
		return sb.String()
	}
	fmt.Fprintf(&sb, "\nO'rtacha: <b>%s so'm/%s</b>\nOddiy oraliq: %s – %s\nEng arzon – eng qimmat: %s – %s\n",
		som(a.Stats.Median), unit, som(a.Stats.P25), som(a.Stats.P75),
		pricing.FormatSom(a.Stats.Min), pricing.FormatSom(a.Stats.Max))
	switch a.Intent {
	case ask.IntentCheapest:
		if len(a.Cheapest) > 0 {
			sb.WriteString("\n<b>Eng arzon takliflar:</b>\n")
			for _, s := range a.Cheapest {
				where := s.Region
				if s.Market != "" {
					where = s.Market + ", " + s.Region
				}
				fmt.Fprintf(&sb, "• %s so'm — %s (%s)\n", pricing.FormatSom(s.PriceSom), html.EscapeString(s.Name), html.EscapeString(where))
			}
		}
	case ask.IntentAreas:
		if len(a.Areas) > 0 {
			sb.WriteString("\n<b>Hududlar bo'yicha (arzonidan):</b>\n")
			for _, ar := range a.Areas {
				fmt.Fprintf(&sb, "• %s — %s so'm (%d ta narx)\n", html.EscapeString(ar.Name), som(ar.Median), ar.Offers)
			}
		} else {
			sb.WriteString("\nHududlarni solishtirish uchun narxlar hali kam.\n")
		}
	}
	fmt.Fprintf(&sb, "\n<i>%d ta narx · %s – %s · manba: sotuvchilar</i>", a.Stats.Offers, dateUz(a.Stats.From), dateUz(a.Stats.To))
	return sb.String()
}

func som(v float64) string { return pricing.FormatSom(int64(math.Round(v))) }
