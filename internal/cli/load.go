package cli

import (
	"errors"
	"fmt"
	"os"

	"github.com/cfaulkingham/hush/internal/keyring"
	"github.com/cfaulkingham/hush/internal/project"
	"github.com/cfaulkingham/hush/internal/store"
	"github.com/spf13/cobra"
)

var ErrProjectMismatch = errors.New("project_id mismatch between config and store")

func (a *App) projectKey() (string, *project.Project, []byte, error) {
	cwd, err := a.Getwd()
	if err != nil {
		return "", nil, nil, err
	}
	p, err := project.Find(cwd)
	if err != nil {
		return "", nil, nil, err
	}
	key, err := keyring.Resolve(a.Ring, p.Config.ProjectID)
	if err != nil {
		return "", nil, nil, err
	}
	return cwd, p, key, nil
}

func loadProjectDocument(p *project.Project, key []byte) (*store.Document, error) {
	doc, err := store.Load(project.StorePath(p.Root), key)
	if err != nil {
		return nil, err
	}
	if doc.ProjectID != p.Config.ProjectID {
		return nil, fmt.Errorf("%w: config has %q, store has %q", ErrProjectMismatch, p.Config.ProjectID, doc.ProjectID)
	}
	return doc, nil
}

func (a *App) loadStore() (string, *project.Project, []byte, *store.Document, error) {
	cwd, p, key, err := a.projectKey()
	if err != nil {
		return "", nil, nil, nil, err
	}
	doc, err := loadProjectDocument(p, key)
	if err != nil {
		return "", nil, nil, nil, err
	}
	return cwd, p, key, doc, nil
}

func (a *App) open(cmd *cobra.Command) (string, *project.Project, []byte, *store.Document, string, error) {
	cwd, p, key, doc, err := a.loadStore()
	if err != nil {
		return "", nil, nil, nil, "", err
	}
	env := a.resolvedEnv(cmd, p)
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

func (a *App) updateStore(p *project.Project, key []byte, update func(*store.Document) error) error {
	return store.Update(project.StorePath(p.Root), key, func(doc *store.Document) error {
		if doc.ProjectID != p.Config.ProjectID {
			return fmt.Errorf("%w: config has %q, store has %q", ErrProjectMismatch, p.Config.ProjectID, doc.ProjectID)
		}
		return update(doc)
	})
}

func (a *App) resolvedEnv(cmd *cobra.Command, p *project.Project) string {
	env := a.envFlag(cmd)
	if env == "" {
		env = p.Config.ActiveEnv
	}
	return env
}
