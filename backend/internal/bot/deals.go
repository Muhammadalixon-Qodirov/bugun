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
	"bugun/internal/market"
	"bugun/internal/pricing"
	"bugun/internal/store/db"
)

// Deal follow-ups (QIZIQISH_VA_SAVDO.md step 3). A day after a buyer opens a
// seller's contact the bot asks whether they bought:
//
//	d:yes:<prompt>  bought: a deal is recorded, then amount and price are asked
//	d:talk:<prompt> still talking: asked once more in 48 hours
//	d:no:<prompt>   did not buy
//	d:skip:<deal>   do not want to give amount and price
//
// Amount and price are typed as a reply to the "Savdo #N" prompt.

var dealPrompt = regexp.MustCompile(`Savdo #(\d+)`)

// SendDealPrompts is the scheduler's job: it sends the questions that are due.
func (b *Bot) SendDealPrompts(ctx context.Context) (int, error) {
	ids, err := b.deals.TakeDue(ctx, time.Now())
	if err != nil {
		return 0, err
	}
	sent := 0
	for _, id := range ids {
		v, err := b.q.DealPromptView(ctx, id)
		if err != nil {
			slog.WarnContext(ctx, "deal prompt view", "err", err)
			continue
		}
		if !v.BotEnabled {
			continue
		}
		b.send(ctx, v.TelegramID, dealPromptText(v), dealPromptButtons(v.ID, v.Asks))
		sent++
		time.Sleep(sendPause)
	}
	return sent, nil
}

func dealPromptText(v db.DealPromptViewRow) string {
	name := v.ProductName
	if v.Variety != "" {
		name += " (" + v.Variety + ")"
	}
	return fmt.Sprintf("🤝 %s kuni <b>%s</b> bo'yicha %s (%s) bilan bog'landingiz.\n<b>Kelishdingizmi?</b>\n\n"+
		"Javobingiz narxlar statistikasini aniqroq qiladi. Sotuvchiga ismingiz ko'rsatilmaydi.",
		dateUz(v.CreatedAt), html.EscapeString(name), html.EscapeString(v.SellerName), html.EscapeString(v.RegionName))
}

// dealPromptButtons: "still talking" is offered only the first time.
func dealPromptButtons(id int64, asks int16) *models.InlineKeyboardMarkup {
	s := strconv.FormatInt(id, 10)
	rows := [][]models.InlineKeyboardButton{{{Text: "✅ Ha, oldim", CallbackData: "d:yes:" + s}}}
	if asks < deals.MaxAsks {
		rows = append(rows, []models.InlineKeyboardButton{{Text: "⏳ Hali gaplashyapmiz", CallbackData: "d:talk:" + s}})
	}
	rows = append(rows, []models.InlineKeyboardButton{{Text: "❌ Yo'q", CallbackData: "d:no:" + s}})
	return &models.InlineKeyboardMarkup{InlineKeyboard: rows}
}

// dealCallback handles every d: button.
func (b *Bot) dealCallback(ctx context.Context, _ *tg.Bot, upd *models.Update) {
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
	id, err := strconv.ParseInt(parts[2], 10, 64)
	if err != nil {
		answer("")
		return
	}
	if parts[1] == "skip" {
		answer("")
		b.edit(ctx, msg, "✅ Rahmat! Savdo yozib qo'yildi.", nil)
		return
	}
	choice := map[string]string{"yes": deals.AnswerYes, "talk": deals.AnswerTalking, "no": deals.AnswerNo}[parts[1]]
	deal, err := b.deals.Answer(ctx, *u, id, choice)
	switch {
	case errors.Is(err, deals.ErrAnswered):
		answer("Bu savolga allaqachon javob bergansiz.")
		if _, err := b.api.EditMessageReplyMarkup(ctx, &tg.EditMessageReplyMarkupParams{
			ChatID: msg.Chat.ID, MessageID: msg.ID, ReplyMarkup: &models.InlineKeyboardMarkup{InlineKeyboard: [][]models.InlineKeyboardButton{}},
		}); err != nil {
			slog.WarnContext(ctx, "telegram edit markup", "err", err)
		}
		return
	case errors.Is(err, deals.ErrNotFound):
		answer("Savol topilmadi.")
		return
	case err != nil:
		slog.ErrorContext(ctx, "deal answer", "err", err)
		answer("Xatolik yuz berdi, qayta urinib ko'ring")
		return
	}
	answer("")
	switch choice {
	case deals.AnswerYes:
		b.edit(ctx, msg, "✅ Rahmat! Savdo yozib qo'yildi.", &models.InlineKeyboardMarkup{InlineKeyboard: [][]models.InlineKeyboardButton{{
			{Text: "⏭ Miqdor va narxni aytmayman", CallbackData: fmt.Sprintf("d:skip:%d", deal.ID)},
		}}})
		b.promptDealDetails(ctx, msg.Chat.ID, deal.ID, deal.Unit)
	case deals.AnswerTalking:
		b.edit(ctx, msg, "⏳ Tushunarli. 2 kundan keyin yana bir marta so'rayman.", nil)
	case deals.AnswerNo:
		b.edit(ctx, msg, "Tushunarli, rahmat! Boshqa sotuvchilarni /savol yoki saytda topishingiz mumkin.", nil)
	}
}

