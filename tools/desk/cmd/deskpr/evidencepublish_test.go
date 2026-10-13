package main

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

func evidenceFixture(t *testing.T) (string, *[][]string, string) {
	t.Helper()
	hermeticGitConfig(t)
	work := newBaseFixture(t)
	calls := withEnv(t, work)
	useRealPublishIdentityGate(t)
	// A single-parent Contents API shape: verifier author, provider committer.
	reauthorTip(t, work, "Assay Verifier", prVerifierEmail)
	mustGit(t, work, "-c", "user.name=GitHub", "-c", "user.email=noreply@github.com", "commit", "--amend", "--no-edit")
	remote := mustGit(t, work, "rev-parse", "HEAD")
	mustGit(t, work, "push", "origin", "HEAD:refs/heads/feature/test-branch")
	return work, calls, remote
}

func createEvidence(check bool) int {
	args := []string{"create", "--title", "publish Evidence", "--body-min", "Issue: #1967\nPublish existing Evidence."}
	if check {
		args = append(args, "--check")
	}
	return run(args)
}

// regression: #1967
func TestCreateRemoteEvidence(t *testing.T) {
	for _, add := range []bool{false, true} {
		t.Run(map[bool]string{false: "remote only", true: "new role commit"}[add], func(t *testing.T) {
			work, calls, remote := evidenceFixture(t)
			before := mustGit(t, work, "cat-file", "commit", remote)
			if !add {
				t.Setenv("DESK_LOOP", "verify-desk")
			}
			if add {
				addCommitAs(t, work, "Assay Worker", prWorkerEmail, "new.txt")
			}
			var rc int
			stderr := captureStderr(t, func() { rc = createEvidence(false) })
			if rc != deskkit.ExitOK {
				t.Fatalf("create existing Evidence rc=%d, want 0\n%s", rc, stderr)
			}
			if !anyPush(*calls) || curForge.created == nil {
				t.Fatal("branch/PR was not published")
			}
			bare := filepath.Join(filepath.Dir(work), "origin.git")
			if got := mustGit(t, bare, "cat-file", "commit", remote); got != before {
				t.Fatal("witnessed commit changed")
			}
			if got, want := remoteRef(t, bare, "refs/heads/feature/test-branch"), mustGit(t, work, "rev-parse", "HEAD"); got != want {
				t.Fatalf("remote=%s want %s", got, want)
			}
		})
	}
}

// regression: #1967
func TestCreateEvidenceNewForeign(t *testing.T) {
	for _, role := range []string{"author", "committer", "provider"} {
		t.Run(role, func(t *testing.T) {
			work, calls, remote := evidenceFixture(t)
			addCommitAs(t, work, "Assay Worker", prWorkerEmail, "bad.txt")
			switch role {
			case "author":
				mustGit(t, work, "commit", "--amend", "--no-edit", "--author=Assay Issue Loop <"+prIssueLoopEmail+">")
			case "committer":
				mustGit(t, work, "-c", "user.email="+prIssueLoopEmail, "commit", "--amend", "--no-edit")
			case "provider":
				mustGit(t, work, "-c", "user.email=noreply@github.com", "commit", "--amend", "--no-edit")
			}
			// Plant a stale-ahead local ref: it must not hide the unpublished commit.
			mustGit(t, work, "update-ref", "refs/remotes/origin/feature/test-branch", "HEAD")
			var rc int
			stderr := captureStderr(t, func() { rc = createEvidence(false) })
			if rc != deskkit.ExitRefused || !strings.Contains(stderr, "bad.txt") {
				t.Fatalf("new %s identity rc=%d; must name new commit\n%s", role, rc, stderr)
			}
			if anyPush(*calls) || curForge.created != nil {
				t.Fatal("foreign commit caused a write")
			}
			if got := remoteRef(t, filepath.Join(filepath.Dir(work), "origin.git"), "refs/heads/feature/test-branch"); got != remote {
				t.Fatal("remote moved")
			}
		})
	}
}

