package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/cfaulkingham/hush/internal/dotenv"
	"github.com/cfaulkingham/hush/internal/keyring"
	"github.com/cfaulkingham/hush/internal/run"
)

func TestExitCodeMissingFileIsOne(t *testing.T) {
	_, err := dotenv.ParseFile(filepath.Join(t.TempDir(), "nope.env"))
	if err == nil {
		t.Fatal("expected error")
	}
	if got := exitCode(err); got != 1 {
		t.Fatalf("missing file exit %d want 1", got)
	}
}

func TestExitCodeUsageIsTwo(t *testing.T) {
	if got := exitCode(usage(fmt.Errorf("bad flag"))); got != 2 {
		t.Fatalf("usage exit %d want 2", got)
	}
}

func TestExitCodeCLIErrorsAreUsage(t *testing.T) {
	dir := t.TempDir()
	app, _, _, _ := newTestApp(t, dir)
	cases := [][]string{
		{"--nope"},                     // unknown flag
		{"bogus"},                      // unknown command
		{"env", "bogus"},               // unknown subcommand
		{"key", "bogus"},               // unknown subcommand
		{"get"},                        // missing required argument
		{"run"},                        // no command to run
		{"set", "K=V", "--from-stdin"}, // inline value with --from-stdin
	}
	for _, args := range cases {
		if code := app.Run(args); code != 2 {
			t.Fatalf("%v: exit %d want 2", args, code)
		}
	}
}

func TestExitCodeKeychainBackendIsThree(t *testing.T) {
	if got := exitCode(keyring.ErrBackend); got != 3 {
		t.Fatalf("keychain backend exit %d want 3", got)
	}
}

func TestExitCodePathErrorIsOne(t *testing.T) {
	err := fmt.Errorf("write: %w", &os.PathError{Op: "write", Path: "x", Err: os.ErrPermission})
	if got := exitCode(err); got != 1 {
		t.Fatalf("path error exit %d want 1", got)
	}
}

func TestExitCodeExecPassthrough(t *testing.T) {
	if got := exitCode(&run.ExitCodeError{Code: 127}); got != 127 {
		t.Fatalf("exec exit %d want 127", got)
	}
	wrapped := fmt.Errorf("wrapper: %w", &run.ExitCodeError{Code: 126, Err: fmt.Errorf("permission denied")})
	if got := exitCode(wrapped); got != 126 {
		t.Fatalf("wrapped exec exit %d want 126", got)
	}
}
