package bot

import (
	"context"
	"strings"
	"testing"
	"time"

	tg "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"bugun/internal/pricing"
	"bugun/internal/store/db"
	"bugun/internal/watch"
)

func TestParseDealDetails(t *testing.T) {
	type want struct {
		qty   float64
		unit  string
		price int64 // 0 = none
		ok    bool
	}
	for text, w := range map[string]want{
		"200 5800":        {200, "", 5800, true},
		"200 kg 5 800":    {200, "kg", 5800, true},
		"200kg 5800 so'm": {200, "kg", 5800, true},
		"2,5 tonna":       {2.5, "tonna", 0, true},
		"150":             {150, "", 0, true},
		"200 kg 5,8 ming": {200, "kg", 5800, true},
		"ko'p":            {ok: false},
		"":                {ok: false},
		"200 kg arzonroq": {ok: false},
	} {
		qty, unit, price, ok := parseDealDetails(text)
		var p int64
		if price != nil {
			p = *price
		}
		if ok != w.ok || (ok && (qty != w.qty || unit != w.unit || p != w.price)) {
			t.Errorf("%q = %v %q %d %v, want %+v", text, qty, unit, p, ok, w)
		}
	}
	if !sameUnit("kg", "kg") || !sameUnit("kilo", "kg") || sameUnit("tonna", "kg") || !sameUnit("dona", "dona") {
		t.Error("sameUnit")
	}
}

func TestDealPromptMessage(t *testing.T) {
	v := db.DealPromptViewRow{ID: 7, ProductName: "Pomidor", Variety: "qizil", SellerName: "Ali <aka>", RegionName: "Chortoq",
		CreatedAt: time.Date(2026, 9, 25, 10, 0, 0, 0, time.UTC)}
	text := dealPromptText(v)
	for _, want := range []string{"25-sentabr", "Pomidor (qizil)", "Ali &lt;aka&gt; (Chortoq)", "Kelishdingizmi?"} {
		if !strings.Contains(text, want) {
			t.Errorf("message lacks %q:\n%s", want, text)
		}
	}
	if kb := dealPromptButtons(7, 1); len(kb.InlineKeyboard) != 3 || kb.InlineKeyboard[1][0].CallbackData != "d:talk:7" {
		t.Errorf("first ask buttons: %+v", kb.InlineKeyboard)
	}
	if kb := dealPromptButtons(7, 2); len(kb.InlineKeyboard) != 2 {
		t.Errorf("second ask offers 'still talking' again: %+v", kb.InlineKeyboard)
	}
}

func TestSellerDigest(t *testing.T) {
	var rows []db.TodaysContactsBySellerRow
	for i := range 10 {
		rows = append(rows, db.TodaysContactsBySellerRow{ListingID: int64(i + 1), ProductName: "Pomidor", Contacts: int32(10 - i)})
	}
	rows[0].Variety = "qizil"
	text, kb := sellerDigest(rows)
	for _, want := range []string{"Pomidor (qizil) — 10 kishi", "… yana 2 ta e'lon", "Sotuv bo'ldimi?"} {
		if !strings.Contains(text, want) {
			t.Errorf("digest lacks %q:\n%s", want, text)
		}
	}
	if n := len(kb.InlineKeyboard); n != digestListings+1 {
		t.Errorf("%d button rows, want %d listings and 'no sale'", n, digestListings)
	}
	if kb.InlineKeyboard[0][0].CallbackData != "s:sold:1" || kb.InlineKeyboard[digestListings][0].CallbackData != "s:none" {
		t.Errorf("buttons: %+v", kb.InlineKeyboard)
	}
}

func TestMineButtons(t *testing.T) {
	stock := 300.0
	items := []mineItem{
		{ID: 1, Name: "Pomidor", Status: "active", Unit: "kg", Stock: &stock},
		{ID: 2, Name: "Olma", Status: "review"},
		{ID: 3, Name: "Nok", Status: "active", Expired: true},
	}
	kb := mineButtons(items, "https://bugun.uz/mahsulotlarim")
	if len(kb.InlineKeyboard) != 2 || kb.InlineKeyboard[0][0].CallbackData != "x:sold:1" || kb.InlineKeyboard[1][0].URL == "" {
		t.Errorf("buttons: %+v, want 'sold' only for the live listing, then the site", kb.InlineKeyboard)
	}
	if got := stockNote(items[0]); got != " · zaxira 300 kg" {
		t.Errorf("stockNote = %q", got)
	}
}

