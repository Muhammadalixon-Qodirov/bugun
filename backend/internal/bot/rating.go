package bot

import (
	"context"
	"errors"
	"fmt"
	"html"
	"log/slog"
	"regexp"
	"strconv"
	"strings"
	"time"

	tg "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"bugun/internal/deals"
	"bugun/internal/store/db"
)

// Ratings in the bot (QIZIQISH_VA_SAVDO.md step 9). Three days after a deal
// is confirmed the buyer is asked:
//
//	k:<deal>:<stars>  1–5 stars; then a comment may be typed as a reply to
//	                  the "Baho #N" prompt

var ratingPrompt = regexp.MustCompile(`Baho #(\d+)`)

// RatingPrompts is the scheduler's job.
func (b *Bot) RatingPrompts(ctx context.Context) (int, error) {
	ids, err := b.deals.TakeToRate(ctx)
	if err != nil {
		return 0, err
	}
	sent := 0
	for _, id := range ids {
		v, err := b.q.RatingPromptView(ctx, id)
		if err != nil {
			slog.WarnContext(ctx, "rating prompt view", "err", err)
			continue
		}
		b.send(ctx, v.TelegramID, ratingPromptText(v), ratingButtons(v.ID))
		sent++
		time.Sleep(sendPause)
	}
	return sent, nil
}

func ratingPromptText(v db.RatingPromptViewRow) string {
	return fmt.Sprintf("⭐ <b>%s</b> bo'yicha %s (%s) bilan savdongiz qanday o'tdi?\n\nBaho boshqa xaridorlarga ishonchli sotuvchini tanlashga yordam beradi.",
		html.EscapeString(v.ProductName), html.EscapeString(v.SellerName), html.EscapeString(v.RegionName))
}

func ratingButtons(dealID int64) *models.InlineKeyboardMarkup {
	row := make([]models.InlineKeyboardButton, 5)
	for i := range row {
		row[i] = models.InlineKeyboardButton{Text: strconv.Itoa(i+1) + "⭐", CallbackData: fmt.Sprintf("k:%d:%d", dealID, i+1)}
	}
	return &models.InlineKeyboardMarkup{InlineKeyboard: [][]models.InlineKeyboardButton{row}}
}

func (b *Bot) ratingCallback(ctx context.Context, _ *tg.Bot, upd *models.Update) {
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
	if len(parts) != 3 {
		answer("")
		return
	}
	dealID, err1 := strconv.ParseInt(parts[1], 10, 64)
	stars, err2 := strconv.Atoi(parts[2])
	if err1 != nil || err2 != nil {
		answer("")
		return
	}
	if _, err := b.deals.Rate(ctx, *u, dealID, stars); err != nil {
		if errors.Is(err, deals.ErrNotRatable) || errors.Is(err, deals.ErrInvalid) {
			answer("Bu savdoni baholab bo'lmaydi.")
			return
		}
		slog.ErrorContext(ctx, "rate deal", "err", err)
		answer("Xatolik yuz berdi, qayta urinib ko'ring")
		return
	}
	answer("Rahmat!")
	b.edit(ctx, msg, "Rahmat! Bahoingiz: "+strings.Repeat("⭐", stars), nil)
	_, err = b.api.SendMessage(ctx, &tg.SendMessageParams{
		ChatID:      msg.Chat.ID,
		Text:        fmt.Sprintf("Baho #%d: izoh qoldirmoqchi bo'lsangiz, shu xabarga javob yozing (ixtiyoriy, 300 belgigacha).", dealID),
		ReplyMarkup: &models.ForceReply{ForceReply: true, InputFieldPlaceholder: "Izoh"},
	})
	if err != nil {
		slog.WarnContext(ctx, "telegram rating comment prompt", "err", err)
	}
}

// ratingReply handles a reply to a "Baho #N" prompt; false when the message
// is not such a reply.
func (b *Bot) ratingReply(ctx context.Context, msg *models.Message, u db.User) bool {
	if msg.ReplyToMessage == nil {
		return false
	}
	m := ratingPrompt.FindStringSubmatch(msg.ReplyToMessage.Text)
	if m == nil {
		return false
	}
	dealID, _ := strconv.ParseInt(m[1], 10, 64)
	r, err := b.deals.Comment(ctx, u, dealID, msg.Text)
	switch {
	case errors.Is(err, deals.ErrInvalid):
		b.send(ctx, msg.Chat.ID, "Izoh 300 belgidan oshmasin.", nil)
	case errors.Is(err, deals.ErrNotRatable):
		b.send(ctx, msg.Chat.ID, "Avval yulduzcha bilan baholang.", nil)
	case err != nil:
		b.fail(ctx, msg.Chat.ID, "rating comment", err)
	case r.Status == "held":
		b.send(ctx, msg.Chat.ID, "Rahmat! Izohingiz tekshiruvdan keyin ko'rinadi.", nil)
	default:
		b.send(ctx, msg.Chat.ID, "Rahmat! Izohingiz saqlandi.", nil)
	}
	return true
}
