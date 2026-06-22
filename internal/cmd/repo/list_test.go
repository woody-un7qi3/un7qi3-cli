package repo

import (
	"context"
	"errors"
	"testing"
	"time"
)

// fakeRunner returns canned stdout/stderr/err for the next Run call, ignoring
// the command. It stands in for `gh repo list` so fetchOrgRepos parsing can be
// exercised without a real gh binary or network.
type fakeRunner struct {
	stdout string
	stderr string
	err    error
}

func (f fakeRunner) Run(_ context.Context, _ string, _ ...string) (string, string, error) {
	return f.stdout, f.stderr, f.err
}

// deadlineRunner blocks until the context is cancelled and returns the context
// error — standing in for a wedged `gh` whose process exec.CommandContext
// would have to kill on timeout. It lets us drive the fetch timeout path
// deterministically without a real hung process.
type deadlineRunner struct{}

func (deadlineRunner) Run(ctx context.Context, _ string, _ ...string) (string, string, error) {
	<-ctx.Done()
	return "", "", ctx.Err()
}

func swapRunner(t *testing.T, r interface {
	Run(context.Context, string, ...string) (string, string, error)
}) {
	t.Helper()
	orig := runner
	runner = r
	t.Cleanup(func() { runner = orig })
}

// fetchOrgRepos must parse a normal gh JSON payload into ghRepo values
// unchanged — this fixes the happy-path output so the timeout refactor stays
// behavior-preserving.
func TestFetchOrgRepos_ParsesGhOutput(t *testing.T) {
	swapRunner(t, fakeRunner{stdout: `[
		{"name":"astro-api","description":"별점 API","visibility":"private","updatedAt":"2026-06-01T00:00:00Z","isArchived":false},
		{"name":"old-repo","description":"","visibility":"public","updatedAt":"2024-01-01T00:00:00Z","isArchived":true}
	]`})

	repos, err := fetchOrgRepos(context.Background(), 100, "")
	if err != nil {
		t.Fatalf("fetchOrgRepos error: %v", err)
	}
	if len(repos) != 2 {
		t.Fatalf("len = %d, want 2", len(repos))
	}
	if repos[0].Name != "astro-api" || repos[0].Description != "별점 API" ||
		repos[0].Visibility != "private" || repos[0].IsArchived {
		t.Errorf("repos[0] = %+v, mismatched", repos[0])
	}
	if repos[1].Name != "old-repo" || !repos[1].IsArchived {
		t.Errorf("repos[1] = %+v, want archived old-repo", repos[1])
	}
}

// A malformed payload must surface a parse error, not panic or silently return
// nil — preserving the prior json.Unmarshal contract.
func TestFetchOrgRepos_ParseErrorWrapped(t *testing.T) {
	swapRunner(t, fakeRunner{stdout: "not json"})

	if _, err := fetchOrgRepos(context.Background(), 100, ""); err == nil {
		t.Fatal("expected parse error, got nil")
	}
}

// A wedged gh that never returns must not hang fetchOrgRepos: the per-call
// timeout fires and a deadline-flavored error comes back. We use a 1ms parent
// deadline so the inner WithTimeout (or the parent) fires immediately rather
// than waiting the full repoFetchTimeout. Without the timeout this test would
// block forever.
func TestFetchOrgRepos_TimeoutDoesNotHang(t *testing.T) {
	swapRunner(t, deadlineRunner{})

	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()

	type result struct {
		err error
	}
	done := make(chan result, 1)
	go func() {
		_, err := fetchOrgRepos(ctx, 100, "")
		done <- result{err: err}
	}()

	select {
	case res := <-done:
		if res.err == nil {
			t.Fatal("expected timeout error, got nil")
		}
		if !errors.Is(res.err, context.DeadlineExceeded) {
			t.Errorf("error = %v, want it to wrap context.DeadlineExceeded", res.err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("fetchOrgRepos hung past the fetch timeout")
	}
}
