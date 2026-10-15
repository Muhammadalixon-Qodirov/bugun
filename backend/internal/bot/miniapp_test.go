package bot

import (
	"testing"

	"github.com/go-telegram/bot/models"
)

func TestWebAppButton(t *testing.T) {
	kb, ok := webAppButton("➕ Yangi e'lon", "https://bugun.uz"+newListingPath).(*models.InlineKeyboardMarkup)
	if !ok || len(kb.InlineKeyboard) != 1 {
		t.Fatalf("markup = %#v", kb)
	}
	if b := kb.InlineKeyboard[0][0]; b.WebApp == nil || b.WebApp.URL != "https://bugun.uz/mahsulotlarim/yangi" || b.URL != "" {
		t.Errorf("https site: %+v, want a Mini App button", b)
	}
	// Telegram refuses http Mini Apps: local development gets a link.
	kb = webAppButton("x", "http://localhost:3000"+newListingPath).(*models.InlineKeyboardMarkup)
	if b := kb.InlineKeyboard[0][0]; b.WebApp != nil || b.URL != "http://localhost:3000/mahsulotlarim/yangi" {
		t.Errorf("http site: %+v, want a plain link", b)
	}
}
