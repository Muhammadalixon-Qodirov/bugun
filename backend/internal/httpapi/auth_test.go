package httpapi

import "testing"

func TestSafeReturnTo(t *testing.T) {
	cases := map[string]string{
		"":                  "/",
		"/profil":           "/profil",
		"//evil.example":    "/",
		"https://evil.test": "/",
		"/\\evil":           "/",
		"/mahsulot/piyoz":   "/mahsulot/piyoz",
	}
	for in, want := range cases {
		if got := safeReturnTo(in); got != want {
			t.Errorf("safeReturnTo(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestNormalizePhone(t *testing.T) {
	cases := map[string]string{
		"998900000000":      "+998900000000",
		"+998 90 000 00 00": "+998900000000",
		"":                  "",
		"12":                "",
	}
	for in, want := range cases {
		if got := normalizePhone(in); got != want {
			t.Errorf("normalizePhone(%q) = %q, want %q", in, got, want)
		}
	}
}
