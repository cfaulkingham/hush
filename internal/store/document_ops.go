package store

import (
	"errors"
	"fmt"
	"sort"
	"time"
)

var (
	ErrSecretNotFound = errors.New("secret not found")
	ErrEnvNotFound    = errors.New("environment not found")
	ErrEnvExists      = errors.New("environment already exists")
	ErrLastEnv        = errors.New("refusing to remove the last environment")
)

func (d *Document) Env(name string) (*Environment, error) {
	if err := ValidateEnvName(name); err != nil {
		return nil, err
	}
	env, ok := d.Environments[name]
	if !ok {
		return nil, fmt.Errorf("%w: environment %s not found. Create it with: hush env new %s", ErrEnvNotFound, name, name)
	}
	return env, nil
}

func (d *Document) NewEnv(name string, now time.Time) error {
	if err := ValidateEnvName(name); err != nil {
		return err
	}
	if _, ok := d.Environments[name]; ok {
		return fmt.Errorf("%w: environment %s already exists", ErrEnvExists, name)
	}
	now = now.UTC()
	d.Environments[name] = &Environment{UpdatedAt: now, Secrets: map[string]Secret{}}
	d.UpdatedAt = now
	return nil
}

func (d *Document) PutSecret(env, key, value string, now time.Time) error {
	if err := ValidateKey(key); err != nil {
		return err
	}
	if err := ValidateValue(value); err != nil {
		return err
	}
	e, err := d.Env(env)
	if err != nil {
		return err
	}
	now = now.UTC()
	e.Secrets[key] = Secret{Value: value, UpdatedAt: now}
	e.UpdatedAt = now
	d.UpdatedAt = now
	return nil
}

func (d *Document) GetSecret(env, key string) (string, error) {
	e, err := d.Env(env)
	if err != nil {
		return "", err
	}
	sec, ok := e.Secrets[key]
	if !ok {
		return "", fmt.Errorf("%w: secret %s not found in %s", ErrSecretNotFound, key, env)
	}
	return sec.Value, nil
}

func (d *Document) DeleteSecret(env, key string, now time.Time) error {
	e, err := d.Env(env)
	if err != nil {
		return err
	}
	if _, ok := e.Secrets[key]; !ok {
		return fmt.Errorf("%w: secret %s not found in %s", ErrSecretNotFound, key, env)
	}
	delete(e.Secrets, key)
	now = now.UTC()
	e.UpdatedAt = now
	d.UpdatedAt = now
	return nil
}

// RemoveEnv deletes an environment and all of its secrets.
func (d *Document) RemoveEnv(name string, now time.Time) error {
	if err := ValidateEnvName(name); err != nil {
		return err
	}
	if _, ok := d.Environments[name]; !ok {
		return fmt.Errorf("%w: environment %s not found", ErrEnvNotFound, name)
	}
	if len(d.Environments) <= 1 {
		return ErrLastEnv
	}
	delete(d.Environments, name)
	now = now.UTC()
	d.UpdatedAt = now
	return nil
}

// RenameEnv moves an environment and its secrets to a new name.
func (d *Document) RenameEnv(oldName, newName string, now time.Time) error {
	if err := ValidateEnvName(newName); err != nil {
		return err
	}
	e, err := d.Env(oldName)
	if err != nil {
		return err
	}
	if oldName == newName {
		return nil
	}
	if _, ok := d.Environments[newName]; ok {
		return fmt.Errorf("%w: environment %s already exists", ErrEnvExists, newName)
	}
	delete(d.Environments, oldName)
	d.Environments[newName] = e
	now = now.UTC()
	e.UpdatedAt = now
	d.UpdatedAt = now
	return nil
}

// CopyEnv seeds a new environment with the secrets of an existing one,
// keeping values in the encrypted store (never on disk as plaintext).
func (d *Document) CopyEnv(from, to string, now time.Time) error {
	src, err := d.Env(from)
	if err != nil {
		return err
	}
	if err := ValidateEnvName(to); err != nil {
		return err
	}
	if _, ok := d.Environments[to]; ok {
		return fmt.Errorf("%w: environment %s already exists", ErrEnvExists, to)
	}
	now = now.UTC()
	secrets := make(map[string]Secret, len(src.Secrets))
	for k, s := range src.Secrets {
		secrets[k] = Secret{Value: s.Value, UpdatedAt: now}
	}
	d.Environments[to] = &Environment{UpdatedAt: now, Secrets: secrets}
	d.UpdatedAt = now
	return nil
}

func (d *Document) ListKeys(env string) ([]string, error) {
	e, err := d.Env(env)
	if err != nil {
		return nil, err
	}
	keys := make([]string, 0, len(e.Secrets))
	for k := range e.Secrets {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys, nil
}

func (d *Document) SecretMap(env string) (map[string]string, error) {
	e, err := d.Env(env)
	if err != nil {
		return nil, err
	}
	m := make(map[string]string, len(e.Secrets))
	for k, sec := range e.Secrets {
		m[k] = sec.Value
	}
	return m, nil
}
