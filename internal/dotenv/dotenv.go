package dotenv

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

func Parse(r io.Reader) (map[string]string, error) {
	b, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	return parseBytes(b)
}

func ParseFile(path string) (map[string]string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("cannot read %s: %w", path, err)
	}
	return parseBytes(b)
}

func parseBytes(b []byte) (map[string]string, error) {
	m, err := godotenv.Unmarshal(string(b))
	// godotenv v1.5 misidentifies quoted values ending in backslashes and
	// trims some escaped edge quotes. Recognize hush's canonical serializer
	// output so every exported value can be imported exactly.
	if canonical, ok := parseCanonical(b); ok {
		return canonical, nil
	}
	if err != nil {
		return nil, sanitizeParseError(err)
	}
	return m, nil
}

var canonicalKeyRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

func parseCanonical(b []byte) (map[string]string, bool) {
	result := map[string]string{}
	if len(b) == 0 {
		return result, true
	}
	src := string(b)
	if !strings.HasSuffix(src, "\n") {
		return nil, false
	}
	lines := strings.Split(strings.TrimSuffix(src, "\n"), "\n")
	previous := ""
	for i, line := range lines {
		key, encoded, ok := strings.Cut(line, "=")
		if !ok || !canonicalKeyRe.MatchString(key) || (i > 0 && key <= previous) {
			return nil, false
		}
		value, ok := decodeCanonicalValue(encoded)
		if !ok || key+"="+quoteDotenv(value) != line {
			return nil, false
		}
		result[key] = value
		previous = key
	}
	return result, true
}

func decodeCanonicalValue(encoded string) (string, bool) {
	if encoded == "" {
		return "", true
	}
	if encoded[0] == '\'' {
		if len(encoded) < 2 || encoded[len(encoded)-1] != '\'' {
			return "", false
		}
		value := encoded[1 : len(encoded)-1]
		return value, !strings.ContainsRune(value, '\'')
	}
	if encoded[0] != '"' {
		return encoded, !strings.ContainsAny(encoded, "\r\n")
	}
	if len(encoded) < 2 || encoded[len(encoded)-1] != '"' {
		return "", false
	}

	var value strings.Builder
	content := encoded[1 : len(encoded)-1]
	for i := 0; i < len(content); i++ {
		if content[i] != '\\' {
			value.WriteByte(content[i])
			continue
		}
		i++
		if i == len(content) {
			return "", false
		}
		switch content[i] {
		case '\\', '"', '$':
			value.WriteByte(content[i])
		case 'n':
			value.WriteByte('\n')
		case 'r':
			value.WriteByte('\r')
		default:
			return "", false
		}
	}
	return value.String(), true
}

// godotenv errors include the input (KEY=value). Keep a short key, drop the
// value, and never echo long stretches of the file.
func sanitizeParseError(err error) error {
	msg := err.Error()
	if strings.Contains(msg, "unterminated quoted value") {
		return fmt.Errorf("unterminated quoted value")
	}
	const marker = " near "
	i := strings.LastIndex(msg, marker)
	if i < 0 {
		return fmt.Errorf("invalid dotenv syntax")
	}
	snippet, uerr := strconv.Unquote(msg[i+len(marker):])
	if uerr != nil {
		return fmt.Errorf("invalid dotenv syntax")
	}
	if n := strings.IndexAny(snippet, "\n\r"); n >= 0 {
		snippet = snippet[:n]
	}
	if eq := strings.IndexAny(snippet, "=:"); eq >= 0 {
		snippet = snippet[:eq]
	}
	snippet = strings.TrimSpace(snippet)
	if r := []rune(snippet); len(r) > 64 {
		snippet = string(r[:64]) + "..."
	}
	out := fmt.Sprintf("%s near %q", msg[:i], snippet)
	// %q escaping can balloon (binary input); never echo long stretches.
	if len(out) > 160 {
		out = fmt.Sprintf("%s near <elided>", msg[:i])
	}
	return errors.New(out)
}

func Serialize(secrets map[string]string) ([]byte, error) {
	keys := make([]string, 0, len(secrets))
	for k := range secrets {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var buf bytes.Buffer
	for _, k := range keys {
		buf.WriteString(k)
		buf.WriteByte('=')
		buf.WriteString(quoteDotenv(secrets[k]))
		buf.WriteByte('\n')
	}
	return buf.Bytes(), nil
}

func quoteDotenv(v string) string {
	if v == "" {
		return ""
	}
	if !needsQuote(v) {
		return v
	}
	// godotenv expands $VAR in double-quoted values and mishandles trailing \".
	if !strings.ContainsAny(v, "'\n\r") && strings.ContainsAny(v, `"$`) {
		return "'" + v + "'"
	}
	return `"` + escapeDotenv(v) + `"`
}

func escapeDotenv(v string) string {
	var b strings.Builder
	for _, r := range v {
		switch r {
		case '\\':
			b.WriteString(`\\`)
		case '"':
			b.WriteString(`\"`)
		case '$':
			b.WriteString(`\$`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func needsQuote(v string) bool {
	if strings.ContainsAny(v, " \t#\"'`$\\") {
		return true
	}
	if strings.ContainsAny(v, "\n\r") {
		return true
	}
	return false
}

// ParseJSON reads a JSON object of string values (the format written by
// SerializeJSON) so import and export round-trip.
func ParseJSON(r io.Reader) (map[string]string, error) {
	b, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	var raw map[string]any
	if err := json.Unmarshal(b, &raw); err != nil {
		// json errors can quote snippets of the input; keep only the kind.
		return nil, fmt.Errorf("invalid json secrets file (%T)", err)
	}
	out := make(map[string]string, len(raw))
	for k, v := range raw {
		s, ok := v.(string)
		if !ok {
			return nil, fmt.Errorf("json value for %q must be a string", k)
		}
		out[k] = s
	}
	return out, nil
}

func SerializeJSON(secrets map[string]string) ([]byte, error) {
	if secrets == nil {
		secrets = map[string]string{}
	}
	b, err := json.MarshalIndent(secrets, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(b, '\n'), nil
}
