package project

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindWalksUp(t *testing.T) {
	root := t.TempDir()
	hush := filepath.Join(root, ".hush")
	if err := os.Mkdir(hush, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hush, "store"), []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	cfg := Config{ProjectID: "pid", ActiveEnv: "development"}
	if err := SaveConfig(root, cfg); err != nil {
		t.Fatal(err)
	}
	nested := filepath.Join(root, "a", "b")
	if err := os.MkdirAll(nested, 0755); err != nil {
		t.Fatal(err)
	}
	p, err := Find(nested)
	if err != nil {
		t.Fatal(err)
	}
	if p.Root != root {
		t.Fatalf("root %s want %s", p.Root, root)
	}
	if p.Config.ProjectID != "pid" || p.Config.ActiveEnv != "development" {
		t.Fatalf("%+v", p.Config)
	}
}

func TestFindNone(t *testing.T) {
	_, err := Find(t.TempDir())
	if err == nil {
		t.Fatal("expected not found")
	}
}

func TestSaveConfigRoundTrip(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".hush"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := SaveConfig(root, Config{ProjectID: "abc", ActiveEnv: "staging"}); err != nil {
		t.Fatal(err)
	}
	got, err := LoadConfig(root)
	if err != nil {
		t.Fatal(err)
	}
	if got.ProjectID != "abc" || got.ActiveEnv != "staging" {
		t.Fatalf("%+v", got)
	}
}
