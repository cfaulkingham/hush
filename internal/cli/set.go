package cli

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/cfaulkingham/hush/internal/store"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

func (a *App) setCmd() *cobra.Command {
	var fromStdin bool
	cmd := &cobra.Command{
		Use:   "set <KEY=VALUE|KEY>",
		Args:  cobra.ExactArgs(1),
		Short: "Set a secret",
		RunE: func(cmd *cobra.Command, args []string) error {
			name, _, inline := strings.Cut(args[0], "=")
			if fromStdin && inline {
				return usage(fmt.Errorf("pass the key name only with --from-stdin (got %q)", name))
			}
			if inline {
				fmt.Fprintln(a.Stderr, "warning: KEY=VALUE on the command line leaks the value into shell history and process listings.")
				fmt.Fprintln(a.Stderr, "Prefer `hush set KEY` (hidden prompt) or `hush set KEY --from-stdin`.")
			}
			_, p, key, err := a.projectKey()
			if err != nil {
				return err
			}
			envName := a.resolvedEnv(cmd, p)
			name, value, err := a.readSetValue(args[0], fromStdin)
			if err != nil {
				return err
			}
			if err := a.updateStore(p, key, func(doc *store.Document) error {
				return doc.PutSecret(envName, name, value, a.Now())
			}); err != nil {
				return err
			}
			fmt.Fprintf(a.Stdout, "Set %s in %s.\n", name, envName)
			return nil
		},
	}
	cmd.Flags().BoolVar(&fromStdin, "from-stdin", false, "read value from stdin")
	return cmd
}

func (a *App) readSetValue(arg string, fromStdin bool) (string, string, error) {
	if i := strings.IndexByte(arg, '='); i >= 0 {
		return arg[:i], arg[i+1:], nil
	}
	name := arg
	if fromStdin {
		b, err := io.ReadAll(a.Stdin)
		if err != nil {
			return "", "", err
		}
		if len(b) > 0 && b[len(b)-1] == '\n' {
			b = b[:len(b)-1]
		}
		if len(b) > 0 && b[len(b)-1] == '\r' {
			b = b[:len(b)-1]
		}
		return name, string(b), nil
	}
	f, ok := a.Stdin.(*os.File)
	if !ok || !term.IsTerminal(int(f.Fd())) {
		return "", "", fmt.Errorf("no value: pass KEY=VALUE, --from-stdin, or run on a TTY")
	}
	fmt.Fprintf(a.Stderr, "Value: ")
	pw, err := term.ReadPassword(int(f.Fd()))
	fmt.Fprintln(a.Stderr)
	if err != nil {
		return "", "", err
	}
	return name, string(pw), nil
}
