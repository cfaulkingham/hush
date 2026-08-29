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

func TestUpdateRefusesLockSymlink(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "store")
	key := bytes.Repeat([]byte{5}, KeySize)
	if err := Save(path, NewDocument("pid", "api", time.Now()), key); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(dir, "target")
	if err := os.WriteFile(target, []byte("unchanged"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, path+".lock"); err != nil {
		t.Fatal(err)
	}
	err := Update(path, key, func(*Document) error { return nil })
	if !errors.Is(err, ErrLockSymlink) {
		t.Fatalf("expected ErrLockSymlink, got %v", err)
	}
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "unchanged" {
		t.Fatalf("lock symlink target changed: %q", got)
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

func TestUpdateSerializesConcurrentMutations(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "store")
	key := bytes.Repeat([]byte{7}, KeySize)
	if err := Save(path, NewDocument("pid", "api", time.Now()), key); err != nil {
		t.Fatal(err)
	}

	firstEntered := make(chan struct{})
	releaseFirst := make(chan struct{})
	secondEntered := make(chan struct{})
	firstDone := make(chan error, 1)
	secondDone := make(chan error, 1)
	now := time.Now()
	go func() {
		firstDone <- Update(path, key, func(doc *Document) error {
			close(firstEntered)
			<-releaseFirst
			return doc.PutSecret("development", "FIRST", "1", now)
		})
	}()
	<-firstEntered
	go func() {
		secondDone <- Update(path, key, func(doc *Document) error {
			close(secondEntered)
			return doc.PutSecret("development", "SECOND", "2", now)
		})
	}()

	select {
	case <-secondEntered:
		t.Fatal("second update entered while the first held the store lock")
	case <-time.After(100 * time.Millisecond):
	}
	close(releaseFirst)
	if err := <-firstDone; err != nil {
		t.Fatal(err)
	}
	if err := <-secondDone; err != nil {
		t.Fatal(err)
	}

	doc, err := Load(path, key)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"FIRST", "SECOND"} {
		if _, err := doc.GetSecret("development", name); err != nil {
			t.Fatalf("lost concurrent update %s: %v", name, err)
		}
	}
}
