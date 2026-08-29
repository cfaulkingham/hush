package main

import (
	"os"

	"github.com/cfaulkingham/hush/internal/cli"
)

func main() {
	os.Exit(cli.Main(os.Args[1:]))
}
