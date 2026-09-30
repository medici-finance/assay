package main

// Fail-first wiring tests for the publish-identity gate (issue #1490 lane B).
//
// withEnv stubs publishIdentityGateFn no-op for the rest of the suite (the shared fixture's
// commits are authored under a generic identity); these tests RESTORE the real gate and
// drive it against the fixture's tip commit, re-authored to the identity each case needs.
//
// FAIL-FIRST. Before the gate was wired, `create`/`update` pushed whatever the branch
// carried — the misattributed-commit push #1490 is about. The refusal subtests below prove
// the wiring stops that push (rc 5, ZERO `git push`); the pass subtest proves a correctly
// attributed branch still goes through. The one-line mutation that re-opens the hole is
// dropping the `publishIdentityGate(...)` call from cmdCreate/cmdUpdate — do that and these
// refusal subtests go green-would-push.

import (
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// The fixture roster binds worker=github:assay-worker-app:300000006. issue-loop is another
// role's bot — the exact cross-role misattribution #1490 observed.
const (
	prWorkerEmail    = "300000006+assay-worker-app[bot]@users.noreply.github.com"
	prIssueLoopEmail = "300000003+assay-issue-loop-app[bot]@users.noreply.github.com"
)

// reauthorTip re-authors the branch tip (the single feature commit ahead of origin/main)
// under the given identity, so origin/main..HEAD carries exactly one commit with that
// author and committer.
func reauthorTip(t *testing.T, work, name, email string) {
	t.Helper()
	mustGit(t, work, "config", "user.email", email)
	mustGit(t, work, "config", "user.name", name)
	mustGit(t, work, "commit", "--amend", "--reset-author", "--no-edit")
}

// useRealPublishIdentityGate restores the production gate over withEnv's no-op stub.
func useRealPublishIdentityGate(t *testing.T) {
	t.Helper()
	stub := publishIdentityGateFn
	publishIdentityGateFn = productionPublishIdentityGateFn
	t.Cleanup(func() { publishIdentityGateFn = stub })
}

func TestPublishIdentityGateWiredCreateRefusesForeignCommit(t *testing.T) {
	work := newBaseFixture(t)
	calls := withEnv(t, work)
	useRealPublishIdentityGate(t)
	// The tip is authored as ANOTHER role's bot — the #1490 stale-identity worktree.
	reauthorTip(t, work, "Assay Issue Loop", prIssueLoopEmail)

	rc := run([]string{"create", "--title", "add feature", "--body-min", "Brief: fixture/01\nbody"})
	if rc != deskkit.ExitRefused {
		t.Fatalf("create with a foreign-identity commit rc = %d, want 5 (refused)", rc)
	}
	if anyCall(gitCalls(*calls), "push") {
		t.Fatal("the gate refused but the branch was still pushed — a mis-attributed commit reached the forge")
	}
}

func TestPublishIdentityGateWiredUpdateRefusesForeignCommit(t *testing.T) {
	work := newBaseFixture(t)
	calls := withEnv(t, work)
	t.Setenv("FAKEGH_LIST_HAS_PR", "1") // an open PR exists on the branch
	useRealPublishIdentityGate(t)
	reauthorTip(t, work, "Assay Issue Loop", prIssueLoopEmail)

	rc := run([]string{"update"})
	if rc != deskkit.ExitRefused {
		t.Fatalf("update with a foreign-identity commit rc = %d, want 5 (refused)", rc)
	}
	if anyCall(gitCalls(*calls), "push") {
		t.Fatal("the gate refused but update still pushed")
	}
}

func TestPublishIdentityGateCorrectIdentityPasses(t *testing.T) {
	work := newBaseFixture(t)
	withEnv(t, work)
	useRealPublishIdentityGate(t)
	// The tip is authored as the worker bot the session role binds to.
	reauthorTip(t, work, "Assay Worker", prWorkerEmail)

	// --check runs every local gate (including this one) and stops before any network call:
	// a clean --check is the gate passing on a correctly-attributed branch.
	rc := run([]string{"create", "--check", "--title", "add feature", "--body-min", "Brief: fixture/01\nbody"})
	if rc != deskkit.ExitOK {
		t.Fatalf("create --check on a correctly-attributed branch rc = %d, want 0 (ok)", rc)
	}
}

// TestPublishIdentityGateSeamIsRealInProduction — the seam these tests replace must, in a
// fresh binary, be the real deskkit.PublishIdentityMatchesRole. It is stubbed by withEnv, so
// asserting the recorded production binding is what makes the wiring tests mean something.
func TestPublishIdentityGateSeamIsRealInProduction(t *testing.T) {
	if productionPublishIdentityGateFn == nil {
		t.Fatal("productionPublishIdentityGateFn is nil — the seam has no recorded production binding")
	}
	// A stubbed-out gate in the shipped binary would leave every wiring test above vacuous.
	if err := productionPublishIdentityGateFn(deskkit.PublishIdentityInput{
		Role:    "worker",
		Commits: func(string, string) ([]deskkit.PublishCommit, error) { return nil, errSeamProbe },
	}); err == nil {
		t.Fatal("productionPublishIdentityGateFn is not the real gate — a real gate reports the probe error, a stub returns nil")
	}
}

// errSeamProbe is a sentinel the seam-reality test feeds through the real gate's Commits
// reader: the real gate turns a reader error into an Unverifiable, a no-op stub returns nil.
var errSeamProbe = deskkit.Unverifiable("seam probe", nil)
