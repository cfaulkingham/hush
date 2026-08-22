package store

import (
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
	if err := d.DeleteSecret("development", "FOO"); err != nil {
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
