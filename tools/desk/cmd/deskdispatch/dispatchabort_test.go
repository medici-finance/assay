package main

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// TestWorktreeCreateFailureReleasesTheOrphanedClaim is the fail-first proof of the defect-1
// fix. Before it, a `deskwt add` failure returned with the durable claim STILL HELD — placed
// one step earlier and never released — so the item was wedged: every later re-dispatch,
// including the operator's corrected re-run, was told "already claimed by a LIVE holder" and a
// human had to hand-delete the ref. The abort now releases the claim exactly as the before_run
// failure path does.
//
// Fail-first on the unfixed code: the release call is absent, so `dispatch-claim.sh release`
// never runs and the step report still reads "The claim is HELD".
func TestWorktreeCreateFailureReleasesTheOrphanedClaim(t *testing.T) {
	s := &stub{}
	_, root := s.install(t)
	plantScripts(t, root)
	s.replies = []reply{
		{match: "remote get-url origin", stdout: "git@github.com:medici-finance/assay.git"},
		{match: "deskwt add", stderr: deskwtStderr, code: deskkit.ExitRefused},
	}

	rc, stderr := runCapturingStderr(t, []string{"item-1", "--root", root,
		"--prompt-file", filepath.Join(t.TempDir(), "p.md")})

	// deskwt REFUSED — the decision passes through as a refusal (5).
	if rc != deskkit.ExitRefused {
		t.Fatalf("rc = %d, want %d", rc, deskkit.ExitRefused)
	}
	// Precondition: the claim WAS acquired, so a release is the thing under test.
	if !s.ran("dispatch-claim.sh acquire") {
		t.Fatal("the claim was never acquired — the precondition for this test does not hold")
	}
	// The fix: the placed claim is released rather than orphaned.
	if !s.ran("dispatch-claim.sh release") {
		t.Error("the claim was NOT released after the worktree-create failure — it is orphaned, and " +
			"every later re-dispatch of this item is wedged behind a claim nobody is acting on")
	}
	report := stepReport(t, stderr)
	if !strings.Contains(report, "The claim was released") {
		t.Errorf("the step report must state the claim was released:\n%s", report)
	}
	if strings.Contains(report, "The claim is HELD") {
		t.Errorf("the step report still frames the claim as HELD — the orphaned-claim wording the fix removes:\n%s", report)
	}
	// deskwt's own verbatim cause survives the reworded wrapper.
	if !strings.Contains(report, "already exists and is CHECKED OUT") {
		t.Errorf("deskwt's own message was dropped from the wrapper:\n%s", report)
	}
	// The wrapper no longer tells the operator to "fix the tree" for what is usually an
	// already-delivered brief.
	if strings.Contains(report, "fix the tree") {
		t.Errorf("the wrapper still frames the failure as a transient tree fault:\n%s", report)
	}
}

// An UNVERIFIABLE deskwt failure releases the claim too — the abort is the same, only the exit
// code (6) differs. The old wrapper orphaned the claim on both codes.
func TestWorktreeCreateUnverifiableFailureAlsoReleasesTheClaim(t *testing.T) {
	s := &stub{}
	_, root := s.install(t)
	plantScripts(t, root)
	s.replies = []reply{
		{match: "remote get-url origin", stdout: "git@github.com:medici-finance/assay.git"},
		{match: "deskwt add", stderr: "git worktree add failed: could not create branch feat/item-1", code: deskkit.ExitUnverifiable},
	}

	rc, stderr := runCapturingStderr(t, []string{"item-1", "--root", root,
		"--prompt-file", filepath.Join(t.TempDir(), "p.md")})
	if rc != deskkit.ExitUnverifiable {
		t.Fatalf("rc = %d, want %d", rc, deskkit.ExitUnverifiable)
	}
	if !s.ran("dispatch-claim.sh release") {
		t.Error("an unverifiable worktree-create failure must also release the placed claim")
	}
	if report := stepReport(t, stderr); !strings.Contains(report, "The claim was released") {
		t.Errorf("the step report must state the claim was released:\n%s", report)
	}
}

// TestWorktreeNameIsSessionScopedToAvoidForeignDirCollision is the fail-first proof of the
// defect-2 fix. deskdispatch derived the worktree DIR name deterministically from the item key
// (`tracker-<item>`) with NO session-uniqueness, so an item whose canonical dir was already
// owned by a FOREIGN session dead-ended with `deskwt add … target already exists` (exit 5). The
// dir name now carries the session suffix (mirroring `deskwt role-init`), while the branch and
// claim key stay deterministic.
//
// Fail-first on the unfixed code: the name is the bare `example-stream-06`, so the session-scoped
// assertion below does not match.
func TestWorktreeNameIsSessionScopedToAvoidForeignDirCollision(t *testing.T) {
	s := &stub{}
	_, root := s.install(t)
	plantScripts(t, root)
	t.Setenv("DESK_SESSION", "sess-xyz")
	s.replies = happyReplies("/private/tmp/tracker-example-stream-06-sess-xyz")

	rc := run([]string{"example-stream/06", "--root", root, "--repo", allowedRepo,
		"--prompt-file", filepath.Join(t.TempDir(), "p.md")})
	if rc != deskkit.ExitOK {
		t.Fatalf("rc = %d, want 0", rc)
	}
	// The worktree DIR name carries the session suffix so a foreign session's leftover
	// /private/tmp/tracker-example-stream-06 cannot dead-end this dispatch with `target already exists`.
	if !s.ran("deskwt add example-stream-06-sess-xyz ") {
		t.Errorf("the worktree name is not session-scoped — a foreign-owned canonical dir would still dead-end the dispatch")
	}
	// The BRANCH stays deterministic on the bare item key — it is the deliverable's
	// cross-session identity, reviewed lineage and all. It must NOT gain the suffix.
	if !s.ran("--branch feat/example-stream-06 --base") {
		t.Error("the branch must stay on the bare item key, not gain the session suffix")
	}
}

// With no resolvable session, the name falls back to the bare item-derived form — the
// pre-session behaviour — so an unconfigured shell is never turned into an unusable name.
func TestWorktreeNameFallsBackToBareWhenNoSession(t *testing.T) {
	s := &stub{}
	_, root := s.install(t)
	plantScripts(t, root)
	// Both session envs empty: install sets DESK_SESSION; clear it and CLAUDE_SESSION_ID.
	t.Setenv("DESK_SESSION", "")
	t.Setenv("CLAUDE_SESSION_ID", "")
	s.replies = happyReplies("/private/tmp/tracker-example-stream-06")

	rc := run([]string{"example-stream/06", "--root", root, "--repo", allowedRepo,
		"--prompt-file", filepath.Join(t.TempDir(), "p.md")})
	if rc != deskkit.ExitOK {
		t.Fatalf("rc = %d, want 0", rc)
	}
	if !s.ran("deskwt add example-stream-06 --branch feat/example-stream-06 --base") {
		t.Error("with no session the worktree name must be the bare item-derived form")
	}
}
