package cli

import (
	"fmt"

	"github.com/cfaulkingham/hush/internal/store"
	"github.com/spf13/cobra"
)

func (a *App) rmCmd() *cobra.Command {
	var dryRun, yes, force bool
	cmd := &cobra.Command{
		Use:   "rm <KEY>",
		Args:  cobra.ExactArgs(1),
		Short: "Remove a secret",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, p, key, doc, envName, err := a.open(cmd)
			if err != nil {
				return err
			}
			if dryRun {
				if _, err := doc.GetSecret(envName, args[0]); err != nil {
					return err
				}
				fmt.Fprintf(a.Stdout, "would remove %s from %s; nothing written.\n", args[0], envName)
				return nil
			}
			ok, err := a.confirm(fmt.Sprintf("Remove %s from %s? [y/N] ", args[0], envName), force || yes)
			if err != nil {
				return err
			}
			if !ok {
				fmt.Fprintln(a.Stdout, "Aborted.")
				return nil
			}
			if err := a.updateStore(p, key, func(doc *store.Document) error {
				return doc.DeleteSecret(envName, args[0], a.Now())
			}); err != nil {
				return err
			}
			fmt.Fprintf(a.Stdout, "Removed %s from %s.\n", args[0], envName)
			return nil
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "show what would be removed without writing")
	cmd.Flags().BoolVar(&yes, "yes", false, "skip the confirmation prompt")
	cmd.Flags().BoolVar(&force, "force", false, "skip the confirmation prompt")
	cmd.ValidArgsFunction = a.completeKeys
	return cmd
}
