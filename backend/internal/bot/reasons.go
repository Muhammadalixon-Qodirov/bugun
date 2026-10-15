package bot

import (
	"context"
	"fmt"
	"html"
	"log/slog"
	"math"
	"strconv"
	"strings"

	tg "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"bugun/internal/insight"
	"bugun/internal/pricing"
	"bugun/internal/store/db"
)

// "Nega o'zgardi?" (TAHLIL_REJA.md 3.5): after a price change of 10% or more
// the bot asks why, with ready answers:
//
//	n:<listing>:<up|down>:<change in ‰>:<reason>

// reasonThreshold is the change that makes the bot ask.
const reasonThreshold = 0.10

// reasonQuestion is the question for a saved price, or ok = false.
func reasonQuestion(s pricing.Saved) (string, *models.InlineKeyboardMarkup, bool) {
	if s.PrevPrice <= 0 || s.Status != "active" {
		return "", nil, false
	}
	change := float64(s.PriceSom)/float64(s.PrevPrice) - 1
	if math.Abs(change) < reasonThreshold {
		return "", nil, false
	}
	dir, codes := "up", insight.UpReasons
	if change < 0 {
		dir, codes = "down", insight.DownReasons
	}
	permille := int(math.Round(change * 1000))
	kb := &models.InlineKeyboardMarkup{}
	for i := 0; i < len(codes); i += 2 {
		var row []models.InlineKeyboardButton
		for _, c := range codes[i:min(i+2, len(codes))] {
			row = append(row, models.InlineKeyboardButton{Text: insight.ReasonLabels[c], CallbackData: fmt.Sprintf("n:%d:%s:%d:%s", s.ListingID, dir, permille, c)})
		}
		kb.InlineKeyboard = append(kb.InlineKeyboard, row)
	}
	text := fmt.Sprintf("❓ <b>%s</b> narxini %s dan %s so'mga (%s) o'zgartirdingiz. <b>Nega?</b>\n\nJavobingiz boshqa tadbirkorlarga bozorni tushunishga yordam beradi. Ismingiz ko'rsatilmaydi.",
		html.EscapeString(s.ProductName), pricing.FormatSom(s.PrevPrice), pricing.FormatSom(s.PriceSom), percent(change))
	return text, kb, true
}

func (b *Bot) reasonCallback(ctx context.Context, _ *tg.Bot, upd *models.Update) {
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
	if len(parts) != 5 || (parts[2] != "up" && parts[2] != "down") || insight.ReasonLabels[parts[4]] == "" {
		answer("")
		return
	}
	listing, err1 := strconv.ParseInt(parts[1], 10, 64)
	permille, err2 := strconv.Atoi(parts[3])
	if err1 != nil || err2 != nil {
		answer("")
		return
	}
	if err := b.q.RecordPriceReason(ctx, db.RecordPriceReasonParams{
		Direction: parts[2], Change: float64(permille) / 1000, Reason: parts[4], ListingID: listing, SellerID: u.ID,
	}); err != nil {
		slog.ErrorContext(ctx, "record price reason", "err", err)
		answer("Xatolik yuz berdi, qayta urinib ko'ring")
		return
	}
	answer("Rahmat!")
	b.edit(ctx, msg, "✅ Rahmat! Sabab yozildi: "+html.EscapeString(insight.ReasonLabels[parts[4]])+".", nil)
}

// dearWarning is the /mening line for a live price well above the market.
func dearWarning(p *pricing.MarketPosition) string {
	if p == nil || p.DiffPercent < 15 {
		return ""
	}
	return fmt.Sprintf("   ⚠️ Bozordan %d%% qimmat: xaridorlar arzonroqni tanlashi mumkin\n", p.DiffPercent)
}
