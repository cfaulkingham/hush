package project

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"github.com/cfaulkingham/hush/internal/safeio"
)

var (
	ErrNotFound      = errors.New("no hush project (run hush init)")
	ErrConfigSymlink = errors.New("refusing to write through symlink .hush/config.json")
	ErrDirSymlink    = errors.New("refusing to use symlink .hush directory")
)

type Config struct {
	ProjectID string `json:"project_id"`
	ActiveEnv string `json:"active_env"`
}

type Project struct {
	Root   string
	Config Config
}

func Dir(root string) string        { return filepath.Join(root, ".hush") }
func StorePath(root string) string  { return filepath.Join(Dir(root), "store") }
func ConfigPath(root string) string { return filepath.Join(Dir(root), "config.json") }

// EnsureDir creates .hush if needed and refuses to use it when it is a
// symlink or not a directory, so project state never lands somewhere the
// user did not intend.
func EnsureDir(root string) error {
	dir := Dir(root)
	fi, err := os.Lstat(dir)
	if err == nil {
		if fi.Mode()&os.ModeSymlink != 0 {
			return ErrDirSymlink
		}
		if !fi.IsDir() {
			return errors.New(".hush exists and is not a directory")
		}
		return nil
	}
	if !os.IsNotExist(err) {
		return err
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	fi, err = os.Lstat(dir)
	if err != nil {
		return err
	}
	if fi.Mode()&os.ModeSymlink != 0 {
		return ErrDirSymlink
	}
	return nil
}

func Find(startDir string) (*Project, error) {
	dir, err := filepath.Abs(startDir)
	if err != nil {
		return nil, err
	}
	for {
		store := StorePath(dir)
		if fi, err := os.Stat(store); err == nil && fi.Mode().IsRegular() {
			cfg, err := LoadConfig(dir)
			if err != nil {
				return nil, err
			}
			return &Project{Root: dir, Config: cfg}, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return nil, ErrNotFound
		}
		dir = parent
	}
}

func LoadConfig(root string) (Config, error) {
	b, err := os.ReadFile(ConfigPath(root))
	if err != nil {
		return Config{}, err
	}
	var cfg Config
	if err := json.Unmarshal(b, &cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func SaveConfig(root string, cfg Config) error {
	if err := EnsureDir(root); err != nil {
		return err
	}
	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	err = safeio.WriteFile(ConfigPath(root), b, 0644)
	if errors.Is(err, safeio.ErrSymlink) {
		return ErrConfigSymlink
	}
	return err
}
