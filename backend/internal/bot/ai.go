package bot

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"html"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	tg "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"bugun/internal/analyst"
	"bugun/internal/llm"
	"bugun/internal/photoprice"
	"bugun/internal/pricing"
	"bugun/internal/speech"
	"bugun/internal/store/db"
)

// AI are the bot's optional model features (spec §9): the analyst behind
// /savol, voice messages to text and prices from a photo. Each may be nil.
type AI struct {
	Analyst       *analyst.Service
	STT           *llm.Client
	STTLanguage   string
	STTMaxSeconds int
	Photos        *photoprice.Reader
	MaxPhotoBytes int64
}

// SetAI turns on the model features and the catalog suggestion buttons.
func (b *Bot) SetAI(a AI) {
	if a.STTMaxSeconds <= 0 {
		a.STTMaxSeconds = 60
	}
	if a.MaxPhotoBytes <= 0 {
		a.MaxPhotoBytes = 10 << 20
	}
	b.ai = a
	b.api.RegisterHandler(tg.HandlerTypeCallbackQueryData, "g:", tg.MatchTypePrefix, b.suggestionCallback)
}

// download fetches a Telegram file, at most limit bytes.
func (b *Bot) download(ctx context.Context, fileID string, limit int64) ([]byte, error) {
	f, err := b.api.GetFile(ctx, &tg.GetFileParams{FileID: fileID})
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, b.api.FileDownloadLink(f), nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, errors.New("telegram file download failed") // the link holds the bot token
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("telegram file download: status %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, errTooLarge
	}
	return data, nil
}

var errTooLarge = errors.New("file too large")

// voiceMessage turns a voice message into text (AI-1) and handles it like
// the same text typed: a price card for a seller, an answer for a question.
func (b *Bot) voiceMessage(ctx context.Context, upd *models.Update, u db.User) {
	msg := upd.Message
	if b.ai.STT == nil {
		b.send(ctx, msg.Chat.ID, "Ovozli xabarlar hozircha qabul qilinmaydi. Iltimos, narxni matn bilan yozing, masalan:\n<code>"+html.EscapeString(priceExample)+"</code>", nil)
		return
	}
	fileID, seconds := "", 0
	switch {
	case msg.Voice != nil:
		fileID, seconds = msg.Voice.FileID, msg.Voice.Duration
	case msg.Audio != nil:
		fileID, seconds = msg.Audio.FileID, msg.Audio.Duration
	}
	if seconds > b.ai.STTMaxSeconds {
		b.send(ctx, msg.Chat.ID, fmt.Sprintf("Ovozli xabar juda uzun. %d soniyagacha gapiring yoki matn bilan yozing.", b.ai.STTMaxSeconds), nil)
		return
	}
	audio, err := b.download(ctx, fileID, speech.MaxBytes(b.ai.STTMaxSeconds))
	if errors.Is(err, errTooLarge) {
		b.send(ctx, msg.Chat.ID, fmt.Sprintf("Ovozli xabar juda uzun. %d soniyagacha gapiring yoki matn bilan yozing.", b.ai.STTMaxSeconds), nil)
		return
	}
	if err != nil {
		b.fail(ctx, msg.Chat.ID, "voice download", err)
		return
	}
	ext := "ogg"
	if f, ok := speech.Detect(audio); ok {
		ext = f.Ext
	}
	text, err := b.ai.STT.Transcribe(llm.WithUser(llm.WithFeature(ctx, "stt"), u.ID), audio, "voice."+ext, b.ai.STTLanguage, float64(seconds))
	if errors.Is(err, llm.ErrLimit) {
		b.send(ctx, msg.Chat.ID, "Hozir ovozli xabarlar koʻp. Iltimos, narxni matn bilan yozing.", nil)
		return
	}
	if err != nil {
		slog.WarnContext(ctx, "voice to text", "err", err)
		b.send(ctx, msg.Chat.ID, "Ovozni matnga oʻgirib boʻlmadi. Iltimos, matn bilan yozing.", nil)
		return
	}
	if strings.TrimSpace(text) == "" {
		b.send(ctx, msg.Chat.ID, "Ovozda soʻz tanilmadi. Aniqroq qayta ayting yoki matn bilan yozing.", nil)
		return
	}
	b.send(ctx, msg.Chat.ID, "🎤 <i>"+html.EscapeString(text)+"</i>", nil)
	msg.Voice, msg.Audio, msg.Text = nil, nil, text
	b.fallback(ctx, nil, upd)
}

