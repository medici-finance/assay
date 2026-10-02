package deskkit

// Range tests for the publish-identity gate's narrowing (issue #1967).
//
// The gate used to judge every commit in refs/remotes/origin/<base>..HEAD, so a PR head that
// already carried a commit by ANOTHER trusted App refused every later push, even one adding
// only the session's own commits. The narrowing excludes what the remote branch already holds
// (an offered RemoteTip). These tests pin both halves of that change on real repositories:
//
//   - it ACCEPTS the reported case — an earlier remote commit by a different trusted App plus
//     new commits by the pushing identity;
//   - it still REFUSES a foreign commit the push adds, alone or mixed with good ones;
//   - it FAILS CLOSED: a tip that is missing, stale, diverged, unknown, or spelled so it could
//     name the pushed commit itself is never used, and the whole range is judged instead.
//
// FAIL-FIRST. internal/deskkit/publishidentity-mutations.json holds the planted mutations
// (run with `go run ./cmd/muhar -spec internal/deskkit/publishidentity-mutations.json` from
// tools/desk); each one reddens at least one test here.

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// rangeRepo is a scratch repository shaped like a mixed-author PR: main recorded as
// origin/main, then on branch feat ONE commit by the verifier App (a different trusted App)
// that the remote branch already holds — recorded as refs/remotes/origin/feat.
type rangeRepo struct {
	t      *testing.T
	dir    string
	remote string // sha of the commit the remote PR head holds
}

func newRangeRepo(t *testing.T) *rangeRepo {
	t.Helper()
	r := &rangeRepo{t: t, dir: t.TempDir()}
	r.git("init", "-b", "main")
	r.git("config", "commit.gpgsign", "false")
	r.commitAs("seed@example.org", "Seed", "README.md", "init")
	r.git("update-ref", "refs/remotes/origin/main", "HEAD")
	r.git("checkout", "-b", "feat")
	r.remote = r.commitAs(verifierEmail, "Assay Verifier", "earlier.txt", "earlier commit by another App")
	r.git("update-ref", "refs/remotes/origin/feat", r.remote)
	return r
}

