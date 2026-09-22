package dotenv

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// FuzzRoundTrip checks that every value Serialize writes is parsed back
// exactly: export -> import must never alter a secret.
func FuzzRoundTrip(f *testing.F) {
	for _, seed := range []string{
		"", "plain", `trailing\`, "line1\nline2\\", `$HOME`, `say "hi"`,
		"'single'", "#hash", "back`tick`", "a=b", "こんにちは", " ", "\r\n",
		`\"`, `'"`, "tab\there", `ends with \`, "=start", "!bang",
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, value string) {
		if !utf8.ValidString(value) || strings.ContainsRune(value, 0) {
			t.Skip()
		}
		in := map[string]string{"KEY_A": value, "KEY_B": "static"}
		b, err := Serialize(in)
		if err != nil {
			t.Fatal(err)
		}
		out, err := parseBytes(b)
		if err != nil {
			t.Fatalf("parse(%q): %v", b, err)
		}
		if out["KEY_A"] != value {
			t.Fatalf("round trip changed value: %q -> %q -> %q", value, b, out["KEY_A"])
		}
		if out["KEY_B"] != "static" {
			t.Fatalf("neighbor key damaged: %q -> %q", b, out["KEY_B"])
		}
	})
}

// FuzzParseNoPanic: parsing arbitrary input must never panic, and parse
// errors must never echo large chunks of the input (secret values).
func FuzzParseNoPanic(f *testing.F) {
	f.Add("A=1\n")
	f.Add("A=\"unterminated\n")
	f.Add("A=\nB=\n")
	f.Add("A='x' y'\n")
	f.Fuzz(func(t *testing.T, in string) {
		m, err := Parse(strings.NewReader(in))
		if err != nil {
			if len(err.Error()) > 256 {
				t.Fatalf("error echoes input: %q", err)
			}
			return
		}
		for k, v := range m {
			if strings.Contains(in, "\x00") && v != "" {
				t.Skip()
			}
			_ = k
		}
	})
}
