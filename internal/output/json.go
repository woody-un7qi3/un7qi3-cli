// Package output provides shared output helpers for JSON and TTY modes.
package output

import (
	"encoding/json"
	"io"
	"reflect"

	"github.com/un7qi3inc/un7qi3-cli/internal/clierr"
)

// WriteJSON encodes v as indented JSON to w with a trailing newline.
func WriteJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// jsonError is the machine-readable error envelope: {"error":{...}}.
type jsonError struct {
	Error jsonErrorBody `json:"error"`
}

type jsonErrorBody struct {
	Type    string `json:"type"`
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// WriteJSONError encodes err as {"error":{"type","code","message"}} to w. The
// code is derived from clierr.Classify so JSON consumers see the same exit-code
// semantics as the process exit status. type is the concrete Go type name of
// the (unwrapped target) error, useful for programmatic branching.
func WriteJSONError(w io.Writer, err error) error {
	if err == nil {
		return nil
	}
	return WriteJSON(w, jsonError{Error: jsonErrorBody{
		Type:    errorType(err),
		Code:    clierr.Classify(err),
		Message: err.Error(),
	}})
}

// errorType returns a short name for err's concrete type (pointer indirection
// stripped), e.g. "RequiredError". Falls back to "error" for anonymous types.
func errorType(err error) string {
	t := reflect.TypeOf(err)
	for t != nil && t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t == nil || t.Name() == "" {
		return "error"
	}
	return t.Name()
}
