package store

import (
	"bytes"
	"crypto/rand"
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func newRekeyStore(t *testing.T) (string, []byte, []byte) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "store")
	oldKey := bytes.Repeat([]byte{1}, KeySize)
	newKey := bytes.Repeat([]byte{2}, KeySize)
	doc := NewDocument("pid", "api", time.Now().UTC())
	if err := doc.PutSecret("development", "FOO", "bar", time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if err := Save(path, doc, oldKey); err != nil {
		t.Fatal(err)
	}
	return path, oldKey, newKey
}

func TestRekey(t *testing.T) {
	path, oldKey, newKey := newRekeyStore(t)
	if err := Rekey(path, oldKey, newKey); err != nil {
		t.Fatal(err)
	}
	doc, err := Load(path, newKey)
	if err != nil {
		t.Fatal(err)
	}
	v, _ := doc.GetSecret("development", "FOO")
	if v != "bar" {
		t.Fatalf("secret lost: %q", v)
	}
	if _, err := Load(path, oldKey); !errors.Is(err, ErrDecrypt) {
		t.Fatalf("old key still decrypts: %v", err)
	}
}

func TestRekeyWrongOldKeyLeavesStoreIntact(t *testing.T) {
	path, oldKey, newKey := newRekeyStore(t)
	wrong := make([]byte, KeySize)
	rand.Read(wrong)
	if err := Rekey(path, wrong, newKey); !errors.Is(err, ErrDecrypt) {
		t.Fatalf("want ErrDecrypt, got %v", err)
	}
	if _, err := Load(path, oldKey); err != nil {
		t.Fatalf("store damaged by failed rekey: %v", err)
	}
}

func TestRekeyRejectsBadNewKey(t *testing.T) {
	path, oldKey, _ := newRekeyStore(t)
	if err := Rekey(path, oldKey, make([]byte, 16)); !errors.Is(err, ErrBadKeySize) {
		t.Fatalf("want ErrBadKeySize, got %v", err)
	}
}
