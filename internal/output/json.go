// Package output provides shared output helpers for JSON and TTY modes.
package output

import (
	"encoding/json"
	"io"
)

// WriteJSON encodes v as indented JSON to w with a trailing newline.
func WriteJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
