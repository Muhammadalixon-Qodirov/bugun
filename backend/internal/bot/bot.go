// Package bot runs the Telegram bot: a quick remote for the web profile.
// It has no sign-up of its own; users are recognised by the Telegram ID that
// the website login stored (spec §8).
package bot

import (
	"context"
	"errors"
	"fmt"
	"html"
	"log/slog"
	"strings"

	tg "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/jackc/pgx/v5"

	"bugun/internal/ask"
	"bugun/internal/channels"
	"bugun/internal/deals"
	"bugun/internal/market"
	"bugun/internal/pricing"
	"bugun/internal/rfq"
	"bugun/internal/roles"
	"bugun/internal/store/db"
	"bugun/internal/watch"
)

type Bot struct {
	api      *tg.Bot
	q        *db.Queries
	prices   *pricing.Service
	listings *market.Service
	rfq      *rfq.Service
	deals    *deals.Service
	watches  *watch.Service
	channels *channels.Service
	ask      *ask.Service
	webURL   string

	ai     AI // optional model features (ai.go)
	limit  *throttle
	notice func(ctx context.Context, upd *models.Update, text string) // tests: replaces tooFast's reply
}

var commands = []models.BotCommand{
	{Command: "narx", Description: "Yangi narx yuborish"},
	{Command: "mening", Description: "Faol narxlarim va bozordagi o'rnim"},
	{Command: "sorov", Description: "Yangi so'rov — RFQ (sotib olish)"},
	{Command: "savol", Description: "Narx haqida savol"},
	{Command: "kuzatuv", Description: "Kuzatayotgan mahsulotlarim (narx tushsa xabar)"},
	{Command: "kanallar", Description: "Ulangan kanal va guruhlarim (narxlar o'zi olinadi)"},
	{Command: "elon", Description: "Yangi e'lon qo'shish, rasmlar bilan"},
	{Command: "profil", Description: "Saytdagi profilni ochish"},
	{Command: "uzish", Description: "Botni profildan uzish"},
	{Command: "yordam", Description: "Qisqa yo'riqnoma"},
}

