// Package secure holds small crypto helpers: random tokens, hashing and
// encryption of personal data at rest.
package secure

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
)

// Token returns a URL-safe random string with n bytes of entropy.
func Token(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err) // crypto/rand never fails on supported platforms
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

// HashToken is used for lookup of session and state tokens; the raw token is never stored.
func HashToken(t string) []byte {
	h := sha256.Sum256([]byte(t))
	return h[:]
}

// Sealer encrypts personal data (AES-256-GCM) and produces a keyed hash for lookups.
type Sealer struct {
	aead    cipher.AEAD
	hashKey []byte
}

func NewSealer(b64Key string) (*Sealer, error) {
	key, err := base64.StdEncoding.DecodeString(b64Key)
	if err != nil || len(key) != 32 {
		return nil, errors.New("DATA_ENCRYPTION_KEY must be 32 bytes, base64 encoded")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte("bugun/lookup-hash"))
	return &Sealer{aead: aead, hashKey: mac.Sum(nil)}, nil
}

func (s *Sealer) Seal(plain string) []byte {
	nonce := make([]byte, s.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		panic(err)
	}
	return s.aead.Seal(nonce, nonce, []byte(plain), nil)
}

func (s *Sealer) Open(sealed []byte) (string, error) {
	n := s.aead.NonceSize()
	if len(sealed) < n {
		return "", fmt.Errorf("sealed value too short")
	}
	plain, err := s.aead.Open(nil, sealed[:n], sealed[n:], nil)
	return string(plain), err
}

func (s *Sealer) Hash(v string) []byte {
	mac := hmac.New(sha256.New, s.hashKey)
	mac.Write([]byte(v))
	return mac.Sum(nil)
}
