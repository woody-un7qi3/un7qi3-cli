// Package clierr defines the CLI's error contract: a small set of domain
// error types plus Classify, which maps any error to a process exit code.
//
// The goal is to keep exit-code logic in one place (consumed by the main
// dispatcher) instead of scattering os.Exit calls across command bodies.
// Commands return errors; main calls Classify(err) to pick the exit code.
//
// Exit code conventions (gh-style):
//
//	0 — success (nil error)
//	1 — runtime error: a precondition failed or an operation could not
//	    complete (e.g. repo not registered, target instance missing)
//	2 — usage error: bad arguments / flags. This is also the default for
//	    any error not otherwise recognized, matching cobra's behaviour for
//	    unknown commands, flag parse failures and Args validators.
//	4 — authentication required (auth.RequiredError / RequiredError below)
package clierr

import (
	"errors"
	"fmt"
)

// RepoNotFoundError signals that the named repo is not registered (e.g. not
// listed under repos.yml). Maps to exit code 1.
type RepoNotFoundError struct{ Name string }

func (e RepoNotFoundError) Error() string {
	return fmt.Sprintf("등록되지 않은 repo: %s", e.Name)
}

// PreconditionError signals that a runtime precondition was not met before an
// operation could proceed (e.g. no target instances, unknown country). Maps to
// exit code 1.
type PreconditionError struct{ Msg string }

func (e PreconditionError) Error() string { return e.Msg }

// InvalidArgError signals a bad argument or flag value supplied by the user.
// Maps to exit code 2 (usage).
type InvalidArgError struct{ Msg string }

func (e InvalidArgError) Error() string { return e.Msg }

// ExitCodeError carries a specific, already-determined process exit code that
// is not one of the fixed runtime/usage/auth buckets. It exists for the case
// where uq must forward a child process's own exit status verbatim — e.g.
// `uq run` shells out to a dev server, and when that child exits non-zero we
// propagate its exact code rather than collapsing every failure to 1. The
// child has already produced its own output, so callers returning this error
// silence cobra's "Error: ..." line; Classify simply yields Code.
type ExitCodeError struct{ Code int }

func (e ExitCodeError) Error() string {
	return fmt.Sprintf("exit code %d", e.Code)
}

// RequiredError signals that an authentication step is required. Maps to exit
// code 4. It mirrors auth.RequiredError so that clierr can recognise the auth
// contract without importing the auth package (which would be a layering
// inversion). Classify also recognises any error whose chain reports an
// AuthRequired() bool via errors.As, which is how auth.RequiredError is
// adapted — see authRequired below.
type RequiredError struct{ Msg string }

func (e *RequiredError) Error() string { return e.Msg }

// authRequired is satisfied by any error type that wants to be treated as
// "authentication required" (exit 4). auth.RequiredError implements it via the
// adapter installed by RegisterAuthRequired, keeping the auth package free of
// a clierr import. RequiredError above also implements it directly.
type authRequired interface {
	authRequired() bool
}

func (e *RequiredError) authRequired() bool { return true }

// matchesAuthRequired is the predicate Classify uses to detect the auth
// contract. It is overridable so that the auth package can register its own
// RequiredError (which clierr must not import) as an exit-4 error. Defaults to
// recognising only types in this package.
var matchesAuthRequired = func(err error) bool {
	var ar authRequired
	return errors.As(err, &ar) && ar.authRequired()
}

// RegisterAuthRequired extends the auth-required predicate with an additional
// matcher. The auth package calls this in init to teach clierr that its
// *auth.RequiredError means exit 4, without creating an import cycle. Matchers
// compose (logical OR) so multiple callers are safe.
func RegisterAuthRequired(match func(err error) bool) {
	prev := matchesAuthRequired
	matchesAuthRequired = func(err error) bool {
		return prev(err) || match(err)
	}
}

// Classify maps err to a process exit code per the conventions documented on
// the package. nil → 0, auth-required → 4, recognised runtime domain errors
// → 1, explicit usage errors → 2, and any other (unrecognised) error → 2 to
// match cobra's existing usage-error default.
func Classify(err error) int {
	if err == nil {
		return 0
	}
	if matchesAuthRequired(err) {
		return 4
	}
	var coded ExitCodeError
	if errors.As(err, &coded) {
		return coded.Code
	}
	var invalid InvalidArgError
	if errors.As(err, &invalid) {
		return 2
	}
	var repoNF RepoNotFoundError
	var precond PreconditionError
	if errors.As(err, &repoNF) || errors.As(err, &precond) {
		return 1
	}
	// Unrecognised errors (cobra usage errors, library errors bubbling out of
	// RunE) keep the historical exit-2 default.
	return 2
}