// promptDealDetails asks for amount and price as a reply to this message.
func (b *Bot) promptDealDetails(ctx context.Context, chatID, dealID int64, unit string) {
	_, err := b.api.SendMessage(ctx, &tg.SendMessageParams{
		ChatID: chatID,
		Text: fmt.Sprintf("Savdo #%d: qancha va 1 %s necha so'mdan? Javob sifatida yozing, masalan: 200 5800\n"+
			"Narxni bilmasangiz, faqat miqdorni yozing.", dealID, unit),
		ReplyMarkup: &models.ForceReply{ForceReply: true, InputFieldPlaceholder: "200 5800"},
	})
	if err != nil {
		slog.WarnContext(ctx, "telegram deal details prompt", "err", err)
	}
}

// dealDetailsReply handles a reply to a "Savdo #N" prompt; false when the
// message is not such a reply.
func (b *Bot) dealDetailsReply(ctx context.Context, msg *models.Message, u db.User) bool {
	if msg.ReplyToMessage == nil {
		return false
	}
	m := dealPrompt.FindStringSubmatch(msg.ReplyToMessage.Text)
	if m == nil {
		return false
	}
	dealID, _ := strconv.ParseInt(m[1], 10, 64)
	qty, unit, price, ok := parseDealDetails(msg.Text)
	d, err := b.q.GetDeal(context.WithoutCancel(ctx), dealID)
	if err == nil && ok && unit != "" && !sameUnit(unit, d.Unit) {
		ok = false
	}
	if err != nil || !ok {
		b.send(ctx, msg.Chat.ID, "Tushunmadim. Miqdorni e'londagi birlikda va narxni raqam bilan yozing, masalan: 200 5800", nil)
		if err == nil {
			b.promptDealDetails(ctx, msg.Chat.ID, dealID, d.Unit)
		}
		return true
	}
	if _, err := b.deals.SetDetails(ctx, u, dealID, &qty, price); err != nil {
		text := "Xatolik yuz berdi, qayta urinib ko'ring."
		switch {
		case errors.Is(err, deals.ErrInvalid):
			text = "Miqdor yoki narx noto'g'ri ko'rinadi. Masalan: 200 5800"
		case errors.Is(err, deals.ErrNotFound):
			text = "Savdo topilmadi."
		case errors.Is(err, deals.ErrClosed):
			text = "Bu savdoni endi o'zgartirib bo'lmaydi."
		default:
			slog.ErrorContext(ctx, "deal details", "err", err)
		}
		b.send(ctx, msg.Chat.ID, "❗ "+text, nil)
		return true
	}
	line := fmt.Sprintf("✅ Yozildi: %s %s", formatQty(qty), d.Unit)
	if price != nil {
		line += fmt.Sprintf(", %s so'mdan", pricing.FormatSom(*price))
	}
	b.send(ctx, msg.Chat.ID, line+". Rahmat!", nil)
	return true
}

var dealDetailsRe = regexp.MustCompile(`^\s*(\d+(?:[.,]\d+)?)\s*([^\d\s]*)\s*(.*?)\s*$`)

