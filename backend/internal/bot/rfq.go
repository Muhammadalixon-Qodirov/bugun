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

	tg "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"bugun/internal/market"
	"bugun/internal/pricing"
	"bugun/internal/rfq"
	"bugun/internal/store/db"
)

// RFQ in the bot (spec §8.4). Buttons carry everything they need in their
// callback data, so nothing is kept in memory between messages:
//
//	r:new:<product>:<qty>  buyer sends the request     r:none:<rfq>  seller has none
//	r:quote:<rfq>          seller wants to quote       o:pick:<offer> buyer picks
//	o:yes:<offer> / o:no:<offer>  seller agrees to share contacts or refuses
//
// A quote itself is typed as a reply to the bot's "So'rov #N" prompt.

var quotePrompt = regexp.MustCompile(`So'rov #(\d+)`)

// sorov answers /sorov: "/sorov kartoshka 200 kg" or instructions.
func (b *Bot) sorov(ctx context.Context, _ *tg.Bot, upd *models.Update) {
	msg := upd.Message
	u, ok := b.requireUser(ctx, msg.Chat.ID, msg.From.ID)
	if !ok {
		return
	}
	if !rfq.CanBuy(*u) {
		b.sendNeedEntrepreneur(ctx, msg.Chat.ID)
		return
	}
	text := strings.TrimSpace(strings.TrimPrefix(msg.Text, "/sorov"))
	if text == "" {
		b.send(ctx, msg.Chat.ID, "Nima kerakligini yozing, masalan:\n<code>kartoshka 200 kg</code>\n\nViloyatingizdagi shu mahsulot sotuvchilari narx taklif qiladi.", nil)
		return
	}
	b.rfqDraft(ctx, msg.Chat.ID, text)
}

// rfqDraft turns a buyer's text into a request card with a send button.
func (b *Bot) rfqDraft(ctx context.Context, chatID int64, text string) {
	d, problem := b.prices.ParseDemand(text)
	switch {
	case problem != nil:
		b.send(ctx, chatID, "❗ "+html.EscapeString(problem.Message), nil)
		return
	case d.Quantity == nil:
		b.send(ctx, chatID, fmt.Sprintf("Qancha %s kerak? Masalan: <code>%s 200 kg</code>", strings.ToLower(html.EscapeString(d.ProductName)), strings.ToLower(html.EscapeString(d.ProductName))), nil)
		return
	}
	qty := strconv.FormatFloat(*d.Quantity, 'f', -1, 64)
	card := fmt.Sprintf("🛒 <b>So'rov</b>: %s — %s %s\nMuddat: %d kun. Viloyatingizdagi sotuvchilar narx taklif qiladi.\n\nYuborilsinmi?",
		html.EscapeString(d.ProductName), formatQty(*d.Quantity), d.Unit, b.rfq.Limits().DefaultDays)
	b.send(ctx, chatID, card, &models.InlineKeyboardMarkup{InlineKeyboard: [][]models.InlineKeyboardButton{{
		{Text: "✅ Yuborish", CallbackData: fmt.Sprintf("r:new:%d:%s", d.ProductID, qty)},
		{Text: "❌ Bekor", CallbackData: "r:cancel"},
	}}})
}

