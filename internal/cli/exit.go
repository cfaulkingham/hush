package cli

import (
	"errors"
	"os"

	"hush/internal/keyring"
)

func exitCode(err error) int {
	if err == nil {
		return 0
	}
	if errors.Is(err, keyring.ErrBackend) {
		return 2
	}
	if errors.Is(err, os.ErrNotExist) {
		return 1
	}
	var pe *os.PathError
	if errors.As(err, &pe) {
		return 2
	}
	return 1
}
