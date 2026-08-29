package cli

import (
	"fmt"

	"github.com/cfaulkingham/hush/internal/store"
	"github.com/spf13/cobra"
)

func (a *App) rmCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rm <KEY>",
		Args:  cobra.ExactArgs(1),
		Short: "Remove a secret",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, p, key, err := a.projectKey()
			if err != nil {
				return err
			}
			envName := a.resolvedEnv(cmd, p)
			if err := a.updateStore(p, key, func(doc *store.Document) error {
				return doc.DeleteSecret(envName, args[0])
			}); err != nil {
				return err
			}
			fmt.Fprintf(a.Stdout, "Removed %s from %s.\n", args[0], envName)
			return nil
		},
	}
}
