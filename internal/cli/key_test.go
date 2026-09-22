package cli

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cfaulkingham/hush/internal/keyring"
	"github.com/cfaulkingham/hush/internal/project"
	"github.com/cfaulkingham/hush/internal/store"
)

// restoreFrom feeds key material on stdin: `hush key restore` never takes
// keys as arguments (argv leaks into process listings and shell history).
func restoreFrom(t *testing.T, app *App, key string, extra ...string) error {
	t.Helper()
	app.Stdin = strings.NewReader(key)
	return runApp(t, app, append([]string{"key", "restore"}, extra...)...)
}

func TestKeyBackupRestore(t *testing.T) {
	dir := t.TempDir()
	app, out, errb, ring := newTestApp(t, dir)
	_ = runApp(t, app, "init", "--name", "api")
	out.Reset()
	errb.Reset()
	if err := runApp(t, app, "key", "backup"); err != nil {
		t.Fatal(err)
	}
	key := strings.TrimSpace(out.String())
	if !strings.HasPrefix(key, "hush_key_v1_") {
		t.Fatalf("stdout %q", out.String())
	}
	if !strings.Contains(errb.String(), "Anyone with it can decrypt") {
		t.Fatalf("stderr %s", errb.String())
	}
	p, _ := project.Find(dir)
	_ = ring.Delete("hush", p.Config.ProjectID)
	t.Setenv("HUSH_KEY", "")
	if err := runApp(t, app, "status"); err == nil {
		t.Fatal("expected missing key")
	}
	if err := restoreFrom(t, app, key); err != nil {
		t.Fatal(err)
	}
	if err := runApp(t, app, "status"); err != nil {
		t.Fatal(err)
	}
}

func TestKeyRestoreRejectsKeyAsArgument(t *testing.T) {
	dir := t.TempDir()
	app, _, _, ring := newTestApp(t, dir)
	_ = runApp(t, app, "init")
	p, _ := project.Find(dir)
	s, _ := ring.Get("hush", p.Config.ProjectID)
	old, _ := ring.Get("hush", p.Config.ProjectID)
	if err := runApp(t, app, "key", "restore", s); err == nil {
		t.Fatal("expected rejection of key on argv")
	} else if !strings.Contains(err.Error(), "shell history") {
		t.Fatalf("unhelpful argv error: %v", err)
	}
	got, _ := ring.Get("hush", p.Config.ProjectID)
	if got != old {
		t.Fatal("keychain mutated by rejected restore")
	}
}

func TestKeyRestoreEmptyInput(t *testing.T) {
	dir := t.TempDir()
	app, _, _, _ := newTestApp(t, dir)
	_ = runApp(t, app, "init")
	err := restoreFrom(t, app, "   \n")
	if err == nil || !strings.Contains(err.Error(), "no key provided") {
		t.Fatalf("%v", err)
	}
}

func TestKeyRestoreWrongKey(t *testing.T) {
	dir := t.TempDir()
	app, _, _, ring := newTestApp(t, dir)
	_ = runApp(t, app, "init")
	p, _ := project.Find(dir)
	old, _ := ring.Get("hush", p.Config.ProjectID)
	raw := make([]byte, 32)
	raw[0] = 9
	bad, _ := keyring.FormatKey(raw)
	if err := restoreFrom(t, app, bad); err == nil {
		t.Fatal("expected fail")
	}
	got, _ := ring.Get("hush", p.Config.ProjectID)
	if got != old {
		t.Fatal("keychain mutated on failed restore")
	}
}

