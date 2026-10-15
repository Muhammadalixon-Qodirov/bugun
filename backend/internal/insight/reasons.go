package insight

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"

	"bugun/internal/store/db"
)

// Why sellers changed prices (TAHLIL_REJA.md 3.5), as they said in the bot.

// ReasonLabels are the ready answers, in the words sellers see.
var ReasonLabels = map[string]string{
	"supply_low":  "Mahsulot kam",
	"demand_up":   "Talab oshdi",
	"costs_up":    "Yoʻl va xarajat qimmatladi",
	"export":      "Eksportga ketyapti",
	"supply_high": "Mahsulot koʻp",
	"demand_down": "Talab kam",
	"sell_fast":   "Tez sotish kerak",
	"import":      "Import kirdi",
	"market":      "Bozorda hamma shunday qildi",
	"other":       "Boshqa sabab",
}

// UpReasons and DownReasons are offered after a rise or a fall.
var (
	UpReasons   = []string{"supply_low", "demand_up", "costs_up", "export", "market", "other"}
	DownReasons = []string{"supply_high", "demand_down", "sell_fast", "import", "market", "other"}
)

type ReasonCount struct {
	Code      string `json:"code"`
	Label     string `json:"label"`
	Direction string `json:"direction" enum:"up,down"`
	Count     int32  `json:"count"`
}

func (s *Service) reasons(ctx context.Context, a *Analytics, p Params, prov pgtype.Int2) error {
	rows, err := s.q.ReasonCounts(ctx, db.ReasonCountsParams{ProductID: p.ProductID, ProvinceID: prov})
	if err != nil {
		return err
	}
	a.Reasons = make([]ReasonCount, len(rows))
	for i, r := range rows {
		a.Reasons[i] = ReasonCount{Code: r.Reason, Label: ReasonLabels[r.Reason], Direction: r.Direction, Count: r.N}
	}
	return nil
}
