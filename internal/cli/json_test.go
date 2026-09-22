package cli

import (
	"encoding/json"
	"testing"
)

func TestStatusJSON(t *testing.T) {
	dir := t.TempDir()
	app, out, _, _ := newTestApp(t, dir)
	_ = runApp(t, app, "init", "--name", "api")
	_ = runApp(t, app, "set", "SECRET=s3cret")
	out.Reset()
	if err := runApp(t, app, "status", "--json"); err != nil {
		t.Fatal(err)
	}
	var s statusJSON
	if err := json.Unmarshal(out.Bytes(), &s); err != nil {
		t.Fatalf("%v: %s", err, out.String())
	}
	if s.Project != "api" || s.Env != "development" || s.Secrets != 1 || s.Key != "keychain" {
		t.Fatalf("%+v", s)
	}
	if out.Len() == 0 || containsValue(out.String(), "s3cret") {
		t.Fatalf("value leaked: %s", out.String())
	}
}

func TestLsJSON(t *testing.T) {
	dir := t.TempDir()
	app, out, _, _ := newTestApp(t, dir)
	_ = runApp(t, app, "init")
	_ = runApp(t, app, "set", "A=1")
	_ = runApp(t, app, "set", "B=2")
	out.Reset()
	if err := runApp(t, app, "ls", "--json"); err != nil {
		t.Fatal(err)
	}
	var l lsJSON
	if err := json.Unmarshal(out.Bytes(), &l); err != nil {
		t.Fatalf("%v: %s", err, out.String())
	}
	if l.Env != "development" || !l.Active || len(l.Keys) != 2 || l.Values != nil {
		t.Fatalf("%+v", l)
	}
	out.Reset()
	if err := runApp(t, app, "ls", "--json", "--values"); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(out.Bytes(), &l); err != nil {
		t.Fatalf("%v: %s", err, out.String())
	}
	if l.Values["A"] != "1" {
		t.Fatalf("%+v", l)
	}
}

func TestEnvLsJSON(t *testing.T) {
	dir := t.TempDir()
	app, out, _, _ := newTestApp(t, dir)
	_ = runApp(t, app, "init")
	_ = runApp(t, app, "env", "new", "staging")
	out.Reset()
	if err := runApp(t, app, "env", "ls", "--json"); err != nil {
		t.Fatal(err)
	}
	var e envListJSON
	if err := json.Unmarshal(out.Bytes(), &e); err != nil {
		t.Fatalf("%v: %s", err, out.String())
	}
	if len(e.Environments) != 2 {
		t.Fatalf("%+v", e)
	}
	for _, env := range e.Environments {
		if env.Name == "development" && !env.Active {
			t.Fatalf("%+v", env)
		}
	}
}

func containsValue(s, v string) bool {
	var m map[string]any
	if json.Unmarshal([]byte(s), &m) == nil {
		for _, val := range m {
			if str, ok := val.(string); ok && str == v {
				return true
			}
		}
	}
	return false
}