// parseDealDetails reads "200 5800", "200 kg 5 800" or "2,5 tonna": an
// amount, an optional unit word and an optional price.
func parseDealDetails(text string) (qty float64, unit string, price *int64, ok bool) {
	m := dealDetailsRe.FindStringSubmatch(strings.ToLower(text))
	if m == nil {
		return 0, "", nil, false
	}
	qty, err := strconv.ParseFloat(strings.Replace(m[1], ",", ".", 1), 64)
	if err != nil || qty <= 0 {
		return 0, "", nil, false
	}
	if m[3] != "" {
		p, ok := parseSom(m[3])
		if !ok {
			return 0, "", nil, false
		}
		price = &p
	}
	return qty, m[2], price, true
}

// sameUnit accepts a typed unit that names the listing's unit ("kg", "kilo").
func sameUnit(typed, unit string) bool {
	typed, unit = strings.Trim(typed, ".,"), strings.ToLower(unit)
	return typed == unit || strings.HasPrefix(typed, unit) || strings.HasPrefix(unit, typed) ||
		(unit == "kg" && strings.HasPrefix(typed, "kilo"))
}

// Sellers' evening question (QIZIQISH_VA_SAVDO.md step 4): at 19:00 a seller
// whose contact was opened today is asked whether they sold:
//
//	s:sold:<listing>  sold one: confirms a buyer's report or waits for one
//	s:none            no sale today

const digestListings = 8

// SellerDealDigest is the scheduler's evening job.
func (b *Bot) SellerDealDigest(ctx context.Context) (int, error) {
	rows, err := b.q.TodaysContactsBySeller(ctx)
	if err != nil {
		return 0, err
	}
	sent := 0
	for i := 0; i < len(rows); {
		j := i
		for j < len(rows) && rows[j].SellerID == rows[i].SellerID {
			j++
		}
		text, kb := sellerDigest(rows[i:j])
		b.send(ctx, rows[i].TelegramID, text, kb)
		sent++
		time.Sleep(sendPause)
		i = j
	}
	return sent, nil
}

func sellerDigest(rows []db.TodaysContactsBySellerRow) (string, *models.InlineKeyboardMarkup) {
	var sb strings.Builder
	sb.WriteString("📞 <b>Bugun raqamingizni ochishdi:</b>\n")
	kb := &models.InlineKeyboardMarkup{}
	for i, r := range rows {
		if i == digestListings {
			fmt.Fprintf(&sb, "\n… yana %d ta e'lon", len(rows)-i)
			break
		}
		name := r.ProductName
		if r.Variety != "" {
			name += " (" + r.Variety + ")"
		}
		fmt.Fprintf(&sb, "\n• %s — %d kishi", html.EscapeString(name), r.Contacts)
		kb.InlineKeyboard = append(kb.InlineKeyboard, []models.InlineKeyboardButton{{
			Text: "✅ " + name + ": sotdim", CallbackData: fmt.Sprintf("s:sold:%d", r.ListingID),
		}})
	}
	sb.WriteString("\n\n<b>Sotuv bo'ldimi?</b> Har savdo uchun bir marta bosing. Javobingiz faqat statistika uchun.")
	kb.InlineKeyboard = append(kb.InlineKeyboard, []models.InlineKeyboardButton{{Text: "Bugun sotuv bo'lmadi", CallbackData: "s:none"}})
	return sb.String(), kb
}

// sellerDealCallback handles the s: buttons.
func (b *Bot) sellerDealCallback(ctx context.Context, _ *tg.Bot, upd *models.Update) {
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
	if cq.Data == "s:none" {
		answer("")
		b.edit(ctx, msg, "Tushunarli, rahmat! Ertaga omad.", nil)
		return
	}
	listingID, err := strconv.ParseInt(strings.TrimPrefix(cq.Data, "s:sold:"), 10, 64)
	if err != nil || !strings.HasPrefix(cq.Data, "s:sold:") {
		answer("")
		return
	}
	deal, matched, err := b.deals.SellerSold(ctx, *u, listingID)
	switch {
	case errors.Is(err, deals.ErrNotFound):
		answer("E'lon topilmadi.")
		return
	case err != nil:
		slog.ErrorContext(ctx, "seller sold", "err", err)
		answer("Xatolik yuz berdi, qayta urinib ko'ring")
		return
	}
	if matched {
		answer("✅ Savdo tasdiqlandi — xaridor ham aytgan edi.")
	} else {
		answer("✅ Yozib qo'yildi.")
	}
	b.dropButton(ctx, msg, cq.Data)
	if !deal.PriceSom.Valid {
		b.promptDealDetails(ctx, msg.Chat.ID, deal.ID, deal.Unit)
	}
}

