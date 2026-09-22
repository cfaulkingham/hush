package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// mustSymlink skips the test where symlinks cannot be created (e.g. Windows
// without developer mode).
func mustSymlink(t *testing.T, oldname, newname string) {
	t.Helper()
	if err := os.Symlink(oldname, newname); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
}

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
	if !strings.Contains(out.String(), "Imported 2 keys into development (2 added, 0 skipped)") {
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
	if !strings.Contains(out.String(), "Imported 1 keys into development (1 added, 1 skipped)") {
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
	if !strings.Contains(out.String(), "(0 added, 1 overwritten, 1 unchanged)") {
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

func TestExportWarnsPlaintext(t *testing.T) {
	dir := t.TempDir()
	app, _, errb, _ := newTestApp(t, dir)
	_ = runApp(t, app, "init")
	_ = runApp(t, app, "set", "TOKEN=hunter2")
	dest := filepath.Join(dir, "out.env")
	errb.Reset()
	if err := runApp(t, app, "export", "-o", dest); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(errb.String(), "plaintext secrets") || !strings.Contains(errb.String(), dest) {
		t.Fatalf("missing plaintext warning: %s", errb.String())
	}
	if strings.Contains(errb.String(), "hunter2") {
		t.Fatalf("value leaked to stderr: %s", errb.String())
	}
}

func TestExportRefusesGitVisiblePath(t *testing.T) {
	dir := t.TempDir()
	app, _, _, _ := newTestApp(t, dir)
	_ = runApp(t, app, "init")
	_ = runApp(t, app, "set", "TOKEN=hunter2")
	dest := filepath.Join(dir, "out.env")
	app.GitCheck = func(string) gitVerdict { return gitNotIgnored }
	err := runApp(t, app, "export", "-o", dest)
	if err == nil || !strings.Contains(err.Error(), "git") {
		t.Fatalf("%v", err)
	}
	if _, statErr := os.Stat(dest); !os.IsNotExist(statErr) {
		t.Fatal("wrote plaintext despite git-visible path")
	}
	app.GitCheck = func(string) gitVerdict { return gitIgnored }
	if err := runApp(t, app, "export", "-o", dest); err != nil {
		t.Fatal(err)
	}
	_ = os.Remove(dest)
	app.GitCheck = func(string) gitVerdict { return gitNotIgnored }
	if err := runApp(t, app, "export", "-o", dest, "--force"); err != nil {
		t.Fatal(err)
	}
}

func TestExportRefusesSymlinkOutput(t *testing.T) {
	dir := t.TempDir()
	app, _, _, _ := newTestApp(t, dir)
	_ = runApp(t, app, "init")
	_ = runApp(t, app, "set", "TOKEN=hunter2")
	target := filepath.Join(dir, "target")
	if err := os.WriteFile(target, []byte("untouched"), 0600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link.env")
	mustSymlink(t, target, link)
	err := runApp(t, app, "export", "-o", link, "--overwrite", "--force")
	if err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("%v", err)
	}
	b, _ := os.ReadFile(target)
	if string(b) != "untouched" {
		t.Fatalf("symlink target clobbered: %q", b)
	}
}

func TestImportCountsUnchanged(t *testing.T) {
	dir := t.TempDir()
	app, out, _, _ := newTestApp(t, dir)
	_ = runApp(t, app, "init")
	_ = runApp(t, app, "set", "FOO=old")
	_ = runApp(t, app, "set", "BAR=x")
	p := filepath.Join(dir, ".env")
	_ = os.WriteFile(p, []byte("FOO=old\nBAR=x\n"), 0644)
	out.Reset()
	if err := runApp(t, app, "import", p, "--overwrite"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "(0 added, 0 overwritten, 2 unchanged)") {
		t.Fatalf("%s", out.String())
	}
}

func TestImportJSONRoundTrip(t *testing.T) {
	dir := t.TempDir()
	app, out, _, _ := newTestApp(t, dir)
	_ = runApp(t, app, "init")
	if err := runApp(t, app, "set", "TOKEN=$ecret"); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(dir, "out.json")
	if err := runApp(t, app, "export", "--format", "json", "-o", dest); err != nil {
		t.Fatal(err)
	}
	if err := runApp(t, app, "rm", "TOKEN", "--yes"); err != nil {
		t.Fatal(err)
	}
	if err := runApp(t, app, "import", dest, "--format", "json"); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := runApp(t, app, "get", "TOKEN"); err != nil {
		t.Fatal(err)
	}
	if out.String() != "$ecret\n" {
		t.Fatalf("json round trip: %q", out.String())
	}
}

func TestImportFromStdin(t *testing.T) {
	dir := t.TempDir()
	app, out, _, _ := newTestApp(t, dir)
	_ = runApp(t, app, "init")
	app.Stdin = bytes.NewBufferString("FROM_STDIN=yes\n")
	if err := runApp(t, app, "import", "-"); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := runApp(t, app, "get", "FROM_STDIN"); err != nil {
		t.Fatal(err)
	}
	if out.String() != "yes\n" {
		t.Fatalf("%q", out.String())
	}
}

func TestImportDryRunWritesNothing(t *testing.T) {
	dir := t.TempDir()
	app, out, _, _ := newTestApp(t, dir)
	_ = runApp(t, app, "init")
	_ = runApp(t, app, "set", "KEEP=old")
	p := filepath.Join(dir, ".env")
	if err := os.WriteFile(p, []byte("KEEP=new\nNEW=hunter2\n"), 0644); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := runApp(t, app, "import", p, "--overwrite", "--dry-run"); err != nil {
		t.Fatal(err)
	}
	s := out.String()
	if !strings.Contains(s, "~ KEEP (changed)") || !strings.Contains(s, "+ NEW (added)") || !strings.Contains(s, "nothing written") {
		t.Fatalf("%s", s)
	}
	if strings.Contains(s, "hunter2") || strings.Contains(s, "new") {
		t.Fatalf("value leaked in diff: %s", s)
	}
	got := &bytes.Buffer{}
	app.Stdout = got
	_ = runApp(t, app, "get", "KEEP")
	if got.String() != "old\n" {
		t.Fatalf("dry run wrote: %q", got.String())
	}
	_ = runApp(t, app, "get", "NEW")
	if got.String() == "hunter2\n" {
		t.Fatal("dry run added NEW")
	}
}

func TestImportOverwriteConfirm(t *testing.T) {
	dir := t.TempDir()
	app, _, _, _ := newTestApp(t, dir)
	_ = runApp(t, app, "init")
	_ = runApp(t, app, "set", "KEEP=old")
	p := filepath.Join(dir, ".env")
	_ = os.WriteFile(p, []byte("KEEP=new\n"), 0644)
	app.Ask = func(string) (string, error) { return "n", nil }
	if err := runApp(t, app, "import", p, "--overwrite"); err != nil {
		t.Fatal(err)
	}
	got := &bytes.Buffer{}
	app.Stdout = got
	_ = runApp(t, app, "get", "KEEP")
	if got.String() != "old\n" {
		t.Fatalf("aborted overwrite wrote: %q", got.String())
	}
	app.Ask = func(string) (string, error) { return "y", nil }
	if err := runApp(t, app, "import", p, "--overwrite"); err != nil {
		t.Fatal(err)
	}
	got.Reset()
	_ = runApp(t, app, "get", "KEEP")
	if got.String() != "new\n" {
		t.Fatalf("confirmed overwrite lost: %q", got.String())
	}
}

func TestRmDryRunAndConfirm(t *testing.T) {
	dir := t.TempDir()
	app, out, _, _ := newTestApp(t, dir)
	_ = runApp(t, app, "init")
	_ = runApp(t, app, "set", "TOKEN=hunter2")
	out.Reset()
	if err := runApp(t, app, "rm", "TOKEN", "--dry-run"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "nothing written") || strings.Contains(out.String(), "hunter2") {
		t.Fatalf("%s", out.String())
	}
	app.Ask = func(string) (string, error) { return "n", nil }
	if err := runApp(t, app, "rm", "TOKEN"); err != nil {
		t.Fatal(err)
	}
	got := &bytes.Buffer{}
	app.Stdout = got
	if err := runApp(t, app, "get", "TOKEN"); err != nil {
		t.Fatal("aborted rm deleted the secret")
	}
	app.Ask = func(string) (string, error) { return "y", nil }
	if err := runApp(t, app, "rm", "TOKEN"); err != nil {
		t.Fatal(err)
	}
	if err := runApp(t, app, "get", "TOKEN"); err == nil {
		t.Fatal("confirmed rm kept the secret")
	}
}