// photoMessage reads prices from a photo of a price list (AI-3). A caption
// with prices is read as text instead; "narx" as the caption just asks for
// the photo to be read.
func (b *Bot) photoMessage(ctx context.Context, upd *models.Update, u db.User) {
	msg := upd.Message
	caption := strings.TrimSpace(msg.Caption)
	if c := strings.ToLower(strings.TrimPrefix(caption, "/")); caption != "" && c != "narx" {
		msg.Text = caption
		b.fallback(ctx, nil, upd)
		return
	}
	if !pricing.CanSubmit(u) {
		b.send(ctx, msg.Chat.ID, "Hozircha faqat matnli xabarlarni qabul qilaman.", nil)
		return
	}
	if !b.ai.Photos.Enabled() {
		b.send(ctx, msg.Chat.ID, "Rasmdan narx oʻqish hozircha yoqilmagan. Iltimos, narxni matn bilan yozing.", nil)
		return
	}
	best := msg.Photo[len(msg.Photo)-1] // Telegram lists sizes smallest first
	if int64(best.FileSize) > b.ai.MaxPhotoBytes {
		b.send(ctx, msg.Chat.ID, "Rasm juda katta. Kichikroq rasm yuboring.", nil)
		return
	}
	data, err := b.download(ctx, best.FileID, b.ai.MaxPhotoBytes)
	if errors.Is(err, errTooLarge) {
		b.send(ctx, msg.Chat.ID, "Rasm juda katta. Kichikroq rasm yuboring.", nil)
		return
	}
	if err != nil {
		b.fail(ctx, msg.Chat.ID, "photo download", err)
		return
	}
	res, err := b.ai.Photos.Read(ctx, u, bytes.NewReader(data))
	switch {
	case errors.Is(err, llm.ErrLimit):
		b.send(ctx, msg.Chat.ID, "Hozir rasmlar koʻp. Iltimos, narxni matn bilan yozing.", nil)
		return
	case err != nil:
		slog.WarnContext(ctx, "photo prices", "err", err)
		b.send(ctx, msg.Chat.ID, "Rasmdan narxlarni oʻqib boʻlmadi. Iltimos, narxni matn bilan yozing.", nil)
		return
	case len(res.Draft.Items) == 0:
		b.send(ctx, msg.Chat.ID, "Rasmda katalogdagi mahsulot narxi topilmadi. Narxni matn bilan yozing, masalan:\n<code>"+html.EscapeString(priceExample)+"</code>", nil)
		return
	}
	id := strconv.FormatInt(res.Draft.ID, 10)
	b.send(ctx, msg.Chat.ID, "📷 Rasmdan oʻqildi.\n\n"+draftCard(res.Draft), &models.InlineKeyboardMarkup{InlineKeyboard: [][]models.InlineKeyboardButton{{
		{Text: "✅ To'g'ri", CallbackData: "p:ok:" + id},
		{Text: "✏️ Tuzatish", CallbackData: "p:fix:" + id},
	}}})
}

// suggestProducts asks which product an unknown word meant (AI-4).
func (b *Bot) suggestProducts(ctx context.Context, chatID int64, text string, sugg []pricing.Suggestion) {
	s := sugg[0]
	var rows [][]models.InlineKeyboardButton
	for _, c := range s.Candidates {
		if len(rows) == 4 {
			break
		}
		rows = append(rows, []models.InlineKeyboardButton{{Text: c.Name, CallbackData: "g:" + strconv.Itoa(int(c.ProductID))}})
	}
	b.send(ctx, chatID, fmt.Sprintf("🤔 «%s» — qaysi mahsulot?\n\nXabaringiz: «%s»", html.EscapeString(s.Word), html.EscapeString(text)),
		&models.InlineKeyboardMarkup{InlineKeyboard: rows})
}

var suggestionPrompt = regexp.MustCompile(`(?s)^🤔 «(.+?)» — qaysi mahsulot\?\s+Xabaringiz: «(.*)»$`)

// suggestionCallback resends the message with the picked product's name.
func (b *Bot) suggestionCallback(ctx context.Context, _ *tg.Bot, upd *models.Update) {
	cq := upd.CallbackQuery
	answer := func(text string) {
		_, _ = b.api.AnswerCallbackQuery(ctx, &tg.AnswerCallbackQueryParams{CallbackQueryID: cq.ID, Text: text})
	}
	msg := cq.Message.Message
	id, err := strconv.Atoi(strings.TrimPrefix(cq.Data, "g:"))
	if msg == nil || err != nil {
		answer("")
		return
	}
	m := suggestionPrompt.FindStringSubmatch(msg.Text)
	u, enabled, err := b.linkedUser(ctx, cq.From.ID)
	if m == nil || err != nil || u == nil || !enabled {
		answer("")
		return
	}
	p, err := b.q.GetProduct(ctx, int16(id))
	if err != nil {
		answer("")
		return
	}
	answer("")
	b.edit(ctx, msg, fmt.Sprintf("✅ «%s» — %s", html.EscapeString(m[1]), html.EscapeString(p.Name)), nil)
	b.submitPrice(ctx, msg.Chat.ID, *u, pricing.ReplaceWord(m[2], m[1], p.Name))
}