// rfqCallback handles every r: and o: button.
func (b *Bot) rfqCallback(ctx context.Context, _ *tg.Bot, upd *models.Update) {
	cq := upd.CallbackQuery
	answer := func(text string) {
		_, _ = b.api.AnswerCallbackQuery(ctx, &tg.AnswerCallbackQueryParams{CallbackQueryID: cq.ID, Text: text})
	}
	msg := cq.Message.Message
	if msg == nil {
		answer("")
		return
	}
	u, enabled, err := b.linkedUser(ctx, cq.From.ID)
	if err != nil || u == nil || !enabled {
		answer("Bot profilingizga ulanmagan")
		return
	}
	parts := strings.Split(cq.Data, ":")
	if len(parts) < 2 {
		answer("")
		return
	}
	idAt := func(i int) int64 {
		if len(parts) <= i {
			return 0
		}
		n, _ := strconv.ParseInt(parts[i], 10, 64)
		return n
	}

	switch parts[0] + ":" + parts[1] {
	case "r:cancel":
		answer("")
		b.edit(ctx, msg, "So'rov bekor qilindi.", nil)
	case "r:new":
		qty, _ := strconv.ParseFloat(strings.Join(parts[3:], ":"), 64)
		c, err := b.rfq.Create(ctx, *u, rfq.Input{ProductID: int16(idAt(2)), Quantity: qty})
		if err != nil {
			answer(rfqErrorText(err))
			return
		}
		answer("Yuborildi ✅")
		text := fmt.Sprintf("✅ <b>So'rov #%d yuborildi</b> — %d ta sotuvchiga.\nTakliflar shu yerga keladi, eng arzoni saytda birinchi turadi.", c.RFQ.ID, c.Sellers)
		if c.Sellers == 0 {
			text = fmt.Sprintf("✅ <b>So'rov #%d ochildi.</b>\nHozircha viloyatingizda bu mahsulotning faol sotuvchisi yo'q; saytdagi narxlarni ham ko'ring.", c.RFQ.ID)
		}
		b.edit(ctx, msg, text, button("So'rovni ochish", b.webURL+"/sorovlar"))
	case "r:quote":
		answer("")
		b.promptQuote(ctx, msg.Chat.ID, idAt(2))
	case "r:none":
		if _, err := b.rfq.Offer(ctx, *u, idAt(2), 0, nil); err != nil {
			answer(rfqErrorText(err))
			return
		}
		answer("Belgilandi")
		b.edit(ctx, msg, html.EscapeString(msg.Text)+"\n\n— Siz: menda yo'q", nil)
	case "o:pick":
		if _, err := b.rfq.Choose(ctx, *u, idAt(2)); err != nil {
			answer(rfqErrorText(err))
			return
		}
		answer("Tanlandi ✅")
		b.edit(ctx, msg, html.EscapeString(msg.Text)+"\n\n✅ Tanlandi. Sotuvchi rozi bo'lsa, kontakt shu yerga keladi.", nil)
	case "o:yes", "o:no":
		agree := parts[1] == "yes"
		if _, err := b.rfq.Answer(ctx, *u, idAt(2), agree); err != nil {
			answer(rfqErrorText(err))
			return
		}
		answer("")
		reply := "\n\n❌ Rad etildi."
		if agree {
			reply = "\n\n🤝 Rozilik berildi. Xaridor kontakti quyida."
		}
		b.edit(ctx, msg, html.EscapeString(msg.Text)+reply, nil)
	default:
		answer("")
	}
}

// promptQuote asks the seller to type a price as a reply to this message.
func (b *Bot) promptQuote(ctx context.Context, chatID, rfqID int64) {
	_, err := b.api.SendMessage(ctx, &tg.SendMessageParams{
		ChatID:      chatID,
		Text:        fmt.Sprintf("So'rov #%d uchun narxingizni javob sifatida yozing (1 birlik uchun, so'm). Masalan: 3200", rfqID),
		ReplyMarkup: &models.ForceReply{ForceReply: true, InputFieldPlaceholder: "3200"},
	})
	if err != nil {
		slog.WarnContext(ctx, "telegram quote prompt", "err", err)
	}
}

// quoteReply handles a seller's reply to a "So'rov #N" prompt; false when the
// message is not such a reply.
func (b *Bot) quoteReply(ctx context.Context, msg *models.Message, u db.User) bool {
	if msg.ReplyToMessage == nil {
		return false
	}
	m := quotePrompt.FindStringSubmatch(msg.ReplyToMessage.Text)
	if m == nil {
		return false
	}
	rfqID, _ := strconv.ParseInt(m[1], 10, 64)
	price, ok := parseSom(msg.Text)
	if !ok {
		b.send(ctx, msg.Chat.ID, "Narxni raqam bilan yozing, masalan: 3200", nil)
		b.promptQuote(ctx, msg.Chat.ID, rfqID)
		return true
	}
	o, err := b.rfq.Offer(ctx, u, rfqID, price, nil)
	if err != nil {
		b.send(ctx, msg.Chat.ID, "❗ "+rfqErrorText(err), nil)
		return true
	}
	b.send(ctx, msg.Chat.ID, fmt.Sprintf("✅ So'rov #%d ga taklifingiz yuborildi: %s so'm. Xaridor tanlasa, sizga xabar keladi.", rfqID, pricing.FormatSom(o.PriceSom.Int64)), nil)
	return true
}