// regression: #1967
func TestCreateEvidenceOffline(t *testing.T) {
	_, calls, _ := evidenceFixture(t)
	var rc int
	stderr := captureStderr(t, func() { rc = createEvidence(true) })
	if rc != deskkit.ExitRefused || !strings.Contains(stderr, "remote branch head was not checked") {
		t.Fatalf("offline result rc=%d lacks remote-proof limit\n%s", rc, stderr)
	}
	if anyPush(*calls) || curForge.openCalls != 0 {
		t.Fatal("offline check contacted forge")
	}
}

// regression: #1967
func TestCreateEvidenceRemoteState(t *testing.T) {
	for _, state := range []string{"wrong branch", "unreadable", "divergent", "unfetched", "wrong repo"} {
		t.Run(state, func(t *testing.T) {
			work, calls, tip := evidenceFixture(t)
			bare := filepath.Join(filepath.Dir(work), "origin.git")
			switch state {
			case "wrong branch":
				mustGit(t, bare, "update-ref", "refs/heads/other", tip)
				mustGit(t, bare, "update-ref", "-d", "refs/heads/feature/test-branch")
			case "unreadable":
				mustGit(t, work, "remote", "set-url", "--push", "origin", "file://"+filepath.Join(t.TempDir(), "missing.git"))
			case "divergent", "unfetched":
				mustGit(t, work, "checkout", "-b", "side", tip)
				addCommitAs(t, work, "Assay Worker", prWorkerEmail, "side.txt")
				mustGit(t, work, "push", "origin", "HEAD:refs/heads/feature/test-branch")
				mustGit(t, work, "checkout", "feature/test-branch")
				if state == "unfetched" {
					tree := mustGit(t, bare, "rev-parse", "refs/heads/feature/test-branch^{tree}")
					unknown := mustGit(t, bare, "-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit-tree", tree, "-p", tip, "-m", "remote only")
					mustGit(t, bare, "update-ref", "refs/heads/feature/test-branch", unknown)
				}
			case "wrong repo":
				mustGit(t, work, "remote", "set-url", "--push", "origin", "https://github.com/another-org/tracker.git")
			}
			rc := createEvidence(false)
			if rc == deskkit.ExitOK {
				t.Fatal("unproven remote state admitted")
			}
			if state == "unreadable" && rc != deskkit.ExitUnverifiable {
				t.Fatalf("unreadable remote rc=%d, want 6", rc)
			}
			if anyPush(*calls) || curForge.created != nil {
				t.Fatal("unproven remote state caused a write")
			}
		})
	}
}

// regression: #1967
func TestCreateEvidenceRace(t *testing.T) {
	work, _, tip := evidenceFixture(t)
	addCommitAs(t, work, "Assay Worker", prWorkerEmail, "new.txt")
	// Rewind AFTER range checking, from the pre-push hook. The resulting push
	// would still be a fast-forward, but would introduce unchecked history.
	bare := filepath.Join(filepath.Dir(work), "origin.git")
	base := mustGit(t, work, "rev-parse", "refs/remotes/origin/main")
	plantHook(t, work, "git --git-dir='"+bare+"' update-ref refs/heads/feature/test-branch "+base+" "+tip+"\n")
	if rc := createEvidence(false); rc == deskkit.ExitOK {
		t.Fatal("push accepted a changed remote tip")
	}
	if curForge.created != nil {
		t.Fatal("opened PR after failed push")
	}
	if got := remoteRef(t, bare, "refs/heads/feature/test-branch"); got != base {
		t.Fatalf("push moved rewound remote to %s", got)
	}
}

