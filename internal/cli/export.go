package cli

import (
	"errors"
	"fmt"
	"os"

	"github.com/cfaulkingham/hush/internal/dotenv"
	"github.com/cfaulkingham/hush/internal/safeio"
	"github.com/spf13/cobra"
)

func (a *App) exportCmd() *cobra.Command {
	var format, output string
	var overwrite, force bool
	cmd := &cobra.Command{
		Use:   "export",
		Args:  cobra.NoArgs,
		Short: "Export secrets",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, _, _, doc, envName, err := a.open(cmd)
			if err != nil {
				return err
			}
			m, err := doc.SecretMap(envName)
			if err != nil {
				return err
			}
			var b []byte
			switch format {
			case "", "dotenv":
				b, err = dotenv.Serialize(m)
			case "json":
				b, err = dotenv.SerializeJSON(m)
			default:
				return fmt.Errorf("unknown format %q (dotenv|json)", format)
			}
			if err != nil {
				return err
			}
			if output == "" {
				_, err = a.Stdout.Write(b)
				return err
			}
			if !overwrite {
				if _, err := os.Lstat(output); err == nil {
					return fmt.Errorf("%s exists (pass --overwrite)", output)
				} else if !os.IsNotExist(err) {
					return err
				}
			}
			if !force && a.gitCheckPath(output) == gitNotIgnored {
				return fmt.Errorf("refusing to write plaintext secrets to %s: git would track it. Use a gitignored path or pass --force", output)
			}
			if err := safeio.WriteFile(output, b, 0600); err != nil {
				if errors.Is(err, safeio.ErrSymlink) {
					return fmt.Errorf("refusing to write plaintext secrets through symlink %s", output)
				}
				return err
			}
			fmt.Fprintf(a.Stderr, "wrote plaintext secrets to %s (mode 0600). Delete it when done.\n", output)
			return nil
		},
	}
	cmd.Flags().StringVar(&format, "format", "dotenv", "dotenv or json")
	cmd.Flags().StringVarP(&output, "output", "o", "", "write to file")
	cmd.Flags().BoolVar(&overwrite, "overwrite", false, "overwrite output file")
	cmd.Flags().BoolVar(&force, "force", false, "write even if the path is not gitignored")
	return cmd
}
