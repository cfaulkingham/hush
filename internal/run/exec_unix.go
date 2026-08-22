//go:build unix

package run

import (
	"os/exec"
	"syscall"
)

func Exec(argv []string, env []string) error {
	if len(argv) == 0 {
		return ErrNoCommand
	}
	bin, err := exec.LookPath(argv[0])
	if err != nil {
		return err
	}
	return syscall.Exec(bin, argv, env)
}
