package secure

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"testing"
)

func testSealer(t *testing.T) *Sealer {
	t.Helper()
	key := make([]byte, 32)
	rand.Read(key)
	s, err := NewSealer(base64.StdEncoding.EncodeToString(key))
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestSealRoundTrip(t *testing.T) {
	s := testSealer(t)
	sealed := s.Seal("+998900000000")
	if bytes.Contains(sealed, []byte("998900000000")) {
		t.Fatal("sealed value contains plaintext")
	}
	got, err := s.Open(sealed)
	if err != nil || got != "+998900000000" {
		t.Fatalf("Open() = %q, %v", got, err)
	}
}

func TestSealIsRandomized(t *testing.T) {
	s := testSealer(t)
	if bytes.Equal(s.Seal("x"), s.Seal("x")) {
		t.Fatal("two seals of the same value are equal")
	}
}

func TestHashIsStable(t *testing.T) {
	s := testSealer(t)
	if !bytes.Equal(s.Hash("+998900000000"), s.Hash("+998900000000")) {
		t.Fatal("hash is not deterministic")
	}
}

func TestNewSealerRejectsShortKey(t *testing.T) {
	if _, err := NewSealer(base64.StdEncoding.EncodeToString([]byte("short"))); err == nil {
		t.Fatal("expected error for short key")
	}
}
