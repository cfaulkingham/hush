package cli

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"
	"hush/internal/keyring"
	"hush/internal/project"
	"hush/internal/store"
)

func (a *App) statusCmd() *cobra.Command {
	return &cobra.Command{
		Use:  "status",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, err := a.Getwd()
			if err != nil {
				return err
			}
			p, err := project.Find(cwd)
			if err != nil {
				return err
			}
			key, kerr := keyring.Resolve(a.Ring, p.Config.ProjectID)
			name := "(unknown)"
			nsecrets := 0
			storeLine := ".hush/store"
			var loadErr error
			if kerr != nil {
				storeLine = "decrypt failed"
				loadErr = kerr
			} else {
				doc, err := store.Load(project.StorePath(p.Root), key)
				if err != nil {
					storeLine = "decrypt failed"
					loadErr = err
				} else {
					name = doc.Name
					if env, err := doc.Env(p.Config.ActiveEnv); err == nil {
						nsecrets = len(env.Secrets)
					}
				}
			}
			fmt.Fprintf(a.Stdout, "project:  %s\n", name)
			fmt.Fprintf(a.Stdout, "id:       %s\n", p.Config.ProjectID)
			fmt.Fprintf(a.Stdout, "env:      %s\n", p.Config.ActiveEnv)
			fmt.Fprintf(a.Stdout, "secrets:  %d\n", nsecrets)
			fmt.Fprintf(a.Stdout, "store:    %s\n", storeLine)
			fmt.Fprintf(a.Stdout, "key:      %s\n", keySource())
			if loadErr != nil {
				if errors.Is(loadErr, store.ErrDecrypt) {
					return store.ErrDecrypt
				}
				return loadErr
			}
			return nil
		},
	}
}
