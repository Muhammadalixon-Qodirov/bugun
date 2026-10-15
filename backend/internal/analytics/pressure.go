package analytics

// Demand weights for the pressure index (spec §10.3): an RFQ says the most
// about real demand, a page view the least. Starting values, to be tuned.
var DemandWeight = map[string]float64{
	"view":     1,
	"search":   2,
	"question": 2,
	"contact":  3,
	"watch":    3, // following a product (QIZIQISH_VA_SAVDO.md)
	"rfq":      5,
	"deal":     8, // a confirmed deal (QIZIQISH_VA_SAVDO.md)
}

// Pressure zones (spec §10.3).
const (
	ZoneSurplus  = "surplus"  // I < 0.8: more supply than usual, good time to buy
	ZoneBalanced = "balanced" // 0.8–1.2
	ZoneShortage = "shortage" // I > 1.2
)

// MinHistory is how many earlier 7-day windows need data before the index is
// shown; with less it would only measure noise.
const MinHistory = 2

// Pressure is I = (T_now / T̄) / (S_now / S̄), where T is weighted demand and
// S the number of sellers with a live price, per 7-day window; the averages
// are over the earlier windows that have data (up to four). ok is false when
// there is not enough history or no supply now.
func Pressure(tNow float64, tPast []float64, sNow float64, sPast []float64) (value float64, zone string, ok bool) {
	tAvg, tn := meanPositive(tPast)
	sAvg, sn := meanPositive(sPast)
	if tn < MinHistory || sn < MinHistory || sNow <= 0 {
		return 0, "", false
	}
	value = (tNow / tAvg) / (sNow / sAvg)
	switch {
	case value < 0.8:
		zone = ZoneSurplus
	case value > 1.2:
		zone = ZoneShortage
	default:
		zone = ZoneBalanced
	}
	return value, zone, true
}

func meanPositive(xs []float64) (float64, int) {
	sum, n := 0.0, 0
	for _, x := range xs {
		if x > 0 {
			sum += x
			n++
		}
	}
	if n == 0 {
		return 0, 0
	}
	return sum / float64(n), n
}
