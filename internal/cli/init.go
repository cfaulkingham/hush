package cli

import (
	"crypto/rand"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"hush/internal/keyring"
	"hush/internal/project"
	"hush/internal/store"
)

func (a *App) initCmd() *cobra.Command {
	var name string
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Create a hush project in the current directory",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, err := a.Getwd()
			if err != nil {
				return err
			}
			hushDir := project.Dir(cwd)
			if fi, err := os.Lstat(hushDir); err == nil && !fi.IsDir() {
				return fmt.Errorf(".hush exists and is not a directory")
			}
			if _, err := os.Lstat(project.StorePath(cwd)); err == nil {
				return errors.New("already a hush project. See hush status.")
			}
			if name == "" {
				name = filepath.Base(cwd)
			}
			id := uuid.NewString()
			raw := make([]byte, 32)
			if _, err := rand.Read(raw); err != nil {
				return err
			}
			keyStr, err := keyring.FormatKey(raw)
			if err != nil {
				return err
			}
			if err := os.MkdirAll(hushDir, 0755); err != nil {
				return err
			}
			if err := project.SaveConfig(cwd, project.Config{ProjectID: id, ActiveEnv: "development"}); err != nil {
				return err
			}
			if err := a.Ring.Set(keyring.Service, id, keyStr); err != nil {
				return err
			}
			doc := store.NewDocument(id, name, a.Now())
			if err := store.Save(project.StorePath(cwd), doc, raw); err != nil {
				return err
			}
			if err := ensureGitignore(cwd); err != nil {
				return err
			}
			fmt.Fprintf(a.Stdout, "Initialized hush project %q (gitignored .hush/).\n", name)
			fmt.Fprintln(a.Stdout, "Key stored in keychain. Next: hush import .env")
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "project name (default: directory name)")
	return cmd
}
