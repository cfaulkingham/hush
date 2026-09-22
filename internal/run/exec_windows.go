//go:build windows

package run

import (
	"errors"
	"os"
	"os/exec"
)

func Exec(argv []string, env []string) error {
	if len(argv) == 0 {
		return ErrNoCommand
	}
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Env = env
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			// The command ran and chose its exit code: pass it through
			// silently instead of os.Exit-ing from library code.
			return &ExitCodeError{Code: ee.ExitCode()}
		}
		code := 126
		if errors.Is(err, exec.ErrNotFound) {
			code = 127
		}
		return &ExitCodeError{Code: code, Err: err}
	}
	return nil
}
