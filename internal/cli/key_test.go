package cli

import (
	"strings"
	"testing"

	"hush/internal/keyring"
	"hush/internal/project"
)

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
	if err := runApp(t, app, "key", "restore", key); err != nil {
		t.Fatal(err)
	}
	if err := runApp(t, app, "status"); err != nil {
		t.Fatal(err)
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
	if err := runApp(t, app, "key", "restore", bad); err == nil {
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
	if err := runApp(t, app, "key", "restore", s); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(errb.String(), "HUSH_KEY is set; not writing keychain") {
		t.Fatalf("%s", errb.String())
	}
	if _, err := ring.Get("hush", p.Config.ProjectID); err == nil {
		t.Fatal("wrote keychain despite HUSH_KEY")
	}
}
