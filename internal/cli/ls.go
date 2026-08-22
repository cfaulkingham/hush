package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func (a *App) lsCmd() *cobra.Command {
	var values bool
	cmd := &cobra.Command{
		Use:   "ls",
		Short: "List secret keys",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			_, p, _, doc, envName, err := a.open(cmd)
			if err != nil {
				return err
			}
			keys, err := doc.ListKeys(envName)
			if err != nil {
				return err
			}
			active := ""
			if envName == p.Config.ActiveEnv {
				active = "  ● active"
			}
			fmt.Fprintf(a.Stdout, "%s  %d secrets%s\n", envName, len(keys), active)
			m, err := doc.SecretMap(envName)
			if err != nil {
				return err
			}
			for _, k := range keys {
				if values {
					fmt.Fprintf(a.Stdout, "  %s  %s\n", k, m[k])
				} else {
					fmt.Fprintf(a.Stdout, "  %s\n", k)
				}
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&values, "values", false, "print secret values")
	return cmd
}
