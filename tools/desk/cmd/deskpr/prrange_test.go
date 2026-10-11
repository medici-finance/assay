package main

// prrange_test.go — the publish-identity range on `deskpr update --pr N` (issue #2432; the
// same defect as #1967 and #2359).
//
// #1967's fix narrowed the gate to the commits a push ADDS, but `update --pr N` kept an
// offline stage that judged the WHOLE range from the default branch (the PR's head branch is
// unknown until the forge is read), and that stage refused before the live stage ever ran. A
// PR whose published head already carried another App's commit — a desk-authored first
// commit, or a stacked PR's base — could never take another commit through the verb. On a
// real `update --pr N` the gate now runs once, at the live stage, against the head the forge
// reports AND the push destination is shown to hold, which HEAD must descend from. `--check`
// cannot read that head, so it still judges the whole range (fail closed) and says how to get
// the narrow judgement offline.
//
// FAIL-FIRST. On the unfixed tree TestUpdatePRMixedBase and TestUpdatePRMergedMain are refused
// (rc 5, "Range judged: refs/remotes/origin/main..HEAD"), and TestUpdatePRCheckWide does not
// carry the --branch hint. The fail-closed tests are pins; the mutations that redden them are
// listed in internal/deskkit/publishidentity-mutations.json.

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// mixedNamedFixture is namedPRFixture with the real publish-identity gate and a PR head
// whose published history carries ANOTHER App's commit: the branch's first commit is the
// verifier App's, the worker's own commit c1 sits on top of it, and the bare remote holds c1
// as refs/heads/heldBranch (the PR head the forge reports). The worktree is on a branch with
// a different name, as in the #2085 shape that needs --pr. Callers add the new commits.
func mixedNamedFixture(t *testing.T) (work string, calls *[][]string, bare, c1 string) {
	t.Helper()
	work = newBaseFixture(t)
	calls = withEnv(t, work)
	useRealPublishIdentityGate(t)
	reauthorTip(t, work, "Assay Verifier", prVerifierEmail)
	mustGit(t, work, "checkout", "-b", "neutral-rework")
	addCommitAs(t, work, "Assay Worker", prWorkerEmail, "own.txt")
	c1 = mustGit(t, work, "rev-parse", "HEAD")
	mustGit(t, work, "push", "origin", "HEAD:refs/heads/"+heldBranch)
	bare = filepath.Join(filepath.Dir(work), "origin.git")
	t.Setenv("FAKEGH_PR_HEAD", heldBranch)
	t.Setenv("FAKEGH_PR_OID", c1)
	return work, calls, bare, c1
}

// wantNotPushed asserts no push ran and the remote PR head still sits at c1.
func wantNotPushed(t *testing.T, calls [][]string, bare, c1 string) {
	t.Helper()
	if anyPush(calls) {
		t.Fatalf("pushed: %v", calls)
	}
	if got := remoteRef(t, bare, "refs/heads/"+heldBranch); got != c1 {
		t.Fatalf("the remote PR head moved to %s, want %s", got, c1)
	}
}

// TestUpdatePRMixedBase — (a) the issue's case: the published head carries another App's
// commit; the push adds only the worker's own. Pre-fix: rc 5, nothing pushed.
func TestUpdatePRMixedBase(t *testing.T) {
	work, calls, bare, _ := mixedNamedFixture(t)
	addCommitAs(t, work, "Assay Worker", prWorkerEmail, "fix.txt")
	head := mustGit(t, work, "rev-parse", "HEAD")
	var rc int
	stderr := captureStderr(t, func() { rc = run([]string{"update", "--pr", "42"}) })
	if rc != deskkit.ExitOK {
		t.Fatalf("update --pr 42 adding only own commits onto a mixed-identity head rc = %d, want 0\n%s", rc, stderr)
	}
	if got := remoteRef(t, bare, "refs/heads/"+heldBranch); got != head {
		t.Fatalf("remote PR head = %s, want HEAD %s", got, head)
	}
	if !pushedFrom(*calls, "HEAD", heldBranch) {
		t.Fatalf("did not push HEAD onto the PR head branch: %v", *calls)
	}
}