// regression: #1967
func TestCreateProofBinding(t *testing.T) {
	for _, changed := range []string{"branch", "source", "destination"} {
		t.Run(changed, func(t *testing.T) {
			work, _, _ := evidenceFixture(t)
			spec, err := prepareCreatePush(branchSpec(work), "main")
			if err != nil {
				t.Fatal(err)
			}
			switch changed {
			case "branch":
				spec.dstBranch = "different"
			case "source":
				addCommitAs(t, work, "Assay Worker", prWorkerEmail, "late.txt")
			case "destination":
				other := filepath.Join(t.TempDir(), "other.git")
				mustGit(t, "", "init", "--bare", other)
				mustGit(t, work, "remote", "set-url", "--push", "origin", "file://"+other)
			}
			if err := pushBranch(spec); err == nil || !strings.Contains(err.Error(), "changed after") {
				t.Fatalf("reused mismatched proof: %v", err)
			}
		})
	}
}

// regression: #1967
func TestCreateProofRemoteAhead(t *testing.T) {
	work := newBaseFixture(t)
	withEnv(t, work)
	useRealPublishIdentityGate(t)
	reauthorTip(t, work, "Assay Worker", prWorkerEmail)
	local := mustGit(t, work, "rev-parse", "HEAD")
	addCommitAs(t, work, "Assay Worker", prWorkerEmail, "remote.txt")
	mustGit(t, work, "push", "origin", "HEAD:refs/heads/feature/test-branch")
	mustGit(t, work, "reset", "--hard", local)
	// Every local commit has the right identity, so only the destination proof
	// can refuse here. A fallback to the wide identity check would pass.
	if _, err := prepareCreatePush(branchSpec(work), "main"); err == nil {
		t.Fatal("destination ahead of HEAD admitted")
	}
}

// regression: #1967
func TestCreateProofSnapshot(t *testing.T) {
	for _, when := range []string{"before preparation", "during identity check"} {
		t.Run(when, func(t *testing.T) {
			work, calls, remote := evidenceFixture(t)
			addCommitAs(t, work, "Assay Issue Loop", prIssueLoopEmail, "unchecked.txt")
			spec := branchSpec(work)
			moveHead := func() { mustGit(t, work, "checkout", "--detach", remote) }
			if when == "before preparation" {
				moveHead()
			} else {
				checked := publishIdentityGateFn
				publishIdentityGateFn = func(in deskkit.PublishIdentityInput) error {
					moveHead()
					return checked(in)
				}
			}
			if _, err := prepareCreatePush(spec, "main"); err == nil {
				t.Fatal("source differing from the identity snapshot admitted")
			}
			if anyPush(*calls) || curForge.created != nil {
				t.Fatal("snapshot mismatch caused publication")
			}
		})
	}
}

// regression: #1967
func TestCreateProofHeadMismatch(t *testing.T) {
	work, _, remote := evidenceFixture(t)
	addCommitAs(t, work, "Assay Worker", prWorkerEmail, "source.txt")
	spec := branchSpec(work)
	mustGit(t, work, "checkout", "--detach", remote)
	if _, err := prepareCreatePush(spec, "main"); err == nil || !strings.Contains(err.Error(), "differs from") {
		t.Fatalf("correct-role source mismatching HEAD was not refused: %v", err)
	}
}

// regression: #1967
func TestUpdateSourceSnapshot(t *testing.T) {
	work, _, remote := mixedPRFixture(t)
	t.Setenv("FAKEGH_PR_OID", remote)
	addTwoWorkerCommits(t, work)
	checked := publishIdentityGateFn
	checks := 0
	publishIdentityGateFn = func(in deskkit.PublishIdentityInput) error {
		checks++
		err := checked(in)
		if checks == 2 && err == nil {
			// The live stage checked its snapshot; a concurrent local commit
			// must not replace it between the gate and the sibling push.
			addCommitAs(t, work, "Assay Issue Loop", prIssueLoopEmail, "late.txt")
		}
		return err
	}
	var rc int
	stderr := captureStderr(t, func() { rc = run([]string{"update"}) })
	if checks != 2 || rc != deskkit.ExitRefused || !strings.Contains(stderr, "source changed after") {
		t.Fatalf("update snapshot binding checks=%d rc=%d: %s", checks, rc, stderr)
	}
}
