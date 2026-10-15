package bot

import (
	"context"
	"fmt"
	"html"
	"strings"
	"time"

	tg "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"bugun/internal/market"
	"bugun/internal/pricing"
	"bugun/internal/store/db"
)

// mineLimit keeps the /mening reply within one readable Telegram message.
const mineLimit = 15

// mineItem is one listing as /mening shows it.
type mineItem struct {
	ID         int64
	Name       string
	PriceSom   int64
	Unit       string
	TradeType  string
	Status     string
	Expired    bool
	ValidUntil time.Time
	Position   *pricing.MarketPosition
	Stock      *float64
}

// mine answers /mening: the seller's listings with their place in the market.
func (b *Bot) mine(ctx context.Context, _ *tg.Bot, upd *models.Update) {
	msg := upd.Message
	u, enabled, err := b.linkedUser(ctx, msg.From.ID)
	if err != nil {
		b.fail(ctx, msg.Chat.ID, "mening", err)
		return
	}
	if u == nil || !enabled {
		b.notLinked(ctx, msg.Chat.ID)
		return
	}
	if !pricing.CanSubmit(*u) {
		b.sendNeedEntrepreneur(ctx, msg.Chat.ID)
		return
	}
	rows, err := b.q.ListSellerListings(ctx, u.ID)
	if err != nil {
		b.fail(ctx, msg.Chat.ID, "mening", err)
		return
	}
	now := time.Now()
	items := make([]mineItem, 0, len(rows))
	for _, r := range rows {
		if r.Status == market.StatusHidden || r.Status == market.StatusRejected {
			continue
		}
		it := mineItem{
			ID: r.ID, Stock: market.Float(r.Stock), Name: r.ProductName, PriceSom: r.PriceSom, Unit: r.Unit, TradeType: r.TradeType,
			Status: r.Status, Expired: r.ValidUntil.Before(now), ValidUntil: r.ValidUntil,
		}
		if r.Variety != "" {
			it.Name += " (" + r.Variety + ")"
		}
		if r.Status == market.StatusActive && !it.Expired {
			pos, err := market.PositionOf(ctx, b.q, db.Listing{
				ID: r.ID, Status: r.Status, RegionID: r.RegionID, ProductID: r.ProductID,
				TradeType: r.TradeType, PriceSom: r.PriceSom,
			})
			if err != nil {
				b.fail(ctx, msg.Chat.ID, "mening", err)
				return
			}
			it.Position = pricing.NewMarketPosition(pos)
		}
		items = append(items, it)
	}
	b.send(ctx, msg.Chat.ID, mineMessage(items), mineButtons(items, b.webURL+"/mahsulotlarim"))
}

// soldButtons caps the "sold" buttons under /mening.
const soldButtons = 8

// mineButtons: a "sold" button for each live listing, then the site link.
func mineButtons(items []mineItem, siteURL string) *models.InlineKeyboardMarkup {
	kb := &models.InlineKeyboardMarkup{}
	for _, it := range items {
		if it.Status != market.StatusActive || it.Expired || len(kb.InlineKeyboard) == soldButtons {
			continue
		}
		kb.InlineKeyboard = append(kb.InlineKeyboard, []models.InlineKeyboardButton{{
			Text: "💰 " + it.Name + ": sotildi", CallbackData: fmt.Sprintf("x:sold:%d", it.ID),
		}})
	}
	kb.InlineKeyboard = append(kb.InlineKeyboard, []models.InlineKeyboardButton{{Text: "Mahsulotlarim", URL: siteURL}})
	return kb
}

// mineMessage renders the /mening reply; live prices first, then held and
// expired ones, at most mineLimit lines.
func mineMessage(items []mineItem) string {
	if len(items) == 0 {
		return "Sizda hozircha faol narx yo'q.\n\nNarx yuborish uchun shunchaki yozing: «kartoshka 4500, 2 tonna»."
	}
	var live, other []mineItem
	for _, it := range items {
		if it.Status == market.StatusActive && !it.Expired {
			live = append(live, it)
		} else {
			other = append(other, it)
		}
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "<b>Narxlaringiz</b> — %d ta faol\n", len(live))
	shown := 0
	for _, it := range append(live, other...) {
		if shown == mineLimit {
			fmt.Fprintf(&sb, "\n… yana %d ta — to'liq ro'yxat saytda.", len(items)-shown)
			break
		}
		shown++
		fmt.Fprintf(&sb, "\n• <b>%s</b> — %s so'm/%s · %s%s\n", html.EscapeString(it.Name), pricing.FormatSom(it.PriceSom), it.Unit, it.TradeType, stockNote(it))
		switch {
		case it.Status == market.StatusReview:
			sb.WriteString("   ⏳ Tekshiruvda\n")
		case it.Expired:
			sb.WriteString("   ⌛ Muddati o'tgan — xaridorlar ko'rmayapti\n")
		default:
			sb.WriteString("   " + positionLine(pricing.Saved{Status: it.Status, Position: it.Position}) + "\n")
			sb.WriteString(dearWarning(it.Position))
		}
	}
	return sb.String()
}

// stockNote is " · zaxira 300 kg" when the seller gave a stock.
func stockNote(it mineItem) string {
	if it.Stock == nil {
		return ""
	}
	return " · zaxira " + formatQty(*it.Stock) + " " + it.Unit
}