// TestUpdatePRMergedMain — (a) the #2417 shape: the default branch moved, the worker merged
// it in (a two-parent merge by the worker), and the published head carries another App's
// commit. Only the merge and the worker's commit are added; main's commits are on the base.
func TestUpdatePRMergedMain(t *testing.T) {
	work, _, bare, _ := mixedNamedFixture(t)
	mustGit(t, work, "checkout", "-q", "main")
	addCommitAs(t, work, "Someone Else", "someone@example.org", "mainline.txt")
	mustGit(t, work, "update-ref", "refs/remotes/origin/main", "HEAD")
	mustGit(t, work, "checkout", "-q", "neutral-rework")
	mustGit(t, work, "config", "user.email", prWorkerEmail)
	mustGit(t, work, "config", "user.name", "Assay Worker")
	mustGit(t, work, "merge", "--no-ff", "--no-edit", "refs/remotes/origin/main")
	head := mustGit(t, work, "rev-parse", "HEAD")
	var rc int
	stderr := captureStderr(t, func() { rc = run([]string{"update", "--pr", "42"}) })
	if rc != deskkit.ExitOK {
		t.Fatalf("update --pr 42 after merging main rc = %d, want 0\n%s", rc, stderr)
	}
	if got := remoteRef(t, bare, "refs/heads/"+heldBranch); got != head {
		t.Fatalf("remote PR head = %s, want HEAD %s", got, head)
	}
}

// TestUpdatePRForeignAdded — (b) the same published head, but the push ADDS a commit under a
// foreign identity, between two of the worker's own: refused, naming that commit and not the
// already-published one, and nothing is pushed.
func TestUpdatePRForeignAdded(t *testing.T) {
	work, calls, bare, c1 := mixedNamedFixture(t)
	addCommitAs(t, work, "Assay Worker", prWorkerEmail, "one.txt")
	addCommitAs(t, work, "Assay Issue Loop", prIssueLoopEmail, "bad.txt")
	addCommitAs(t, work, "Assay Worker", prWorkerEmail, "three.txt")
	var rc int
	stderr := captureStderr(t, func() { rc = run([]string{"update", "--pr", "42"}) })
	if rc != deskkit.ExitRefused {
		t.Fatalf("update --pr 42 adding a foreign-identity commit rc = %d, want 5\n%s", rc, stderr)
	}
	if !strings.Contains(stderr, "add bad.txt") || !strings.Contains(stderr, "assay-issue-loop-app") {
		t.Fatalf("the refusal does not name the added foreign commit:\n%s", stderr)
	}
	if strings.Contains(stderr, "is authored by Assay Verifier") {
		t.Fatalf("the refusal re-judged the already-published commit:\n%s", stderr)
	}
	wantNotPushed(t, *calls, bare, c1)
}

// TestUpdatePRCheckWide — (c) offline: `--check` cannot read the PR's head, so it judges the
// whole range from the default branch and refuses on the published foreign commit (never
// "nothing to judge"), and the refusal names the way to the narrow judgement offline.
func TestUpdatePRCheckWide(t *testing.T) {
	work, calls, bare, c1 := mixedNamedFixture(t)
	addCommitAs(t, work, "Assay Worker", prWorkerEmail, "fix.txt")
	var rc int
	stderr := captureStderr(t, func() { rc = run([]string{"update", "--pr", "42", "--check"}) })
	if rc != deskkit.ExitRefused {
		t.Fatalf("update --pr 42 --check with the head unknown offline rc = %d, want 5 (whole range)\n%s", rc, stderr)
	}
	for _, want := range []string{"Range judged: refs/remotes/origin/main..HEAD", "--branch"} {
		if !strings.Contains(stderr, want) {
			t.Errorf("the --check refusal does not say %q:\n%s", want, stderr)
		}
	}
	wantNotPushed(t, *calls, bare, c1)
	// --branch names the head offline: --check then judges only what the push adds.
	*calls = nil
	if rc := run([]string{"update", "--branch", heldBranch, "--check"}); rc != deskkit.ExitOK {
		t.Fatalf("update --branch %s --check rc = %d, want 0", heldBranch, rc)
	}
}

