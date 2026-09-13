package main

import (
	"os/exec"
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
// exact shape GitHub Actions hands `lint`/`windows-smoke` on every PR.
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
