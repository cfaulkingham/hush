package project

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

var ErrNotFound = errors.New("no hush project (run hush init)")

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
	if err := os.MkdirAll(Dir(root), 0755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	return os.WriteFile(ConfigPath(root), b, 0644)
}
