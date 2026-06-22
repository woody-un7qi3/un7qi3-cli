package clierr_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/un7qi3inc/un7qi3-cli/internal/clierr"
)

func TestClassify(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want int
	}{
		{"nil", nil, 0},
		{"plain error → usage default", errors.New("boom"), 2},
		{"RepoNotFound → runtime", clierr.RepoNotFoundError{Name: "foo"}, 1},
		{"Precondition → runtime", clierr.PreconditionError{Msg: "no instances"}, 1},
		{"InvalidArg → usage", clierr.InvalidArgError{Msg: "bad flag"}, 2},
		{"ExitCodeError → its own code", clierr.ExitCodeError{Code: 137}, 137},
		{"wrapped ExitCodeError", fmt.Errorf("ctx: %w", clierr.ExitCodeError{Code: 3}), 3},
		{"RequiredError → auth", &clierr.RequiredError{Msg: "login"}, 4},
		{"wrapped RepoNotFound", fmt.Errorf("ctx: %w", clierr.RepoNotFoundError{Name: "x"}), 1},
		{"wrapped Required", fmt.Errorf("ctx: %w", &clierr.RequiredError{Msg: "y"}), 4},
		{"auth wins over wrapped runtime", fmt.Errorf("%w", &clierr.RequiredError{Msg: "z"}), 4},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := clierr.Classify(tc.err); got != tc.want {
				t.Errorf("Classify(%v) = %d, want %d", tc.err, got, tc.want)
			}
		})
	}
}

// errType lets a test error report itself as auth-required to verify the
// RegisterAuthRequired adapter path (used by internal/auth).
type registeredAuthErr struct{ msg string }

func (e registeredAuthErr) Error() string { return e.msg }

func TestRegisterAuthRequired(t *testing.T) {
	// Before registration, the custom type falls through to the usage default.
	if got := clierr.Classify(registeredAuthErr{"x"}); got != 2 {
		t.Fatalf("pre-register Classify = %d, want 2", got)
	}
	clierr.RegisterAuthRequired(func(err error) bool {
		var re registeredAuthErr
		return errors.As(err, &re)
	})
	if got := clierr.Classify(registeredAuthErr{"x"}); got != 4 {
		t.Errorf("post-register Classify = %d, want 4", got)
	}
	// Wrapped form must also be recognised.
	if got := clierr.Classify(fmt.Errorf("ctx: %w", registeredAuthErr{"x"})); got != 4 {
		t.Errorf("wrapped post-register Classify = %d, want 4", got)
	}
}
