package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"hush/internal/dotenv"
)

func (a *App) exportCmd() *cobra.Command {
	var format, output string
	var overwrite bool
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
			flags := os.O_WRONLY | os.O_CREATE | os.O_EXCL
			if overwrite {
				flags = os.O_WRONLY | os.O_CREATE | os.O_TRUNC
			}
			f, err := os.OpenFile(output, flags, 0600)
			if err != nil {
				if os.IsExist(err) {
					return fmt.Errorf("%s exists (pass --overwrite)", output)
				}
				return err
			}
			defer f.Close()
			if _, err := f.Write(b); err != nil {
				return err
			}
			return os.Chmod(output, 0600)
		},
	}
	cmd.Flags().StringVar(&format, "format", "dotenv", "dotenv or json")
	cmd.Flags().StringVarP(&output, "output", "o", "", "write to file")
	cmd.Flags().BoolVar(&overwrite, "overwrite", false, "overwrite output file")
	return cmd
}
