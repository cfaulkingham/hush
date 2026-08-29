package cli

import (
	"errors"
	"strings"
	"testing"

	"github.com/cfaulkingham/hush/internal/project"
)

func TestStatusAfterInit(t *testing.T) {
	dir := t.TempDir()
	app, out, _, _ := newTestApp(t, dir)
	if err := runApp(t, app, "init", "--name", "api"); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := runApp(t, app, "status"); err != nil {
		t.Fatal(err)
	}
	s := out.String()
	for _, want := range []string{"project:", "api", "env:", "development", "secrets:", "0", "store:", "key:", "keychain"} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing %q in %s", want, s)
		}
	}
	if strings.Contains(s, "hush_key_v1_") {
		t.Fatal("status leaked key")
	}
}

func TestEnvNewUseLs(t *testing.T) {
	dir := t.TempDir()
	app, out, _, _ := newTestApp(t, dir)
	if err := runApp(t, app, "init", "--name", "api"); err != nil {
		t.Fatal(err)
	}
	if err := runApp(t, app, "env", "new", "staging"); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := runApp(t, app, "env", "ls"); err != nil {
		t.Fatal(err)
	}
	s := out.String()
	if !strings.Contains(s, "development") || !strings.Contains(s, "staging") {
		t.Fatalf("%s", s)
	}
	if err := runApp(t, app, "use", "staging"); err != nil {
		t.Fatal(err)
	}
	cfg, err := project.LoadConfig(dir)
	if err != nil || cfg.ActiveEnv != "staging" {
		t.Fatalf("%+v %v", cfg, err)
	}
	out.Reset()
	if err := runApp(t, app, "env", "new", "production", "--use"); err != nil {
		t.Fatal(err)
	}
	cfg, _ = project.LoadConfig(dir)
	if cfg.ActiveEnv != "production" {
		t.Fatalf("%+v", cfg)
	}
}

func TestUseMissingEnv(t *testing.T) {
	dir := t.TempDir()
	app, _, _, _ := newTestApp(t, dir)
	_ = runApp(t, app, "init")
	err := runApp(t, app, "use", "nope")
	if err == nil || !strings.Contains(err.Error(), "hush env new nope") {
		t.Fatalf("got %v", err)
	}
}

func TestStatusHUSH_KEYLabel(t *testing.T) {
	dir := t.TempDir()
	app, out, _, ring := newTestApp(t, dir)
	if err := runApp(t, app, "init", "--name", "api"); err != nil {
		t.Fatal(err)
	}
	p, _ := project.Find(dir)
	s, err := ring.Get("hush", p.Config.ProjectID)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("HUSH_KEY", s)
	out.Reset()
	if err := runApp(t, app, "status"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "HUSH_KEY") {
		t.Fatalf("%s", out.String())
	}
}

func TestStatusRejectsProjectIDMismatch(t *testing.T) {
	dir := t.TempDir()
	app, out, _, ring := newTestApp(t, dir)
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
	t.Setenv("HUSH_KEY", master)
	out.Reset()
	err = runApp(t, app, "status")
	if !errors.Is(err, ErrProjectMismatch) {
		t.Fatalf("expected project mismatch, got %v", err)
	}
	if !strings.Contains(out.String(), "validation failed") {
		t.Fatalf("status did not report validation failure: %s", out.String())
	}
}

func TestStatusRejectsMissingActiveEnvironment(t *testing.T) {
	dir := t.TempDir()
	app, _, _, _ := newTestApp(t, dir)
	if err := runApp(t, app, "init"); err != nil {
		t.Fatal(err)
	}
	p, err := project.Find(dir)
	if err != nil {
		t.Fatal(err)
	}
	p.Config.ActiveEnv = "missing"
	if err := project.SaveConfig(dir, p.Config); err != nil {
		t.Fatal(err)
	}
	err = runApp(t, app, "status")
	if err == nil || !strings.Contains(err.Error(), "environment missing not found") {
		t.Fatalf("got %v", err)
	}
}
