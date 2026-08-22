package cli

import (
	"strings"
	"testing"
)

func TestVersion(t *testing.T) {
	dir := t.TempDir()
	app, out, _, _ := newTestApp(t, dir)
	if err := runApp(t, app, "--version"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "hush 0.1.0") {
		t.Fatalf("%s", out.String())
	}
	out.Reset()
	if err := runApp(t, app, "version"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "hush 0.1.0") {
		t.Fatalf("%s", out.String())
	}
}
