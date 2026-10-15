package bot

import (
	"context"
	"errors"
	"fmt"
	"html"
	"log/slog"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	tg "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/google/uuid"

	"bugun/internal/analytics"
	"bugun/internal/market"
	"bugun/internal/pricing"
)

// Automatic pricing in the bot (DINAMIK_NARX.md): after each run a seller gets
// one message with every price that moved and why, and an undo button per
// change:
//
//	a:undo:<change>  put the old price back (pauses auto pricing for 24 hours)

// PricesChanged tells each seller about their automatic changes.
func (b *Bot) PricesChanged(ctx context.Context, changes []market.AutoChange) {
	bySeller := map[uuid.UUID][]market.AutoChange{}
	var order []uuid.UUID
	for _, c := range changes {
		id := c.Listing.SellerID
		if _, ok := bySeller[id]; !ok {
			order = append(order, id)
		}
		bySeller[id] = append(bySeller[id], c)
	}
	for _, id := range order {
		cs := bySeller[id]
		names := make([]string, len(cs))
		for i, c := range cs {
			names[i] = b.listingName(ctx, c)
		}
		b.sendTo(ctx, id, autoPriceMessage(cs, names), undoButtons(cs, names))
		time.Sleep(sendPause)
	}
}

func (b *Bot) listingName(ctx context.Context, c market.AutoChange) string {
	name := "Mahsulot"
	if p, err := b.q.GetProduct(ctx, c.Listing.ProductID); err == nil {
		name = p.Name
	}
	if c.Listing.Variety != "" {
		name += " (" + c.Listing.Variety + ")"
	}
	return name
}

func autoPriceMessage(cs []market.AutoChange, names []string) string {
	var sb strings.Builder
	sb.WriteString("🤖 <b>Avtomatik narx</b> — oraliqingiz ichida yangilandi:\n")
	for i, c := range cs {
		fmt.Fprintf(&sb, "\n• <b>%s</b>: %s → <b>%s</b> so'm/%s (%s)",
			html.EscapeString(names[i]), pricing.FormatSom(c.Change.OldPrice), pricing.FormatSom(c.Change.NewPrice),
			c.Listing.Unit, percent(float64(c.Change.NewPrice)/float64(c.Change.OldPrice)-1))
		fmt.Fprintf(&sb, "\n  Viloyatda median %s so'm (%d narx)", pricing.FormatSom(int64(math.Round(c.Reasons.Median))), c.Reasons.Peers)
		if why := contribText(c.Reasons.Contrib); why != "" {
			sb.WriteString("\n  Nega: " + why)
		} else if why := factorText(c.Reasons.Factors); why != "" {
			sb.WriteString("; " + why)
		}
	}
	sb.WriteString("\n\nRozimasangiz — «Qaytarish»: eski narx qaytadi va 24 soat avtomatik o'zgarmaydi.")
	return sb.String()
}

func undoButtons(cs []market.AutoChange, names []string) *models.InlineKeyboardMarkup {
	rows := make([][]models.InlineKeyboardButton, 0, len(cs))
	for i, c := range cs {
		rows = append(rows, []models.InlineKeyboardButton{{
			Text:         "↩️ " + names[i] + ": " + pricing.FormatSom(c.Change.OldPrice) + " ga qaytarish",
			CallbackData: "a:undo:" + strconv.FormatInt(c.Change.ID, 10),
		}})
	}
	return &models.InlineKeyboardMarkup{InlineKeyboard: rows}
}

// contribText lists each reason with its exact share of the move, biggest
// first.
func contribText(fs []analytics.Factor) string {
	sorted := append([]analytics.Factor(nil), fs...)
	sort.SliceStable(sorted, func(i, j int) bool { return math.Abs(sorted[i].Share) > math.Abs(sorted[j].Share) })
	var parts []string
	for _, f := range sorted {
		if w := factorWord(f); w != "" {
			parts = append(parts, w+" "+percent(f.Share))
		}
	}
	return strings.Join(parts, ", ")
}

// factorText explains the factors in plain words, biggest first as listed.
func factorText(fs []analytics.Factor) string {
	var parts []string
	for _, f := range fs {
		if w := factorWord(f); w != "" {
			parts = append(parts, w)
		}
	}
	return strings.Join(parts, ", ")
}

