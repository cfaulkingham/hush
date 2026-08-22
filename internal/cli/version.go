package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func (a *App) versionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Fprintf(a.Stdout, "hush %s\n", cmd.Root().Version)
		},
	}
}
