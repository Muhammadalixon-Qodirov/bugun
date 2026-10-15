package bot

import (
	"context"
	"errors"
	"fmt"
	"html"
	"log/slog"
	"strconv"
	"strings"
	"time"

	tg "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"bugun/internal/pricing"
	"bugun/internal/store/db"
	"bugun/internal/tz"
)

const priceExample = "kartoshka 4500, 2 tonna\npiyoz kilosi 3800\nolma yashigi 200 ming, optom"

var monthsUz = [...]string{"yanvar", "fevral", "mart", "aprel", "may", "iyun", "iyul", "avgust", "sentabr", "oktabr", "noyabr", "dekabr"}

// dateUz renders a UTC time as a Tashkent calendar date, e.g. "2-oktabr".
func dateUz(t time.Time) string {
	t = t.In(tz.Tashkent)
	return fmt.Sprintf("%d-%s", t.Day(), monthsUz[t.Month()-1])
}

func (b *Bot) priceHelp(ctx context.Context, _ *tg.Bot, upd *models.Update) {
	msg := upd.Message
	u, enabled, err := b.linkedUser(ctx, msg.From.ID)
	if err != nil {
		b.fail(ctx, msg.Chat.ID, "narx", err)
		return
	}
	if u == nil || !enabled {
		b.notLinked(ctx, msg.Chat.ID)
		return
	}
	if !pricing.CanSubmit(*u) {
		b.sendNeedEntrepreneur(ctx, msg.Chat.ID)
		return
	}
	b.send(ctx, msg.Chat.ID, "Narxni oddiy matn bilan yozing, bir xabarda bir nechta mahsulot bo'lishi mumkin:\n\n<code>"+
		html.EscapeString(priceExample)+"</code>\n\nMen kartochka qaytaraman — tekshirib, «To'g'ri» ni bosasiz.", nil)
}

// submitPrice handles a free-text message from a seller.
func (b *Bot) submitPrice(ctx context.Context, chatID int64, u db.User, text string) {
	d, err := b.prices.CreateDraft(ctx, u, text, pricing.ViaBot)
	switch {
	case errors.Is(err, pricing.ErrTooLong):
		b.send(ctx, chatID, "Xabar juda uzun. Iltimos, qisqaroq yozing (2000 belgigacha).", nil)
		return
	case err != nil:
		b.fail(ctx, chatID, "create draft", err)
		return
	}
	if len(d.Items) == 0 && len(d.Suggestions) > 0 { // unknown word, likely products (AI-4)
		b.suggestProducts(ctx, chatID, text, d.Suggestions)
		return
	}
	if len(d.Items) == 0 {
		var sb strings.Builder
		if len(d.Problems) > 0 {
			for _, p := range d.Problems {
				sb.WriteString("❗ " + html.EscapeString(p.Message) + "\n")
			}
		} else {
			sb.WriteString("Xabarda mahsulot tanilmadi.\n")
		}
		sb.WriteString("\nMasalan:\n<code>" + html.EscapeString(priceExample) + "</code>")
		sb.WriteString("\n\nSotib olmoqchi bo'lsangiz: <code>/sorov kartoshka 200 kg</code>")
		b.send(ctx, chatID, sb.String(), nil)
		return
	}
	id := strconv.FormatInt(d.ID, 10)
	b.send(ctx, chatID, draftCard(d), &models.InlineKeyboardMarkup{InlineKeyboard: [][]models.InlineKeyboardButton{{
		{Text: "✅ To'g'ri", CallbackData: "p:ok:" + id},
		{Text: "✏️ Tuzatish", CallbackData: "p:fix:" + id},
	}}})
}

func draftCard(d pricing.Draft) string {
	var sb strings.Builder
	sb.WriteString("<b>Tekshiring:</b>\n\n")
	for i, it := range d.Items {
		fmt.Fprintf(&sb, "%d. <b>%s</b> — %s so'm/%s", i+1, html.EscapeString(it.ProductName), pricing.FormatSom(it.PriceSom), it.Unit)
		if it.Quantity != nil {
			fmt.Fprintf(&sb, " · %s %s", formatQty(*it.Quantity), it.Unit)
		}
		fmt.Fprintf(&sb, " · %s\n", it.TradeType)
		for _, n := range it.Notes {
			sb.WriteString("   ⚠️ " + html.EscapeString(n) + "\n")
		}
	}
	for _, p := range d.Problems {
		sb.WriteString("\n❗ " + html.EscapeString(p.Message))
	}
	sb.WriteString("\n\nHammasi to'g'rimi? Tasdiqlanmagan narx saqlanmaydi.")
	return sb.String()
}