// parseSom reads a typed price: "3200", "3 200", "3.200", "3,2 ming".
func parseSom(text string) (int64, bool) {
	t := strings.ToLower(strings.TrimSpace(text))
	mult := 1.0
	for _, w := range []string{"ming", "минг", "тысяч"} {
		if strings.Contains(t, w) {
			mult = 1000
			t = strings.ReplaceAll(t, w, "")
		}
	}
	t = strings.NewReplacer("so'm", "", "сум", "", "som", "", " ", "").Replace(t)
	if mult == 1000 {
		t = strings.ReplaceAll(t, ",", ".")
	} else {
		t = strings.NewReplacer(".", "", ",", "").Replace(t)
	}
	f, err := strconv.ParseFloat(t, 64)
	if err != nil || f <= 0 {
		return 0, false
	}
	return int64(f * mult), true
}

func rfqErrorText(err error) string {
	var ve *rfq.ValidationError
	switch {
	case errors.As(err, &ve):
		return ve.Message
	case errors.Is(err, rfq.ErrTooManyOpen):
		return fmt.Sprintf("Bir vaqtda ko'pi bilan %d ta ochiq so'rov bo'lishi mumkin.", rfq.OpenLimit(err))
	case errors.Is(err, rfq.ErrClosed):
		return "Bu so'rov yopilgan."
	case errors.Is(err, rfq.ErrNotEligible):
		return "Javob berish uchun viloyatda shu mahsulotdan faol e'loningiz bo'lishi kerak."
	case errors.Is(err, rfq.ErrOfferLocked):
		return "Xaridor taklifingizni tanlagan, endi o'zgartirib bo'lmaydi."
	case errors.Is(err, rfq.ErrBadState):
		return "Bu amal allaqachon bajarilgan."
	case errors.Is(err, rfq.ErrNotBuyer), errors.Is(err, rfq.ErrNotSeller):
		return needEntrepreneur
	case errors.Is(err, rfq.ErrNotFound):
		return "So'rov topilmadi."
	}
	slog.Error("bot rfq", "err", err)
	return "Xatolik yuz berdi, qayta urinib ko'ring."
}

// requireUser loads the linked profile or tells the person how to link it.
func (b *Bot) requireUser(ctx context.Context, chatID, tgID int64) (*db.User, bool) {
	u, enabled, err := b.linkedUser(ctx, tgID)
	if err != nil {
		b.fail(ctx, chatID, "user", err)
		return nil, false
	}
	if u == nil || !enabled {
		b.notLinked(ctx, chatID)
		return nil, false
	}
	return u, true
}

// --- rfq.Notifier: messages about RFQ events, sent in the background ---

func (b *Bot) RFQCreated(ctx context.Context, r db.Rfq, sellers []uuid.UUID) {
	go func() {
		p, err := b.q.GetProduct(ctx, r.ProductID)
		if err != nil {
			slog.WarnContext(ctx, "rfq notify", "err", err)
			return
		}
		reg, _ := b.q.GetRegion(ctx, r.RegionID)
		text := fmt.Sprintf("🛒 <b>Yangi so'rov #%d</b>\n%s — %s %s\n📍 %s · %sgacha",
			r.ID, html.EscapeString(p.Name), formatQty(floatOf(r.Quantity)), r.Unit, html.EscapeString(reg.Name), dateUz(r.Deadline))
		if r.MaxPriceSom.Valid {
			text += fmt.Sprintf("\n💰 Xaridor narxi: %s so'mgacha", pricing.FormatSom(r.MaxPriceSom.Int64))
		}
		if r.Note.Valid {
			text += "\n📝 " + html.EscapeString(r.Note.String)
		}
		kb := &models.InlineKeyboardMarkup{InlineKeyboard: [][]models.InlineKeyboardButton{{
			{Text: "💰 Narx taklif qilish", CallbackData: fmt.Sprintf("r:quote:%d", r.ID)},
			{Text: "Menda yo'q", CallbackData: fmt.Sprintf("r:none:%d", r.ID)},
		}}}
		for _, id := range sellers {
			b.sendTo(ctx, id, text, kb)
		}
	}()
}

