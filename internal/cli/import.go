package cli

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"sort"

	"github.com/cfaulkingham/hush/internal/dotenv"
	"github.com/cfaulkingham/hush/internal/store"
	"github.com/spf13/cobra"
)

func (a *App) importCmd() *cobra.Command {
	var overwrite, dryRun, yes bool
	var format string
	cmd := &cobra.Command{
		Use:   "import <file|->",
		Args:  cobra.ExactArgs(1),
		Short: "Import secrets from a .env or JSON file",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, p, key, doc, err := a.loadStore()
			if err != nil {
				return err
			}
			envName := a.resolvedEnv(cmd, p)
			if _, err := doc.Env(envName); err != nil {
				return err
			}
			parsed, err := parseImport(args[0], format, a.Stdin)
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
			added, skipped, overwritten, unchanged := planImport(existing, names, parsed, overwrite)
			if dryRun {
				printPlan(a.Stdout, envName, existing, names, parsed, overwrite)
				if overwrite {
					fmt.Fprintf(a.Stdout, "%d added, %d overwritten, %d unchanged (dry run; nothing written).\n", added, overwritten, unchanged)
				} else {
					fmt.Fprintf(a.Stdout, "%d added, %d skipped (dry run; nothing written).\n", added, skipped)
				}
				return nil
			}
			if overwrite && overwritten > 0 {
				ok, err := a.confirm(fmt.Sprintf("Overwrite existing values in %s? [y/N] ", envName), yes)
				if err != nil {
					return err
				}
				if !ok {
					fmt.Fprintln(a.Stdout, "Aborted.")
					return nil
				}
			}
			if err := a.updateStore(p, key, func(doc *store.Document) error {
				existing, err := doc.SecretMap(envName)
				if err != nil {
					return err
				}
				added, skipped, overwritten, unchanged = planImport(existing, names, parsed, overwrite)
				for _, k := range names {
					if _, had := existing[k]; had && !overwrite {
						continue
					}
					if err := doc.PutSecret(envName, k, parsed[k], a.Now()); err != nil {
						return err
					}
				}
				return nil
			}); err != nil {
				return err
			}
			if overwrite {
				fmt.Fprintf(a.Stdout, "Imported %d keys into %s (%d added, %d overwritten, %d unchanged).\n", added+overwritten, envName, added, overwritten, unchanged)
			} else {
				fmt.Fprintf(a.Stdout, "Imported %d keys into %s (%d added, %d skipped).\n", added, envName, added, skipped)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&overwrite, "overwrite", false, "replace existing keys")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "show the masked diff without writing")
	cmd.Flags().BoolVar(&yes, "yes", false, "skip the confirmation prompt")
	cmd.Flags().StringVar(&format, "format", "dotenv", "dotenv or json")
	return cmd
}

func parseImport(path, format string, stdin io.Reader) (map[string]string, error) {
	if path == "-" {
		switch format {
		case "", "dotenv":
			return dotenv.Parse(stdin)
		case "json":
			return dotenv.ParseJSON(stdin)
		default:
			return nil, fmt.Errorf("unknown format %q (dotenv|json)", format)
		}
	}
	if format == "" || format == "dotenv" {
		return dotenv.ParseFile(path)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("cannot read %s: %w", path, err)
	}
	switch format {
	case "json":
		return dotenv.ParseJSON(bytes.NewReader(b))
	default:
		return nil, fmt.Errorf("unknown format %q (dotenv|json)", format)
	}
}

// planImport classifies each key the way the import would treat it.
func planImport(existing map[string]string, names []string, parsed map[string]string, overwrite bool) (added, skipped, overwritten, unchanged int) {
	for _, k := range names {
		old, had := existing[k]
		switch {
		case !had:
			added++
		case !overwrite:
			skipped++
		case old != parsed[k]:
			overwritten++
		default:
			unchanged++
		}
	}
	return added, skipped, overwritten, unchanged
}

// printPlan shows what an import would do. Values are never echoed.
func printPlan(w io.Writer, envName string, existing map[string]string, names []string, parsed map[string]string, overwrite bool) {
	fmt.Fprintf(w, "%s:\n", envName)
	for _, k := range names {
		old, had := existing[k]
		switch {
		case !had:
			fmt.Fprintf(w, "  + %s (added)\n", k)
		case !overwrite:
			fmt.Fprintf(w, "  = %s (exists; pass --overwrite to replace)\n", k)
		case old != parsed[k]:
			fmt.Fprintf(w, "  ~ %s (changed)\n", k)
		default:
			fmt.Fprintf(w, "  = %s (unchanged)\n", k)
		}
	}
}
