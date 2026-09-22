package cli

import (
	"errors"

	"github.com/cfaulkingham/hush/internal/keyring"
	"github.com/cfaulkingham/hush/internal/run"
)

// usageError marks flag and argument mistakes so scripts can tell bad
// invocations (exit 2) from runtime failures (exit 1).
type usageError struct{ err error }

func (e *usageError) Error() string { return e.err.Error() }
func (e *usageError) Unwrap() error { return e.err }

func usage(err error) error { return &usageError{err} }

// exitCode maps errors to process exit codes:
//
//	0   success
//	1   runtime failure
//	2   usage error (flags, arguments, unknown command)
//	3   keychain backend failure
//	126 hush run: command found but not executable
//	127 hush run: command not found
//
// Anything a `hush run` child exits with is passed through unchanged.
func exitCode(err error) int {
	if err == nil {
		return 0
	}
	var ee *run.ExitCodeError
	if errors.As(err, &ee) {
		return ee.Code
	}
	var ue *usageError
	if errors.As(err, &ue) {
		return 2
	}
	if errors.Is(err, keyring.ErrBackend) {
		return 3
	}
	return 1
}
