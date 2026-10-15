package auth

import (
	"errors"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"
)

const testBotToken = "123456:TEST-token-for-tests-only"

func initFields(authDate time.Time) url.Values {
	return url.Values{
		"query_id":  {"AAHdF6IQAAAAAN0XohDhrOrc"},
		"user":      {`{"id":900000701,"first_name":"Test","last_name":"Patient","username":"test_patient","language_code":"uz"}`},
		"auth_date": {strconv.FormatInt(authDate.Unix(), 10)},
		"signature": {"ignored-but-signed"},
	}
}

func TestValidateInitData(t *testing.T) {
	now := time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)
	good := SignInitData(testBotToken, initFields(now.Add(-10*time.Minute)))

	id, err := ValidateInitData(good, testBotToken, time.Hour, now)
	if err != nil {
		t.Fatal(err)
	}
	if id.TelegramID != 900000701 || id.Name != "Test Patient" || id.Username != "test_patient" || id.Phone != "" {
		t.Errorf("identity = %+v", id)
	}

	v, _ := url.ParseQuery(good)
	if want := v.Get("hash"); len(want) != 64 {
		t.Errorf("hash %q is not hex SHA-256", want)
	}

	tampered := strings.Replace(good, "900000701", "900000702", 1)
	old := SignInitData(testBotToken, initFields(now.Add(-2*time.Hour)))
	future := SignInitData(testBotToken, initFields(now.Add(10*time.Minute)))
	noUser := initFields(now)
	noUser.Del("user")
	badDate := initFields(now)
	badDate.Set("auth_date", "yesterday")
	dup := good + "&auth_date=" + strconv.FormatInt(now.Unix(), 10)

	for name, tc := range map[string]struct {
		data, token string
		want        error
	}{
		"other bot's token": {good, "654321:other", ErrInitDataInvalid},
		"tampered user":     {tampered, testBotToken, ErrInitDataInvalid},
		"no hash":           {"user=%7B%7D&auth_date=1", testBotToken, ErrInitDataInvalid},
		"not hex":           {good[:strings.Index(good, "hash=")] + "hash=zz", testBotToken, ErrInitDataInvalid},
		"too old":           {old, testBotToken, ErrInitDataExpired},
		"from the future":   {future, testBotToken, ErrInitDataExpired},
		"no user":           {SignInitData(testBotToken, noUser), testBotToken, ErrInitDataInvalid},
		"bad auth_date":     {SignInitData(testBotToken, badDate), testBotToken, ErrInitDataInvalid},
		"repeated field":    {dup, testBotToken, ErrInitDataInvalid},
		"not configured":    {good, "", ErrNotConfigured},
		"garbage":           {"%zz", testBotToken, ErrInitDataInvalid},
	} {
		if _, err := ValidateInitData(tc.data, tc.token, time.Hour, now); !errors.Is(err, tc.want) {
			t.Errorf("%s: err = %v, want %v", name, err, tc.want)
		}
	}
}

// Test vector computed independently (Python hmac/hashlib) for fixed fields,
// so the signing code is not only checked against itself.
func TestInitDataKnownVector(t *testing.T) {
	data := "auth_date=1790000000&query_id=Q1&user=%7B%22id%22%3A1%2C%22first_name%22%3A%22A%22%7D&hash=" + knownHash
	if _, err := ValidateInitData(data, "1:ABC", 100*365*24*time.Hour, time.Unix(1790000100, 0)); err != nil {
		t.Fatal(err)
	}
}

const knownHash = "124091f6a70f95b3aae3a8728526e6d6e9c2a423ede1112ef98f5f105993da5f"
