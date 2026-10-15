package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Telegram Mini App sign-in: the Mini App hands the page its init data, a
// query string signed with the bot's token
// (https://core.telegram.org/bots/webapps#validating-data-received-via-the-mini-app).

var (
	ErrInitDataInvalid = errors.New("init data signature is invalid")
	ErrInitDataExpired = errors.New("init data is too old")
)

// futureSkew tolerates a Telegram clock slightly ahead of ours.
const futureSkew = time.Minute

// WebAppSecret is the key init data is signed with: HMAC-SHA256 of the bot
// token under the key "WebAppData".
func WebAppSecret(botToken string) []byte {
	m := hmac.New(sha256.New, []byte("WebAppData"))
	m.Write([]byte(botToken))
	return m.Sum(nil)
}

// SignInitData signs fields the way Telegram does and returns the init data
// string. The API never signs; tests and local tools do.
func SignInitData(botToken string, fields url.Values) string {
	v := url.Values{}
	for k, vs := range fields {
		if k != "hash" {
			v[k] = vs
		}
	}
	v.Set("hash", hex.EncodeToString(initDataHash(WebAppSecret(botToken), v)))
	return v.Encode()
}

// initDataHash is HMAC-SHA256 of the data-check-string: every field but
// hash, as key=value sorted by key, joined by newlines.
func initDataHash(secret []byte, v url.Values) []byte {
	keys := make([]string, 0, len(v))
	for k := range v {
		if k != "hash" {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	lines := make([]string, len(keys))
	for i, k := range keys {
		lines[i] = k + "=" + v.Get(k)
	}
	m := hmac.New(sha256.New, secret)
	m.Write([]byte(strings.Join(lines, "\n")))
	return m.Sum(nil)
}

// ValidateInitData checks the Mini App's init data against the bot token
// and returns who opened the Mini App. Data signed more than maxAge before
// now is refused, so a leaked string stops working.
func ValidateInitData(initData, botToken string, maxAge time.Duration, now time.Time) (Identity, error) {
	if botToken == "" {
		return Identity{}, ErrNotConfigured
	}
	v, err := url.ParseQuery(initData)
	if err != nil {
		return Identity{}, ErrInitDataInvalid
	}
	got, err := hex.DecodeString(v.Get("hash"))
	if err != nil || len(got) != sha256.Size {
		return Identity{}, ErrInitDataInvalid
	}
	for _, vs := range v {
		if len(vs) != 1 { // a repeated field would be signed one way and read another
			return Identity{}, ErrInitDataInvalid
		}
	}
	if !hmac.Equal(got, initDataHash(WebAppSecret(botToken), v)) {
		return Identity{}, ErrInitDataInvalid
	}

	sec, err := strconv.ParseInt(v.Get("auth_date"), 10, 64)
	if err != nil || sec <= 0 {
		return Identity{}, ErrInitDataInvalid
	}
	signed := time.Unix(sec, 0)
	if now.Sub(signed) > maxAge || signed.Sub(now) > futureSkew {
		return Identity{}, ErrInitDataExpired
	}

	var u struct {
		ID        int64  `json:"id"`
		FirstName string `json:"first_name"`
		LastName  string `json:"last_name"`
		Username  string `json:"username"`
	}
	if err := json.Unmarshal([]byte(v.Get("user")), &u); err != nil || u.ID <= 0 {
		return Identity{}, fmt.Errorf("%w: no user", ErrInitDataInvalid)
	}
	name := strings.TrimSpace(u.FirstName + " " + u.LastName)
	if name == "" {
		name = u.Username
	}
	if name == "" {
		name = "Foydalanuvchi"
	}
	// The Mini App never shares the phone number; the login keeps one it has.
	return Identity{TelegramID: u.ID, Username: u.Username, Name: name}, nil
}