func factorWord(f analytics.Factor) string {
	up := f.Share > 0
	switch f.Code {
	case "market":
		return "viloyat medianiga yaqinlashish"
	case "rounding":
		return "yaxlitlash"
	case "goal":
		return map[bool]string{true: "maqsad: foyda", false: "maqsad: tez sotish"}[up]
	case "pressure":
		return map[bool]string{true: "talab odatdagidan yuqori", false: "taklif odatdagidan ko'p"}[up]
	case "trend":
		return map[bool]string{true: "bozor narxi hafta ichida oshdi", false: "bozor narxi hafta ichida tushdi"}[up]
	case "response":
		return "ko'p ko'rilyapti, kam bog'lanishyapti"
	case "rfq":
		return "so'rovlarda boshqa sotuvchilar tanlandi"
	case "sales":
		return map[bool]string{true: "boshqalardan tezroq sotilyapti", false: "bog'lanishlar bor, lekin sotuv yo'q"}[up]
	case "stock":
		return map[bool]string{true: "zaxirangiz kam", false: "zaxirangiz ko'p"}[up]
	case "cap":
		return "chegara: oraliqingiz yoki kunlik 5%"
	}
	return ""
}

// percent formats a share as "+3,6%" (Uzbek decimal comma).
func percent(x float64) string {
	s := strconv.FormatFloat(math.Round(x*1000)/10, 'f', -1, 64)
	if x > 0 {
		s = "+" + s
	}
	return strings.Replace(s, ".", ",", 1) + "%"
}

// autoPriceCallback handles the undo button.
func (b *Bot) autoPriceCallback(ctx context.Context, _ *tg.Bot, upd *models.Update) {
	cq := upd.CallbackQuery
	answer := func(text string) {
		_, _ = b.api.AnswerCallbackQuery(ctx, &tg.AnswerCallbackQueryParams{CallbackQueryID: cq.ID, Text: text, ShowAlert: text != ""})
	}
	msg := cq.Message.Message
	u, enabled, err := b.linkedUser(ctx, cq.From.ID)
	if err != nil || u == nil || !enabled || msg == nil {
		answer("Bot profilingizga ulanmagan")
		return
	}
	id, err := strconv.ParseInt(strings.TrimPrefix(cq.Data, "a:undo:"), 10, 64)
	if err != nil || !strings.HasPrefix(cq.Data, "a:undo:") {
		answer("")
		return
	}
	l, c, err := b.listings.RevertAutoPrice(ctx, *u, id)
	switch {
	case errors.Is(err, market.ErrReverted):
		answer("Bu o'zgarish allaqachon qaytarilgan.")
	case errors.Is(err, market.ErrStale):
		answer("Narx shundan keyin yana o'zgargan. Kerakli narxni saytda yoki shu yerda yozib qo'ying.")
	case errors.Is(err, market.ErrNotFound):
		answer("O'zgarish topilmadi.")
	case err != nil:
		slog.ErrorContext(ctx, "revert auto price", "err", err)
		answer("Xatolik yuz berdi, qayta urinib ko'ring")
		return
	default:
		answer("")
		b.send(ctx, msg.Chat.ID, fmt.Sprintf("↩️ Narx %s so'm/%s ga qaytarildi. 24 soat avtomatik o'zgarmaydi.",
			pricing.FormatSom(c.OldPrice), l.Unit), nil)
	}
	b.dropButton(ctx, msg, cq.Data)
}

// dropButton removes a used button from the message, keeping the others.
func (b *Bot) dropButton(ctx context.Context, msg *models.Message, data string) {
	if msg.ReplyMarkup == nil {
		return
	}
	var rows [][]models.InlineKeyboardButton
	for _, row := range msg.ReplyMarkup.InlineKeyboard {
		var keep []models.InlineKeyboardButton
		for _, btn := range row {
			if btn.CallbackData != data {
				keep = append(keep, btn)
			}
		}
		if len(keep) > 0 {
			rows = append(rows, keep)
		}
	}
	if rows == nil {
		rows = [][]models.InlineKeyboardButton{}
	}
	if _, err := b.api.EditMessageReplyMarkup(ctx, &tg.EditMessageReplyMarkupParams{
		ChatID: msg.Chat.ID, MessageID: msg.ID, ReplyMarkup: &models.InlineKeyboardMarkup{InlineKeyboard: rows},
	}); err != nil {
		slog.WarnContext(ctx, "telegram edit markup", "err", err)
	}
}
