package cli

import (
	"fmt"
	"sort"

	"github.com/spf13/cobra"
	"hush/internal/project"
)

func (a *App) envCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "env", Short: "Manage environments"}
	cmd.AddCommand(a.envLsCmd(), a.envNewCmd())
	return cmd
}

func (a *App) envLsCmd() *cobra.Command {
	return &cobra.Command{
		Use:  "ls",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			_, p, _, doc, err := a.loadStore()
			if err != nil {
				return err
			}
			names := make([]string, 0, len(doc.Environments))
			width := 0
			for name := range doc.Environments {
				names = append(names, name)
				if len(name) > width {
					width = len(name)
				}
			}
			sort.Strings(names)
			for _, name := range names {
				n := 0
				if env := doc.Environments[name]; env != nil {
					n = len(env.Secrets)
				}
				mark := ""
				if name == p.Config.ActiveEnv {
					mark = "  ●"
				}
				fmt.Fprintf(a.Stdout, "  %-*s  %d secrets%s\n", width, name, n, mark)
			}
			return nil
		},
	}
}

func (a *App) envNewCmd() *cobra.Command {
	var use bool
	cmd := &cobra.Command{
		Use:   "new <name>",
		Args:  cobra.ExactArgs(1),
		Short: "Create an empty environment",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, p, key, doc, err := a.loadStore()
			if err != nil {
				return err
			}
			if err := doc.NewEnv(args[0], a.Now()); err != nil {
				return err
			}
			if err := a.save(p, doc, key); err != nil {
				return err
			}
			fmt.Fprintf(a.Stdout, "Created environment %s.\n", args[0])
			if use {
				p.Config.ActiveEnv = args[0]
				if err := project.SaveConfig(p.Root, p.Config); err != nil {
					return err
				}
				fmt.Fprintf(a.Stdout, "Now using %s.\n", args[0])
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&use, "use", false, "switch to the new environment")
	return cmd
}
