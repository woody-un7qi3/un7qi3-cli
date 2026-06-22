// Package exec provides helpers for invoking external CLI tools (gh, aws,
// gcloud, ...). It centralizes verbose echoing and stderr handling so the
// rest of the codebase does not deal with os/exec directly.
package exec

import (
	"bytes"
	"context"
	"fmt"
	"os"
	osexec "os/exec"
	"strings"
	"sync/atomic"
)

// Runner abstracts external command execution so consumers (auth probes,
// doctor checks, ...) can be unit-tested with a fake. It is intentionally
// small: a single stdout/stderr-capturing call that respects a context.
//
// On a non-zero exit the returned error wraps the underlying *exec.ExitError
// with the "name args: msg" shape Run uses, so callers that surface
// err.Error() keep an identical message whether they hold a Runner or call
// the package-level Run helper.
type Runner interface {
	Run(ctx context.Context, name string, args ...string) (stdout, stderr string, err error)
}

// OSRunner is the production Runner. It executes commands via os/exec and
// reuses the package verbose echo, so injected and direct call paths log
// identically.
type OSRunner struct{}

// Run implements Runner using os/exec.CommandContext.
func (OSRunner) Run(ctx context.Context, name string, args ...string) (stdout, stderr string, err error) {
	echo(name, args)

	cmd := osexec.CommandContext(ctx, name, args...)
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	runErr := cmd.Run()
	stdout, stderr = outBuf.String(), errBuf.String()
	if runErr != nil {
		msg := strings.TrimSpace(stderr)
		if msg == "" {
			msg = strings.TrimSpace(stdout)
		}
		if msg == "" {
			return stdout, stderr, fmt.Errorf("%s %s: %w", name, strings.Join(args, " "), runErr)
		}
		return stdout, stderr, fmt.Errorf("%s %s: %s", name, strings.Join(args, " "), msg)
	}
	return stdout, stderr, nil
}

// defaultRunner is the Runner used by helpers that don't take one explicitly.
var defaultRunner Runner = OSRunner{}

// Default returns the process-wide production Runner.
func Default() Runner { return defaultRunner }

// verbose mirrors the root command's --verbose flag. The cmd package sets it
// from PersistentPreRun so external callers can echo executed commands.
var verbose atomic.Bool

// SetVerbose toggles verbose command echoing.
func SetVerbose(v bool) {
	verbose.Store(v)
}

// Verbose reports the current verbose state.
func Verbose() bool {
	return verbose.Load()
}

// echo prints "$ name arg1 arg2 ..." to stderr when verbose is enabled.
func echo(name string, args []string) {
	if !verbose.Load() {
		return
	}
	parts := append([]string{name}, args...)
	fmt.Fprintln(os.Stderr, "$", strings.Join(parts, " "))
}

// Run executes the command, capturing stdout. On non-zero exit the returned
// error includes the captured stderr (trimmed) so callers can present a
// human-friendly message without re-running the command.
func Run(name string, args ...string) ([]byte, error) {
	echo(name, args)

	cmd := osexec.Command(name, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = strings.TrimSpace(stdout.String())
		}
		if msg == "" {
			return stdout.Bytes(), fmt.Errorf("%s %s: %w", name, strings.Join(args, " "), err)
		}
		return stdout.Bytes(), fmt.Errorf("%s %s: %s", name, strings.Join(args, " "), msg)
	}
	return stdout.Bytes(), nil
}

// RunInteractive runs the command sharing stdin/stdout/stderr with the parent
// process. Use this for tools that drive their own TTY UI (browser login,
// device-code prompts).
func RunInteractive(name string, args ...string) error {
	echo(name, args)

	cmd := osexec.Command(name, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// RunIn behaves like Run but executes inside dir as the working directory.
func RunIn(dir, name string, args ...string) ([]byte, error) {
	echo(name, args)

	cmd := osexec.Command(name, args...)
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = strings.TrimSpace(stdout.String())
		}
		if msg == "" {
			return stdout.Bytes(), fmt.Errorf("%s %s: %w", name, strings.Join(args, " "), err)
		}
		return stdout.Bytes(), fmt.Errorf("%s %s: %s", name, strings.Join(args, " "), msg)
	}
	return stdout.Bytes(), nil
}

// LookPath reports whether the named binary is on $PATH.
func LookPath(name string) bool {
	_, err := osexec.LookPath(name)
	return err == nil
}

// RunCombined executes the command and returns stdout+stderr merged, regardless
// of exit code. The returned error is the underlying *exec.ExitError (or nil
// for success). Use this for tools that emit useful info on stderr even on
// success — notably `gh auth status`.
func RunCombined(name string, args ...string) ([]byte, error) {
	echo(name, args)

	cmd := osexec.Command(name, args...)
	out, err := cmd.CombinedOutput()
	return out, err
}
