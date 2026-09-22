package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func (a *App) getCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get <KEY>",
		Args:  cobra.ExactArgs(1),
		Short: "Print a secret value",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, _, _, doc, envName, err := a.open(cmd)
			if err != nil {
				return err
			}
			v, err := doc.GetSecret(envName, args[0])
			if err != nil {
				return err
			}
			fmt.Fprintln(a.Stdout, v)
			return nil
		},
	}
	cmd.ValidArgsFunction = a.completeKeys
	return cmd
}
