package main

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// gateRefusal runs a dispatch whose decision gate refuses AFTER the home was created, with
// extra stub replies ahead of the happy path. It returns the stub, the home and stderr.
func gateRefusal(t *testing.T, extra ...reply) (*stub, string, string) {
	t.Helper()
	s := &stub{}
	_, root := s.install(t)
	plantScripts(t, root)
	home := filepath.Join(t.TempDir(), "home")
	s.replies = append(extra, happyReplies(home)...)
	s.replies = append(s.replies,
		reply{match: "decision-issue.sh ensure", stderr: "refused: no decision surface", code: deskkit.ExitRefused})
	rc, stderr := runCapturingStderr(t, []string{"item-1", "--root", root, "--gate-human", "--brief", "spec.md",
		"--prompt-file", filepath.Join(t.TempDir(), "p.md")})
	if rc == deskkit.ExitOK {
		t.Fatal("a failed decision gate must fail the dispatch")
	}
	return s, home, stderr
}

// A post-create abort that releases the claim also removes the home it created, through
// `deskwt remove` (never a raw delete). Pre-fix the home was left behind on every abort.
// regression: #6847
func TestAbortRemovesNewHome(t *testing.T) {
	s, home, stderr := gateRefusal(t)
	if !s.ran("dispatch-claim.sh release") {
		t.Fatal("the claim was not released — the precondition for this test does not hold")
	}
	if !s.ran("deskwt remove " + home) {
		t.Fatalf("the aborted dispatch left its home %s behind:\n%s", home, stderr)
	}
	if !strings.Contains(stderr, "its worktree "+home+" was removed") {
		t.Errorf("the abort must say the home was removed:\n%s", stderr)
	}
}

// A claim that could not be released keeps the home: deskwt would refuse a held run anyway,
// and the home is the evidence for the hand release.
// regression: #6847
func TestAbortKeepsHomeOnHeldClaim(t *testing.T) {
	s, _, stderr := gateRefusal(t,
		reply{match: "dispatch-claim.sh release", stderr: "forge down", code: deskkit.ExitUnverifiable})
	if s.ran("deskwt remove") {
		t.Fatalf("a home was removed while its claim stayed held:\n%s", stderr)
	}
}

// A refusal by deskwt is reported, never fatal, and never retried with force.
// regression: #6847
func TestAbortReportsRemoveRefusal(t *testing.T) {
	s, home, stderr := gateRefusal(t,
		reply{match: "deskwt remove", stderr: "refused: dirty", code: deskkit.ExitRefused})
	if !s.ran("deskwt remove " + home) {
		t.Fatalf("no removal was attempted:\n%s", stderr)
	}
	if !strings.Contains(stderr, "was NOT removed") {
		t.Errorf("the refusal must be reported:\n%s", stderr)
	}
	for _, c := range s.calls {
		if strings.Contains(strings.Join(c, " "), "--force") {
			t.Fatalf("a forced call ran: %v", c)
		}
	}
}
