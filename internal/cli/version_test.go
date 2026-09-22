package cli

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestVersion(t *testing.T) {
	dir := t.TempDir()
	app, out, _, _ := newTestApp(t, dir)
	if err := runApp(t, app, "--version"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "hush "+version) {
		t.Fatalf("%s", out.String())
	}
	out.Reset()
	if err := runApp(t, app, "version"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "hush "+version) {
		t.Fatalf("%s", out.String())
	}
	out.Reset()
	if err := runApp(t, app, "version", "--json"); err != nil {
		t.Fatal(err)
	}
	var v versionJSON
	if err := json.Unmarshal(out.Bytes(), &v); err != nil {
		t.Fatalf("%v: %s", err, out.String())
	}
	if v.Version != version || v.Go == "" {
		t.Fatalf("%+v", v)
	}
}
