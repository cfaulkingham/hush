package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestSetGetLsRm(t *testing.T) {
	dir := t.TempDir()
	app, out, _, _ := newTestApp(t, dir)
	if err := runApp(t, app, "init", "--name", "api"); err != nil {
		t.Fatal(err)
	}
	if err := runApp(t, app, "set", "DATABASE_URL=postgres://localhost/api"); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "postgres://") {
		t.Fatal("set leaked value")
	}
	out.Reset()
	if err := runApp(t, app, "get", "DATABASE_URL"); err != nil {
		t.Fatal(err)
	}
	if out.String() != "postgres://localhost/api\n" {
		t.Fatalf("get %q", out.String())
	}
	out.Reset()
	if err := runApp(t, app, "ls"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "DATABASE_URL") {
		t.Fatalf("%s", out.String())
	}
	if strings.Contains(out.String(), "postgres://") {
		t.Fatal("ls leaked value")
	}
	out.Reset()
	if err := runApp(t, app, "ls", "--values"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "postgres://localhost/api") {
		t.Fatalf("%s", out.String())
	}
	if err := runApp(t, app, "rm", "DATABASE_URL"); err != nil {
		t.Fatal(err)
	}
	if err := runApp(t, app, "get", "DATABASE_URL"); err == nil {
		t.Fatal("expected missing")
	}
}

func TestSetFromStdin(t *testing.T) {
	dir := t.TempDir()
	app, _, _, _ := newTestApp(t, dir)
	app.Stdin = bytes.NewBufferString("s3cret\n")
	if err := runApp(t, app, "init"); err != nil {
		t.Fatal(err)
	}
	if err := runApp(t, app, "set", "TOKEN", "--from-stdin"); err != nil {
		t.Fatal(err)
	}
	out := &bytes.Buffer{}
	app.Stdout = out
	if err := runApp(t, app, "get", "TOKEN"); err != nil {
		t.Fatal(err)
	}
	if out.String() != "s3cret\n" {
		t.Fatalf("%q", out.String())
	}
}

func TestSetEqualsInValue(t *testing.T) {
	dir := t.TempDir()
	app, out, _, _ := newTestApp(t, dir)
	_ = runApp(t, app, "init")
	if err := runApp(t, app, "set", "K=a=b=c"); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	_ = runApp(t, app, "get", "K")
	if out.String() != "a=b=c\n" {
		t.Fatalf("%q", out.String())
	}
}

func TestGetMissing(t *testing.T) {
	dir := t.TempDir()
	app, _, _, _ := newTestApp(t, dir)
	_ = runApp(t, app, "init")
	err := runApp(t, app, "get", "NOPE")
	if err == nil || !strings.Contains(err.Error(), "secret NOPE not found in development") {
		t.Fatalf("%v", err)
	}
}

func TestSetRejectsBadKey(t *testing.T) {
	dir := t.TempDir()
	app, _, _, _ := newTestApp(t, dir)
	_ = runApp(t, app, "init")
	if err := runApp(t, app, "set", "FOO-BAR=1"); err == nil {
		t.Fatal("expected validation error")
	}
}
