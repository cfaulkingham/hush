package dotenv

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
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
	m, err := godotenv.Unmarshal(string(b))
	if err != nil {
		return nil, sanitizeParseError(err)
	}
	return m, nil
}

func ParseFile(path string) (map[string]string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("cannot read %s: %w", path, err)
	}
	m, err := godotenv.Unmarshal(string(b))
	if err != nil {
		return nil, sanitizeParseError(err)
	}
	return m, nil
}

// godotenv errors include the rest of the line (KEY=value). Keep the key, drop the value.
func sanitizeParseError(err error) error {
	msg := err.Error()
	if strings.Contains(msg, "unterminated quoted value") {
		return fmt.Errorf("unterminated quoted value")
	}
	const marker = " near "
	i := strings.LastIndex(msg, marker)
	if i < 0 {
		return err
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
	return fmt.Errorf("%s near %q", msg[:i], snippet)
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
