package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/cfaulkingham/hush/internal/dotenv"
	"github.com/cfaulkingham/hush/internal/keyring"
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

func TestExitCodeOtherPathErrorIsTwo(t *testing.T) {
	err := fmt.Errorf("write: %w", &os.PathError{Op: "write", Path: "x", Err: os.ErrPermission})
	if got := exitCode(err); got != 2 {
		t.Fatalf("path error exit %d want 2", got)
	}
}

func TestExitCodeKeyringBackendIsTwo(t *testing.T) {
	if got := exitCode(keyring.ErrBackend); got != 2 {
		t.Fatalf("got %d", got)
	}
}
