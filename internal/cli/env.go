package cli

import (
	"fmt"
	"sort"

	"github.com/cfaulkingham/hush/internal/project"
	"github.com/cfaulkingham/hush/internal/store"
	"github.com/spf13/cobra"
)

func (a *App) envCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "env", Short: "Manage environments"}
	cmd.AddCommand(a.envLsCmd(), a.envNewCmd(), a.envCopyCmd(), a.envRenameCmd(), a.envRmCmd())
	return cmd
}

func (a *App) envLsCmd() *cobra.Command {
	var jsonOut bool
	cmd := &cobra.Command{
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
			if jsonOut {
				out := envListJSON{Environments: make([]envJSON, 0, len(names))}
				for _, name := range names {
					n := 0
					if env := doc.Environments[name]; env != nil {
						n = len(env.Secrets)
					}
					out.Environments = append(out.Environments, envJSON{Name: name, Secrets: n, Active: name == p.Config.ActiveEnv})
				}
				return writeJSON(a.Stdout, out)
			}
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
	cmd.Flags().BoolVar(&jsonOut, "json", false, "machine-readable output")
	return cmd
}

type envJSON struct {
	Name    string `json:"name"`
	Secrets int    `json:"secrets"`
	Active  bool   `json:"active"`
}

type envListJSON struct {
	Environments []envJSON `json:"environments"`
}

func (a *App) envNewCmd() *cobra.Command {
	var use bool
	cmd := &cobra.Command{
		Use:   "new <name>",
		Args:  cobra.ExactArgs(1),
		Short: "Create an empty environment",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, p, key, err := a.projectKey()
			if err != nil {
				return err
			}
			if err := a.updateStore(p, key, func(doc *store.Document) error {
				return doc.NewEnv(args[0], a.Now())
			}); err != nil {
				return err
			}
			fmt.Fprintf(a.Stdout, "Created environment %s.\n", args[0])
			if use {
				p.Config.ActiveEnv = args[0]
				if err := project.SaveConfig(p.Root, p.Config); err != nil {
					return fmt.Errorf("environment %s was created but not activated: %w", args[0], err)
				}
				fmt.Fprintf(a.Stdout, "Now using %s.\n", args[0])
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&use, "use", false, "switch to the new environment")
	return cmd
}

func (a *App) envCopyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "copy <from> <to>",
		Args:  cobra.ExactArgs(2),
		Short: "Seed a new environment from an existing one (values stay in the store)",
		RunE: func(cmd *cobra.Command, args []string) error {
			from, to := args[0], args[1]
			_, p, key, err := a.projectKey()
			if err != nil {
				return err
			}
			copied := 0
			if err := a.updateStore(p, key, func(doc *store.Document) error {
				if err := doc.CopyEnv(from, to, a.Now()); err != nil {
					return err
				}
				env, err := doc.Env(to)
				if err != nil {
					return err
				}
				copied = len(env.Secrets)
				return nil
			}); err != nil {
				return err
			}
			fmt.Fprintf(a.Stdout, "Copied %s to %s (%d secrets).\n", from, to, copied)
			return nil
		},
	}
	cmd.ValidArgsFunction = a.completeEnvs
	return cmd
}

func (a *App) envRenameCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rename <old> <new>",
		Args:  cobra.ExactArgs(2),
		Short: "Rename an environment",
		RunE: func(cmd *cobra.Command, args []string) error {
			oldName, newName := args[0], args[1]
			_, p, key, err := a.projectKey()
			if err != nil {
				return err
			}
			if err := a.updateStore(p, key, func(doc *store.Document) error {
				return doc.RenameEnv(oldName, newName, a.Now())
			}); err != nil {
				return err
			}
			fmt.Fprintf(a.Stdout, "Renamed %s to %s.\n", oldName, newName)
			if p.Config.ActiveEnv == oldName {
				p.Config.ActiveEnv = newName
				if err := project.SaveConfig(p.Root, p.Config); err != nil {
					return fmt.Errorf("environment was renamed but the active environment was not updated: %w", err)
				}
				fmt.Fprintf(a.Stdout, "Now using %s.\n", newName)
			}
			return nil
		},
	}
	cmd.ValidArgsFunction = a.completeEnvs
	return cmd
}

func (a *App) envRmCmd() *cobra.Command {
	var force, yes, dryRun bool
	cmd := &cobra.Command{
		Use:   "rm <name>",
		Args:  cobra.ExactArgs(1),
		Short: "Remove an environment and its secrets",
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			_, p, key, doc, err := a.loadStore()
			if err != nil {
				return err
			}
			if name == p.Config.ActiveEnv {
				return fmt.Errorf("%s is the active environment; `hush use <other>` first", name)
			}
			env, err := doc.Env(name)
			if err != nil {
				return err
			}
			if dryRun {
				fmt.Fprintf(a.Stdout, "would remove %s (%d secrets); nothing written.\n", name, len(env.Secrets))
				return nil
			}
			ok, err := a.confirm(fmt.Sprintf("Remove environment %s and its %d secrets? [y/N] ", name, len(env.Secrets)), force || yes)
			if err != nil {
				return err
			}
			if !ok {
				fmt.Fprintln(a.Stdout, "Aborted.")
				return nil
			}
			n := len(env.Secrets)
			if err := a.updateStore(p, key, func(doc *store.Document) error {
				return doc.RemoveEnv(name, a.Now())
			}); err != nil {
				return err
			}
			fmt.Fprintf(a.Stdout, "Removed environment %s (%d secrets).\n", name, n)
			return nil
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "skip the confirmation prompt")
	cmd.Flags().BoolVar(&yes, "yes", false, "skip the confirmation prompt")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "show what would be removed without writing")
	cmd.ValidArgsFunction = a.completeEnvs
	return cmd
}
