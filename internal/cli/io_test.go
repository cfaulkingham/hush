package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestImportExportRoundTrip(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")
	if err := os.WriteFile(envPath, []byte("FOO=bar\nBAZ=qux\n"), 0644); err != nil {
		t.Fatal(err)
	}
	app, out, _, _ := newTestApp(t, dir)
	_ = runApp(t, app, "init")
	if err := runApp(t, app, "import", envPath); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Imported 2 keys into development (0 skipped)") {
		t.Fatalf("%s", out.String())
	}
	out.Reset()
	if err := runApp(t, app, "export"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "FOO=bar") || !strings.Contains(out.String(), "BAZ=qux") {
		t.Fatalf("%s", out.String())
	}
}

func TestImportSkipAndOverwrite(t *testing.T) {
	dir := t.TempDir()
	app, out, _, _ := newTestApp(t, dir)
	_ = runApp(t, app, "init")
	_ = runApp(t, app, "set", "FOO=old")
	p := filepath.Join(dir, ".env")
	_ = os.WriteFile(p, []byte("FOO=new\nBAR=x\n"), 0644)
	out.Reset()
	if err := runApp(t, app, "import", p); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Imported 1 keys into development (1 skipped)") {
		t.Fatalf("%s", out.String())
	}
	out.Reset()
	_ = runApp(t, app, "get", "FOO")
	if out.String() != "old\n" {
		t.Fatalf("skipped failed: %q", out.String())
	}
	out.Reset()
	if err := runApp(t, app, "import", p, "--overwrite"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "1 overwritten") {
		t.Fatalf("%s", out.String())
	}
	out.Reset()
	_ = runApp(t, app, "get", "FOO")
	if out.String() != "new\n" {
		t.Fatalf("%q", out.String())
	}
}

func TestImportRejectsBadKeyNoPartialWrite(t *testing.T) {
	dir := t.TempDir()
	app, out, _, _ := newTestApp(t, dir)
	_ = runApp(t, app, "init")
	p := filepath.Join(dir, ".env")
	_ = os.WriteFile(p, []byte("GOOD=1\nBAD-KEY=2\n"), 0644)
	if err := runApp(t, app, "import", p); err == nil {
		t.Fatal("expected error")
	}
	out.Reset()
	if err := runApp(t, app, "get", "GOOD"); err == nil {
		t.Fatal("partial import wrote GOOD")
	}
}

func TestExportJSONAndOutputFile(t *testing.T) {
	dir := t.TempDir()
	app, out, _, _ := newTestApp(t, dir)
	_ = runApp(t, app, "init")
	_ = runApp(t, app, "set", "A=1")
	out.Reset()
	if err := runApp(t, app, "export", "--format", "json"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"A": "1"`) {
		t.Fatalf("%s", out.String())
	}
	dest := filepath.Join(dir, "out.env")
	if err := runApp(t, app, "export", "-o", dest); err != nil {
		t.Fatal(err)
	}
	if err := runApp(t, app, "export", "-o", dest); err == nil {
		t.Fatal("expected refuse overwrite")
	}
	if err := runApp(t, app, "export", "-o", dest, "--overwrite"); err != nil {
		t.Fatal(err)
	}
	fi, _ := os.Stat(dest)
	if fi.Mode().Perm() != 0600 {
		t.Fatalf("mode %o", fi.Mode().Perm())
	}
}

func TestImportMissingFile(t *testing.T) {
	dir := t.TempDir()
	app, _, _, _ := newTestApp(t, dir)
	_ = runApp(t, app, "init")
	err := runApp(t, app, "import", filepath.Join(dir, "nope.env"))
	if err == nil || !strings.Contains(err.Error(), "cannot read") {
		t.Fatalf("%v", err)
	}
}

func TestImportMissingFileExitCode(t *testing.T) {
	dir := t.TempDir()
	app, _, errb, _ := newTestApp(t, dir)
	_ = runApp(t, app, "init")
	missing := filepath.Join(dir, "nope.env")
	code := app.Run([]string{"import", missing})
	if code != 1 {
		t.Fatalf("exit %d want 1", code)
	}
	if !strings.Contains(errb.String(), "cannot read") {
		t.Fatalf("stderr %s", errb.String())
	}
}

func TestImportExportDollarRoundTrip(t *testing.T) {
	dir := t.TempDir()
	app, out, _, _ := newTestApp(t, dir)
	_ = runApp(t, app, "init")
	if err := runApp(t, app, "set", "TOKEN=$HOME"); err != nil {
		t.Fatal(err)
	}
	exported := filepath.Join(dir, "out.env")
	if err := runApp(t, app, "export", "-o", exported); err != nil {
		t.Fatal(err)
	}
	if err := runApp(t, app, "import", exported, "--overwrite"); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := runApp(t, app, "get", "TOKEN"); err != nil {
		t.Fatal(err)
	}
	if out.String() != "$HOME\n" {
		t.Fatalf("got %q", out.String())
	}
}

func TestImportParseErrorDoesNotEchoValue(t *testing.T) {
	dir := t.TempDir()
	app, _, errb, _ := newTestApp(t, dir)
	_ = runApp(t, app, "init")
	p := filepath.Join(dir, ".env")
	if err := os.WriteFile(p, []byte("FOO-BAR=hunter2\n"), 0644); err != nil {
		t.Fatal(err)
	}
	code := app.Run([]string{"import", p})
	if code != 1 {
		t.Fatalf("exit %d want 1", code)
	}
	if strings.Contains(errb.String(), "hunter2") {
		t.Fatalf("leaked value: %s", errb.String())
	}
}

func TestImportMissingEnv(t *testing.T) {
	dir := t.TempDir()
	app, _, _, _ := newTestApp(t, dir)
	_ = runApp(t, app, "init")
	p := filepath.Join(dir, ".env")
	_ = os.WriteFile(p, []byte("FOO=bar\n"), 0644)
	err := runApp(t, app, "import", p, "--env", "staging")
	if err == nil || !strings.Contains(err.Error(), "staging") {
		t.Fatalf("%v", err)
	}
}
