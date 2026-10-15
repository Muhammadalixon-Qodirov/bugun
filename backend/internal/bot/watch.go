package bot

import (
	"context"
	"errors"
	"fmt"
	"html"
	"log/slog"
	"math"
	"strconv"
	"strings"
	"time"

	tg "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"bugun/internal/pricing"
	"bugun/internal/watch"
)

// Watches in the bot (QIZIQISH_VA_SAVDO.md step 8):
//
//	w:add:<product>:<province>:<trade>  follow (from a /savol answer)
//	w:del:<watch>                       stop following (from /kuzatuv)

func watchButton(productID, province int16, trade string) models.InlineKeyboardButton {
	return models.InlineKeyboardButton{Text: "🔔 Kuzatish", CallbackData: fmt.Sprintf("w:add:%d:%d:%s", productID, province, trade)}
}

// kuzatuv answers /kuzatuv: the products the user follows.
func (b *Bot) kuzatuv(ctx context.Context, _ *tg.Bot, upd *models.Update) {
	msg := upd.Message
	u, ok := b.requireUser(ctx, msg.Chat.ID, msg.From.ID)
	if !ok {
		return
	}
	rows, err := b.watches.List(ctx, *u)
	if err != nil {
		b.fail(ctx, msg.Chat.ID, "kuzatuv", err)
		return
	}
	if len(rows) == 0 {
		b.send(ctx, msg.Chat.ID, "Siz hali hech narsani kuzatmayapsiz.\n\nNarxni so'rang (masalan <code>Samarqandda piyoz qancha?</code>) va javob ostidagi «🔔 Kuzatish» tugmasini bosing: narx tushsa, xabar beraman.", nil)
		return
	}
	var sb strings.Builder
	sb.WriteString("🔔 <b>Kuzatayotganlaringiz</b>\n")
	kb := &models.InlineKeyboardMarkup{}
	for _, r := range rows {
		where := r.ProvinceName
		if where == "" {
			where = "butun respublika"
		}
		line := fmt.Sprintf("\n• <b>%s</b> — %s", html.EscapeString(r.ProductName), html.EscapeString(where))
		if r.MaxPrice.Valid {
			line += fmt.Sprintf(", %s so'mdan arzon", pricing.FormatSom(r.MaxPrice.Int64))
		}
		sb.WriteString(line)
		kb.InlineKeyboard = append(kb.InlineKeyboard, []models.InlineKeyboardButton{{
			Text: "❌ " + r.ProductName + " (" + where + ")", CallbackData: fmt.Sprintf("w:del:%d", r.ID),
		}})
	}
	sb.WriteString("\n\nYangi arzon narx chiqsa yoki median 5% dan ko'p tushsa, kuniga ko'pi bilan bir marta xabar beraman.")
	b.send(ctx, msg.Chat.ID, sb.String(), kb)
}

func (b *Bot) watchCallback(ctx context.Context, _ *tg.Bot, upd *models.Update) {
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
	parts := strings.Split(cq.Data, ":")
	switch {
	case len(parts) == 5 && parts[1] == "add":
		pid, err1 := strconv.ParseInt(parts[2], 10, 16)
		prov, err2 := strconv.ParseInt(parts[3], 10, 16)
		if err1 != nil || err2 != nil || (parts[4] != "ulgurji" && parts[4] != "chakana") {
			answer("")
			return
		}
		_, err := b.watches.Follow(ctx, *u, int16(pid), int16(prov), parts[4])
		switch {
		case errors.Is(err, watch.ErrTooMany):
			answer("Ko'pi bilan 20 ta mahsulotni kuzatish mumkin. /kuzatuv dan keraksizini o'chiring.")
		case err != nil:
			slog.ErrorContext(ctx, "add watch", "err", err)
			answer("Xatolik yuz berdi, qayta urinib ko'ring")
		default:
			answer("🔔 Kuzatuvga qo'shildi. Narx tushsa, xabar beraman.")
			b.dropButton(ctx, msg, cq.Data)
		}
	case len(parts) == 3 && parts[1] == "del":
		id, err := strconv.ParseInt(parts[2], 10, 64)
		if err != nil {
			answer("")
			return
		}
		if err := b.watches.Remove(ctx, *u, id); err != nil && !errors.Is(err, watch.ErrNotFound) {
			slog.ErrorContext(ctx, "remove watch", "err", err)
			answer("Xatolik yuz berdi, qayta urinib ko'ring")
			return
		}
		answer("O'chirildi")
		b.dropButton(ctx, msg, cq.Data)
	default:
		answer("")
	}
}

// WatchAlerts is the scheduler's job: it tells followers about cheaper prices.
func (b *Bot) WatchAlerts(ctx context.Context) (int, error) {
	alerts, err := b.watches.Check(ctx, time.Now())
	for _, a := range alerts {
		b.send(ctx, a.Watch.TelegramID, watchAlertText(a),
			button("Ko'rish", fmt.Sprintf("%s/mahsulot/%s?region=%d", b.webURL, a.Watch.ProductSlug, a.Watch.ProvinceID.Int16)))
		time.Sleep(sendPause)
	}
	return len(alerts), err
}

func watchAlertText(a watch.Alert) string {
	w := a.Watch
	where := w.ProvinceName
	if where == "" {
		where = "respublika bo'yicha"
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "🔔 <b>%s</b> (%s)", html.EscapeString(w.ProductName), html.EscapeString(where))
	if a.Cheaper {
		fmt.Fprintf(&sb, "\nYangi narx: <b>%s so'm/%s</b> — %s.", pricing.FormatSom(a.Price), w.Unit, html.EscapeString(a.Region))
	}
	if a.Dropped {
		fmt.Fprintf(&sb, "\nMedian %s dan %s so'mga tushdi (%s).",
			pricing.FormatSom(int64(math.Round(a.Peak))), pricing.FormatSom(int64(math.Round(a.Median))), percent(a.Median/a.Peak-1))
	}
	sb.WriteString("\n\nKuzatuvni o'chirish: /kuzatuv")
	return sb.String()
}
