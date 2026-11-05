package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// A worker opens its PR, releases its claim and stops, leaving its home clean, pushed and
// at the PR head. The rework dispatch of that PR then failed at worktree-create: the same
// session re-derives the same target ("target already exists (never clobbered)"), and a
// later session finds the earlier home still holding the branch ("CHECKED OUT in the
// worktree"). Both ran through the shipped allocator, exit 5, until a human removed the
// home by hand. These tests run the real allocator twice over one verified resume.

// finishedHome allocates the first dispatch's home and stamps its run key the way
// dispatch() does right after allocation.
func finishedHome(t *testing.T, r realResume) string {
	t.Helper()
	first := createDispatchWorktree(r.o, r.plan)
	if first.err != nil {
		t.Fatalf("first allocation: %v; %s", first.err, first.stderr)
	}
	home := strings.TrimSpace(first.stdout) // under the allocator's t.TempDir prefix
	r.git(home, "config", "extensions.worktreeConfig", "true")
	r.git(home, "config", "--worktree", "assay.runKey", r.plan.claimKey)
	return home
}

// regression: #4272
func TestReworkAdoptsSameHome(t *testing.T) {
	r := newRealResume(t)
	home := finishedHome(t, r)

	again := createDispatchWorktree(r.o, r.plan)
	if again.err != nil {
		t.Fatalf("rework dispatch refused its own finished home: %v; %s", again.err, again.stderr)
	}
	if got := strings.TrimSpace(again.stdout); got != home {
		t.Fatalf("rework home=%q, want the finished home %q", got, home)
	}
}

// regression: #4272
func TestReworkAdoptsEarlierHome(t *testing.T) {
	r := newRealResume(t)
	home := finishedHome(t, r)

	later := r.plan
	later.wtName = fmt.Sprintf("resume-later-%d", time.Now().UnixNano())
	again := createDispatchWorktree(r.o, later)
	if again.err != nil {
		t.Fatalf("rework dispatch refused an earlier session's finished home: %v; %s", again.err, again.stderr)
	}
	if got := strings.TrimSpace(again.stdout); got != home {
		t.Fatalf("rework home=%q, want the earlier finished home %q", got, home)
	}
}

// A home with anything in it that is not on the forge is never adopted: the refusal stays,
// names the clause, and the file is left exactly where it was.
func TestReworkRefusesDirtyHome(t *testing.T) {
	r := newRealResume(t)
	home := finishedHome(t, r)
	stray := filepath.Join(home, "notes.txt")
	if err := os.WriteFile(stray, []byte("unpushed\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	again := createDispatchWorktree(r.o, r.plan)
	if exitCodeOf(again.err) != deskkit.ExitRefused {
		t.Fatalf("dirty home: err=%v, want a refusal; %s", again.err, again.stderr)
	}
	if !strings.Contains(again.stderr, "never clobbered") || !strings.Contains(again.stderr, "uncommitted or untracked") {
		t.Fatalf("refusal does not name the clause:\n%s", again.stderr)
	}
	if _, err := os.Stat(stray); err != nil {
		t.Fatalf("refusal touched the home: %v", err)
	}
	if hint := worktreeCreateHint("worker", r.branch, again.stderr); !strings.Contains(hint, "NOT provably finished") {
		t.Fatalf("hint sends the operator to remove an unproven home: %s", hint)
	}
}

// A home stamped for a DIFFERENT run is somebody else's, whatever it holds.
func TestReworkRefusesForeignRun(t *testing.T) {
	r := newRealResume(t)
	home := finishedHome(t, r)
	r.git(home, "config", "--worktree", "assay.runKey", "assay--issue-7")

	later := r.plan
	later.wtName = fmt.Sprintf("resume-foreign-%d", time.Now().UnixNano())
	again := createDispatchWorktree(r.o, later)
	if exitCodeOf(again.err) != deskkit.ExitRefused || !strings.Contains(again.stderr, "CHECKED OUT in the worktree") ||
		!strings.Contains(again.stderr, "assay.runKey") {
		t.Fatalf("foreign home: err=%v, want the held-branch refusal naming the run key; %s", again.err, again.stderr)
	}
}
