package bot

import (
	"context"
	"errors"
	"fmt"
	"html"
	"log/slog"
	"strconv"
	"strings"

	tg "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"bugun/internal/channels"
	"bugun/internal/pricing"
	"bugun/internal/store/db"
)

// Connected channels and groups (KANAL_ULASH.md). The bot reads prices from
// chats whose owner added it and never writes into a group or channel:
//
//	c:prov:<id>          choose the viloyat of a chat
//	c:setp:<id>:<prov>   set it
//	c:upd:<id>:<1|0>     posts update the owner's listings, or stop
//	c:off:<id>           disconnect (the bot leaves the chat)

// reportPosts: the owner hears about the first posts only, then it is quiet.
const reportPosts = 3

// allowedUpdates are the update types the bot asks Telegram for.
var allowedUpdates = tg.AllowedUpdates{
	models.AllowedUpdateMessage, models.AllowedUpdateEditedMessage, models.AllowedUpdateCallbackQuery,
	models.AllowedUpdateChannelPost, models.AllowedUpdateEditedChannelPost, models.AllowedUpdateMyChatMember,
}

// chatGuard routes everything that is not a private chat: joins and leaves,
// channel posts and group messages. Nothing outside private chats reaches the
// normal handlers, so the bot never answers in a group.
func (b *Bot) chatGuard(next tg.HandlerFunc) tg.HandlerFunc {
	return func(ctx context.Context, api *tg.Bot, upd *models.Update) {
		switch {
		case upd.MyChatMember != nil:
			b.chatMember(ctx, upd.MyChatMember)
		case upd.ChannelPost != nil:
			b.chatPost(ctx, upd.ChannelPost, false)
		case upd.EditedChannelPost != nil:
			b.chatPost(ctx, upd.EditedChannelPost, true)
		case upd.Message != nil && upd.Message.Chat.Type != models.ChatTypePrivate:
			b.chatPost(ctx, upd.Message, false)
		case upd.EditedMessage != nil && upd.EditedMessage.Chat.Type != models.ChatTypePrivate:
			b.chatPost(ctx, upd.EditedMessage, true)
		case upd.EditedMessage != nil:
			// Edits in private chats are not re-read.
		case upd.CallbackQuery != nil && upd.CallbackQuery.Message.Message != nil &&
			upd.CallbackQuery.Message.Message.Chat.Type != models.ChatTypePrivate:
			// The bot posts no buttons outside private chats.
		default:
			next(ctx, api, upd)
		}
	}
}

func chatKind(t models.ChatType) (string, bool) {
	switch t {
	case models.ChatTypeChannel:
		return channels.KindChannel, true
	case models.ChatTypeGroup:
		return channels.KindGroup, true
	case models.ChatTypeSupergroup:
		return channels.KindSupergroup, true
	}
	return "", false
}

// chatMember handles the bot being added to or removed from a chat.
func (b *Bot) chatMember(ctx context.Context, m *models.ChatMemberUpdated) {
	kind, ok := chatKind(m.Chat.Type)
	if !ok {
		return
	}
	switch m.NewChatMember.Type {
	case models.ChatMemberTypeLeft, models.ChatMemberTypeBanned:
		if err := b.channels.Removed(ctx, m.Chat.ID); err != nil {
			slog.ErrorContext(ctx, "channel removed", "err", err)
		}
		return
	case models.ChatMemberTypeAdministrator, models.ChatMemberTypeMember:
	default:
		return
	}
	owner, _, err := b.linkedUser(ctx, m.From.ID)
	if err != nil {
		slog.ErrorContext(ctx, "channel owner", "err", err)
		return
	}
	ch, err := b.channels.Connected(ctx, channels.Chat{ID: m.Chat.ID, Kind: kind, Title: m.Chat.Title, Username: m.Chat.Username}, m.From.ID, owner)
	if err != nil {
		slog.ErrorContext(ctx, "channel connected", "err", err)
		return
	}
	b.send(ctx, m.From.ID, connectedText(ch, owner, m.NewChatMember.Type), nil)
}

