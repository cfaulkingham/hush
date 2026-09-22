package run

import "errors"

var ErrNoCommand = errors.New("hush run requires a command. Example: hush run -- npm start")

// ExitCodeError carries the exit code the process should exit with. Exec
// returns one instead of calling os.Exit so callers control process teardown.
// Err is optional: a child process exiting nonzero has a code but no error
// message, and callers should not print anything for it.
type ExitCodeError struct {
	Code int
	Err  error
}

func (e *ExitCodeError) Error() string {
	if e.Err == nil {
		return ""
	}
	return e.Err.Error()
}

func (e *ExitCodeError) Unwrap() error { return e.Err }