// analystAnswer answers /savol and questions written to the bot with the
// assistant, continuing the user's conversation of the last 30 minutes.
// Rule-based price answers keep the usual card.
func (b *Bot) analystAnswer(ctx context.Context, chatID int64, u *db.User, question string) {
	an := b.ai.Analyst
	a, err := an.Converse(ctx, analyst.ChatRequest{SessionID: an.LatestSession(ctx, u.ID), Message: question, Lang: "uz", User: u, Key: "u:" + u.ID.String()})
	if err != nil {
		b.fail(ctx, chatID, "savol", err)
		return
	}
	if a.Rules != nil && a.Kind == analyst.KindAnswer {
		b.sendAnswer(ctx, chatID, *a.Rules)
		return
	}
	if a.Status == analyst.StatusNoProduct && len(a.Sources) == 0 && a.Mode == "rules" {
		b.send(ctx, chatID, "Savolda mahsulot tanilmadi. Masalan: <code>Samarqandda piyoz qancha?</code>", nil)
		return
	}
	b.send(ctx, chatID, analystText(a.Answer), b.chatButtons(a.Answer))
}

// chatButtons are the options of a question back and the proposed actions.
func (b *Bot) chatButtons(a analyst.Answer) models.ReplyMarkup {
	var rows [][]models.InlineKeyboardButton
	for i, o := range a.Options {
		rows = append(rows, []models.InlineKeyboardButton{{Text: o, CallbackData: fmt.Sprintf("ch:%d", i)}})
	}
	for _, act := range a.Actions {
		switch act.Type {
		case analyst.ToolWatchPrice:
			pid, _ := act.Params["product_id"].(int16)
			prov, _ := act.Params["province_id"].(int16)
			trade, _ := act.Params["trade"].(string)
			btn := watchButton(pid, prov, trade)
			btn.Text = "🔔 " + act.Label
			rows = append(rows, []models.InlineKeyboardButton{btn})
		case analyst.ToolCreateRequest:
			slug, _ := act.Params["product"].(string)
			rows = append(rows, []models.InlineKeyboardButton{{Text: "📨 " + act.Label, URL: b.webURL + "/sorovlar?yangi=" + url.QueryEscape(slug)}})
		}
	}
	if len(rows) == 0 {
		return nil
	}
	return &models.InlineKeyboardMarkup{InlineKeyboard: rows}
}

// chatCallback sends the option a user pressed as their next message.
func (b *Bot) chatCallback(ctx context.Context, _ *tg.Bot, upd *models.Update) {
	cq := upd.CallbackQuery
	answer := func(text string) {
		_, _ = b.api.AnswerCallbackQuery(ctx, &tg.AnswerCallbackQueryParams{CallbackQueryID: cq.ID, Text: text})
	}
	msg := cq.Message.Message
	u, enabled, err := b.linkedUser(ctx, cq.From.ID)
	if err != nil || u == nil || !enabled || msg == nil || b.ai.Analyst == nil {
		answer("Bot profilingizga ulanmagan")
		return
	}
	i, err := strconv.Atoi(strings.TrimPrefix(cq.Data, "ch:"))
	opts := b.ai.Analyst.Options(ctx, b.ai.Analyst.LatestSession(ctx, u.ID))
	if err != nil || i < 0 || i >= len(opts) {
		answer("Bu savol eskirgan, qaytadan yozing")
		return
	}
	answer("")
	b.edit(ctx, msg, html.EscapeString(msg.Text)+"\n\n➡️ "+html.EscapeString(opts[i]), nil)
	b.analystAnswer(ctx, msg.Chat.ID, u, opts[i])
}

// analystText is an analyst answer with where its numbers come from.
func analystText(a analyst.Answer) string {
	var sb strings.Builder
	sb.WriteString(html.EscapeString(a.Text))
	var demo float64
	var from, to string
	obs := 0
	for _, s := range a.Sources {
		if s.Error != "" {
			continue
		}
		obs += s.Observations
		demo = max(demo, s.DemoShare)
		if s.DataFrom != "" && (from == "" || s.DataFrom < from) {
			from = s.DataFrom
		}
		if s.DataTo > to {
			to = s.DataTo
		}
	}
	if obs > 0 {
		fmt.Fprintf(&sb, "\n\n<i>%d ta kuzatuv", obs)
		if from != "" {
			fmt.Fprintf(&sb, " · %s – %s", from, to)
		}
		sb.WriteString(" · manba: Bugun maʼlumotlari")
		if demo > 0 {
			fmt.Fprintf(&sb, " · demo ulushi %.0f%%", demo*100)
		}
		sb.WriteString("</i>")
	}
	return sb.String()
}
