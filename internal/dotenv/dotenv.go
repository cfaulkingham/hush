package dotenv

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/joho/godotenv"
)

func Parse(r io.Reader) (map[string]string, error) {
	b, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	return godotenv.Unmarshal(string(b))
}

func ParseFile(path string) (map[string]string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("cannot read %s: %w", path, err)
	}
	return godotenv.Unmarshal(string(b))
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
	// godotenv mishandles values ending with \" inside double quotes; prefer single quotes when safe
	if strings.Contains(v, `"`) && !strings.ContainsAny(v, "'\n\r") {
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
