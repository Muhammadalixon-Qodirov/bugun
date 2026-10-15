package bot

import (
	"context"
	"log/slog"
	"slices"

	"bugun/internal/monitor"
	"bugun/internal/store/db"
)

// MonitoringAlerts sends the serious monitoring alerts to every admin: users
// with role "admin" and the Telegram ids in ADMIN_TELEGRAM_IDS (ML_REJA.md 3).
func (b *Bot) MonitoringAlerts(ctx context.Context, admins []int64, alerts []db.MonitoringAlert) {
	ids, err := b.q.AdminTelegramIDs(ctx)
	if err != nil {
		slog.WarnContext(ctx, "monitoring admins", "err", err)
	}
	for _, id := range admins {
		if !slices.Contains(ids, id) {
			ids = append(ids, id)
		}
	}
	text := monitor.Text(alerts)
	for _, id := range ids {
		b.send(ctx, id, text, nil)
	}
}
