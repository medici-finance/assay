package main

import (
	"os/exec"
	"strings"
	"testing"
)

// consumedfragment_shallow_test.go — regression for #999: CI's default
// `actions/checkout` (depth 1) leaves `consumedFragmentIndex.build()` reading a
// SHALLOW clone. `git log --name-only -- changelog/` does not error there — it
// just returns the truncated single-commit history — so the pre-fix code sets
// ok=true on an incomplete `tracked` set and a genuinely-consumed fragment reads
// as "never tracked": a false PROBLEM, not the could-not-check the three-state
// design (linkcheck.go:456-459) promises for unreadable history.
//
// This is deliberately a REAL `git clone --depth 1` (via `file://`, since git
// silently ignores --depth on a same-filesystem local-path clone — confirmed by
// hand before writing this test), not the no-history-at-all fixture
// TestConsumedChangelogFragmentUnreadableHistoryIsCouldNotCheck already covers.
// That test's tree never had a git repo; this one has a full, real history with
// the fragment genuinely committed-then-removed, cloned down to depth 1 — the
// exact shape GitHub Actions hands `lint` on every PR.
func TestConsumedFragmentIndexShallowCloneIsCouldNotCheck(t *testing.T) {
	full, _ := writeConsumedFragmentFixture(t, true)

	shallowParent := t.TempDir()
	shallow := shallowParent + "/shallow"
	cmd := exec.Command("git", "clone", "--depth", "1", "file://"+full, shallow)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git clone --depth 1: %v\n%s", err, out)
	}

	// Confirm the fixture really is shallow, not an accidental full clone.
	verify := exec.Command("git", "-C", shallow, "rev-parse", "--is-shallow-repository")
	out, err := verify.Output()
	if err != nil {
		t.Fatalf("git rev-parse --is-shallow-repository: %v", err)
	}
	if got := string(out); got != "true\n" {
		t.Fatalf("test setup is not actually shallow (got %q) — the clone step needs revisiting", got)
	}

	ix := &consumedFragmentIndex{root: shallow}
	exempt, checked := ix.consumed("changelog/rel-15-consumed.md")
	if checked {
		t.Fatalf("shallow clone: got checked=true (exempt=%v), want checked=false (could-not-check) — "+
			"a shallow clone's truncated `git log` must never be read as a definitive answer", exempt)
	}
	if exempt {
		t.Errorf("shallow clone: got exempt=true, want false alongside checked=false")
	}
}

// regression: #3364
//
// TestShallowFragmentNoteNamesShallow: on a shallow clone the fragment PROBLEM
// still fires (three-state: never rounded up to a pass), and its note names the
// SHALLOW clone and the checkout-depth remedy, so a red regen is not read as an
// authoring defect in the brief. A tree with no git repo at all keeps the
// "history was unreadable" note and never claims to be shallow.
func TestShallowFragmentNoteNamesShallow(t *testing.T) {
	full, _ := writeConsumedFragmentFixture(t, true)
	shallow := t.TempDir() + "/shallow"
	if out, err := exec.Command("git", "clone", "--depth", "1", "file://"+full, shallow).CombinedOutput(); err != nil {
		t.Fatalf("git clone --depth 1: %v\n%s", err, out)
	}
	problems := linkProblems(shallow, []string{shallow + "/docs/streams/rel/brief-15-release.md"})
	if len(problems) != 4 {
		t.Fatalf("got %d problems, want 4 — a shallow clone exempts nothing: %v", len(problems), problems)
	}
	var noted int
	for _, p := range problems {
		if strings.Contains(p, "SHALLOW clone") {
			noted++
			if !strings.Contains(p, "could-not-check") || !strings.Contains(p, "fetch-depth: 0") {
				t.Errorf("shallow note must say could-not-check and name the remedy: %v", p)
			}
		}
	}
	if noted != 2 {
		t.Errorf("want the shallow note on exactly the 2 fragment-shaped paths, got %d: %v", noted, problems)
	}

	root, brief := writeConsumedFragmentFixture(t, false)
	for _, p := range linkProblems(root, []string{brief}) {
		if strings.Contains(p, "SHALLOW") {
			t.Errorf("a tree with no git repo is not a shallow clone: %v", p)
		}
	}
}
