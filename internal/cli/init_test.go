package cli

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"hush/internal/keyring"
	"hush/internal/project"
	"hush/internal/store"
)

func newTestApp(t *testing.T, dir string) (*App, *bytes.Buffer, *bytes.Buffer, *keyring.Memory) {
	t.Helper()
	t.Setenv("HUSH_KEY", "")
	t.Setenv("NO_COLOR", "1")
	out, errb := &bytes.Buffer{}, &bytes.Buffer{}
	ring := keyring.NewMemory()
	app := &App{
		Ring:    ring,
		Stdout:  out,
		Stderr:  errb,
		Stdin:   bytes.NewReader(nil),
		Getwd:   func() (string, error) { return dir, nil },
		Environ: func() []string { return []string{} },
		Exec:    func(argv, env []string) error { return nil },
		Now:     func() time.Time { return time.Date(2026, 8, 22, 12, 0, 0, 0, time.UTC) },
	}
	return app, out, errb, ring
}

func runApp(t *testing.T, app *App, args ...string) error {
	t.Helper()
	cmd := app.Root()
	cmd.SetArgs(args)
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	return cmd.Execute()
}

func TestInitCreatesStoreAndGitignore(t *testing.T) {
	dir := t.TempDir()
	app, out, _, ring := newTestApp(t, dir)
	if err := runApp(t, app, "init", "--name", "api"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `Initialized hush project "api"`) {
		t.Fatalf("stdout: %s", out.String())
	}
	if !strings.Contains(out.String(), "Next: hush import .env") {
		t.Fatalf("stdout: %s", out.String())
	}
	p, err := project.Find(dir)
	if err != nil {
		t.Fatal(err)
	}
	if p.Config.ActiveEnv != "development" || p.Config.ProjectID == "" {
		t.Fatalf("%+v", p.Config)
	}
	key, err := keyring.Resolve(ring, p.Config.ProjectID)
	if err != nil {
		t.Fatal(err)
	}
	doc, err := store.Load(project.StorePath(dir), key)
	if err != nil {
		t.Fatal(err)
	}
	if doc.Name != "api" || doc.ProjectID != p.Config.ProjectID {
		t.Fatalf("doc %+v", doc)
	}
	keys, _ := doc.ListKeys("development")
	if len(keys) != 0 {
		t.Fatalf("keys %v", keys)
	}
	gi, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(gi), ".hush/") {
		t.Fatalf("gitignore %s", gi)
	}
	fi, err := os.Stat(project.StorePath(dir))
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != 0600 {
		t.Fatalf("mode %o", fi.Mode().Perm())
	}
}

func TestInitRefusesExisting(t *testing.T) {
	dir := t.TempDir()
	app, _, _, _ := newTestApp(t, dir)
	if err := runApp(t, app, "init", "--name", "api"); err != nil {
		t.Fatal(err)
	}
	err := runApp(t, app, "init")
	if err == nil || !strings.Contains(err.Error(), "already a hush project") {
		t.Fatalf("got %v", err)
	}
}

func TestInitDoesNotDuplicateGitignore(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte(".hush/\n"), 0644); err != nil {
		t.Fatal(err)
	}
	app, _, _, _ := newTestApp(t, dir)
	if err := runApp(t, app, "init"); err != nil {
		t.Fatal(err)
	}
	gi, _ := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if strings.Count(string(gi), ".hush/") != 1 {
		t.Fatalf("%s", gi)
	}
}

func TestInitDefaultNameIsDirBase(t *testing.T) {
	parent := t.TempDir()
	dir := filepath.Join(parent, "myapp")
	if err := os.Mkdir(dir, 0755); err != nil {
		t.Fatal(err)
	}
	app, out, _, _ := newTestApp(t, dir)
	if err := runApp(t, app, "init"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"myapp"`) {
		t.Fatalf("stdout %s", out.String())
	}
}
