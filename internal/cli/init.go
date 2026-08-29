package cli

import (
	"crypto/rand"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/cfaulkingham/hush/internal/keyring"
	"github.com/cfaulkingham/hush/internal/project"
	"github.com/cfaulkingham/hush/internal/safeio"
	"github.com/cfaulkingham/hush/internal/store"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
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
			hushDirExisted := false
			if fi, err := os.Lstat(hushDir); err == nil {
				hushDirExisted = true
				if !fi.IsDir() {
					return fmt.Errorf(".hush exists and is not a directory")
				}
			} else if !os.IsNotExist(err) {
				return err
			}
			if _, err := os.Lstat(project.StorePath(cwd)); err == nil {
				return errors.New("already a hush project. See hush status.")
			} else if !os.IsNotExist(err) {
				return err
			}
			if os.Getenv("HUSH_KEY") != "" {
				return errors.New("HUSH_KEY is set; unset it before running hush init")
			}
			configPath := project.ConfigPath(cwd)
			var previousConfig []byte
			var previousConfigMode os.FileMode
			previousConfigRegular := false
			configExisted := false
			if fi, err := os.Lstat(configPath); err == nil {
				configExisted = true
				if fi.Mode().IsRegular() {
					previousConfig, err = os.ReadFile(configPath)
					if err != nil {
						return err
					}
					previousConfigMode = fi.Mode().Perm()
					previousConfigRegular = true
				}
			} else if !os.IsNotExist(err) {
				return err
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
			// Fail before creating project state if the project cannot be ignored.
			if err := ensureGitignore(cwd); err != nil {
				return err
			}
			if err := os.MkdirAll(hushDir, 0755); err != nil {
				return err
			}
			rollback := func(cause error) error {
				var cleanupErrs []error
				if err := os.Remove(project.StorePath(cwd)); err != nil && !os.IsNotExist(err) {
					cleanupErrs = append(cleanupErrs, err)
				}
				switch {
				case previousConfigRegular:
					if err := safeio.WriteFile(configPath, previousConfig, previousConfigMode); err != nil {
						cleanupErrs = append(cleanupErrs, err)
					}
				case !configExisted:
					if err := os.Remove(configPath); err != nil && !os.IsNotExist(err) {
						cleanupErrs = append(cleanupErrs, err)
					}
				}
				if !hushDirExisted {
					if err := os.Remove(hushDir); err != nil && !os.IsNotExist(err) {
						cleanupErrs = append(cleanupErrs, err)
					}
				}
				return errors.Join(append([]error{cause}, cleanupErrs...)...)
			}
			if err := project.SaveConfig(cwd, project.Config{ProjectID: id, ActiveEnv: "development"}); err != nil {
				return rollback(err)
			}
			doc := store.NewDocument(id, name, a.Now())
			if err := store.Save(project.StorePath(cwd), doc, raw); err != nil {
				return rollback(err)
			}
			if err := a.Ring.Set(keyring.Service, id, keyStr); err != nil {
				deleteErr := a.Ring.Delete(keyring.Service, id)
				return rollback(errors.Join(err, deleteErr))
			}
			fmt.Fprintf(a.Stdout, "Initialized hush project %q (gitignored .hush/).\n", name)
			fmt.Fprintln(a.Stdout, "Key stored in keychain. Next: hush import .env")
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "project name (default: directory name)")
	return cmd
}
