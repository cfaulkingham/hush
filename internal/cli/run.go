package cli

import (
	"github.com/cfaulkingham/hush/internal/run"
	"github.com/spf13/cobra"
)

func (a *App) runCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run [--] <command> [args...]",
		Short: "Run a command with secrets in the environment",
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return run.ErrNoCommand
			}
			_, _, _, doc, envName, err := a.open(cmd)
			if err != nil {
				return err
			}
			secrets, err := doc.SecretMap(envName)
			if err != nil {
				return err
			}
			env := run.Overlay(a.Environ(), secrets)
			return a.Exec(args, env)
		},
	}
	cmd.Flags().SetInterspersed(false)
	return cmd
}
