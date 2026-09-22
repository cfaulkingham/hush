package cli

import (
	"fmt"
	"os"

	"github.com/cfaulkingham/hush/internal/keyring"
	"github.com/cfaulkingham/hush/internal/project"
	"github.com/spf13/cobra"
)

func (a *App) deinitCmd() *cobra.Command {
	var force, yes, dryRun bool
	cmd := &cobra.Command{
		Use:   "deinit",
		Args:  cobra.NoArgs,
		Short: "Remove hush from this project (deletes the secrets and the keychain entry)",
		Long: "Deletes .hush/ (the encrypted store goes with it) and the project's\n" +
			"keychain entry. The .gitignore line is left in place. Anyone holding a\n" +
			"`hush key backup` copy keeps the ability to decrypt a copy of the store.",
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, err := a.Getwd()
			if err != nil {
				return err
			}
			p, err := project.Find(cwd)
			if err != nil {
				return err
			}
			n := a.countSecrets(p)
			if dryRun {
				fmt.Fprintf(a.Stdout, "would delete %s (%d secrets) and the keychain entry; nothing written.\n", project.Dir(p.Root), n)
				return nil
			}
			ok, err := a.confirm(fmt.Sprintf("Delete %s (%d secrets) and the keychain entry? [y/N] ", project.Dir(p.Root), n), force || yes)
			if err != nil {
				return err
			}
			if !ok {
				fmt.Fprintln(a.Stdout, "Aborted.")
				return nil
			}
			if err := os.RemoveAll(project.Dir(p.Root)); err != nil {
				return err
			}
			if a.hushKey() != "" {
				fmt.Fprintln(a.Stderr, "HUSH_KEY is set; keychain entry untouched")
			} else if err := a.Ring.Delete(keyring.Service, p.Config.ProjectID); err != nil {
				return fmt.Errorf(".hush/ deleted but keychain entry could not be removed: %w", err)
			}
			fmt.Fprintln(a.Stdout, "Deinitialized hush. .gitignore may still list .hush/.")
			return nil
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "skip the confirmation prompt")
	cmd.Flags().BoolVar(&yes, "yes", false, "skip the confirmation prompt")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "show what would be deleted without writing")
	return cmd
}

// countSecrets is best-effort for the confirmation prompt: a missing key or a
// broken store must not block deinit (removing hush is the escape hatch).
func (a *App) countSecrets(p *project.Project) int {
	key, _, err := keyring.Resolve(a.Ring, p.Config.ProjectID, a.hushKey())
	if err != nil {
		return 0
	}
	doc, err := loadProjectDocument(p, key)
	if err != nil {
		return 0
	}
	total := 0
	for _, env := range doc.Environments {
		if env != nil {
			total += len(env.Secrets)
		}
	}
	return total
}