// New connects the bot. perMinute caps the messages and button presses one
// user may send a minute (0 = DefaultMessagesPerMinute).
func New(token string, q *db.Queries, prices *pricing.Service, listings *market.Service, rfqs *rfq.Service, dealsSvc *deals.Service, watches *watch.Service, chans *channels.Service, asker *ask.Service, webURL string, perMinute int) (*Bot, error) {
	b := &Bot{q: q, prices: prices, listings: listings, rfq: rfqs, deals: dealsSvc, watches: watches, channels: chans, ask: asker, webURL: strings.TrimRight(webURL, "/"),
		limit: newThrottle(perMinute)}
	api, err := tg.New(token, tg.WithDefaultHandler(b.fallback), tg.WithMiddlewares(b.chatGuard, b.throttled), tg.WithAllowedUpdates(allowedUpdates), tg.WithErrorsHandler(func(err error) {
		slog.Warn("telegram bot", "err", err)
	}))
	if err != nil {
		return nil, fmt.Errorf("telegram bot: %w", err)
	}
	b.api = api
	api.RegisterHandler(tg.HandlerTypeMessageText, "start", tg.MatchTypeCommandStartOnly, b.start)
	api.RegisterHandler(tg.HandlerTypeMessageText, "yordam", tg.MatchTypeCommandStartOnly, b.help)
	api.RegisterHandler(tg.HandlerTypeMessageText, "profil", tg.MatchTypeCommandStartOnly, b.profile)
	api.RegisterHandler(tg.HandlerTypeMessageText, "uzish", tg.MatchTypeCommandStartOnly, b.disconnect)
	api.RegisterHandler(tg.HandlerTypeMessageText, "narx", tg.MatchTypeCommandStartOnly, b.priceHelp)
	api.RegisterHandler(tg.HandlerTypeCallbackQueryData, "p:", tg.MatchTypePrefix, b.priceCallback)
	api.RegisterHandler(tg.HandlerTypeMessageText, "mening", tg.MatchTypeCommandStartOnly, b.mine)
	api.RegisterHandler(tg.HandlerTypeMessageText, "sorov", tg.MatchTypeCommandStartOnly, b.sorov)
	api.RegisterHandler(tg.HandlerTypeCallbackQueryData, "r:", tg.MatchTypePrefix, b.rfqCallback)
	api.RegisterHandler(tg.HandlerTypeCallbackQueryData, "o:", tg.MatchTypePrefix, b.rfqCallback)
	api.RegisterHandler(tg.HandlerTypeCallbackQueryData, "m:", tg.MatchTypePrefix, b.reminderCallback)
	api.RegisterHandler(tg.HandlerTypeCallbackQueryData, "a:", tg.MatchTypePrefix, b.autoPriceCallback)
	api.RegisterHandler(tg.HandlerTypeCallbackQueryData, "d:", tg.MatchTypePrefix, b.dealCallback)
	api.RegisterHandler(tg.HandlerTypeCallbackQueryData, "s:", tg.MatchTypePrefix, b.sellerDealCallback)
	api.RegisterHandler(tg.HandlerTypeCallbackQueryData, "x:", tg.MatchTypePrefix, b.soldCallback)
	api.RegisterHandler(tg.HandlerTypeCallbackQueryData, "w:", tg.MatchTypePrefix, b.watchCallback)
	api.RegisterHandler(tg.HandlerTypeCallbackQueryData, "k:", tg.MatchTypePrefix, b.ratingCallback)
	api.RegisterHandler(tg.HandlerTypeCallbackQueryData, "ch:", tg.MatchTypePrefix, b.chatCallback)
	api.RegisterHandler(tg.HandlerTypeCallbackQueryData, "n:", tg.MatchTypePrefix, b.reasonCallback)
	api.RegisterHandler(tg.HandlerTypeCallbackQueryData, "c:", tg.MatchTypePrefix, b.channelCallback)
	api.RegisterHandler(tg.HandlerTypeMessageText, "elon", tg.MatchTypeCommandStartOnly, b.newListing)
	api.RegisterHandler(tg.HandlerTypeMessageText, "kanallar", tg.MatchTypeCommandStartOnly, b.kanallar)
	api.RegisterHandler(tg.HandlerTypeMessageText, "kuzatuv", tg.MatchTypeCommandStartOnly, b.kuzatuv)
	api.RegisterHandler(tg.HandlerTypeMessageText, "savol", tg.MatchTypeCommandStartOnly, b.savol)
	return b, nil
}

// Run long-polls Telegram until ctx is cancelled.
func (b *Bot) Run(ctx context.Context) {
	// A webhook left from another deployment would block polling.
	if _, err := b.api.DeleteWebhook(ctx, &tg.DeleteWebhookParams{}); err != nil {
		slog.Warn("telegram delete webhook", "err", err)
	}
	if _, err := b.api.SetMyCommands(ctx, &tg.SetMyCommandsParams{Commands: commands}); err != nil {
		slog.Warn("telegram set commands", "err", err)
	}
	slog.Info("telegram bot polling")
	b.api.Start(ctx)
}

