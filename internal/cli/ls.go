package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func (a *App) lsCmd() *cobra.Command {
	var values, jsonOut bool
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
			active := envName == p.Config.ActiveEnv
			if jsonOut {
				out := lsJSON{Env: envName, Active: active, Keys: keys}
				if values {
					m, err := doc.SecretMap(envName)
					if err != nil {
						return err
					}
					out.Values = m
				}
				return writeJSON(a.Stdout, out)
			}
			mark := ""
			if active {
				mark = "  ● active"
			}
			fmt.Fprintf(a.Stdout, "%s  %d secrets%s\n", envName, len(keys), mark)
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
	cmd.Flags().BoolVar(&jsonOut, "json", false, "machine-readable output (names only unless --values)")
	return cmd
}

type lsJSON struct {
	Env    string            `json:"env"`
	Active bool              `json:"active"`
	Keys   []string          `json:"keys"`
	Values map[string]string `json:"values,omitempty"`
}
