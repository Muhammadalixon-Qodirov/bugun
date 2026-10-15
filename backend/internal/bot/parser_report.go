package bot

import (
	"context"
	"fmt"
	"html"
	"slices"
	"strings"

	"bugun/internal/pricing"
)

// ParserReport sends the admins last week's parser figures (MATN_TUSHUNISH.md 1).
func (b *Bot) ParserReport(ctx context.Context, admins []int64) (int, error) {
	st, err := b.prices.Stats(ctx, 7)
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
	text := parserReportText(st)
	for _, id := range ids {
		b.send(ctx, id, text, nil)
	}
	return len(ids), nil
}

func parserReportText(st pricing.ParserStats) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "📊 <b>Narx matnlari, oxirgi %d kun</b>\n\n", st.Days)
	if st.Messages == 0 {
		sb.WriteString("Bu hafta matn kelmadi.")
		return sb.String()
	}
	pct := func(n int32) string { return fmt.Sprintf("%.0f%%", float64(n)/float64(st.Messages)*100) }
	fmt.Fprintf(&sb, "Jami: %d (sayt %d, bot %d, kanal %d)\n", st.Messages, st.Web, st.Bot, st.Channel)
	fmt.Fprintf(&sb, "✅ Toʻliq tushunildi: %d (%s)\n", st.Full, pct(st.Full))
	fmt.Fprintf(&sb, "🟡 Qisman: %d (%s)\n", st.Partial, pct(st.Partial))
	fmt.Fprintf(&sb, "❌ Tushunilmadi: %d (%s)\n", st.Failed, pct(st.Failed))
	if st.ByModel > 0 {
		fmt.Fprintf(&sb, "🤖 AI yordam berdi: %d\n", st.ByModel)
	}
	if st.FirstTry != nil {
		fmt.Fprintf(&sb, "\nKartochkalar: %d tasdiqlandi, %d bekor qilindi, %d javobsiz. Birinchi urinishda: <b>%.0f%%</b>\n",
			st.Confirmed, st.Discarded, st.Expired, *st.FirstTry*100)
	}
	if len(st.Unknown) > 0 {
		sb.WriteString("\nEng koʻp tanilmagan soʻzlar: ")
		for i, w := range st.Unknown {
			if i == 8 {
				break
			}
			if i > 0 {
				sb.WriteString(", ")
			}
			fmt.Fprintf(&sb, "%s (%d)", html.EscapeString(w.Word), w.Count)
		}
	}
	return sb.String()
}
