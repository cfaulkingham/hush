package main

import (
	"os"

	"hush/internal/cli"
)

func main() {
	os.Exit(cli.Main(os.Args[1:]))
}
