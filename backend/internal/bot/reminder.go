package bot

import (
	"context"
	"fmt"
	"html"
	"log/slog"
	"strings"
	"time"

	tg "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"bugun/internal/market"
	"bugun/internal/pricing"
)

// Weekly reminder (spec §8.3): every Monday 09:00 Tashkent a seller sees their
// prices and answers with one button, "unchanged" (all renewed for a week)
// or "changed" (how to send new ones).

const reminderLines = 10

// sendPause keeps bulk messages under Telegram's ~30 per second limit.
const sendPause = 40 * time.Millisecond

var reminderButtons = &models.InlineKeyboardMarkup{InlineKeyboard: [][]models.InlineKeyboardButton{{
	{Text: "✅ Ha, o'zgarmadi", CallbackData: "m:ok"},
	{Text: "✏️ O'zgardi", CallbackData: "m:edit"},
}}}

// WeeklyReminder messages every seller with live or recently expired prices.
// It is the scheduler's Monday job and returns how many were sent.
func (b *Bot) WeeklyReminder(ctx context.Context) (int, error) {
	ids, err := b.q.SellersToRemind(ctx)
	if err != nil {
		return 0, err
	}
	sent := 0
	now := time.Now()
	for _, id := range ids {
		rows, err := b.q.ListSellerListings(ctx, id)
		if err != nil {
			slog.WarnContext(ctx, "reminder listings", "err", err)
			continue
		}
		var items []mineItem
		for _, r := range rows {
			if r.Status != market.StatusActive {
				continue
			}
			name := r.ProductName
			if r.Variety != "" {
				name += " (" + r.Variety + ")"
			}
			items = append(items, mineItem{ID: r.ID, Stock: market.Float(r.Stock), Name: name, PriceSom: r.PriceSom, Unit: r.Unit, TradeType: r.TradeType,
				Status: r.Status, Expired: r.ValidUntil.Before(now), ValidUntil: r.ValidUntil})
		}
		if len(items) == 0 {
			continue
		}
		b.sendTo(ctx, id, reminderMessage(items), reminderButtons)
		sent++
		time.Sleep(sendPause)
	}
	return sent, nil
}

func reminderMessage(items []mineItem) string {
	var sb strings.Builder
	sb.WriteString("☀️ <b>Yangi hafta!</b> Narxlaringiz hali shumi?\n")
	for i, it := range items {
		if i == reminderLines {
			fmt.Fprintf(&sb, "\n… yana %d ta", len(items)-i)
			break
		}
		state := "amal qiladi " + dateUz(it.ValidUntil) + "gacha"
		if it.Expired {
			state = "⌛ muddati o'tgan — xaridorlar ko'rmayapti"
		}
		fmt.Fprintf(&sb, "\n• %s — %s so'm/%s%s · %s", html.EscapeString(it.Name), pricing.FormatSom(it.PriceSom), it.Unit, stockNote(it), state)
	}
	sb.WriteString("\n\nO'zgarmagan bo'lsa, bitta tugma — hammasi yana 7 kun ko'rinadi.")
	for _, it := range items {
		if it.Stock != nil {
			sb.WriteString(" Zaxira kamaygan bo'lsa, /mening dagi «Sotildi» tugmasi bilan belgilang.")
			break
		}
	}
	return sb.String()
}

// reminderCallback handles the two reminder buttons.
func (b *Bot) reminderCallback(ctx context.Context, _ *tg.Bot, upd *models.Update) {
	cq := upd.CallbackQuery
	answer := func(text string) {
		_, _ = b.api.AnswerCallbackQuery(ctx, &tg.AnswerCallbackQueryParams{CallbackQueryID: cq.ID, Text: text})
	}
	msg := cq.Message.Message
	u, enabled, err := b.linkedUser(ctx, cq.From.ID)
	if err != nil || u == nil || !enabled || msg == nil {
		answer("Bot profilingizga ulanmagan")
		return
	}
	switch cq.Data {
	case "m:ok":
		n, until, err := b.listings.RenewAll(ctx, *u)
		if err != nil {
			slog.ErrorContext(ctx, "reminder renew", "err", err)
			answer("Xatolik yuz berdi, qayta urinib ko'ring")
			return
		}
		answer("Uzaytirildi ✅")
		b.edit(ctx, msg, fmt.Sprintf("✅ %d ta narx %sgacha uzaytirildi. Rahmat!", n, dateUz(until)), button("Mahsulotlarim", b.webURL+"/mahsulotlarim"))
	case "m:edit":
		answer("")
		b.edit(ctx, msg, "✏️ Yangi narxlarni shu yerga yozing, masalan:\n<code>"+html.EscapeString(priceExample)+"</code>\n\nYoki saytda tahrirlang.", button("Mahsulotlarim", b.webURL+"/mahsulotlarim"))
	default:
		answer("")
	}
}
