//go:build unix

package run

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
)

func Exec(argv []string, env []string) error {
	if len(argv) == 0 {
		return ErrNoCommand
	}
	bin, err := exec.LookPath(argv[0])
	if err != nil {
		code := 127
		if errors.Is(err, os.ErrPermission) {
			code = 126
		}
		return &ExitCodeError{Code: code, Err: err}
	}
	// On success syscall.Exec never returns: the child becomes this process
	// and its exit status propagates naturally.
	if err := syscall.Exec(bin, argv, env); err != nil {
		code := 126
		if errors.Is(err, syscall.ENOENT) {
			code = 127
		}
		return &ExitCodeError{Code: code, Err: err}
	}
	return nil
}
