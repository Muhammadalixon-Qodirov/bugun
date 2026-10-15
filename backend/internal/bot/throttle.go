package bot

import (
	"context"
	"strings"
	"sync"
	"time"

	tg "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

// DefaultMessagesPerMinute is how many messages and button presses one user
// may send a minute (config: BOT_MESSAGES_PER_MINUTE).
const DefaultMessagesPerMinute = 20

// throttle is a token bucket per Telegram user: a user may send perMinute
// updates at once, and the bucket refills evenly over a minute. It keeps one
// flooding user from tying up the database and Telegram's rate limits that
// every other user shares.
type throttle struct {
	perMinute float64

	mu        sync.Mutex
	users     map[int64]*bucket
	lastSweep time.Time
}

type bucket struct {
	tokens float64
	at     time.Time
	warned bool // told to slow down since the last update that passed
}

func newThrottle(perMinute int) *throttle {
	if perMinute <= 0 {
		perMinute = DefaultMessagesPerMinute
	}
	return &throttle{perMinute: float64(perMinute), users: map[int64]*bucket{}}
}

// allow takes a token for the user. When there is none, warn is true only
// the first time since the user's last accepted update, so the bot says
// "slow down" once instead of answering a flood with a flood.
func (t *throttle) allow(user int64, now time.Time) (ok, warn bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.sweep(now)
	b := t.users[user]
	if b == nil {
		b = &bucket{tokens: t.perMinute, at: now}
		t.users[user] = b
	}
	b.tokens = min(t.perMinute, b.tokens+now.Sub(b.at).Minutes()*t.perMinute)
	b.at = now
	if b.tokens >= 1 {
		b.tokens--
		b.warned = false
		return true, false
	}
	warn = !b.warned
	b.warned = true
	return false, warn
}

// sweep forgets users whose bucket is full again, every few minutes, so the
// map does not grow with every user ever seen.
func (t *throttle) sweep(now time.Time) {
	if now.Sub(t.lastSweep) < 5*time.Minute {
		return
	}
	t.lastSweep = now
	for id, b := range t.users {
		if now.Sub(b.at) >= time.Minute {
			delete(t.users, id)
		}
	}
}

// updateSender is the user behind a private message or a button press.
func updateSender(upd *models.Update) (id int64, lang string, ok bool) {
	switch {
	case upd.Message != nil && upd.Message.From != nil:
		return upd.Message.From.ID, upd.Message.From.LanguageCode, true
	case upd.CallbackQuery != nil:
		return upd.CallbackQuery.From.ID, upd.CallbackQuery.From.LanguageCode, true
	}
	return 0, "", false
}

// slowDown is the one reply a flooding user gets, in their Telegram language
// when the site speaks it.
func slowDown(lang string) string {
	switch strings.ToLower(strings.SplitN(lang, "-", 2)[0]) {
	case "ru":
		return "Слишком много сообщений подряд. Пожалуйста, подождите минуту и попробуйте снова."
	case "en":
		return "Too many messages in a row. Please wait a minute and try again."
	}
	return "Juda ko'p xabar yubordingiz. Iltimos, bir daqiqa kutib, qayta urinib ko'ring."
}

// throttled drops updates from a user over the limit. It sits after
// chatGuard, so only private chats and button presses reach it.
func (b *Bot) throttled(next tg.HandlerFunc) tg.HandlerFunc {
	return func(ctx context.Context, api *tg.Bot, upd *models.Update) {
		id, lang, ok := updateSender(upd)
		if !ok || b.limit == nil {
			next(ctx, api, upd)
			return
		}
		allowed, warn := b.limit.allow(id, time.Now())
		if allowed {
			next(ctx, api, upd)
			return
		}
		if warn {
			b.tooFast(ctx, upd, slowDown(lang))
		}
	}
}

// tooFast tells the user to slow down: as a message, or as the answer to the
// button they pressed.
func (b *Bot) tooFast(ctx context.Context, upd *models.Update, text string) {
	if b.notice != nil {
		b.notice(ctx, upd, text)
		return
	}
	if cq := upd.CallbackQuery; cq != nil {
		_, _ = b.api.AnswerCallbackQuery(ctx, &tg.AnswerCallbackQueryParams{CallbackQueryID: cq.ID, Text: text, ShowAlert: true})
		return
	}
	b.send(ctx, upd.Message.Chat.ID, text, nil)
}
