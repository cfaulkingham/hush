package cli

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/cfaulkingham/hush/internal/run"
	"github.com/spf13/cobra"
)

func (a *App) runCmd() *cobra.Command {
	var require, requireFile string
	cmd := &cobra.Command{
		Use:   "run [--] <command> [args...]",
		Short: "Run a command with secrets in the environment",
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return usage(run.ErrNoCommand)
			}
			_, _, _, doc, envName, err := a.open(cmd)
			if err != nil {
				return err
			}
			secrets, err := doc.SecretMap(envName)
			if err != nil {
				return err
			}
			missing, err := missingSecrets(secrets, require, requireFile, a.Stdin)
			if err != nil {
				return err
			}
			if len(missing) > 0 {
				return fmt.Errorf("missing required secrets in %s: %s", envName, strings.Join(missing, ", "))
			}
			env := run.Overlay(a.Environ(), secrets)
			return a.Exec(args, env)
		},
	}
	cmd.Flags().SetInterspersed(false)
	cmd.Flags().StringVar(&require, "require", "", "comma-separated secret names that must be set")
	cmd.Flags().StringVar(&requireFile, "require-file", "", "dotenv or .json file whose keys must be set (values ignored)")
	return cmd
}

// missingSecrets returns required key names that are missing or empty. Only
// names are ever reported — never values.
func missingSecrets(secrets map[string]string, require, requireFile string, stdin io.Reader) ([]string, error) {
	need := map[string]bool{}
	for _, k := range strings.Split(require, ",") {
		k = strings.TrimSpace(k)
		if k != "" {
			need[k] = true
		}
	}
	if requireFile != "" {
		format := "dotenv"
		if strings.HasSuffix(requireFile, ".json") {
			format = "json"
		}
		parsed, err := parseImport(requireFile, format, stdin)
		if err != nil {
			return nil, err
		}
		for k := range parsed {
			need[k] = true
		}
	}
	missing := make([]string, 0, len(need))
	for k := range need {
		if v, ok := secrets[k]; !ok || v == "" {
			missing = append(missing, k)
		}
	}
	sort.Strings(missing)
	return missing, nil
}
