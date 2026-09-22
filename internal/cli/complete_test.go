package cli

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestCompleteKeys(t *testing.T) {
	dir := t.TempDir()
	app, _, _, _ := newTestApp(t, dir)
	_ = runApp(t, app, "init")
	_ = runApp(t, app, "set", "FOO=1")
	_ = runApp(t, app, "set", "BAR=2")
	cmd := app.Root()
	got, directive := app.completeKeys(cmd, nil, "")
	if directive != cobra.ShellCompDirectiveNoFileComp {
		t.Fatalf("directive %v", directive)
	}
	if strings.Join(got, ",") != "BAR,FOO" {
		t.Fatalf("%v", got)
	}
	got, _ = app.completeKeys(cmd, nil, "FO")
	if strings.Join(got, ",") != "FOO" {
		t.Fatalf("prefix filter: %v", got)
	}
	got, _ = app.completeKeys(cmd, []string{"FOO"}, "")
	if len(got) != 0 {
		t.Fatalf("extra args: %v", got)
	}
}

func TestCompleteEnvs(t *testing.T) {
	dir := t.TempDir()
	app, _, _, _ := newTestApp(t, dir)
	_ = runApp(t, app, "init")
	_ = runApp(t, app, "env", "new", "staging")
	cmd := app.Root()
	got, _ := app.completeEnvs(cmd, nil, "")
	if strings.Join(got, ",") != "development,staging" {
		t.Fatalf("%v", got)
	}
	got, _ = app.completeEnvs(cmd, nil, "sta")
	if strings.Join(got, ",") != "staging" {
		t.Fatalf("%v", got)
	}
}

func TestCompleteFailsSilentlyWithoutProject(t *testing.T) {
	dir := t.TempDir()
	app, _, _, _ := newTestApp(t, dir)
	cmd := app.Root()
	got, directive := app.completeKeys(cmd, nil, "")
	if len(got) != 0 || directive != cobra.ShellCompDirectiveNoFileComp {
		t.Fatalf("%v %v", got, directive)
	}
}

func TestCompletionCommandExists(t *testing.T) {
	dir := t.TempDir()
	app, _, _, _ := newTestApp(t, dir)
	out := &strings.Builder{}
	app.Stdout = out
	cmd := app.Root()
	cmd.SetArgs([]string{"completion"})
	cmd.Execute() // registers the lazy completion command
	found := false
	for _, c := range cmd.Commands() {
		if c.Name() == "completion" {
			found = true
		}
	}
	if !found {
		t.Fatal("no completion command")
	}
}
