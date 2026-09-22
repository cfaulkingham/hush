package cli

import (
	"fmt"

	"github.com/cfaulkingham/hush/internal/project"
	"github.com/spf13/cobra"
)

func (a *App) useCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "use <env>",
		Args:  cobra.ExactArgs(1),
		Short: "Set the active environment",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, p, _, doc, err := a.loadStore()
			if err != nil {
				return err
			}
			env, err := doc.Env(args[0])
			if err != nil {
				return err
			}
			p.Config.ActiveEnv = args[0]
			if err := project.SaveConfig(p.Root, p.Config); err != nil {
				return err
			}
			fmt.Fprintf(a.Stdout, "Now using %s (%d secrets).\n", args[0], len(env.Secrets))
			return nil
		},
	}
	cmd.ValidArgsFunction = a.completeEnvs
	return cmd
}
