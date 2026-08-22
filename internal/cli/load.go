package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"hush/internal/keyring"
	"hush/internal/project"
	"hush/internal/store"
)

func (a *App) loadStore() (string, *project.Project, []byte, *store.Document, error) {
	cwd, err := a.Getwd()
	if err != nil {
		return "", nil, nil, nil, err
	}
	p, err := project.Find(cwd)
	if err != nil {
		return "", nil, nil, nil, err
	}
	key, err := keyring.Resolve(a.Ring, p.Config.ProjectID)
	if err != nil {
		return "", nil, nil, nil, err
	}
	doc, err := store.Load(project.StorePath(p.Root), key)
	if err != nil {
		return "", nil, nil, nil, err
	}
	if doc.ProjectID != p.Config.ProjectID {
		return "", nil, nil, nil, fmt.Errorf("project_id mismatch between config and store")
	}
	return cwd, p, key, doc, nil
}

func (a *App) open(cmd *cobra.Command) (string, *project.Project, []byte, *store.Document, string, error) {
	cwd, p, key, doc, err := a.loadStore()
	if err != nil {
		return "", nil, nil, nil, "", err
	}
	env := a.envFlag(cmd)
	if env == "" {
		env = p.Config.ActiveEnv
	}
	if _, err := doc.Env(env); err != nil {
		return "", nil, nil, nil, "", err
	}
	return cwd, p, key, doc, env, nil
}

func keySource() string {
	if os.Getenv("HUSH_KEY") != "" {
		return "HUSH_KEY"
	}
	return "keychain"
}

func (a *App) save(p *project.Project, doc *store.Document, key []byte) error {
	return store.Save(project.StorePath(p.Root), doc, key)
}
