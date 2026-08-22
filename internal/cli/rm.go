package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"hush/internal/project"
	"hush/internal/store"
)

func (a *App) rmCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rm <KEY>",
		Args:  cobra.ExactArgs(1),
		Short: "Remove a secret",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, p, key, doc, envName, err := a.open(cmd)
			if err != nil {
				return err
			}
			if err := doc.DeleteSecret(envName, args[0]); err != nil {
				return err
			}
			if err := store.Save(project.StorePath(p.Root), doc, key); err != nil {
				return err
			}
			fmt.Fprintf(a.Stdout, "Removed %s from %s.\n", args[0], envName)
			return nil
		},
	}
}
