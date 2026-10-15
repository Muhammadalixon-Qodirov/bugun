// Package tz holds the one time zone the product runs on. Timestamps are
// stored and sent in UTC; market days, schedules and messages follow
// Asia/Tashkent wall-clock time (CLAUDE §4.3).
package tz

import (
	"time"
	_ "time/tzdata" // Asia/Tashkent must resolve even on hosts without zoneinfo
)

// Tashkent is Asia/Tashkent. Should the zone database be missing it falls
// back to a fixed UTC+5, which Uzbekistan has kept without daylight saving
// since 1992.
var Tashkent = func() *time.Location {
	loc, err := time.LoadLocation("Asia/Tashkent")
	if err != nil {
		return time.FixedZone("Asia/Tashkent", 5*60*60)
	}
	return loc
}()
