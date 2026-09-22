package cli

import (
	"os"
	"strings"
	"testing"

	"github.com/cfaulkingham/hush/internal/project"
)

func TestDeinitRemovesProjectAndKey(t *testing.T) {
	dir := t.TempDir()
	app, out, _, ring := newTestApp(t, dir)
	_ = runApp(t, app, "init")
	_ = runApp(t, app, "set", "SECRET=s3cret")
	p, _ := project.Find(dir)
	out.Reset()
	if err := runApp(t, app, "deinit", "--yes"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Deinitialized") {
		t.Fatalf("%s", out.String())
	}
	if _, err := os.Stat(project.Dir(dir)); !os.IsNotExist(err) {
		t.Fatal(".hush/ survived deinit")
	}
	if _, err := ring.Get("hush", p.Config.ProjectID); err == nil {
		t.Fatal("keychain entry survived deinit")
	}
	if err := runApp(t, app, "status"); err == nil {
		t.Fatal("project still found after deinit")
	}
}

func TestDeinitConfirmAndDryRun(t *testing.T) {
	dir := t.TempDir()
	app, out, _, ring := newTestApp(t, dir)
	_ = runApp(t, app, "init")
	p, _ := project.Find(dir)
	out.Reset()
	if err := runApp(t, app, "deinit", "--dry-run"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "nothing written") {
		t.Fatalf("%s", out.String())
	}
	if _, err := os.Stat(project.Dir(dir)); err != nil {
		t.Fatal("dry run deleted .hush/")
	}
	app.Ask = func(string) (string, error) { return "n", nil }
	if err := runApp(t, app, "deinit"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(project.Dir(dir)); err != nil {
		t.Fatal("aborted deinit deleted .hush/")
	}
	if _, err := ring.Get("hush", p.Config.ProjectID); err != nil {
		t.Fatal("aborted deinit deleted keychain entry")
	}
}

func TestDeinitCountsSecrets(t *testing.T) {
	dir := t.TempDir()
	app, out, _, _ := newTestApp(t, dir)
	_ = runApp(t, app, "init")
	_ = runApp(t, app, "set", "A=1")
	_ = runApp(t, app, "set", "B=2")
	var prompt string
	app.Ask = func(p string) (string, error) {
		prompt = p
		return "n", nil
	}
	out.Reset()
	if err := runApp(t, app, "deinit"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(prompt, "2 secrets") {
		t.Fatalf("prompt %q", prompt)
	}
}
