package cli

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/cfaulkingham/hush/internal/keyring"
	"github.com/cfaulkingham/hush/internal/run"
	"github.com/spf13/cobra"
)

type App struct {
	Ring    keyring.Ring
	Stdout  io.Writer
	Stderr  io.Writer
	Stdin   io.Reader
	Getwd   func() (string, error)
	Environ func() []string
	Exec    func(argv, env []string) error
	Now     func() time.Time
	// ReadSecret reads one secret without echoing it (terminal prompt).
	ReadSecret func(prompt string) (string, error)
	// Ask reads one visible line (confirmation prompts). nil means
	// non-interactive: confirmations proceed (use --dry-run to preview).
	Ask func(prompt string) (string, error)
	// GitCheck reports how git would treat a path before plaintext writes.
	GitCheck func(path string) gitVerdict
	// Getenv reads the process environment (HUSH_KEY override). nil falls
	// back to os.Getenv.
	Getenv func(name string) string
}

func NewOSApp() *App {
	return &App{
		Ring:       keyring.OS{},
		Stdout:     os.Stdout,
		Stderr:     os.Stderr,
		Stdin:      os.Stdin,
		Getwd:      os.Getwd,
		Environ:    os.Environ,
		Exec:       run.Exec,
		Now:        time.Now,
		ReadSecret: readTerminalSecret,
		Ask:        askTerminal,
		GitCheck:   gitCheck,
		Getenv:     os.Getenv,
	}
}

// hushKey returns the HUSH_KEY override ("" when unset), routed through the
// injectable environment.
func (a *App) hushKey() string {
	if a.Getenv != nil {
		return a.Getenv("HUSH_KEY")
	}
	return os.Getenv("HUSH_KEY")
}

func (a *App) Root() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "hush",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	cmd.Version = version
	cmd.SetVersionTemplate("hush {{.Version}}\n")
	cmd.SetOut(a.Stdout)
	cmd.SetErr(a.Stderr)
	cmd.SetFlagErrorFunc(func(c *cobra.Command, err error) error { return usage(err) })
	cmd.PersistentFlags().String("env", "", "environment (overrides active)")
	_ = cmd.RegisterFlagCompletionFunc("env", a.completeEnvs)
	cmd.AddCommand(a.initCmd(), a.deinitCmd(), a.statusCmd(), a.useCmd(), a.envCmd(), a.setCmd(), a.getCmd(), a.lsCmd(), a.rmCmd(), a.importCmd(), a.exportCmd(), a.runCmd(), a.keyCmd(), a.versionCmd())
	tagUsage(cmd)
	return cmd
}

// unknownCommand rejects stray positional arguments on command groups.
func unknownCommand(cmd *cobra.Command, args []string) error {
	if len(args) > 0 {
		return usage(fmt.Errorf("unknown command %q for %q", args[0], cmd.CommandPath()))
	}
	return nil
}

// tagUsage wires argument handling so mistakes are usage errors (exit 2):
// non-runnable command groups (hush, hush env, hush key) reject unknown
// subcommands instead of silently showing help with exit 0, and every
// command's Args validator error is classified as a usage error.
func tagUsage(cmd *cobra.Command) {
	if cmd.HasSubCommands() && !cmd.Runnable() {
		if cmd.Args == nil {
			cmd.Args = unknownCommand
		}
		cmd.RunE = func(c *cobra.Command, argv []string) error { return c.Help() }
	}
	if args := cmd.Args; args != nil {
		cmd.Args = func(c *cobra.Command, argv []string) error {
			if err := args(c, argv); err != nil {
				return usage(err)
			}
			return nil
		}
	}
	for _, sub := range cmd.Commands() {
		tagUsage(sub)
	}
}

func Main(args []string) int {
	return NewOSApp().Run(args)
}

func (a *App) Run(args []string) int {
	cmd := a.Root()
	cmd.SetArgs(args)
	if err := cmd.Execute(); err != nil {
		if msg := err.Error(); msg != "" {
			fmt.Fprintln(a.Stderr, msg)
		}
		return exitCode(err)
	}
	return 0
}

func (a *App) envFlag(cmd *cobra.Command) string {
	v, _ := cmd.Flags().GetString("env")
	return v
}
