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

func (d *Document) DeleteSecret(env, key string) error {
	e, err := d.Env(env)
	if err != nil {
		return err
	}
	if _, ok := e.Secrets[key]; !ok {
		return fmt.Errorf("%w: secret %s not found in %s", ErrSecretNotFound, key, env)
	}
	delete(e.Secrets, key)
	now := time.Now().UTC()
	e.UpdatedAt = now
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
