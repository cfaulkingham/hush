package store

import (
	"errors"
	"testing"
	"time"
)

func TestPutGetDeleteSecret(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	d := NewDocument("id", "api", now)
	if err := d.PutSecret("development", "FOO", "bar", now); err != nil {
		t.Fatal(err)
	}
	v, err := d.GetSecret("development", "FOO")
	if err != nil || v != "bar" {
		t.Fatalf("got %q %v", v, err)
	}
	keys, err := d.ListKeys("development")
	if err != nil || len(keys) != 1 || keys[0] != "FOO" {
		t.Fatalf("keys %v %v", keys, err)
	}
	m, err := d.SecretMap("development")
	if err != nil || m["FOO"] != "bar" {
		t.Fatalf("map %v %v", m, err)
	}
	if err := d.DeleteSecret("development", "FOO", now); err != nil {
		t.Fatal(err)
	}
	if _, err := d.GetSecret("development", "FOO"); err == nil {
		t.Fatal("expected missing")
	}
}

func TestPutSecretMissingEnv(t *testing.T) {
	d := NewDocument("id", "api", time.Now().UTC())
	if err := d.PutSecret("staging", "FOO", "bar", time.Now().UTC()); err == nil {
		t.Fatal("expected error")
	}
}

func TestNewEnv(t *testing.T) {
	now := time.Now().UTC()
	d := NewDocument("id", "api", now)
	if err := d.NewEnv("staging", now); err != nil {
		t.Fatal(err)
	}
	if err := d.NewEnv("staging", now); err == nil {
		t.Fatal("expected duplicate error")
	}
	if err := d.PutSecret("staging", "FOO", "x", now); err != nil {
		t.Fatal(err)
	}
}

func TestCopyRenameRemoveEnv(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	d := NewDocument("id", "api", now)
	if err := d.PutSecret("development", "FOO", "bar", now); err != nil {
		t.Fatal(err)
	}
	if err := d.CopyEnv("development", "staging", now); err != nil {
		t.Fatal(err)
	}
	v, err := d.GetSecret("staging", "FOO")
	if err != nil || v != "bar" {
		t.Fatalf("copy: %q %v", v, err)
	}
	if err := d.CopyEnv("development", "staging", now); err == nil {
		t.Fatal("expected duplicate copy error")
	}
	if err := d.CopyEnv("nope", "x", now); err == nil {
		t.Fatal("expected missing source error")
	}
	if err := d.RenameEnv("staging", "prod", now); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Env("staging"); err == nil {
		t.Fatal("old name survived rename")
	}
	if v, _ := d.GetSecret("prod", "FOO"); v != "bar" {
		t.Fatal("rename lost secrets")
	}
	if err := d.RenameEnv("prod", "development", now); err == nil {
		t.Fatal("expected rename onto existing error")
	}
	if err := d.RemoveEnv("prod", now); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Env("prod"); err == nil {
		t.Fatal("env survived remove")
	}
	if err := d.RemoveEnv("development", now); !errors.Is(err, ErrLastEnv) {
		t.Fatalf("last env: %v", err)
	}
}