func connectedText(ch db.TelegramChannel, owner *db.User, as models.ChatMemberType) string {
	what := "kanal"
	if ch.Kind != channels.KindChannel {
		what = "guruh"
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "📡 «%s» %si ulandi.\n\n", html.EscapeString(ch.Title), what)
	switch {
	case owner == nil:
		sb.WriteString("Postlardagi narxlarni olish uchun avval saytga Telegram orqali kiring, keyin /kanallar dan viloyatni tanlang.")
	case ch.Status != "active":
		sb.WriteString("Narxlar qaysi viloyat uchun olinishini tanlang: /kanallar")
	default:
		sb.WriteString("Endi bu yerdagi postlardagi narxlar platformaga oʻzi tushadi. Sozlash: /kanallar")
	}
	if ch.Kind != channels.KindChannel {
		sb.WriteString("\n\nBot guruhda hech narsa yozmaydi, faqat narxlarni oʻqiydi.")
		if as != models.ChatMemberTypeAdministrator {
			sb.WriteString(" Barcha xabarlarni oʻqishi uchun botni guruhda admin qiling, aks holda faqat @bot tilga olingan xabarlarni koʻradi.")
		}
	}
	return sb.String()
}

// chatPost reads prices from a channel post or group message. It never
// answers in the chat; the owner hears about the first posts privately.
func (b *Bot) chatPost(ctx context.Context, msg *models.Message, edited bool) {
	text := msg.Text
	if text == "" {
		text = msg.Caption
	}
	if strings.TrimSpace(text) == "" {
		return
	}
	p := channels.Post{ChatID: msg.Chat.ID, MessageID: int64(msg.ID), Text: text, Edited: edited}
	if msg.From != nil && msg.Chat.Type != models.ChatTypeChannel {
		p.SenderTg = msg.From.ID
	}
	res, ok, err := b.channels.Ingest(ctx, p)
	if err != nil {
		slog.ErrorContext(ctx, "channel post", "err", err)
		return
	}
	if !ok || edited || len(res.Taken) == 0 || res.PostsSeen > reportPosts || !res.Channel.OwnerID.Valid {
		return
	}
	b.sendTo(ctx, res.Channel.OwnerID.Bytes, postReport(res), nil)
}

func postReport(r channels.Ingested) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "📡 «%s»: %d ta narx olindi\n", html.EscapeString(r.Channel.Title), len(r.Taken))
	for _, it := range r.Taken {
		fmt.Fprintf(&sb, "\n• %s — %s soʻm/%s", html.EscapeString(it.ProductName), pricing.FormatSom(it.PriceSom), it.Unit)
	}
	if r.ByModel {
		sb.WriteString("\n\n🤖 Post odatiy shaklda emas edi: narxlarni sunʼiy intellekt oʻqidi, har bir raqam postda borligi tekshirildi. Aniqroq boʻlishi uchun «mahsulot narx» koʻrinishida yozing, masalan «pomidor 6000».")
	}
	if r.Held > 0 {
		fmt.Fprintf(&sb, "\n\n⏳ %d tasi bozor narxidan juda farq qiladi, tekshiruvdan soʻng hisobga olinadi.", r.Held)
	}
	if n := len(r.Problems); n > 0 {
		fmt.Fprintf(&sb, "\n\nTushunilmadi: «%s»", html.EscapeString(r.Problems[0].Raw))
		if n > 1 {
			fmt.Fprintf(&sb, " va yana %d qator", n-1)
		}
	}
	if r.Listings > 0 {
		fmt.Fprintf(&sb, "\n\n🔄 Eʼlonlaringiz ham yangilandi (%d).", r.Listings)
	}
	fmt.Fprintf(&sb, "\n\nBu hisobot faqat dastlabki %d ta post uchun keladi.", reportPosts)
	return sb.String()
}

// kanallar answers /kanallar: the chats this person connected.
func (b *Bot) kanallar(ctx context.Context, _ *tg.Bot, upd *models.Update) {
	msg := upd.Message
	u, ok := b.requireUser(ctx, msg.Chat.ID, msg.From.ID)
	if !ok {
		return
	}
	text, kb, err := b.channelsList(ctx, *u)
	if err != nil {
		b.fail(ctx, msg.Chat.ID, "kanallar", err)
		return
	}
	b.send(ctx, msg.Chat.ID, text, kb)
}

