package bot

import (
	"context"
	"testing"
	"time"

	tg "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

func TestThrottleBucket(t *testing.T) {
	th := newThrottle(3)
	now := time.Date(2026, 9, 26, 10, 0, 0, 0, time.UTC)
	for i := range 3 {
		if ok, _ := th.allow(1, now); !ok {
			t.Fatalf("message %d of a burst of 3 refused", i+1)
		}
	}
	if ok, warn := th.allow(1, now); ok || !warn {
		t.Errorf("4th message: ok %v warn %v, want refused with one warning", ok, warn)
	}
	if ok, warn := th.allow(1, now.Add(time.Second)); ok || warn {
		t.Errorf("5th message: ok %v warn %v, want refused silently", ok, warn)
	}
	if ok, _ := th.allow(2, now); !ok {
		t.Error("another user is limited by the first one's flood")
	}
	// A third of a minute brings one token back (3 a minute).
	if ok, _ := th.allow(1, now.Add(20*time.Second)); !ok {
		t.Error("refilled token refused")
	}
	// After a message passes, the next refusal warns again.
	if ok, warn := th.allow(1, now.Add(21*time.Second)); ok || !warn {
		t.Errorf("after a pass: ok %v warn %v, want a new warning", ok, warn)
	}
	// Idle users are forgotten.
	th.allow(3, now.Add(10*time.Minute))
	if _, kept := th.users[1]; kept {
		t.Error("idle user kept after the sweep")
	}
}

func TestThrottledMiddleware(t *testing.T) {
	var handled int
	var notices []string
	b := &Bot{limit: newThrottle(2), notice: func(_ context.Context, _ *models.Update, text string) { notices = append(notices, text) }}
	h := b.throttled(func(context.Context, *tg.Bot, *models.Update) { handled++ })
	msg := func(user int64, lang string) *models.Update {
		return &models.Update{Message: &models.Message{From: &models.User{ID: user, LanguageCode: lang}, Chat: models.Chat{ID: user, Type: models.ChatTypePrivate}}}
	}
	for range 10 {
		h(context.Background(), nil, msg(42, "ru"))
	}
	if handled != 2 {
		t.Errorf("handled %d of 10 messages, want 2", handled)
	}
	if len(notices) != 1 || notices[0] != slowDown("ru") {
		t.Errorf("notices %q, want one Russian notice", notices)
	}
	// Button presses count too.
	h(context.Background(), nil, &models.Update{CallbackQuery: &models.CallbackQuery{ID: "q", From: models.User{ID: 42}}})
	if handled != 2 || len(notices) != 1 {
		t.Errorf("callback over the limit: handled %d, notices %d", handled, len(notices))
	}
	// Updates without a sender (the bot joining a channel) pass.
	h(context.Background(), nil, &models.Update{MyChatMember: &models.ChatMemberUpdated{}})
	if handled != 3 {
		t.Errorf("update without a user was throttled")
	}
	if slowDown("") == slowDown("en") || slowDown("uz") != slowDown("") || slowDown("en-GB") != slowDown("en") {
		t.Error("language fallback")
	}
}