// TestUpdatePRHeadUnread — (c) the PR head cannot be read or anchored: not reported, not
// fetched here, or reported at a commit the destination does not hold. Each is refused and
// nothing is pushed — never passed as an empty range.
func TestUpdatePRHeadUnread(t *testing.T) {
	cases := map[string]func(t *testing.T, work, c1 string){
		"not reported": func(t *testing.T, _, _ string) { t.Setenv("FAKEGH_PR_OID", "") },
		"not fetched":  func(t *testing.T, _, _ string) { t.Setenv("FAKEGH_PR_OID", strings.Repeat("ab", 20)) },
		"destination disagrees": func(t *testing.T, work, c1 string) {
			// The forge reports HEAD itself, which the destination does not hold.
			t.Setenv("FAKEGH_PR_OID", mustGit(t, work, "rev-parse", "HEAD"))
		},
	}
	for name, setup := range cases {
		t.Run(name, func(t *testing.T) {
			work, calls, bare, c1 := mixedNamedFixture(t)
			addCommitAs(t, work, "Assay Issue Loop", prIssueLoopEmail, "bad.txt")
			setup(t, work, c1)
			if rc := run([]string{"update", "--pr", "42"}); rc == deskkit.ExitOK {
				t.Fatal("update --pr 42 with an unreadable PR head passed")
			}
			wantNotPushed(t, *calls, bare, c1)
		})
	}
}

// TestUpdatePRDiverged — a PR head that is NOT an ancestor of HEAD (the remote moved to a
// line this checkout does not descend from) anchors nothing: refused, nothing pushed.
func TestUpdatePRDiverged(t *testing.T) {
	work, calls, bare, c1 := mixedNamedFixture(t)
	mustGit(t, work, "checkout", "-q", "-b", "side", c1)
	addCommitAs(t, work, "Assay Worker", prWorkerEmail, "side.txt")
	side := mustGit(t, work, "rev-parse", "HEAD")
	mustGit(t, work, "push", "origin", "HEAD:refs/heads/"+heldBranch)
	mustGit(t, work, "checkout", "-q", "neutral-rework")
	addCommitAs(t, work, "Assay Issue Loop", prIssueLoopEmail, "bad.txt")
	t.Setenv("FAKEGH_PR_OID", side)
	if rc := run([]string{"update", "--pr", "42"}); rc == deskkit.ExitOK {
		t.Fatal("update --pr 42 onto a diverged PR head passed")
	}
	wantNotPushed(t, *calls, bare, side)
}

// TestUpdateDivergedLive — the same on the default path: the offline stage anchors on the
// local tracking ref, but the forge's live head is on another line. The live stage must not
// use it, judge the whole range, and refuse on the published foreign commit.
func TestUpdateDivergedLive(t *testing.T) {
	work, calls, remote := mixedPRFixture(t)
	mustGit(t, work, "checkout", "-q", "-b", "side")
	addCommitAs(t, work, "Assay Worker", prWorkerEmail, "side.txt")
	side := mustGit(t, work, "rev-parse", "HEAD")
	mustGit(t, work, "checkout", "-q", "feature/test-branch")
	addCommitAs(t, work, "Assay Worker", prWorkerEmail, "one.txt")
	t.Setenv("FAKEGH_PR_OID", side)
	if rc := run([]string{"update", "--check"}); rc != deskkit.ExitOK {
		t.Fatalf("precondition: the offline stage anchors on %s (rc %d)", remote, rc)
	}
	*calls = nil
	if rc := run([]string{"update"}); rc != deskkit.ExitRefused {
		t.Fatalf("update with a diverged live PR head rc = %d, want 5 (whole range)", rc)
	}
	if anyPush(*calls) {
		t.Fatal("pushed on a diverged live head")
	}
}

// TestUpdateNoTrackRefWide — the default path is unchanged: with no local tracking ref for
// the branch, the offline stage judges the whole range and refuses before any forge read,
// even though the forge would report a usable head.
func TestUpdateNoTrackRefWide(t *testing.T) {
	work, calls, remote := mixedPRFixture(t)
	t.Setenv("FAKEGH_PR_OID", remote)
	mustGit(t, work, "update-ref", "-d", "refs/remotes/origin/feature/test-branch")
	addTwoWorkerCommits(t, work)
	var rc int
	stderr := captureStderr(t, func() { rc = run([]string{"update"}) })
	if rc != deskkit.ExitRefused || !strings.Contains(stderr, "does not resolve") {
		t.Fatalf("update with no tracking ref rc = %d, want 5 naming the unresolved tip\n%s", rc, stderr)
	}
	if anyPush(*calls) {
		t.Fatal("pushed")
	}
}
