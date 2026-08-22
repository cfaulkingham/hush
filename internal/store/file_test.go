package store

import (
	"bytes"
	"crypto/rand"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSaveLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "store")
	key := make([]byte, KeySize)
	rand.Read(key)
	doc := NewDocument("pid", "api", time.Now().UTC())
	if err := doc.PutSecret("development", "FOO", "bar", time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if err := Save(path, doc, key); err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != 0600 {
		t.Fatalf("mode %o", fi.Mode().Perm())
	}
	got, err := Load(path, key)
	if err != nil {
		t.Fatal(err)
	}
	v, _ := got.GetSecret("development", "FOO")
	if v != "bar" {
		t.Fatalf("got %q", v)
	}
}

func TestWriteFileRefusesSymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	if err := os.WriteFile(target, []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "store")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	if err := WriteFile(link, []byte("HUSH1notreal")); !errors.Is(err, ErrSymlink) {
		t.Fatalf("expected ErrSymlink, got %v", err)
	}
}

func TestWriteFileLeavesExistingOnFailure(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("chmod 0555 still writable as root")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "store")
	first := bytes.Repeat([]byte("a"), 32)
	if err := WriteFile(path, first); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(dir, 0755) })
	if err := WriteFile(path, bytes.Repeat([]byte("b"), 32)); err == nil {
		t.Fatal("expected failure")
	}
	got, err := os.ReadFile(path)
	if err != nil {
		os.Chmod(dir, 0755)
		got, err = os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
	}
	if !bytes.Equal(got, first) {
		t.Fatalf("existing file mutated: %q", got)
	}
}

func TestLoadVersionMismatch(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "store")
	key := bytes.Repeat([]byte{9}, KeySize)
	doc := NewDocument("pid", "api", time.Now().UTC())
	doc.Version = 99
	blob, err := Encrypt(doc, key)
	if err != nil {
		t.Fatal(err)
	}
	// bypass Decrypt version check by writing ciphertext of v99 — Encrypt does not check version
	if err := WriteFile(path, blob); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path, key); err == nil {
		t.Fatal("expected version error")
	}
}
