package tz

import (
	"testing"
	"time"
)

func TestTashkentIsUTCPlusFive(t *testing.T) {
	for _, s := range []string{"2026-01-15T12:00:00Z", "2026-07-15T12:00:00Z"} {
		at, _ := time.Parse(time.RFC3339, s)
		if _, off := at.In(Tashkent).Zone(); off != 5*60*60 {
			t.Errorf("offset at %s = %d, want %d", s, off, 5*60*60)
		}
	}
	if Tashkent.String() != "Asia/Tashkent" {
		t.Errorf("zone name = %q", Tashkent.String())
	}
}