// linkedUser returns the profile bound to this Telegram account, or nil.
// Profiles that disconnected the bot are treated as not linked.
func (b *Bot) linkedUser(ctx context.Context, tgID int64) (*db.User, bool, error) {
	u, err := b.q.GetUserByTelegramID(ctx, tgID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return &u, u.BotEnabled, nil
}

func (b *Bot) start(ctx context.Context, _ *tg.Bot, upd *models.Update) {
	msg := upd.Message
	u, enabled, err := b.linkedUser(ctx, msg.From.ID)
	if err != nil {
		b.fail(ctx, msg.Chat.ID, "start", err)
		return
	}
	switch {
	case u == nil:
		b.send(ctx, msg.Chat.ID,
			"Assalomu alaykum! Bu <b>Bugun</b> boti — O'zbekiston bozorlaridagi real narxlar.\n\n"+
				"Bot saytdagi profilingiz bilan ishlaydi. Saytga Telegram orqali kiring — bot avtomatik ulanadi, qayta ro'yxatdan o'tish shart emas.",
			button("Saytga kirish", b.webURL+"/kirish"))
	case !enabled:
		b.send(ctx, msg.Chat.ID,
			"Bot profilingizdan uzilgan. Qayta ulash uchun saytdagi profil sahifasida «Botni qayta ulash» tugmasini bosing.",
			button("Profilni ochish", b.webURL+"/profil"))
	case !u.Role.Valid:
		b.send(ctx, msg.Chat.ID,
			fmt.Sprintf("Salom, %s! Profilingiz ulangan ✅\n\nRo'yxatdan o'tishni yakunlash uchun saytda rol va bozoringizni tanlang.", html.EscapeString(u.DisplayName)),
			button("Yakunlash", b.webURL+"/boshlash"))
	default:
		b.send(ctx, msg.Chat.ID,
			fmt.Sprintf("Salom, %s! Profilingiz ulangan ✅\n\n%s", html.EscapeString(u.DisplayName), roleHint(u.Role.String)),
			button("Kabinetni ochish", b.webURL+"/profil"))
	}
}

func roleHint(role string) string {
	if roles.IsEntrepreneur(role) {
		return "Narx yuborish uchun shunchaki yozing: «kartoshka 4500, 2 tonna». Yo'riqnoma: /narx\n" +
			"Sotib olmoqchi bo'lsangiz, /sorov yuboring — mos sotuvchilar sizga narx taklif qiladi."
	}
	return "Narxlar haqida savol berish uchun /savol buyrug'idan foydalaning."
}

// needEntrepreneur is the answer to a selling or buying action from someone
// who may not do it: market staff, or an entrepreneur without a region.
const needEntrepreneur = "Buning uchun profilda «Tadbirkor» rolini tanlang va hududingizni kiriting."

func (b *Bot) sendNeedEntrepreneur(ctx context.Context, chatID int64) {
	b.send(ctx, chatID, needEntrepreneur, button("Profilni ochish", b.webURL+"/profil"))
}

func (b *Bot) help(ctx context.Context, _ *tg.Bot, upd *models.Update) {
	var sb strings.Builder
	sb.WriteString("<b>Buyruqlar</b>\n")
	for _, c := range commands {
		fmt.Fprintf(&sb, "/%s — %s\n", c.Command, html.EscapeString(c.Description))
	}
	b.send(ctx, upd.Message.Chat.ID, sb.String(), nil)
}

func (b *Bot) profile(ctx context.Context, _ *tg.Bot, upd *models.Update) {
	msg := upd.Message
	u, enabled, err := b.linkedUser(ctx, msg.From.ID)
	if err != nil {
		b.fail(ctx, msg.Chat.ID, "profil", err)
		return
	}
	if u == nil || !enabled {
		b.notLinked(ctx, msg.Chat.ID)
		return
	}
	b.send(ctx, msg.Chat.ID, "Saytdagi profilingiz:", button("Profilni ochish", b.webURL+"/profil"))
}

func (b *Bot) disconnect(ctx context.Context, _ *tg.Bot, upd *models.Update) {
	msg := upd.Message
	u, enabled, err := b.linkedUser(ctx, msg.From.ID)
	if err != nil {
		b.fail(ctx, msg.Chat.ID, "uzish", err)
		return
	}
	if u == nil || !enabled {
		b.notLinked(ctx, msg.Chat.ID)
		return
	}
	if err := b.q.SetBotEnabled(ctx, db.SetBotEnabledParams{ID: u.ID, BotEnabled: false}); err != nil {
		b.fail(ctx, msg.Chat.ID, "uzish", err)
		return
	}
	slog.InfoContext(ctx, "bot disconnected", "user_id", u.ID)
	b.send(ctx, msg.Chat.ID, "Bot profilingizdan uzildi. Endi bot hech qanday amal bajarmaydi. Qayta ulash — saytdagi profil sahifasida.", nil)
}

// fallback answers anything that is not a known command.
func (b *Bot) fallback(ctx context.Context, _ *tg.Bot, upd *models.Update) {
	msg := upd.Message
	if msg == nil || msg.From == nil {
		return
	}
	u, enabled, err := b.linkedUser(ctx, msg.From.ID)
	if err != nil {
		b.fail(ctx, msg.Chat.ID, "fallback", err)
		return
	}
	if u == nil || !enabled {
		b.notLinked(ctx, msg.Chat.ID)
		return
	}
	if msg.Text != "" && b.quoteReply(ctx, msg, *u) {
		return
	}
	if msg.Text != "" && b.dealDetailsReply(ctx, msg, *u) {
		return
	}
	if msg.Text != "" && b.soldReply(ctx, msg, *u) {
		return
	}
	if msg.Text != "" && b.ratingReply(ctx, msg, *u) {
		return
	}
	// A short reply right after the assistant asked something back.
	if msg.Text != "" && !strings.HasPrefix(msg.Text, "/") && b.ai.Analyst != nil && len([]rune(msg.Text)) <= 80 &&
		b.ai.Analyst.AwaitingReply(ctx, u.ID) {
		b.analystAnswer(ctx, msg.Chat.ID, u, msg.Text)
		return
	}
	if msg.Text != "" && !strings.HasPrefix(msg.Text, "/") && looksLikeQuestion(msg.Text) {
		msg.Text = "/savol " + msg.Text
		b.savol(ctx, nil, upd)
		return
	}
	// Plain text from an entrepreneur is a price; a request for quotes is
	// always the explicit /sorov.
	if msg.Text != "" && !strings.HasPrefix(msg.Text, "/") && pricing.CanSubmit(*u) {
		b.submitPrice(ctx, msg.Chat.ID, *u, msg.Text)
		return
	}
	if msg.Text == "" && (msg.Voice != nil || msg.Audio != nil) {
		b.voiceMessage(ctx, upd, *u)
		return
	}
	if msg.Text == "" && len(msg.Photo) > 0 {
		b.photoMessage(ctx, upd, *u)
		return
	}
	if msg.Text == "" {
		b.send(ctx, msg.Chat.ID, "Hozircha faqat matnli xabarlarni qabul qilaman. Narxni yozib yuboring.", nil)
		return
	}
	b.send(ctx, msg.Chat.ID, "Bu buyruqni tushunmadim. Mavjud buyruqlar ro'yxati: /yordam", nil)
}

func (b *Bot) notLinked(ctx context.Context, chatID int64) {
	b.send(ctx, chatID, "Bot hali profilingizga ulanmagan. Saytga Telegram orqali kiring — bot avtomatik ulanadi.",
		button("Saytga kirish", b.webURL+"/kirish"))
}

func (b *Bot) fail(ctx context.Context, chatID int64, op string, err error) {
	slog.ErrorContext(ctx, "bot "+op, "err", err)
	b.send(ctx, chatID, "Kechirasiz, xatolik yuz berdi. Birozdan keyin qayta urinib ko'ring.", nil)
}

func (b *Bot) send(ctx context.Context, chatID int64, text string, markup models.ReplyMarkup) {
	_, err := b.api.SendMessage(ctx, &tg.SendMessageParams{
		ChatID:      chatID,
		Text:        text,
		ParseMode:   models.ParseModeHTML,
		ReplyMarkup: markup,
	})
	if err != nil {
		slog.WarnContext(ctx, "telegram send", "err", err)
	}
}

func button(text, url string) models.ReplyMarkup {
	return &models.InlineKeyboardMarkup{InlineKeyboard: [][]models.InlineKeyboardButton{{{Text: text, URL: url}}}}
}
