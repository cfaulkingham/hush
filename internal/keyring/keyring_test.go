package keyring

import (
	"bytes"
	"encoding/hex"
	"testing"
)

func TestFormatParseKey(t *testing.T) {
	raw := bytes.Repeat([]byte{0xab}, 32)
	s, err := FormatKey(raw)
	if err != nil {
		t.Fatal(err)
	}
	want := Prefix + hex.EncodeToString(raw)
	if s != want {
		t.Fatalf("got %s want %s", s, want)
	}
	got, err := ParseKey(s)
	if err != nil || !bytes.Equal(got, raw) {
		t.Fatalf("got %x %v", got, err)
	}
	got, err = ParseKey("HUSH_KEY_V1_" + hex.EncodeToString(raw))
	if err == nil {
		t.Fatal("prefix must be exact hush_key_v1_")
	}
	_, err = ParseKey(Prefix + hex.EncodeToString(bytes.Repeat([]byte{1}, 31)))
	if err == nil {
		t.Fatal("expected length error")
	}
	upper := Prefix + "AB" + hex.EncodeToString(raw)[2:]
	got, err = ParseKey(upper)
	if err != nil || !bytes.Equal(got, raw) {
		t.Fatalf("hex should be case-insensitive: %v", err)
	}
}

func TestMemorySetGet(t *testing.T) {
	m := NewMemory()
	if err := m.Set(Service, "pid", "secret"); err != nil {
		t.Fatal(err)
	}
	got, err := m.Get(Service, "pid")
	if err != nil || got != "secret" {
		t.Fatalf("%q %v", got, err)
	}
	if _, err := m.Get(Service, "missing"); err == nil {
		t.Fatal("expected not found")
	}
}

func TestResolvePrefersHUSH_KEY(t *testing.T) {
	raw := bytes.Repeat([]byte{3}, 32)
	s, err := FormatKey(raw)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("HUSH_KEY", s)
	m := NewMemory()
	other, _ := FormatKey(bytes.Repeat([]byte{4}, 32))
	_ = m.Set(Service, "pid", other)
	got, err := Resolve(m, "pid")
	if err != nil || !bytes.Equal(got, raw) {
		t.Fatalf("got %x %v", got, err)
	}
}

func TestResolveKeychain(t *testing.T) {
	t.Setenv("HUSH_KEY", "")
	raw := bytes.Repeat([]byte{5}, 32)
	s, _ := FormatKey(raw)
	m := NewMemory()
	_ = m.Set(Service, "pid", s)
	got, err := Resolve(m, "pid")
	if err != nil || !bytes.Equal(got, raw) {
		t.Fatalf("got %x %v", got, err)
	}
}

func TestResolveMissing(t *testing.T) {
	t.Setenv("HUSH_KEY", "")
	_, err := Resolve(NewMemory(), "pid")
	if err == nil {
		t.Fatal("expected ErrNoKey")
	}
}
