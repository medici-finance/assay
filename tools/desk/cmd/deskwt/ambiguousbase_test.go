package main

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// plantAmbiguousBase reproduces the observed defect condition: a LOCAL branch literally
// named `origin/main` sitting alongside the real `refs/remotes/origin/main`, pointed at a
// DIFFERENT (older) commit. From then on the short name `origin/main` matches two refs and
// git prefers refs/heads/, i.e. the stale one.
func plantAmbiguousBase(t *testing.T, work string) (stale string) {
	t.Helper()
	remoteTip := mustGit(t, work, "rev-parse", "refs/remotes/origin/main")
	writeFile(t, filepath.Join(work, "later.md"), "a commit only the remote ref has\n")
	mustGit(t, work, "add", "later.md")
	mustGit(t, work, "commit", "-m", "advance the remote ref past the local decoy")
	mustGit(t, work, "update-ref", "refs/remotes/origin/main", mustGit(t, work, "rev-parse", "HEAD"))
	// The decoy: refs/heads/origin/main, parked at the ORIGINAL (now stale) tip.
	mustGit(t, work, "update-ref", "refs/heads/origin/main", remoteTip)
	return remoteTip
}

// TestGitResolvesAmbiguousBaseAtExitZero is the POSITIVE CONTROL for this guard
// (docs/three-state-instrument-rule.md, "Positive-control requirement"): it proves the
// underlying defect is real at this revision rather than assuming it. Without it, a green
// guard test proves only that the guard's own code runs.
//
// It asserts the raw git behaviour the guard exists to intercept: with the name ambiguous,
// `git rev-parse --verify --quiet origin/main^{commit}` — the exact probe cmdAdd used to
// rely on — SUCCEEDS, and returns the STALE sha. Exit 0 plus a wrong answer is precisely
// checked-clean reported where the true state is could-not-check.
func TestGitResolvesAmbiguousBaseAtExitZero(t *testing.T) {
	work := newRepo(t)
	stale := plantAmbiguousBase(t, work)
	remote := mustGit(t, work, "rev-parse", "refs/remotes/origin/main")
	if stale == remote {
		t.Fatalf("fixture did not diverge: stale and remote are both %s", stale)
	}

	got, err := runGit(work, "rev-parse", "--verify", "--quiet", "origin/main^{commit}")
	if err != nil {
		t.Fatalf("positive control void: rev-parse already fails on the ambiguous name (%v) — "+
			"the guard would be redundant, re-derive it", err)
	}
	if got != stale {
		t.Fatalf("positive control void: rev-parse returned %s, want the stale %s "+
			"(git no longer prefers refs/heads/ — re-derive the guard)", got, stale)
	}
}

// TestAddAmbiguousBaseIsCouldNotCheck is the guard itself: cmdAdd must refuse with exit 6
// (could-not-check) and must NOT run `git worktree add` at all.
func TestAddAmbiguousBaseIsCouldNotCheck(t *testing.T) {
	work := newRepo(t)
	plantAmbiguousBase(t, work)
	calls := withEnv(t, work)

	if rc := run([]string{"add", "amb"}); rc != deskkit.ExitUnverifiable {
		t.Fatalf("add with an ambiguous default --base rc = %d, want %d (could-not-check)",
			rc, deskkit.ExitUnverifiable)
	}
	if hasWorktreeVerb(*calls, "add") {
		t.Fatalf("git worktree add ran on an ambiguous --base: %v", gitCalls(*calls))
	}
}

// TestAddFullyQualifiedBaseUnambiguous is the escape hatch the refusal message points at:
// the SAME repo, the same two refs, but a fully-qualified --base resolves to exactly one
// ref and the add succeeds. This is what keeps the guard from being a blanket refusal.
func TestAddFullyQualifiedBaseUnambiguous(t *testing.T) {
	work := newRepo(t)
	plantAmbiguousBase(t, work)
	withEnv(t, work)

	if rc := run([]string{"add", "fq", "--base", "refs/remotes/origin/main"}); rc != deskkit.ExitOK {
		t.Fatalf("add with a fully-qualified --base rc = %d, want 0", rc)
	}
}

// TestRefCandidatesOrdering pins the two properties the refusal message depends on:
// candidates[0] is the ref git would have picked (refs/heads/ before refs/remotes/), and
// an unambiguous name yields exactly one candidate.
func TestRefCandidatesOrdering(t *testing.T) {
	work := newRepo(t)
	plantAmbiguousBase(t, work)

	cands, err := refCandidates(work, "origin/main")
	if err != nil {
		t.Fatalf("refCandidates: %v", err)
	}
	want := []string{"refs/heads/origin/main", "refs/remotes/origin/main"}
	if strings.Join(cands, ",") != strings.Join(want, ",") {
		t.Fatalf("refCandidates = %v, want %v (git-preference order)", cands, want)
	}

	one, err := refCandidates(work, "refs/remotes/origin/main")
	if err != nil {
		t.Fatalf("refCandidates (qualified): %v", err)
	}
	if len(one) != 1 || one[0] != "refs/remotes/origin/main" {
		t.Fatalf("qualified refCandidates = %v, want exactly [refs/remotes/origin/main]", one)
	}

	none, err := refCandidates(work, "origin/no-such-ref")
	if err != nil {
		t.Fatalf("refCandidates (absent): %v", err)
	}
	if len(none) != 0 {
		t.Fatalf("absent refCandidates = %v, want empty (the rev-parse gate owns this case)", none)
	}
}
