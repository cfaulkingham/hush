package store

import (
	"bytes"
	"crypto/rand"
	"testing"
	"time"
)

func TestEncryptDecryptRoundTrip(t *testing.T) {
	key := make([]byte, KeySize)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 22, 12, 0, 0, 0, time.UTC)
	doc := NewDocument("550e8400-e29b-41d4-a716-446655440000", "api", now)
	doc.Environments["development"].Secrets["DATABASE_URL"] = Secret{
		Value:     "postgres://localhost/api",
		UpdatedAt: now,
	}

	blob, err := Encrypt(doc, key)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	if !bytes.HasPrefix(blob, []byte(Magic)) {
		t.Fatalf("blob missing magic, got %q", blob[:min(len(blob), 8)])
	}
	if len(blob) < 5+24+16 {
		t.Fatalf("blob too short: %d", len(blob))
	}

	got, err := Decrypt(blob, key)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if got.ProjectID != doc.ProjectID || got.Name != "api" || got.Version != 1 {
		t.Fatalf("meta mismatch: %+v", got)
	}
	sec, ok := got.Environments["development"].Secrets["DATABASE_URL"]
	if !ok || sec.Value != "postgres://localhost/api" {
		t.Fatalf("secret mismatch: %#v", got.Environments)
	}

	blob2, err := Encrypt(doc, key)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(blob, blob2) {
		t.Fatal("expected fresh nonce to change ciphertext")
	}
}

func TestDecryptWrongKey(t *testing.T) {
	key := bytes.Repeat([]byte{1}, KeySize)
	other := bytes.Repeat([]byte{2}, KeySize)
	doc := NewDocument("id", "n", time.Now().UTC())
	blob, err := Encrypt(doc, key)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Decrypt(blob, other); err == nil {
		t.Fatal("expected error")
	}
}

func TestDecryptBadMagic(t *testing.T) {
	key := bytes.Repeat([]byte{1}, KeySize)
	if _, err := Decrypt([]byte("NOPE1xxxxxxxx"), key); err == nil {
		t.Fatal("expected error")
	}
}

func TestDecryptTruncated(t *testing.T) {
	key := bytes.Repeat([]byte{1}, KeySize)
	doc := NewDocument("id", "n", time.Now().UTC())
	blob, err := Encrypt(doc, key)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Decrypt(blob[:len(blob)-2], key); err == nil {
		t.Fatal("expected error")
	}
}

func TestEncryptRejectsBadKeySize(t *testing.T) {
	doc := NewDocument("id", "n", time.Now().UTC())
	if _, err := Encrypt(doc, []byte("short")); err == nil {
		t.Fatal("expected error")
	}
}