func (r *rangeRepo) git(args ...string) string {
	r.t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = r.dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		r.t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

// commitAs makes one commit authored AND committed as email, returning its sha.
func (r *rangeRepo) commitAs(email, name, file, subject string) string {
	r.t.Helper()
	r.git("config", "user.email", email)
	r.git("config", "user.name", name)
	writeFile(r.t, filepath.Join(r.dir, file), subject+"\n")
	r.git("add", file)
	r.git("commit", "-m", subject)
	return r.git("rev-parse", "HEAD")
}

func (r *rangeRepo) gate(tip string) error {
	return PublishIdentityMatchesRole(PublishIdentityInput{Dir: r.dir, Base: "main", RemoteTip: tip, Role: "worker"})
}

// wantRefusedNaming asserts exit 5 and that the refusal names every want string.
func wantRefusedNaming(t *testing.T, err error, want ...string) {
	t.Helper()
	if ExitCodeOf(err) != ExitRefused {
		t.Fatalf("exit = %d, want 5 (refused); err = %v", ExitCodeOf(err), err)
	}
	for _, w := range want {
		if !strings.Contains(err.Error(), w) {
			t.Errorf("refusal does not name %q: %s", w, err.Error())
		}
	}
}

// TestPubRangeAcceptsIssueCase — the reported case: the remote PR head holds a commit by a
// different trusted App, the push adds two commits by the pushing identity. Anchored on the
// remote tip — by its remote-tracking ref or by the forge's full sha — it passes. The wide
// walk (no tip) still refuses on the earlier commit, which is the pre-fix behaviour.
func TestPubRangeAcceptsIssueCase(t *testing.T) {
	r := newRangeRepo(t)
	r.commitAs(workerEmail, "Assay Worker", "one.txt", "first new commit")
	r.commitAs(workerEmail, "Assay Worker", "two.txt", "second new commit")

	if err := r.gate("refs/remotes/origin/feat"); err != nil {
		t.Fatalf("anchored on the remote-tracking ref, two own commits must pass, got: %v", err)
	}
	if err := r.gate(r.remote); err != nil {
		t.Fatalf("anchored on the forge-reported head sha, two own commits must pass, got: %v", err)
	}
	wantRefusedNaming(t, r.gate(""), "earlier commit by another App")
}

// TestPubRangeAcceptsMergedMain — a branch that merged main after the remote head: commits
// main brought in are on origin/main and stay excluded; only the session's merge commit and
// own work are judged.
func TestPubRangeAcceptsMergedMain(t *testing.T) {
	r := newRangeRepo(t)
	r.git("checkout", "main")
	r.commitAs("someone@example.org", "Someone", "mainline.txt", "mainline moved on")
	r.git("update-ref", "refs/remotes/origin/main", "HEAD")
	r.git("checkout", "feat")
	r.git("config", "user.email", workerEmail)
	r.git("config", "user.name", "Assay Worker")
	r.git("merge", "--no-ff", "--no-edit", "main")
	r.commitAs(workerEmail, "Assay Worker", "after.txt", "work after the merge")

	if err := r.gate("refs/remotes/origin/feat"); err != nil {
		t.Fatalf("a branch that merged main must pass when every added commit is the worker's, got: %v", err)
	}
}

// TestPubRangeRefusesNewForeign — (a) a foreign-identity commit the push ADDS is refused even
// though the range is anchored on a valid remote tip.
func TestPubRangeRefusesNewForeign(t *testing.T) {
	r := newRangeRepo(t)
	r.commitAs(issueLoopEmail, "Assay Issue Loop", "bad.txt", "added under a foreign identity")
	wantRefusedNaming(t, r.gate("refs/remotes/origin/feat"), "added under a foreign identity", "assay-issue-loop-app")
	wantRefusedNaming(t, r.gate(r.remote), "added under a foreign identity")
}

// TestPubRangeRefusesMixedNew — (b) a mixed push: the earlier remote commit is fine to leave
// alone and two of the new commits are the worker's, but one NEW commit is foreign. It is
// refused, and the refusal names the foreign NEW commit, not the earlier remote one.
func TestPubRangeRefusesMixedNew(t *testing.T) {
	r := newRangeRepo(t)
	r.commitAs(workerEmail, "Assay Worker", "one.txt", "good new commit")
	r.commitAs(issueLoopEmail, "Assay Issue Loop", "bad.txt", "foreign new commit")
	r.commitAs(workerEmail, "Assay Worker", "three.txt", "another good one")
	err := r.gate("refs/remotes/origin/feat")
	wantRefusedNaming(t, err, "foreign new commit")
	if err != nil && strings.Contains(err.Error(), "earlier commit by another App") {
		t.Errorf("the refusal named the earlier REMOTE commit, not the foreign new one: %s", err)
	}
}

// TestPubRangeFailsClosed — (c) a tip that is missing, stale, diverged, unknown, or spelled
// so it could name the pushed commit itself is NEVER used to shrink the range. Every case is
// built so that honouring the tip WOULD hide the foreign commit; the gate must refuse anyway.
func TestPubRangeFailsClosed(t *testing.T) {
	// foreignTip leaves the branch with a foreign commit at HEAD, on top of the remote head.
	foreignTip := func(t *testing.T) (*rangeRepo, string) {
		r := newRangeRepo(t)
		sha := r.commitAs(issueLoopEmail, "Assay Issue Loop", "bad.txt", "foreign commit to hide")
		return r, sha
	}

	t.Run("head_spelling", func(t *testing.T) {
		r, _ := foreignTip(t)
		wantRefusedNaming(t, r.gate("HEAD"), "foreign commit to hide", "was not used")
	})
	t.Run("local_branch_ref", func(t *testing.T) {
		r, _ := foreignTip(t)
		wantRefusedNaming(t, r.gate("refs/heads/feat"), "foreign commit to hide", "was not used")
	})
	t.Run("abbreviated_sha", func(t *testing.T) {
		r, sha := foreignTip(t)
		wantRefusedNaming(t, r.gate(sha[:12]), "foreign commit to hide", "was not used")
	})
	t.Run("revision_syntax", func(t *testing.T) {
		r, _ := foreignTip(t)
		// A remote-tracking ref planted AT the foreign commit, reached through revision syntax.
		r.git("update-ref", "refs/remotes/origin/planted", "HEAD")
		for _, tip := range []string{"refs/remotes/origin/planted^0", "refs/remotes/origin/planted@{0}", "refs/remotes/origin/feat..HEAD"} {
			wantRefusedNaming(t, r.gate(tip), "foreign commit to hide", "was not used")
		}
	})
	t.Run("other_remote", func(t *testing.T) {
		r, _ := foreignTip(t)
		r.git("update-ref", "refs/remotes/upstream/feat", "HEAD")
		wantRefusedNaming(t, r.gate("refs/remotes/upstream/feat"), "foreign commit to hide", "was not used")
	})
	t.Run("missing_tracking_ref_first_push", func(t *testing.T) {
		r, _ := foreignTip(t)
		r.git("update-ref", "-d", "refs/remotes/origin/feat")
		// Wide walk: the earlier remote commit is judged too, and the refusal says why.
		wantRefusedNaming(t, r.gate("refs/remotes/origin/feat"), "was not used", "does not resolve")
	})
	t.Run("unknown_full_sha", func(t *testing.T) {
		r, _ := foreignTip(t)
		wantRefusedNaming(t, r.gate(strings.Repeat("ab", 20)), "was not used", "does not resolve")
	})
	t.Run("stale_behind_tracking_ref", func(t *testing.T) {
		// The remote really holds the earlier commit, but the tracking ref lags at main's
		// seed: the range it yields is a SUPERSET of what the push adds, never smaller.
		r, _ := foreignTip(t)
		r.git("update-ref", "refs/remotes/origin/feat", "refs/remotes/origin/main")
		wantRefusedNaming(t, r.gate("refs/remotes/origin/feat"), "foreign commit to hide")
		commits, err := defaultPublishCommits(r.dir, resolvePublishRange(r.dir, "main", "refs/remotes/origin/feat"))
		if err != nil || len(commits) != 2 {
			t.Fatalf("a lagging tip must judge both the earlier and the new commit, got %d commits (err %v)", len(commits), err)
		}
	})
	t.Run("diverged_tip_not_ancestor", func(t *testing.T) {
		// A tip built ON TOP of the foreign commit, on another line (a force-moved remote
		// head): it excludes the foreign commit, but it is not an ancestor of HEAD, so it is
		// not used.
		r, sha := foreignTip(t)
		r.git("checkout", "-b", "side", sha)
		side := r.commitAs(workerEmail, "Assay Worker", "side.txt", "side line")
		r.git("checkout", "feat")
		r.git("reset", "--hard", sha)
		r.commitAs(workerEmail, "Assay Worker", "main-line.txt", "pushed line")
		r.git("update-ref", "refs/remotes/origin/feat", side)
		wantRefusedNaming(t, r.gate("refs/remotes/origin/feat"), "foreign commit to hide", "not an ancestor")
		wantRefusedNaming(t, r.gate(side), "foreign commit to hide", "not an ancestor")
	})
}

// TestPubRangeResolve pins resolvePublishRange's verdicts directly, so a regression in the
// spelling or ancestry rule is named here rather than inferred from a gate verdict.
func TestPubRangeResolve(t *testing.T) {
	r := newRangeRepo(t)
	r.commitAs(workerEmail, "Assay Worker", "one.txt", "own")

	if rng := resolvePublishRange(r.dir, "main", ""); rng.Narrowed() || rng.Widened != "" {
		t.Errorf("no tip offered: want the whole range and no widening note, got %+v", rng)
	}
	if rng := resolvePublishRange(r.dir, "main", "refs/remotes/origin/feat"); !rng.Narrowed() || rng.Tip != r.remote {
		t.Errorf("valid tracking ref: want narrowed to %s, got %+v", r.remote, rng)
	}
	for _, bad := range []string{"HEAD", "feat", "origin/feat", "refs/heads/feat", "refs/remotes/origin/", "-x", "refs/remotes/origin/feat~1"} {
		if rng := resolvePublishRange(r.dir, "main", bad); rng.Narrowed() || rng.Widened == "" {
			t.Errorf("tip %q must widen with a note, got %+v", bad, rng)
		}
	}
}
