package main

import (
	"errors"
	"os"
	"strings"
	"testing"
)

// End-to-end cover for the per-stream max-concurrent knob (methodology-metrics/13).
//
// The unit tests in nextup_test.go pin the capping arithmetic. This file pins the
// whole path a real board takes — stream README frontmatter → parse → lint →
// nextUp → emitted STATUS.md — because the declaration is only worth anything if
// a stream can actually SET it in a file and have the board honour it.
//
// The fixture root has two streams with two eligible todo briefs each: `serial`
// declares `max-concurrent: 1`, `plain` declares nothing. Every case below asserts
// on BOTH, so "the cap bound" and "the default moved" can never be confused.

// serialRepoRoot copies the serialrepo fixture into a temp root.
func serialRepoRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS("testdata/serialrepo")); err != nil {
		t.Fatal(err)
	}
	return root
}

// serialRows counts the Next-up rows belonging to one stream.
func serialRows(section, stream string) int {
	n := 0
	for _, line := range strings.Split(section, "\n") {
		if strings.HasPrefix(line, "| "+stream+" |") {
			n++
		}
	}
	return n
}

func TestSerializedStreamEndToEnd(t *testing.T) {
	t.Run("declared cap binds: two eligible briefs, one offered", func(t *testing.T) {
		root := serialRepoRoot(t)
		stubRemoteBranches(t, nil, nil) // remote READ, nothing claimed
		if code := run(root, "write", nil, nil, ""); code != 0 {
			t.Fatalf("run exited %d, want 0", code)
		}
		sec := nextUpSection(t, readStatus(t, root))
		if got := serialRows(sec, "serial"); got != 1 {
			t.Errorf("max-concurrent: 1 with 2 eligible briefs offered %d rows, want 1:\n%s", got, sec)
		}
		if got := serialRows(sec, "plain"); got != 2 {
			t.Errorf("the undeclared stream must be untouched: offered %d rows, want 2:\n%s", got, sec)
		}
	})

	t.Run("in-flight subtracts: one claimed, zero offered", func(t *testing.T) {
		root := serialRepoRoot(t)
		stubRemoteBranches(t, []string{"fix/serial-01-in-flight"}, nil)
		if code := run(root, "write", nil, nil, ""); code != 0 {
			t.Fatalf("run exited %d, want 0", code)
		}
		sec := nextUpSection(t, readStatus(t, root))
		if got := serialRows(sec, "serial"); got != 0 {
			t.Errorf("one brief in flight under max-concurrent: 1 must offer 0 more, got %d:\n%s", got, sec)
		}
		if got := serialRows(sec, "plain"); got != 2 {
			t.Errorf("the undeclared stream must be untouched: offered %d rows, want 2:\n%s", got, sec)
		}
	})

	t.Run("could-not-check: withheld and reported, never failed open", func(t *testing.T) {
		root := serialRepoRoot(t)
		stubRemoteBranches(t, nil, errors.New("ls-remote timed out"))
		var code int
		stderr := captureStderr(t, func() { code = run(root, "write", nil, nil, "") })
		if code != 0 {
			t.Fatalf("run exited %d, want 0 — the board still renders, wearing its degradation", code)
		}
		sec := nextUpSection(t, readStatus(t, root))
		if got := serialRows(sec, "serial"); got != 0 {
			t.Errorf("with in-flight unknowable, a serialized stream must offer 0, got %d — this is the fail-open:\n%s", got, sec)
		}
		if !strings.Contains(sec, "COULD NOT CHECK") {
			t.Errorf("the board must REPORT the withholding, not just do it:\n%s", sec)
		}
		if !strings.Contains(stderr, "could-not-check") || !strings.Contains(stderr, "serial") {
			t.Errorf("stderr must name the withheld stream; got:\n%s", stderr)
		}
		// The default is untouched by the same degradation.
		if got := serialRows(sec, "plain"); got != 2 {
			t.Errorf("a stream that declared nothing must keep today's semantics exactly: offered %d rows, want 2:\n%s", got, sec)
		}
	})

	t.Run("lint accepts a declared max-concurrent", func(t *testing.T) {
		root := serialRepoRoot(t)
		stubRemoteBranches(t, nil, nil)
		if code := run(root, "lint", nil, nil, ""); code != 0 {
			t.Fatalf("--lint exited %d on a valid max-concurrent: 1, want 0", code)
		}
	})
}
