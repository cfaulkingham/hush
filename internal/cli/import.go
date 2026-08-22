package cli

import (
	"fmt"
	"sort"

	"github.com/spf13/cobra"
	"hush/internal/dotenv"
	"hush/internal/project"
	"hush/internal/store"
)

func (a *App) importCmd() *cobra.Command {
	var overwrite bool
	cmd := &cobra.Command{
		Use:   "import <file>",
		Args:  cobra.ExactArgs(1),
		Short: "Import secrets from a .env file",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, p, key, doc, envName, err := a.open(cmd)
			if err != nil {
				return err
			}
			parsed, err := dotenv.ParseFile(args[0])
			if err != nil {
				return err
			}
			names := make([]string, 0, len(parsed))
			for k := range parsed {
				names = append(names, k)
			}
			sort.Strings(names)
			for _, k := range names {
				if err := store.ValidateKey(k); err != nil {
					return err
				}
				if err := store.ValidateValue(parsed[k]); err != nil {
					return err
				}
			}
			existing, err := doc.SecretMap(envName)
			if err != nil {
				return err
			}
			added, skipped, overwritten := 0, 0, 0
			for _, k := range names {
				old, had := existing[k]
				if had && !overwrite {
					skipped++
					continue
				}
				if err := doc.PutSecret(envName, k, parsed[k], a.Now()); err != nil {
					return err
				}
				if !had {
					added++
					continue
				}
				if old != parsed[k] {
					overwritten++
				} else {
					added++
				}
			}
			if err := store.Save(project.StorePath(p.Root), doc, key); err != nil {
				return err
			}
			if overwrite {
				fmt.Fprintf(a.Stdout, "Imported %d keys into %s (%d overwritten).\n", added+overwritten, envName, overwritten)
			} else {
				fmt.Fprintf(a.Stdout, "Imported %d keys into %s (%d skipped).\n", added, envName, skipped)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&overwrite, "overwrite", false, "replace existing keys")
	return cmd
}