func (b *Bot) channelsList(ctx context.Context, u db.User) (string, models.ReplyMarkup, error) {
	rows, err := b.channels.Mine(ctx, u)
	if err != nil {
		return "", nil, err
	}
	if len(rows) == 0 {
		return "Sizda ulangan kanal yoki guruh yoʻq.\n\nUlash uchun botni kanalingizga admin qilib qoʻshing (hech qanday huquq bermasa ham boʻladi) yoki savdo guruhingizga qoʻshing. Shundan keyin u yerga yozilgan narxlar platformaga oʻzi tushadi.", nil, nil
	}
	var sb strings.Builder
	sb.WriteString("📡 <b>Ulangan kanal va guruhlar</b>\n")
	kb := &models.InlineKeyboardMarkup{}
	for _, c := range rows {
		what := "kanal"
		if c.Kind != channels.KindChannel {
			what = "guruh"
		}
		fmt.Fprintf(&sb, "\n• <b>%s</b> (%s)", html.EscapeString(c.Title), what)
		if c.Status != "active" {
			sb.WriteString(" — ⏳ viloyat tanlanmagan")
		} else {
			fmt.Fprintf(&sb, " — %s · %d post, %d narx", html.EscapeString(c.ProvinceName), c.PostsSeen, c.PricesTaken)
			if c.Kind == channels.KindChannel {
				state := "oʻchiq"
				if c.UpdateListings {
					state = "yoqilgan"
				}
				fmt.Fprintf(&sb, " · eʼlonlarni yangilash: %s", state)
			}
		}
		id := strconv.FormatInt(c.ID, 10)
		row := []models.InlineKeyboardButton{{Text: "🗺 Viloyat", CallbackData: "c:prov:" + id}}
		if c.Kind == channels.KindChannel && c.Status == "active" {
			if c.UpdateListings {
				row = append(row, models.InlineKeyboardButton{Text: "🔄 Yangilashni oʻchirish", CallbackData: "c:upd:" + id + ":0"})
			} else {
				row = append(row, models.InlineKeyboardButton{Text: "🔄 Eʼlonlarni yangilash", CallbackData: "c:upd:" + id + ":1"})
			}
		}
		row = append(row, models.InlineKeyboardButton{Text: "❌ Uzish", CallbackData: "c:off:" + id})
		kb.InlineKeyboard = append(kb.InlineKeyboard, row)
	}
	sb.WriteString("\n\n«Eʼlonlarni yangilash» yoqilsa, kanalingizdagi postlar sizning saytdagi narxlaringizni ham oʻzgartiradi.")
	return sb.String(), kb, nil
}

func (b *Bot) channelCallback(ctx context.Context, _ *tg.Bot, upd *models.Update) {
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
	if len(parts) < 3 {
		answer("")
		return
	}
	id, err := strconv.ParseInt(parts[2], 10, 64)
	if err != nil {
		answer("")
		return
	}
	switch {
	case parts[1] == "prov":
		provinces, err := b.q.ListProvinces(ctx)
		if err != nil {
			answer("Xatolik yuz berdi")
			return
		}
		kb := &models.InlineKeyboardMarkup{}
		for i := 0; i < len(provinces); i += 2 {
			var row []models.InlineKeyboardButton
			for _, p := range provinces[i:min(i+2, len(provinces))] {
				row = append(row, models.InlineKeyboardButton{Text: p.Name, CallbackData: fmt.Sprintf("c:setp:%d:%d", id, p.ID)})
			}
			kb.InlineKeyboard = append(kb.InlineKeyboard, row)
		}
		answer("")
		b.edit(ctx, msg, "Narxlar qaysi viloyat uchun olinsin?", kb)
		return
	case parts[1] == "setp" && len(parts) == 4:
		prov, err := strconv.ParseInt(parts[3], 10, 16)
		if err != nil {
			answer("")
			return
		}
		_, err = b.channels.SetProvince(ctx, *u, id, int16(prov))
		if !b.channelErr(ctx, answer, err) {
			return
		}
		answer("Saqlandi ✅")
	case parts[1] == "upd" && len(parts) == 4:
		_, err := b.channels.SetUpdateListings(ctx, *u, id, parts[3] == "1")
		if !b.channelErr(ctx, answer, err) {
			return
		}
		answer("Saqlandi ✅")
	case parts[1] == "off":
		chat, err := b.channels.Disconnect(ctx, *u, id)
		if !b.channelErr(ctx, answer, err) {
			return
		}
		if _, err := b.api.LeaveChat(ctx, &tg.LeaveChatParams{ChatID: chat}); err != nil {
			slog.WarnContext(ctx, "leave chat", "err", err)
		}
		answer("Uzildi")
	default:
		answer("")
		return
	}
	text, kb, err := b.channelsList(ctx, *u)
	if err != nil {
		slog.ErrorContext(ctx, "channels list", "err", err)
		return
	}
	b.edit(ctx, msg, text, kb)
}

// channelErr answers a failed change; true when there was no error.
func (b *Bot) channelErr(ctx context.Context, answer func(string), err error) bool {
	switch {
	case err == nil:
		return true
	case errors.Is(err, channels.ErrNotFound):
		answer("Kanal topilmadi yoki uzilgan.")
	default:
		slog.ErrorContext(ctx, "channel setting", "err", err)
		answer("Xatolik yuz berdi, qayta urinib ko'ring")
	}
	return false
}
