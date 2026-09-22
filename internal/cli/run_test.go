package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cfaulkingham/hush/internal/project"
)

func TestRunOverlaysAndExec(t *testing.T) {
	dir := t.TempDir()
	var gotArgv, gotEnv []string
	app, _, _, _ := newTestApp(t, dir)
	app.Environ = func() []string { return []string{"PATH=/bin", "FOO=parent"} }
	app.Exec = func(argv, env []string) error {
		gotArgv = append([]string{}, argv...)
		gotEnv = append([]string{}, env...)
		return nil
	}
	_ = runApp(t, app, "init")
	_ = runApp(t, app, "set", "FOO=from-hush")
	_ = runApp(t, app, "set", "BAR=baz")
	if err := runApp(t, app, "run", "npm", "start"); err != nil {
		t.Fatal(err)
	}
	if len(gotArgv) != 2 || gotArgv[0] != "npm" || gotArgv[1] != "start" {
		t.Fatalf("argv %v", gotArgv)
	}
	m := map[string]string{}
	for _, kv := range gotEnv {
		k, v, _ := strings.Cut(kv, "=")
		m[k] = v
	}
	if m["FOO"] != "from-hush" || m["BAR"] != "baz" || m["PATH"] != "/bin" {
		t.Fatalf("%v", m)
	}
}

func TestRunRequiresCommand(t *testing.T) {
	dir := t.TempDir()
	app, _, _, _ := newTestApp(t, dir)
	_ = runApp(t, app, "init")
	err := runApp(t, app, "run")
	if err == nil || !strings.Contains(err.Error(), "hush run -- npm start") {
		t.Fatalf("%v", err)
	}
}

func TestRunEnvFlag(t *testing.T) {
	dir := t.TempDir()
	var gotEnv []string
	app, _, _, _ := newTestApp(t, dir)
	app.Exec = func(argv, env []string) error {
		gotEnv = env
		return nil
	}
	_ = runApp(t, app, "init")
	_ = runApp(t, app, "env", "new", "staging")
	_ = runApp(t, app, "set", "--env", "staging", "ONLY=yes")
	if err := runApp(t, app, "run", "--env", "staging", "--", "true"); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, kv := range gotEnv {
		if kv == "ONLY=yes" {
			found = true
		}
	}
	if !found {
		t.Fatalf("%v", gotEnv)
	}
}

func TestRunDoesNotPassHUSHKeyToChild(t *testing.T) {
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
	t.Setenv("HUSH_KEY", master)
	app.Environ = func() []string { return []string{"PATH=/bin", "HUSH_KEY=" + master} }
	var childEnv []string
	app.Exec = func(argv, env []string) error {
		childEnv = append([]string{}, env...)
		return nil
	}
	if err := runApp(t, app, "run", "true"); err != nil {
		t.Fatal(err)
	}
	for _, kv := range childEnv {
		if strings.HasPrefix(strings.ToUpper(kv), "HUSH_KEY=") {
			t.Fatalf("master key leaked to child: %v", childEnv)
		}
	}
}

func TestRunRequire(t *testing.T) {
	dir := t.TempDir()
	app, _, _, _ := newTestApp(t, dir)
	executed := false
	app.Exec = func(argv, env []string) error {
		executed = true
		return nil
	}
	_ = runApp(t, app, "init")
	_ = runApp(t, app, "set", "PRESENT=hunter2")
	err := runApp(t, app, "run", "--require", "PRESENT,MISSING", "--", "true")
	if err == nil || !strings.Contains(err.Error(), "MISSING") {
		t.Fatalf("%v", err)
	}
	if strings.Contains(err.Error(), "hunter2") {
		t.Fatalf("value leaked: %v", err)
	}
	if executed {
		t.Fatal("executed despite missing secret")
	}
	if err := runApp(t, app, "run", "--require", "PRESENT", "--", "true"); err != nil {
		t.Fatal(err)
	}
	if !executed {
		t.Fatal("did not execute with all secrets present")
	}
}

func TestRunRequireFile(t *testing.T) {
	dir := t.TempDir()
	app, _, _, _ := newTestApp(t, dir)
	executed := false
	app.Exec = func(argv, env []string) error {
		executed = true
		return nil
	}
	_ = runApp(t, app, "init")
	example := filepath.Join(dir, ".env.example")
	if err := os.WriteFile(example, []byte("A=1\nB=2\n"), 0644); err != nil {
		t.Fatal(err)
	}
	err := runApp(t, app, "run", "--require-file", example, "--", "true")
	if err == nil || !strings.Contains(err.Error(), "A, B") {
		t.Fatalf("%v", err)
	}
	if executed {
		t.Fatal("executed despite missing secrets")
	}
	_ = runApp(t, app, "set", "A=x")
	_ = runApp(t, app, "set", "B=y")
	if err := runApp(t, app, "run", "--require-file", example, "--", "true"); err != nil {
		t.Fatal(err)
	}
	if !executed {
		t.Fatal("did not execute")
	}
}
