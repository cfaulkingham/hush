package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"hush/internal/keyring"
	"hush/internal/project"
	"hush/internal/store"
)

func (a *App) keyCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "key", Short: "Backup or restore the project key"}
	cmd.AddCommand(a.keyBackupCmd(), a.keyRestoreCmd())
	return cmd
}

func (a *App) keyBackupCmd() *cobra.Command {
	return &cobra.Command{
		Use:  "backup",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			_, _, key, _, err := a.loadStore()
			if err != nil {
				return err
			}
			s, err := keyring.FormatKey(key)
			if err != nil {
				return err
			}
			fmt.Fprintln(a.Stdout, s)
			fmt.Fprintln(a.Stderr, "This is the project key. Anyone with it can decrypt .hush/store.")
			return nil
		},
	}
}

func (a *App) keyRestoreCmd() *cobra.Command {
	return &cobra.Command{
		Use:  "restore <key>",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, err := a.Getwd()
			if err != nil {
				return err
			}
			p, err := project.Find(cwd)
			if err != nil {
				return err
			}
			raw, err := keyring.ParseKey(args[0])
			if err != nil {
				return err
			}
			if _, err := store.Load(project.StorePath(p.Root), raw); err != nil {
				return err
			}
			if os.Getenv("HUSH_KEY") != "" {
				fmt.Fprintln(a.Stderr, "HUSH_KEY is set; not writing keychain")
				return nil
			}
			s, err := keyring.FormatKey(raw)
			if err != nil {
				return err
			}
			if err := a.Ring.Set(keyring.Service, p.Config.ProjectID, s); err != nil {
				return err
			}
			fmt.Fprintln(a.Stdout, "Key restored. Store decrypts OK.")
			return nil
		},
	}
}