// The seller's "sold" button under /mening (QIZIQISH_VA_SAVDO.md step 5):
//
//	x:sold:<listing>  asks how much was sold, as a reply to "Sotildi #N"

var soldPrompt = regexp.MustCompile(`Sotildi #(\d+)`)

func (b *Bot) soldCallback(ctx context.Context, _ *tg.Bot, upd *models.Update) {
	cq := upd.CallbackQuery
	_, _ = b.api.AnswerCallbackQuery(ctx, &tg.AnswerCallbackQueryParams{CallbackQueryID: cq.ID})
	msg := cq.Message.Message
	listingID, err := strconv.ParseInt(strings.TrimPrefix(cq.Data, "x:sold:"), 10, 64)
	if err != nil || msg == nil {
		return
	}
	b.promptSold(ctx, msg.Chat.ID, listingID)
}

func (b *Bot) promptSold(ctx context.Context, chatID, listingID int64) {
	_, err := b.api.SendMessage(ctx, &tg.SendMessageParams{
		ChatID:      chatID,
		Text:        fmt.Sprintf("Sotildi #%d: qancha sotdingiz (e'londagi birlikda) va necha so'mdan? Masalan: 200 5800\nNarxni aytmasangiz ham bo'ladi.", listingID),
		ReplyMarkup: &models.ForceReply{ForceReply: true, InputFieldPlaceholder: "200 5800"},
	})
	if err != nil {
		slog.WarnContext(ctx, "telegram sold prompt", "err", err)
	}
}

// soldReply handles a reply to a "Sotildi #N" prompt; false when the message
// is not such a reply.
func (b *Bot) soldReply(ctx context.Context, msg *models.Message, u db.User) bool {
	if msg.ReplyToMessage == nil {
		return false
	}
	m := soldPrompt.FindStringSubmatch(msg.ReplyToMessage.Text)
	if m == nil {
		return false
	}
	listingID, _ := strconv.ParseInt(m[1], 10, 64)
	qty, unit, price, ok := parseDealDetails(msg.Text)
	l, err := b.q.GetListing(ctx, listingID)
	if err != nil || l.SellerID != u.ID {
		b.send(ctx, msg.Chat.ID, "E'lon topilmadi.", nil)
		return true
	}
	if !ok || (unit != "" && !sameUnit(unit, l.Unit)) {
		b.send(ctx, msg.Chat.ID, fmt.Sprintf("Tushunmadim. Miqdorni %s da va narxni raqam bilan yozing, masalan: 200 5800", l.Unit), nil)
		b.promptSold(ctx, msg.Chat.ID, listingID)
		return true
	}
	sold, err := b.listings.MarkSold(ctx, u, listingID, qty, price)
	var ve *market.ValidationError
	switch {
	case errors.As(err, &ve):
		b.send(ctx, msg.Chat.ID, "❗ "+ve.Message, nil)
		return true
	case err != nil:
		b.fail(ctx, msg.Chat.ID, "sold", err)
		return true
	}
	text := fmt.Sprintf("✅ Yozildi: %s %s sotildi.", formatQty(qty), l.Unit)
	if sold.Matched {
		text += " Xaridor ham aytgan edi — savdo tasdiqlandi."
	}
	switch {
	case sold.SoldOut:
		text += "\n📦 Zaxira tugadi, e'lon yashirildi. Yana bo'lsa, yangi narx va miqdorni yozing, masalan: «pomidor 6000, 2 tonna»."
	case sold.Listing.Stock.Valid:
		text += "\nQoldi: " + formatQty(*market.Float(sold.Listing.Stock)) + " " + l.Unit + "."
	}
	b.send(ctx, msg.Chat.ID, text, nil)
	return true
}