func TestWatchAlertText(t *testing.T) {
	a := watch.Alert{
		Watch:   db.WatchesToCheckRow{ProductName: "Piyoz", ProvinceName: "Namangan viloyati", Unit: "kg"},
		Cheaper: true, Price: 2800, Region: "Chortoq",
		Dropped: true, Peak: 3200, Median: 3000,
	}
	text := watchAlertText(a)
	for _, want := range []string{"Piyoz", "Namangan viloyati", "2 800 so'm/kg</b> — Chortoq", "3 200 dan 3 000 so'mga tushdi (-6,3%)", "/kuzatuv"} {
		if !strings.Contains(text, want) {
			t.Errorf("alert lacks %q:\n%s", want, text)
		}
	}
	if b := watchButton(9, 12, "ulgurji"); b.CallbackData != "w:add:9:12:ulgurji" {
		t.Errorf("watch button data %q", b.CallbackData)
	}
}

func TestRatingPrompt(t *testing.T) {
	kb := ratingButtons(42)
	if len(kb.InlineKeyboard) != 1 || len(kb.InlineKeyboard[0]) != 5 || kb.InlineKeyboard[0][4].CallbackData != "k:42:5" {
		t.Errorf("rating buttons: %+v", kb.InlineKeyboard)
	}
	text := ratingPromptText(db.RatingPromptViewRow{ProductName: "Piyoz", SellerName: "Vali & o'g'illari", RegionName: "Chortoq"})
	if !strings.Contains(text, "Vali &amp; o&#39;g&#39;illari (Chortoq)") {
		t.Errorf("prompt: %s", text)
	}
}

func TestReasonQuestion(t *testing.T) {
	up := pricing.Saved{Item: pricing.Item{ProductName: "Piyoz", PriceSom: 4800}, ListingID: 7, Status: "active", PrevPrice: 4000}
	text, kb, ok := reasonQuestion(up)
	if !ok || !strings.Contains(text, "4 000 dan 4 800 so'mga (+20%)") || kb.InlineKeyboard[0][0].CallbackData != "n:7:up:200:supply_low" {
		t.Errorf("rise: %v %q %+v", ok, text, kb)
	}
	down := up
	down.PriceSom = 3000
	if _, kb, ok := reasonQuestion(down); !ok || kb.InlineKeyboard[0][0].CallbackData != "n:7:down:-250:supply_high" {
		t.Errorf("fall: %v %+v", ok, kb)
	}
	for name, s := range map[string]pricing.Saved{
		"small change":   {Item: pricing.Item{PriceSom: 4200}, Status: "active", PrevPrice: 4000},
		"new listing":    {Item: pricing.Item{PriceSom: 4200}, Status: "active"},
		"held for check": {Item: pricing.Item{PriceSom: 9000}, Status: "review", PrevPrice: 4000},
	} {
		if _, _, ok := reasonQuestion(s); ok {
			t.Errorf("%s: asked", name)
		}
	}
	for _, kb := range [][]models.InlineKeyboardButton{kb.InlineKeyboard[0]} {
		for _, b := range kb {
			if len(b.CallbackData) > 64 {
				t.Errorf("callback data over 64 bytes: %q", b.CallbackData)
			}
		}
	}
	if dearWarning(&pricing.MarketPosition{DiffPercent: 18}) == "" || dearWarning(&pricing.MarketPosition{DiffPercent: 10}) != "" || dearWarning(nil) != "" {
		t.Error("dear warning")
	}
}

// Nothing outside a private chat reaches the normal handlers.
func TestChatGuard(t *testing.T) {
	b := &Bot{}
	calls := 0
	next := func(context.Context, *tg.Bot, *models.Update) { calls++ }
	h := b.chatGuard(next)
	group := &models.Message{Chat: models.Chat{ID: -5, Type: models.ChatTypeSupergroup}, From: &models.User{ID: 1}} // no text: never parsed
	private := &models.Message{Chat: models.Chat{ID: 5, Type: models.ChatTypePrivate}, From: &models.User{ID: 1}, Text: "salom"}
	for _, u := range []*models.Update{
		{Message: group},
		{EditedMessage: group},
		{EditedMessage: private},
		{CallbackQuery: &models.CallbackQuery{Message: models.MaybeInaccessibleMessage{Message: group}}},
	} {
		h(context.Background(), nil, u)
	}
	if calls != 0 {
		t.Errorf("%d updates from outside private chats reached the handlers", calls)
	}
	h(context.Background(), nil, &models.Update{Message: private})
	if calls != 1 {
		t.Error("a private message did not reach the handlers")
	}
}