// positionLine tells the seller where the price stands (spec §8.2):
// "12 sotuvchi ichida 9-o'rindasiz · o'rtacha 4 200 so'm (+7%)".
func positionLine(s pricing.Saved) string {
	if s.Status == "review" {
		return "⏳ Narx odatdagidan juda farq qiladi, tekshiruvdan so'ng chiqadi"
	}
	p := s.Position
	if p == nil {
		return "📊 Bu mahsulot bo'yicha viloyatda narxlar hali kam"
	}
	diff := ""
	switch {
	case p.DiffPercent > 0:
		diff = fmt.Sprintf(" (+%d%%)", p.DiffPercent)
	case p.DiffPercent < 0:
		diff = fmt.Sprintf(" (%d%%)", p.DiffPercent)
	}
	return fmt.Sprintf("📊 %d sotuvchi ichida %d-o'rindasiz · o'rtacha %s so'm%s",
		p.Total, p.Rank, pricing.FormatSom(p.Median), diff)
}

func formatQty(q float64) string {
	if q == float64(int64(q)) {
		return pricing.FormatSom(int64(q))
	}
	return strconv.FormatFloat(q, 'f', -1, 64)
}

// priceCallback handles the confirmation buttons under a draft card.
func (b *Bot) priceCallback(ctx context.Context, _ *tg.Bot, upd *models.Update) {
	cq := upd.CallbackQuery
	answer := func(text string) {
		_, _ = b.api.AnswerCallbackQuery(ctx, &tg.AnswerCallbackQueryParams{CallbackQueryID: cq.ID, Text: text})
	}
	parts := strings.Split(cq.Data, ":")
	if len(parts) != 3 || cq.Message.Message == nil {
		answer("")
		return
	}
	draftID, err := strconv.ParseInt(parts[2], 10, 64)
	if err != nil {
		answer("")
		return
	}
	msg := cq.Message.Message
	u, enabled, err := b.linkedUser(ctx, cq.From.ID)
	if err != nil || u == nil || !enabled {
		answer("Bot profilingizga ulanmagan")
		return
	}

	switch parts[1] {
	case "fix":
		if err := b.prices.Discard(ctx, u.ID, draftID); err != nil {
			slog.ErrorContext(ctx, "discard draft", "err", err)
		}
		answer("")
		b.edit(ctx, msg, "✏️ Bekor qilindi. Tuzatilgan matnni qayta yuboring, masalan:\n<code>"+html.EscapeString(priceExample)+"</code>", nil)
	case "ok":
		saved, err := b.prices.Confirm(ctx, *u, draftID)
		if errors.Is(err, pricing.ErrDraftGone) {
			answer("Bu kartochka eskirgan yoki allaqachon saqlangan")
			b.edit(ctx, msg, "Bu kartochka eskirgan yoki allaqachon saqlangan. Narxni qayta yuboring.", nil)
			return
		}
		if err != nil {
			slog.ErrorContext(ctx, "confirm draft", "err", err)
			answer("Xatolik yuz berdi, qayta urinib ko'ring")
			return
		}
		answer("Saqlandi ✅")
		var sb strings.Builder
		sb.WriteString("✅ <b>Saqlandi</b>\n\n")
		for _, s := range saved {
			fmt.Fprintf(&sb, "• %s — %s so'm/%s\n", html.EscapeString(s.ProductName), pricing.FormatSom(s.PriceSom), s.Unit)
			sb.WriteString("   " + positionLine(s) + "\n")
		}
		fmt.Fprintf(&sb, "\nNarx %s gacha amal qiladi.", dateUz(saved[0].ValidUntil))
		b.edit(ctx, msg, sb.String(), button("Kabinetni ochish", b.webURL+"/kabinet"))
		for _, s := range saved {
			if text, kb, ok := reasonQuestion(s); ok {
				b.send(ctx, msg.Chat.ID, text, kb)
			}
		}
	default:
		answer("")
	}
}

func (b *Bot) edit(ctx context.Context, msg *models.Message, text string, markup models.ReplyMarkup) {
	_, err := b.api.EditMessageText(ctx, &tg.EditMessageTextParams{
		ChatID: msg.Chat.ID, MessageID: msg.ID, Text: text, ParseMode: models.ParseModeHTML, ReplyMarkup: markup,
	})
	if err != nil {
		slog.WarnContext(ctx, "telegram edit", "err", err)
	}
}
