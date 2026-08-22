package cli

import (
	"strings"
	"testing"
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
