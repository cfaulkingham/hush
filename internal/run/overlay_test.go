package run

import "testing"

func TestOverlayWins(t *testing.T) {
	parent := []string{"PATH=/bin", "FOO=from-parent", "BAR=keep"}
	out := Overlay(parent, map[string]string{"FOO": "from-hush", "BAZ": "new"})
	got := map[string]string{}
	for _, kv := range out {
		i := indexByte(kv, '=')
		if i < 0 {
			t.Fatalf("bad entry %q", kv)
		}
		got[kv[:i]] = kv[i+1:]
	}
	if got["FOO"] != "from-hush" {
		t.Fatalf("FOO %q", got["FOO"])
	}
	if got["BAR"] != "keep" || got["BAZ"] != "new" || got["PATH"] != "/bin" {
		t.Fatalf("%v", got)
	}
}

func TestExecNoCommand(t *testing.T) {
	if err := Exec(nil, nil); err == nil {
		t.Fatal("expected error")
	}
}

func TestOverlayStripsMasterKey(t *testing.T) {
	out := Overlay(
		[]string{"PATH=/bin", "HUSH_KEY=master", "hush_key=case-variant"},
		map[string]string{"SAFE": "yes", "HUSH_KEY": "stored-master"},
	)
	for _, kv := range out {
		if kv == "HUSH_KEY=master" || kv == "hush_key=case-variant" || kv == "HUSH_KEY=stored-master" {
			t.Fatalf("master key leaked into child environment: %v", out)
		}
	}
	if len(out) != 2 {
		t.Fatalf("unexpected environment: %v", out)
	}
}

func indexByte(s string, c byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == c {
			return i
		}
	}
	return -1
}
