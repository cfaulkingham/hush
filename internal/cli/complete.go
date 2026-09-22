package cli

import (
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

// Completions run on every TAB press: they must be fast and fail silently
// when there is no project, no key, or a broken store.

func matchPrefix(names []string, toComplete string) []string {
	out := make([]string, 0, len(names))
	for _, n := range names {
		if strings.HasPrefix(n, toComplete) {
			out = append(out, n)
		}
	}
	return out
}

// completeKeys completes secret names in the resolved environment.
func (a *App) completeKeys(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	_, _, _, doc, envName, err := a.open(cmd)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	names, err := doc.ListKeys(envName)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return matchPrefix(names, toComplete), cobra.ShellCompDirectiveNoFileComp
}

// completeEnvs completes environment names (positional args and --env).
func (a *App) completeEnvs(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	_, _, _, doc, err := a.loadStore()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	names := make([]string, 0, len(doc.Environments))
	for name := range doc.Environments {
		names = append(names, name)
	}
	sort.Strings(names)
	return matchPrefix(names, toComplete), cobra.ShellCompDirectiveNoFileComp
}
