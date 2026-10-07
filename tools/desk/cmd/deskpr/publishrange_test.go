package main

// Wiring tests for the publish-identity range narrowing (issue #1967).
//
// `deskpr update` onto a PR head that already carries a commit by ANOTHER trusted App used to
// be refused, because the gate re-judged every commit in origin/<base>..HEAD — including the
// ones the remote PR head already held. update now judges only what the push ADDS, in two
// stages: offline (and under --check) against the local remote-tracking ref, then — before
// anything is pushed — against the forge's LIVE PR head. Both fail closed to the whole range.
//
// The fixture's branch is shaped like the issue: the remote PR head holds one commit by the
// verifier App; the session then adds commits of its own.
//
// FAIL-FIRST. Before the narrowing, TestUpdateCheckAcceptsMixedPR and TestUpdatePushesMixedPR
// were refused (rc 5). internal/deskkit/publishidentity-mutations.json holds the planted mutations
// that drop the live stage or the narrowing; each reddens a test here.

import (
	"path/filepath"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

const prVerifierEmail = "300000005+assay-verifier-app[bot]@users.noreply.github.com"

// mixedPRFixture returns a work tree whose branch carries ONE commit by the verifier App
// that the remote already holds (recorded as refs/remotes/origin/feature/test-branch), plus
// the sha of that remote head. Callers add the session's new commits on top.
func mixedPRFixture(t *testing.T) (string, *[][]string, string) {
	t.Helper()
	work := newBaseFixture(t)
	calls := withEnv(t, work)
	t.Setenv("FAKEGH_LIST_HAS_PR", "1") // open PR #42 on the branch
	useRealPublishIdentityGate(t)
	reauthorTip(t, work, "Assay Verifier", prVerifierEmail)
	remote := mustGit(t, work, "rev-parse", "HEAD")
	mustGit(t, work, "update-ref", "refs/remotes/origin/feature/test-branch", remote)
	return work, calls, remote
}

// addCommitAs adds one commit authored AND committed as email.
func addCommitAs(t *testing.T, work, name, email, file string) {
	t.Helper()
	mustGit(t, work, "config", "user.email", email)
	mustGit(t, work, "config", "user.name", name)
	writeFile(t, filepath.Join(work, file), file+"\n")
	mustGit(t, work, "add", file)
	mustGit(t, work, "commit", "-m", "add "+file)
}

func addTwoWorkerCommits(t *testing.T, work string) {
	t.Helper()
	addCommitAs(t, work, "Assay Worker", prWorkerEmail, "one.txt")
	addCommitAs(t, work, "Assay Worker", prWorkerEmail, "two.txt")
}

// TestUpdateCheckAcceptsMixedPR — the issue's case under --check: the earlier remote commit is
// another trusted App's, the two new ones are the worker's. Pre-fix: rc 5.
func TestUpdateCheckAcceptsMixedPR(t *testing.T) {
	work, calls, _ := mixedPRFixture(t)
	addTwoWorkerCommits(t, work)
	if rc := run([]string{"update", "--check"}); rc != deskkit.ExitOK {
		t.Fatalf("update --check on a mixed-author PR adding only own commits rc = %d, want 0", rc)
	}
	if anyPush(*calls) {
		t.Fatal("--check pushed")
	}
}

// TestUpdatePushesMixedPR — the real run: the forge reports the PR head as the remote commit,
// the live stage anchors on it, and the push goes out. Pre-fix: rc 5, no push.
func TestUpdatePushesMixedPR(t *testing.T) {
	work, calls, remote := mixedPRFixture(t)
	t.Setenv("FAKEGH_PR_OID", remote)
	addTwoWorkerCommits(t, work)
	if rc := run([]string{"update"}); rc != deskkit.ExitOK {
		t.Fatalf("update on a mixed-author PR adding only own commits rc = %d, want 0", rc)
	}
	if !pushedTo(*calls, "feature/test-branch") {
		t.Fatalf("update did not push: %v", *calls)
	}
}

// TestUpdateRefusesNewForeign — (a)+(b): anchored on the real remote head, a NEW foreign commit
// among the session's own is still refused, offline and live; nothing is pushed.
func TestUpdateRefusesNewForeign(t *testing.T) {
	work, calls, remote := mixedPRFixture(t)
	t.Setenv("FAKEGH_PR_OID", remote)
	addCommitAs(t, work, "Assay Worker", prWorkerEmail, "one.txt")
	addCommitAs(t, work, "Assay Issue Loop", prIssueLoopEmail, "bad.txt")
	if rc := run([]string{"update"}); rc != deskkit.ExitRefused {
		t.Fatalf("update adding a foreign commit rc = %d, want 5", rc)
	}
	if anyPush(*calls) {
		t.Fatal("a foreign NEW commit was pushed")
	}
}

// TestUpdateLiveHeadOverrulesRef — (c) the stale-AHEAD tracking ref: the local
// remote-tracking ref claims the remote holds a foreign commit, but the forge's live PR head
// is older (the remote was force-moved back). The offline stage passes on the local
// estimate; the live stage must re-judge against the forge head and refuse BEFORE pushing.
func TestUpdateLiveHeadOverrulesRef(t *testing.T) {
	work, calls, remote := mixedPRFixture(t)
	t.Setenv("FAKEGH_PR_OID", remote) // the forge: the PR head is the verifier commit
	addCommitAs(t, work, "Assay Issue Loop", prIssueLoopEmail, "bad.txt")
	// This checkout's tracking ref wrongly claims the foreign commit is already on the remote.
	mustGit(t, work, "update-ref", "refs/remotes/origin/feature/test-branch", "HEAD")
	addCommitAs(t, work, "Assay Worker", prWorkerEmail, "one.txt")

	if rc := run([]string{"update", "--check"}); rc != deskkit.ExitOK {
		t.Fatalf("precondition: the offline stage should trust the local ref (rc %d) — else this test proves nothing about the live stage", rc)
	}
	*calls = nil
	if rc := run([]string{"update"}); rc != deskkit.ExitRefused {
		t.Fatalf("update with a stale-ahead tracking ref rc = %d, want 5 — the live PR head must be re-judged", rc)
	}
	if anyPush(*calls) {
		t.Fatal("pushed a foreign commit the forge's PR head does not hold")
	}
}

// TestUpdateNoLiveHeadFailsClosed — (c) the forge reports no head sha: the live stage cannot
// anchor, so it judges the whole range (which includes the other App's earlier commit) and
// refuses — closed, never open — naming why.
func TestUpdateNoLiveHeadFailsClosed(t *testing.T) {
	work, calls, _ := mixedPRFixture(t)
	t.Setenv("FAKEGH_PR_OID", "")
	addTwoWorkerCommits(t, work)
	if rc := run([]string{"update"}); rc != deskkit.ExitRefused {
		t.Fatalf("update with no live PR head rc = %d, want 5 (whole range judged)", rc)
	}
	if anyPush(*calls) {
		t.Fatal("pushed without an anchor the forge confirmed")
	}
}

// TestCreateJudgesWholeRange — create offers no remote tip (no PR head exists yet), so a
// commit by another App anywhere on the branch is still refused, whatever a local tracking
// ref says.
func TestCreateJudgesWholeRange(t *testing.T) {
	work, calls, _ := mixedPRFixture(t)
	t.Setenv("FAKEGH_LIST_HAS_PR", "")
	addTwoWorkerCommits(t, work)
	rc := run([]string{"create", "--title", "add feature", "--body-min", "Brief: fixture/01\nbody"})
	if rc != deskkit.ExitRefused {
		t.Fatalf("create on a branch carrying another App's commit rc = %d, want 5", rc)
	}
	if anyPush(*calls) {
		t.Fatal("create pushed")
	}
}
