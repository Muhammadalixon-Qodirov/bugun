package bot

import (
	"context"
	"strings"

	tg "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"bugun/internal/market"
)

// newListingPath is the site's new-listing page; inside the Mini App the
// page signs the seller in from Telegram's init data
// (POST /v1/auth/telegram/webapp).
const newListingPath = "/mahsulotlarim/yangi"

// webAppButton opens url inside Telegram as a Mini App. Telegram only
// accepts https Mini Apps, so a local http site gets a plain link instead.
func webAppButton(text, url string) models.ReplyMarkup {
	if !strings.HasPrefix(url, "https://") {
		return button(text, url)
	}
	return &models.InlineKeyboardMarkup{InlineKeyboard: [][]models.InlineKeyboardButton{{{Text: text, WebApp: &models.WebAppInfo{URL: url}}}}}
}

// newListing (/elon) opens the new-listing form as a Mini App, so a seller
// adds a product with photos without leaving Telegram.
func (b *Bot) newListing(ctx context.Context, _ *tg.Bot, upd *models.Update) {
	msg := upd.Message
	u, enabled, err := b.linkedUser(ctx, msg.From.ID)
	if err != nil {
		b.fail(ctx, msg.Chat.ID, "elon", err)
		return
	}
	if u == nil || !enabled {
		b.notLinked(ctx, msg.Chat.ID)
		return
	}
	if !market.CanSell(*u) {
		b.sendNeedEntrepreneur(ctx, msg.Chat.ID)
		return
	}
	b.send(ctx, msg.Chat.ID,
		"Yangi mahsulot qo'shish uchun tugmani bosing: shakl shu yerda, Telegram ichida ochiladi. Narx, miqdor va rasmlarni kiriting.",
		webAppButton("➕ Yangi e'lon", b.webURL+newListingPath))
}
