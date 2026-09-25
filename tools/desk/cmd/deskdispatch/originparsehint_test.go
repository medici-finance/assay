package main

// originparsehint_test.go — FAIL-FIRST coverage for issue 1470 lane B: the worktree-create
// hint told an operator to hunt for a merged/open PR when `deskwt add` had actually failed on
// its own origin-remote resolution (`currentRepo`/`parseRepo`,
// tools/desk/cmd/deskwt/deskwt.go), which runs before any branch logic and names no branch at
// all. The old code applied the "branch already existing — look for a PR" sentence
// UNCONDITIONALLY to every message none of the more specific cases matched, so a remote-parse
// failure fell into it by default. This file pins the fix: the parse-class message gets its
// own accurate hint, and a genuinely unrecognized message gets NO guessed cause at all.

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// The exact deskwt wording for the two shapes `currentRepo`/`parseRepo` can fail with
// (tools/desk/cmd/deskwt/deskwt.go: currentRepo wraps parseRepo's error, and parseRepo's own
// normRepoPath error is the inner one), kept here verbatim so a change to either message
// breaks this test rather than silently re-merging this class into the branch-exists one.
const (
	deskwtOriginParseSaid = "cannot parse origin repo from git@example-host:only-one-segment: " +
		"cannot parse owner/repo from \"only-one-segment\""
	// A message that names neither a branch nor the origin-parse pattern — the residual
	// "anything else" class this fix must stop guessing a branch story for.
	deskwtUnknownSaid = "refused: origin example-org/other-repo is not in the desk-tools repo set"
)

// The origin-remote-parse message must NOT render the branch/PR hint, and must name the
// remote as the cause instead of the branch.
func TestWorktreeCreateHintOriginParseClassNeverBlamesTheBranch(t *testing.T) {
	got := worktreeCreateHint("worker", "feat/item-1", deskwtOriginParseSaid)
	if strings.Contains(got, "already delivered or in progress") || strings.Contains(got, "DELIVERED") ||
		strings.Contains(got, "look for a merged or open PR") {
		t.Errorf("the origin-remote-parse hint still sends the operator PR-hunting:\n%s", got)
	}
	if strings.Contains(got, "The brief's branch") {
		t.Errorf("the origin-remote-parse hint blames a branch deskwt never mentioned:\n%s", got)
	}
	if !strings.Contains(got, "origin remote") || !strings.Contains(got, "remote get-url origin") {
		t.Errorf("the origin-remote-parse hint does not point at the remote as the cause:\n%s", got)
	}
	if !strings.Contains(got, "NOT the cause") {
		t.Errorf("the origin-remote-parse hint does not say the branch was NOT the cause:\n%s", got)
	}
}

// The same origin-parse class must be handled uniformly whatever branch name the dispatch
// happens to carry — the message names no branch, so the branch argument must not leak in.
func TestWorktreeCreateHintOriginParseClassIgnoresBranchArg(t *testing.T) {
	got := worktreeCreateHint("worker", "feat/some-other-item", deskwtOriginParseSaid)
	if strings.Contains(got, "feat/some-other-item") {
		t.Errorf("the origin-remote-parse hint names a branch that is not the cause:\n%s", got)
	}
}

// A message that matches neither the branch-exists shape nor the origin-parse shape must not
// be told as a branch story either — the "anything else" class the old unconditional fallback
// mishandled.
func TestWorktreeCreateHintUnknownClassGuessesNothing(t *testing.T) {
	got := worktreeCreateHint("worker", "feat/item-1", deskwtUnknownSaid)
	if strings.Contains(got, "already delivered or in progress") || strings.Contains(got, "DELIVERED") ||
		strings.Contains(got, "look for a merged or open PR") || strings.Contains(got, "already exists") {
		t.Errorf("the unknown-class hint still guesses a branch/PR cause:\n%s", got)
	}
	if strings.Contains(got, "origin remote") {
		t.Errorf("the unknown-class hint wrongly claims this is the origin-parse class:\n%s", got)
	}
	if !strings.Contains(got, "no cause is guessed") {
		t.Errorf("the unknown-class hint does not say plainly that no cause is guessed:\n%s", got)
	}
}

// The branch-exists class (deskwt's message still names the branch shape via "already
// exists") is the ONLY one that keeps today's guessed cause — confirms (a) is untouched by
// this fix, alongside the parse-class (b) and unknown (c) tests above.
func TestWorktreeCreateHintBranchExistsClassUnchanged(t *testing.T) {
	got := worktreeCreateHint("worker", "feat/item-1", "branch feat/item-1 already exists")
	if !strings.Contains(got, "already delivered or in progress") || !strings.Contains(got, "feat/item-1") {
		t.Errorf("the branch-exists hint regressed:\n%s", got)
	}
}

// End-to-end: a real `deskwt add` failure carrying the origin-parse message must reach the
// operator's step report WITHOUT the branch/PR hint, and the parse failure is UNVERIFIABLE
// (currentRepo/parseRepo build a deskkit.Unverifiable, never a Refused) — so the dispatch's
// own exit code must pass that through rather than reading it as a decided refusal.
func TestWorktreeCreateOriginParseFailureReachesTheStepReportWithoutTheBranchHint(t *testing.T) {
	s := &stub{}
	_, root := s.install(t)
	plantScripts(t, root)
	s.replies = []reply{
		{match: "remote get-url origin", stdout: "git@github.com:medici-finance/assay.git"},
		{match: "deskwt add", stderr: deskwtOriginParseSaid, code: deskkit.ExitUnverifiable},
	}

	promptFile := filepath.Join(t.TempDir(), "p.md")
	rc, stderr := runCapturingStderr(t, []string{"item-1", "--root", root, "--prompt-file", promptFile})

	if rc != deskkit.ExitUnverifiable {
		t.Errorf("rc = %d, want %d (deskwt's origin-parse failure is unverifiable, not a decided refusal)",
			rc, deskkit.ExitUnverifiable)
	}
	report := stepReport(t, stderr)
	if !strings.Contains(report, deskwtOriginParseSaid) {
		t.Errorf("the step report does not carry deskwt's own origin-parse message:\n%s", report)
	}
	if strings.Contains(report, "already delivered or in progress") || strings.Contains(report, "look for a merged or open PR") {
		t.Errorf("the step report sends the operator PR-hunting over an origin-remote-parse failure:\n%s", report)
	}
	if !strings.Contains(report, "remote get-url origin") {
		t.Errorf("the step report does not point the operator at the origin remote:\n%s", report)
	}
	if !strings.Contains(report, "The claim was released") {
		t.Errorf("the step report does not state the claim was released:\n%s", report)
	}
}
