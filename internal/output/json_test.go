package output_test

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/un7qi3inc/un7qi3-cli/internal/clierr"
	"github.com/un7qi3inc/un7qi3-cli/internal/output"
)

func TestWriteJSONError(t *testing.T) {
	tests := []struct {
		name        string
		err         error
		wantType    string
		wantCode    int
		wantMessage string
	}{
		{
			name:        "plain error",
			err:         errors.New("boom"),
			wantType:    "errorString", // *errors.errorString concrete name
			wantCode:    2,
			wantMessage: "boom",
		},
		{
			name:        "repo not found",
			err:         clierr.RepoNotFoundError{Name: "foo"},
			wantType:    "RepoNotFoundError",
			wantCode:    1,
			wantMessage: "등록되지 않은 repo: foo",
		},
		{
			name:        "auth required (pointer type)",
			err:         &clierr.RequiredError{Msg: "login"},
			wantType:    "RequiredError",
			wantCode:    4,
			wantMessage: "login",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var buf strings.Builder
			if err := output.WriteJSONError(&buf, tc.err); err != nil {
				t.Fatalf("WriteJSONError: %v", err)
			}
			var got struct {
				Error struct {
					Type    string `json:"type"`
					Code    int    `json:"code"`
					Message string `json:"message"`
				} `json:"error"`
			}
			if err := json.Unmarshal([]byte(buf.String()), &got); err != nil {
				t.Fatalf("unmarshal %q: %v", buf.String(), err)
			}
			if got.Error.Type != tc.wantType {
				t.Errorf("type = %q, want %q", got.Error.Type, tc.wantType)
			}
			if got.Error.Code != tc.wantCode {
				t.Errorf("code = %d, want %d", got.Error.Code, tc.wantCode)
			}
			if got.Error.Message != tc.wantMessage {
				t.Errorf("message = %q, want %q", got.Error.Message, tc.wantMessage)
			}
		})
	}
}

func TestWriteJSONErrorNil(t *testing.T) {
	var buf strings.Builder
	if err := output.WriteJSONError(&buf, nil); err != nil {
		t.Fatalf("WriteJSONError(nil): %v", err)
	}
	if buf.Len() != 0 {
		t.Errorf("expected no output for nil error, got %q", buf.String())
	}
}
