package cli

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/spf13/cobra"
	"hush/internal/keyring"
	"hush/internal/run"
	"hush/internal/ui"
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
}

func NewOSApp() *App {
	return &App{
		Ring:    keyring.OS{},
		Stdout:  os.Stdout,
		Stderr:  os.Stderr,
		Stdin:   os.Stdin,
		Getwd:   os.Getwd,
		Environ: os.Environ,
		Exec:    run.Exec,
		Now:     time.Now,
	}
}

func (a *App) Root() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "hush",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	cmd.PersistentFlags().String("env", "", "environment (overrides active)")
	cmd.PersistentFlags().Bool("plain", false, "disable color")
	cmd.AddCommand(a.initCmd(), a.statusCmd(), a.useCmd(), a.envCmd())
	return cmd
}

func Main(args []string) int {
	app := NewOSApp()
	cmd := app.Root()
	cmd.SetArgs(args)
	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(app.Stderr, err.Error())
		return exitCode(err)
	}
	return 0
}

func (a *App) color(cmd *cobra.Command) bool {
	plain, _ := cmd.Flags().GetBool("plain")
	return ui.ColorOn(plain)
}

func (a *App) envFlag(cmd *cobra.Command) string {
	v, _ := cmd.Flags().GetString("env")
	return v
}
