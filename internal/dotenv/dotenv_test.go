package dotenv

import (
	"bytes"
	"path/filepath"
	"runtime"
	"testing"
)

func testdata(t *testing.T, name string) string {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..", "testdata", "dotenv", name)
}

func TestParseFileFixtures(t *testing.T) {
	cases := []struct {
		file string
		key  string
		want string
	}{
		{"simple.env", "FOO", "bar"},
		{"simple.env", "BAZ", "qux"},
		{"comments.env", "FOO", "bar"},
		{"export.env", "FOO", "bar"},
		{"quotes.env", "SINGLE", "hello world"},
		{"quotes.env", "DOUBLE", "hello world"},
		{"quotes.env", "HASH", "foo#bar"},
		{"blank.env", "EMPTY", ""},
		{"unicode.env", "GREETING", "こんにちは"},
		{"duplicate.env", "FOO", "second"},
	}
	for _, tc := range cases {
		m, err := ParseFile(testdata(t, tc.file))
		if err != nil {
			t.Fatalf("%s: %v", tc.file, err)
		}
		if m[tc.key] != tc.want {
			t.Fatalf("%s %s: got %q want %q", tc.file, tc.key, m[tc.key], tc.want)
		}
	}
}

func TestParseMultiline(t *testing.T) {
	m, err := ParseFile(testdata(t, "multiline.env"))
	if err != nil {
		t.Fatal(err)
	}
	if m["CERT"] != "line1\nline2" {
		t.Fatalf("got %q", m["CERT"])
	}
}

func TestSerializeRoundTrip(t *testing.T) {
	in := map[string]string{
		"FOO":      "bar",
		"SPACED":   "hello world",
		"HASH":     "foo#bar",
		"EMPTY":    "",
		"NL":       "a\nb",
		"QUOTE":    `say "hi"`,
		"GREETING": "こんにちは",
	}
	b, err := Serialize(in)
	if err != nil {
		t.Fatal(err)
	}
	got, err := Parse(bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	for k, v := range in {
		if got[k] != v {
			t.Fatalf("%s: got %q want %q\nserialized:\n%s", k, got[k], v, b)
		}
	}
}

func TestSerializeJSON(t *testing.T) {
	b, err := SerializeJSON(map[string]string{"B": "2", "A": "1"})
	if err != nil {
		t.Fatal(err)
	}
	want := "{\n  \"A\": \"1\",\n  \"B\": \"2\"\n}\n"
	if string(b) != want {
		t.Fatalf("got %q", b)
	}
}

func TestParseMissingFile(t *testing.T) {
	_, err := ParseFile(filepath.Join(t.TempDir(), "nope.env"))
	if err == nil {
		t.Fatal("expected error")
	}
}
