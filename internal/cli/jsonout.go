package cli

import (
	"encoding/json"
	"io"
)

// writeJSON emits machine-readable output. Schemas carry names and counts
// only — never secret values unless a command explicitly says so.
func writeJSON(w io.Writer, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	_, err = w.Write(b)
	return err
}
