package run

import "errors"

var ErrNoCommand = errors.New("hush run requires a command. Example: hush run -- npm start")