func (b *Bot) OfferMade(ctx context.Context, r db.Rfq, o db.Offer) {
	go func() {
		p, _ := b.q.GetProduct(ctx, r.ProductID)
		seller, err := b.q.GetUser(ctx, o.SellerID)
		if err != nil {
			slog.WarnContext(ctx, "rfq notify", "err", err)
			return
		}
		text := fmt.Sprintf("💰 <b>So'rov #%d</b> ga taklif:\n%s — %s so'm/%s\nSotuvchi: %s",
			r.ID, html.EscapeString(p.Name), pricing.FormatSom(o.PriceSom.Int64), r.Unit, html.EscapeString(publicName(seller)))
		b.sendTo(ctx, r.BuyerID, text, &models.InlineKeyboardMarkup{InlineKeyboard: [][]models.InlineKeyboardButton{
			{{Text: "✅ Tanlash", CallbackData: fmt.Sprintf("o:pick:%d", o.ID)}},
			{{Text: "Barcha takliflar", URL: fmt.Sprintf("%s/sorovlar/%d", b.webURL, r.ID)}},
		}})
	}()
}

func (b *Bot) OfferChosen(ctx context.Context, r db.Rfq, o db.Offer) {
	go func() {
		p, _ := b.q.GetProduct(ctx, r.ProductID)
		text := fmt.Sprintf("✅ Xaridor <b>So'rov #%d</b> bo'yicha taklifingizni tanladi:\n%s — %s so'm/%s, %s %s\n\nKontakt almashishga rozimisiz?",
			r.ID, html.EscapeString(p.Name), pricing.FormatSom(o.PriceSom.Int64), r.Unit, formatQty(floatOf(r.Quantity)), r.Unit)
		b.sendTo(ctx, o.SellerID, text, &models.InlineKeyboardMarkup{InlineKeyboard: [][]models.InlineKeyboardButton{{
			{Text: "🤝 Roziman", CallbackData: fmt.Sprintf("o:yes:%d", o.ID)},
			{Text: "Yo'q", CallbackData: fmt.Sprintf("o:no:%d", o.ID)},
		}}})
	}()
}

func (b *Bot) OfferAnswered(ctx context.Context, r db.Rfq, o db.Offer) {
	go func() {
		if o.Status != rfq.StatusConfirmed {
			b.sendTo(ctx, r.BuyerID, fmt.Sprintf("Sotuvchi So'rov #%d bo'yicha bu safar rad etdi. Boshqa takliflarni ko'ring.", r.ID),
				button("Takliflar", fmt.Sprintf("%s/sorovlar/%d", b.webURL, r.ID)))
			return
		}
		buyer, err1 := b.q.GetUser(ctx, r.BuyerID)
		seller, err2 := b.q.GetUser(ctx, o.SellerID)
		if err := errors.Join(err1, err2); err != nil {
			slog.WarnContext(ctx, "rfq notify", "err", err)
			return
		}
		site := button("Telefon va batafsil", fmt.Sprintf("%s/sorovlar/%d", b.webURL, r.ID))
		b.sendTo(ctx, r.BuyerID, fmt.Sprintf("🤝 Sotuvchi rozi bo'ldi (So'rov #%d).\n%s", r.ID, contactLine(seller)), site)
		b.sendTo(ctx, o.SellerID, fmt.Sprintf("🤝 So'rov #%d — xaridor bilan bog'laning.\n%s", r.ID, contactLine(buyer)), button("Batafsil", b.webURL+"/kabinet"))
	}()
}

// contactLine shows the person's Telegram when they allow it; phone numbers
// stay on the site, where they are decrypted.
func contactLine(u db.User) string {
	name := html.EscapeString(publicName(u))
	if u.ContactTelegram && u.TelegramUsername.Valid {
		return fmt.Sprintf("%s: @%s", name, html.EscapeString(u.TelegramUsername.String))
	}
	return name + ": aloqa ma'lumoti saytda"
}

func publicName(u db.User) string {
	if u.PublicName.Valid && u.PublicName.String != "" {
		return u.PublicName.String
	}
	return strings.SplitN(u.DisplayName, " ", 2)[0]
}

func floatOf(n pgtype.Numeric) float64 {
	if f := market.Float(n); f != nil {
		return *f
	}
	return 0
}

// sendTo messages a user by profile id, if their bot is on.
func (b *Bot) sendTo(ctx context.Context, userID uuid.UUID, text string, markup models.ReplyMarkup) {
	u, err := b.q.GetUser(ctx, userID)
	if err != nil {
		slog.WarnContext(ctx, "bot send to", "err", err)
		return
	}
	if !u.BotEnabled {
		return
	}
	b.send(ctx, u.TelegramID, text, markup)
}

var _ rfq.Notifier = (*Bot)(nil)
