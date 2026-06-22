package exec

import (
	"context"
	"strings"
	"testing"
)

// OSRunner.Run must capture stdout and stderr separately on success.
func TestOSRunner_CapturesStreams(t *testing.T) {
	var r OSRunner
	stdout, stderr, err := r.Run(context.Background(), "sh", "-c", "printf out; printf err 1>&2")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if stdout != "out" {
		t.Errorf("stdout = %q, want %q", stdout, "out")
	}
	if stderr != "err" {
		t.Errorf("stderr = %q, want %q", stderr, "err")
	}
}

// On non-zero exit with stderr content, the error message embeds the trimmed
// stderr in the "name args: msg" shape. auth probes surface err.Error() via
// trimMsg, so this shape is part of the contract.
func TestOSRunner_ErrorEmbedsStderr(t *testing.T) {
	var r OSRunner
	_, _, err := r.Run(context.Background(), "sh", "-c", "echo boom 1>&2; exit 3")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	msg := err.Error()
	if !strings.Contains(msg, "boom") {
		t.Errorf("err = %q, want it to embed stderr", msg)
	}
	if !strings.HasPrefix(msg, "sh -c ") {
		t.Errorf("err = %q, want %q prefix", msg, "sh -c ")
	}
}

// With no output at all, the error wraps the underlying exec error via %w so
// the chain is preserved for errors.Is/As.
func TestOSRunner_ErrorWrapsWhenSilent(t *testing.T) {
	var r OSRunner
	_, _, err := r.Run(context.Background(), "sh", "-c", "exit 1")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "exit status 1") {
		t.Errorf("err = %q, want wrapped exit status", err.Error())
	}
}

func TestDefault_IsOSRunner(t *testing.T) {
	if _, ok := Default().(OSRunner); !ok {
		t.Errorf("Default() = %T, want OSRunner", Default())
	}
}
