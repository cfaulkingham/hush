package keyring

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"testing"
)

func TestSealUnsealRoundTrip(t *testing.T) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		t.Fatal(err)
	}
	s, err := SealKey(raw, "correct horse battery")
	if err != nil {
		t.Fatal(err)
	}
	if !IsSealed(s) || !strings.HasPrefix(s, SealPrefix) {
		t.Fatalf("sealed key %q", s)
	}
	if strings.Contains(s, hex.EncodeToString(raw)) {
		t.Fatal("raw key hex leaked into sealed blob")
	}
	got, err := UnsealKey(s, "correct horse battery")
	if err != nil {
		t.Fatal(err)
	}
	if hex.EncodeToString(got) != hex.EncodeToString(raw) {
		t.Fatal("round trip mismatch")
	}
	again, err := SealKey(raw, "correct horse battery")
	if err != nil {
		t.Fatal(err)
	}
	if again == s {
		t.Fatal("two seals produced identical blobs (salt/nonce reuse)")
	}
}

func TestUnsealWrongPassphrase(t *testing.T) {
	raw := make([]byte, 32)
	rand.Read(raw)
	s, _ := SealKey(raw, "right")
	_, err := UnsealKey(s, "wrong")
	if !errors.Is(err, ErrWrongPassphrase) {
		t.Fatalf("want ErrWrongPassphrase, got %v", err)
	}
}

func TestSealKeyValidation(t *testing.T) {
	raw := make([]byte, 32)
	if _, err := SealKey(raw, ""); !errors.Is(err, ErrEmptyPassphrase) {
		t.Fatalf("empty passphrase: %v", err)
	}
	if _, err := SealKey(make([]byte, 16), "x"); !errors.Is(err, ErrBadKey) {
		t.Fatalf("short key: %v", err)
	}
}

func TestUnsealBadInput(t *testing.T) {
	if _, err := UnsealKey("hush_key_v1_"+strings.Repeat("00", 32), "x"); !errors.Is(err, ErrBadSealedKey) {
		t.Fatalf("raw key: %v", err)
	}
	if _, err := UnsealKey(SealPrefix+"!!!not-base64!!!", "x"); !errors.Is(err, ErrBadSealedKey) {
		t.Fatalf("bad base64: %v", err)
	}
	if _, err := UnsealKey(SealPrefix+"AAAA", "x"); !errors.Is(err, ErrBadSealedKey) {
		t.Fatalf("short blob: %v", err)
	}
}
