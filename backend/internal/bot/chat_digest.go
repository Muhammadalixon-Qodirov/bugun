package bot

import (
	"context"
	"fmt"
	"html"
	"slices"
	"strings"

	"bugun/internal/analyst"
)

// ChatDigest sends the admins the day's assistant figures and the questions
// it could not answer (CHATBOT_REJA.md §6), one message a day, none on a
// quiet day.
func (b *Bot) ChatDigest(ctx context.Context, admins []int64) (int, error) {
	if b.ai.Analyst == nil {
		return 0, nil
	}
	st, err := b.ai.Analyst.Stats(ctx, 1)
	if err != nil {
		return 0, err
	}
	if st.Answers == 0 {
		return 0, nil
	}
	list, err := b.ai.Analyst.Unanswered(ctx, 1)
	if err != nil {
		return 0, err
	}
	ids, err := b.q.AdminTelegramIDs(ctx)
	if err != nil {
		return 0, err
	}
	for _, id := range admins {
		if !slices.Contains(ids, id) {
			ids = append(ids, id)
		}
	}
	text := chatDigestText(st, list)
	for _, id := range ids {
		b.send(ctx, id, text, nil)
	}
	return len(ids), nil
}

var reasonWords = map[string]string{"off_topic": "mavzudan tashqari", "no_data": "maʼlumot yoʻq", "no_tool": "asbob yoʻq", "tool_error": "xato", "unclear": "noaniq"}

func chatDigestText(st analyst.ChatStats, list []analyst.UnansweredGroup) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "💬 <b>Yordamchi, oxirgi 24 soat</b>\n\n%d ta suhbat, %d ta javob: %d ta javob berildi, %d ta aniqlashtirildi, %d ta javobsiz.",
		st.Conversations, st.Answers, st.Answered, st.Clarified, st.Unanswered)
	if st.Fallbacks > 0 {
		fmt.Fprintf(&sb, "\nModel oʻrniga qoidalar javob berdi: %d marta.", st.Fallbacks)
	}
	if len(list) > 0 {
		sb.WriteString("\n\n<b>Javob bera olmagan savollar:</b>")
		for i, q := range list {
			if i == 8 {
				fmt.Fprintf(&sb, "\n… va yana %d ta", len(list)-8)
				break
			}
			fmt.Fprintf(&sb, "\n• %s (%d marta, %s)", html.EscapeString(q.Question), q.Asked, reasonWords[q.Reason])
		}
		sb.WriteString("\n\nToʻliq roʻyxat: admin panel → Yordamchi.")
	}
	return sb.String()
}