func TestKeyRestoreDoesNotWriteWhenHUSH_KEYSet(t *testing.T) {
	dir := t.TempDir()
	app, _, errb, ring := newTestApp(t, dir)
	_ = runApp(t, app, "init")
	p, _ := project.Find(dir)
	s, _ := ring.Get("hush", p.Config.ProjectID)
	_ = ring.Delete("hush", p.Config.ProjectID)
	t.Setenv("HUSH_KEY", s)
	if err := restoreFrom(t, app, s); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(errb.String(), "HUSH_KEY is set; not writing keychain") {
		t.Fatalf("%s", errb.String())
	}
	if _, err := ring.Get("hush", p.Config.ProjectID); err == nil {
		t.Fatal("wrote keychain despite HUSH_KEY")
	}
}

func TestKeyRestoreRejectsProjectIDMismatch(t *testing.T) {
	dir := t.TempDir()
	app, _, _, ring := newTestApp(t, dir)
	if err := runApp(t, app, "init"); err != nil {
		t.Fatal(err)
	}
	p, err := project.Find(dir)
	if err != nil {
		t.Fatal(err)
	}
	master, err := ring.Get("hush", p.Config.ProjectID)
	if err != nil {
		t.Fatal(err)
	}
	p.Config.ProjectID = "different-project"
	if err := project.SaveConfig(dir, p.Config); err != nil {
		t.Fatal(err)
	}
	err = restoreFrom(t, app, master)
	if !errors.Is(err, ErrProjectMismatch) {
		t.Fatalf("expected project mismatch, got %v", err)
	}
	if _, err := ring.Get("hush", "different-project"); !errors.Is(err, keyring.ErrNotFound) {
		t.Fatalf("stored key under mismatched project: %v", err)
	}
}

func TestKeyBackupWrapAndRestore(t *testing.T) {
	dir := t.TempDir()
	app, out, errb, ring := newTestApp(t, dir)
	_ = runApp(t, app, "init")
	app.ReadSecret = func(string) (string, error) { return "correct horse battery", nil }
	out.Reset()
	errb.Reset()
	if err := runApp(t, app, "key", "backup", "--wrap"); err != nil {
		t.Fatal(err)
	}
	sealed := strings.TrimSpace(out.String())
	if !strings.HasPrefix(sealed, keyring.SealPrefix) {
		t.Fatalf("stdout %q", sealed)
	}
	raw, err := keyring.ParseKey(strings.TrimSpace(sealed))
	if err == nil {
		t.Fatalf("sealed key parses as raw key: %x", raw)
	}
	if strings.Contains(sealed, "hush_key_v1_") {
		t.Fatal("raw key leaked inside sealed blob")
	}
	p, _ := project.Find(dir)
	_ = ring.Delete("hush", p.Config.ProjectID)
	t.Setenv("HUSH_KEY", "")
	app.ReadSecret = func(string) (string, error) { return "correct horse battery", nil }
	if err := restoreFrom(t, app, sealed); err != nil {
		t.Fatal(err)
	}
	if err := runApp(t, app, "status"); err != nil {
		t.Fatal(err)
	}
}

func TestKeyRestoreWrappedWrongPassphrase(t *testing.T) {
	dir := t.TempDir()
	app, out, _, ring := newTestApp(t, dir)
	_ = runApp(t, app, "init")
	app.ReadSecret = func(string) (string, error) { return "right", nil }
	out.Reset()
	if err := runApp(t, app, "key", "backup", "--wrap"); err != nil {
		t.Fatal(err)
	}
	sealed := strings.TrimSpace(out.String())
	p, _ := project.Find(dir)
	_ = ring.Delete("hush", p.Config.ProjectID)
	t.Setenv("HUSH_KEY", "")
	app.ReadSecret = func(string) (string, error) { return "wrong", nil }
	err := restoreFrom(t, app, sealed)
	if !errors.Is(err, keyring.ErrWrongPassphrase) {
		t.Fatalf("want ErrWrongPassphrase, got %v", err)
	}
	if _, err := ring.Get("hush", p.Config.ProjectID); !errors.Is(err, keyring.ErrNotFound) {
		t.Fatal("keychain written despite wrong passphrase")
	}
}

