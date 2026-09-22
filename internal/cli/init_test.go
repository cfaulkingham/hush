package cli

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cfaulkingham/hush/internal/keyring"
	"github.com/cfaulkingham/hush/internal/project"
	"github.com/cfaulkingham/hush/internal/store"
)

type failingSetRing struct {
	*keyring.Memory
}

func (f failingSetRing) Set(string, string, string) error {
	return errors.New("injected keychain failure")
}

func newTestApp(t *testing.T, dir string) (*App, *bytes.Buffer, *bytes.Buffer, *keyring.Memory) {
	t.Helper()
	t.Setenv("HUSH_KEY", "")
	out, errb := &bytes.Buffer{}, &bytes.Buffer{}
	ring := keyring.NewMemory()
	app := &App{
		Ring:     ring,
		Stdout:   out,
		Stderr:   errb,
		Stdin:    bytes.NewReader(nil),
		Getwd:    func() (string, error) { return dir, nil },
		Environ:  func() []string { return []string{} },
		Exec:     func(argv, env []string) error { return nil },
		Now:      func() time.Time { return time.Date(2026, 8, 22, 12, 0, 0, 0, time.UTC) },
		GitCheck: func(string) gitVerdict { return gitUnknown },
	}
	return app, out, errb, ring
}

func runApp(t *testing.T, app *App, args ...string) error {
	t.Helper()
	cmd := app.Root()
	cmd.SetArgs(args)
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
	key, _, err := keyring.Resolve(ring, p.Config.ProjectID, "")
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

func TestInitRefusesHUSHKeyWithoutCreatingProject(t *testing.T) {
	dir := t.TempDir()
	app, _, _, _ := newTestApp(t, dir)
	t.Setenv("HUSH_KEY", "hush_key_v1_"+strings.Repeat("0", 64))
	err := runApp(t, app, "init")
	if err == nil || !strings.Contains(err.Error(), "unset it") {
		t.Fatalf("got %v", err)
	}
	if _, err := os.Stat(project.Dir(dir)); !os.IsNotExist(err) {
		t.Fatalf("init left project state: %v", err)
	}
}

func TestInitGitignoreFailureLeavesNoProject(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, ".gitignore"), 0755); err != nil {
		t.Fatal(err)
	}
	app, _, _, _ := newTestApp(t, dir)
	if err := runApp(t, app, "init"); err == nil {
		t.Fatal("expected gitignore failure")
	}
	if _, err := os.Stat(project.Dir(dir)); !os.IsNotExist(err) {
		t.Fatalf("init left project state: %v", err)
	}
}

func TestInitKeychainFailureRollsBackProject(t *testing.T) {
	dir := t.TempDir()
	app, _, _, ring := newTestApp(t, dir)
	app.Ring = failingSetRing{Memory: ring}
	err := runApp(t, app, "init")
	if err == nil || !strings.Contains(err.Error(), "injected keychain failure") {
		t.Fatalf("got %v", err)
	}
	if _, err := os.Stat(project.Dir(dir)); !os.IsNotExist(err) {
		t.Fatalf("init left project state: %v", err)
	}
}

func TestInitRollbackRestoresPreexistingConfig(t *testing.T) {
	dir := t.TempDir()
	old := project.Config{ProjectID: "old-project", ActiveEnv: "staging"}
	if err := project.SaveConfig(dir, old); err != nil {
		t.Fatal(err)
	}
	app, _, _, ring := newTestApp(t, dir)
	app.Ring = failingSetRing{Memory: ring}
	if err := runApp(t, app, "init"); err == nil {
		t.Fatal("expected keychain failure")
	}
	got, err := project.LoadConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got != old {
		t.Fatalf("rollback changed existing config: got %+v want %+v", got, old)
	}
	if _, err := os.Stat(project.StorePath(dir)); !os.IsNotExist(err) {
		t.Fatalf("rollback left store: %v", err)
	}
}