func TestKeyBackupWrapMismatch(t *testing.T) {
	dir := t.TempDir()
	app, _, _, _ := newTestApp(t, dir)
	_ = runApp(t, app, "init")
	n := 0
	app.ReadSecret = func(string) (string, error) {
		n++
		if n == 1 {
			return "one", nil
		}
		return "two", nil
	}
	err := runApp(t, app, "key", "backup", "--wrap")
	if err == nil || !strings.Contains(err.Error(), "passphrases do not match") {
		t.Fatalf("%v", err)
	}
}

func TestKeyWrapPassphraseFile(t *testing.T) {
	dir := t.TempDir()
	app, out, _, ring := newTestApp(t, dir)
	_ = runApp(t, app, "init")
	passFile := filepath.Join(dir, "pass.txt")
	if err := os.WriteFile(passFile, []byte("filepass\n"), 0600); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := runApp(t, app, "key", "backup", "--wrap", "--passphrase-file", passFile); err != nil {
		t.Fatal(err)
	}
	sealed := strings.TrimSpace(out.String())
	p, _ := project.Find(dir)
	_ = ring.Delete("hush", p.Config.ProjectID)
	t.Setenv("HUSH_KEY", "")
	if err := restoreFrom(t, app, sealed, "--passphrase-file", passFile); err != nil {
		t.Fatal(err)
	}
}

func TestKeyRotate(t *testing.T) {
	dir := t.TempDir()
	app, out, _, ring := newTestApp(t, dir)
	_ = runApp(t, app, "init")
	_ = runApp(t, app, "set", "SECRET=keepme")
	p, _ := project.Find(dir)
	old, _ := ring.Get("hush", p.Config.ProjectID)
	out.Reset()
	if err := runApp(t, app, "key", "rotate"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Key rotated") {
		t.Fatalf("%s", out.String())
	}
	fresh, _ := ring.Get("hush", p.Config.ProjectID)
	if fresh == old {
		t.Fatal("key unchanged after rotate")
	}
	out.Reset()
	if err := runApp(t, app, "get", "SECRET"); err != nil {
		t.Fatal(err)
	}
	if out.String() != "keepme\n" {
		t.Fatalf("secret lost in rotate: %q", out.String())
	}
	oldRaw, _ := keyring.ParseKey(old)
	if _, err := loadProjectDocument(p, oldRaw); !errors.Is(err, store.ErrDecrypt) {
		t.Fatalf("old key still decrypts: %v", err)
	}
	if err := restoreFrom(t, app, old); err == nil {
		t.Fatal("old key restored over rotated key")
	}
}

func TestKeyRotateRollsBackOnKeychainFailure(t *testing.T) {
	dir := t.TempDir()
	app, _, _, ring := newTestApp(t, dir)
	_ = runApp(t, app, "init")
	_ = runApp(t, app, "set", "SECRET=keepme")
	p, _ := project.Find(dir)
	old, _ := ring.Get("hush", p.Config.ProjectID)
	app.Ring = failingSetRing{ring}
	err := runApp(t, app, "key", "rotate")
	if err == nil {
		t.Fatal("expected keychain failure")
	}
	app.Ring = ring
	oldRaw, _ := keyring.ParseKey(old)
	if _, err := loadProjectDocument(p, oldRaw); err != nil {
		t.Fatalf("store not rolled back to old key: %v", err)
	}
	if err := runApp(t, app, "status"); err != nil {
		t.Fatal(err)
	}
}

func TestKeyRotateRefusesWhenHUSH_KEYSet(t *testing.T) {
	dir := t.TempDir()
	app, _, _, _ := newTestApp(t, dir)
	_ = runApp(t, app, "init")
	t.Setenv("HUSH_KEY", "hush_key_v1_"+strings.Repeat("00", 32))
	err := runApp(t, app, "key", "rotate")
	if err == nil || !strings.Contains(err.Error(), "HUSH_KEY is set") {
		t.Fatalf("%v", err)
	}
}
