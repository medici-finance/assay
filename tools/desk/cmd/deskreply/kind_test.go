package main

// kind_test.go — `--kind` (#5020): on a forge whose issue and merge-request numbers collide,
// the untyped post refuses could-not-check, and --kind mr posts on the merge request's
// thread only. Nothing is ever read or written on the issue at the same number.

import (
	"fmt"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// collidingForge serves an open merge request at feature/test-branch AND an issue at every
// number, like GitLab's two sequences. The untyped writes and reads refuse in the shared
// resolver's words (internal/deskkit/forge_gitlab.go getIssue); the typed post records the
// kind it was asked for.
type collidingForge struct {
	deskkit.Forge
	untyped    int
	typedKinds []deskkit.TargetKind
}

func collideErr(fr deskkit.ForgeRepo, n int) error {
	return deskkit.Unverifiable(fmt.Sprintf(
		"could-not-check: %s carries BOTH issue #%d and merge request !%d", fr.Slug(), n, n), nil)
}

func (f *collidingForge) GetPullRequest(fr deskkit.ForgeRepo, n int) (*deskkit.PullRequest, error) {
	return &deskkit.PullRequest{Number: n, State: "open", HeadRef: "feature/test-branch", HeadSHA: "0000000000000000000000000000000000000001"}, nil
}

func (f *collidingForge) GetIssue(fr deskkit.ForgeRepo, n int) (*deskkit.Issue, error) {
	f.untyped++
	return nil, collideErr(fr, n)
}

func (f *collidingForge) PostComment(fr deskkit.ForgeRepo, n int, body string) (*deskkit.CommentRef, error) {
	f.untyped++
	return nil, collideErr(fr, n)
}

func (f *collidingForge) PostCommentTyped(fr deskkit.ForgeRepo, n int, kind deskkit.TargetKind, body string) (*deskkit.CommentRef, error) {
	f.typedKinds = append(f.typedKinds, kind)
	return &deskkit.CommentRef{DatabaseID: 901}, nil
}

func plantColliding(t *testing.T) *collidingForge {
	t.Helper()
	fg := &collidingForge{}
	old := forgeForFn
	forgeForFn = func(repo string) (deskkit.Forge, deskkit.ForgeRepo, error) {
		owner, name := splitOwnerRepo(repo)
		return fg, deskkit.ForgeRepo{Owner: owner, Name: name}, nil
	}
	t.Cleanup(func() { forgeForFn = old })
	return fg
}

// regression: #5020
func TestReplyKindMRPostsOnTheMROnly(t *testing.T) {
	for _, val := range []string{"mr", "pr", "MR"} {
		t.Run(val, func(t *testing.T) {
			work := newBaseFixture(t)
			withEnv(t, work)
			fg := plantColliding(t)
			bf := bodyFileWith(t, "a plain reply body")
			if err := cmdReply([]string{"example-org/tracker", "7", "--body-file", bf, "--kind", val}); err != nil {
				t.Fatalf("--kind %s on a colliding number: %v", val, err)
			}
			if fg.untyped != 0 {
				t.Fatalf("--kind %s must not take the untyped path; got %d untyped calls", val, fg.untyped)
			}
			if len(fg.typedKinds) != 1 || fg.typedKinds[0] != deskkit.TargetChange {
				t.Fatalf("--kind %s must post on the merge request only; typed posts = %v", val, fg.typedKinds)
			}
		})
	}
}

// regression: #5020
func TestReplyKindMRWorkpadCreatesOnTheMR(t *testing.T) {
	work := newBaseFixture(t)
	withEnv(t, work)
	fg := plantColliding(t)
	swapWorkpadSeams(t,
		func(deskkit.Forge, deskkit.ForgeRepo, int, string) ([]workpadCandidate, error) { return nil, nil },
		func(deskkit.Forge, deskkit.ForgeRepo, string, string) error {
			t.Fatal("no candidate: the workpad must create, not edit")
			return nil
		})
	bf := bodyFileWith(t, workpadBody("example@abc1234", "- a step"))
	if err := cmdReply([]string{"example-org/tracker", "7", "--workpad", "--body-file", bf, "--kind", "mr"}); err != nil {
		t.Fatalf("--workpad --kind mr on a colliding number: %v", err)
	}
	if fg.untyped != 0 || len(fg.typedKinds) != 1 || fg.typedKinds[0] != deskkit.TargetChange {
		t.Fatalf("the workpad create must post on the merge request only; untyped=%d typed=%v", fg.untyped, fg.typedKinds)
	}
}

// regression: #5020
func TestReplyKindMRDryRunWritesNothing(t *testing.T) {
	work := newBaseFixture(t)
	withEnv(t, work)
	fg := plantColliding(t)
	bf := bodyFileWith(t, "a plain reply body")
	out := captureStdout(t, func() {
		if err := cmdReply([]string{"example-org/tracker", "7", "--body-file", bf, "--kind", "mr", "--dry-run"}); err != nil {
			t.Fatalf("--kind mr --dry-run: %v", err)
		}
	})
	if !strings.Contains(out, "DRY-RUN: would post on PR #7") {
		t.Fatalf("dry-run stdout = %q", out)
	}
	if fg.untyped != 0 || len(fg.typedKinds) != 0 {
		t.Fatalf("--dry-run must write nothing; untyped=%d typed=%v", fg.untyped, fg.typedKinds)
	}
}

// regression: #5020
func TestReplyKindIssueRefusedTouchesNothing(t *testing.T) {
	work := newBaseFixture(t)
	calls := withEnv(t, work)
	fg := plantColliding(t)
	bf := bodyFileWith(t, "a plain reply body")
	err := cmdReply([]string{"example-org/tracker", "7", "--body-file", bf, "--kind", "issue"})
	if code := deskkit.ExitCodeOf(err); code != deskkit.ExitRefused {
		t.Fatalf("--kind issue must refuse exit %d; got %d (%v)", deskkit.ExitRefused, code, err)
	}
	if !strings.Contains(err.Error(), "deskfile attach --kind issue") {
		t.Fatalf("refusal must name the verb that comments on an issue: %v", err)
	}
	if fg.untyped != 0 || len(fg.typedKinds) != 0 {
		t.Fatalf("--kind issue must make no forge call; untyped=%d typed=%v", fg.untyped, fg.typedKinds)
	}
	if len(*calls) != 0 {
		t.Fatalf("--kind issue must refuse before any mint or git call; got %v", *calls)
	}
}

// regression: #5020
func TestReplyNoKindStillRefusesCollision(t *testing.T) {
	work := newBaseFixture(t)
	withEnv(t, work)
	fg := plantColliding(t)
	bf := bodyFileWith(t, "a plain reply body")
	err := cmdReply([]string{"example-org/tracker", "7", "--body-file", bf})
	if code := deskkit.ExitCodeOf(err); code != deskkit.ExitUnverifiable {
		t.Fatalf("no --kind on a colliding number must stay exit %d; got %d (%v)", deskkit.ExitUnverifiable, code, err)
	}
	if !strings.Contains(err.Error(), "carries BOTH issue #7 and merge request !7") {
		t.Fatalf("the could-not-check text must be unchanged; got %v", err)
	}
	if len(fg.typedKinds) != 0 {
		t.Fatalf("no --kind must never take the typed post; got %v", fg.typedKinds)
	}
}

// regression: #5020
func TestReplyBadKindRefusedLikeSiblings(t *testing.T) {
	work := newBaseFixture(t)
	withEnv(t, work)
	fg := plantColliding(t)
	bf := bodyFileWith(t, "a plain reply body")
	err := cmdReply([]string{"example-org/tracker", "7", "--body-file", bf, "--kind", "bogus"})
	_, want := deskkit.ParseTargetKind("bogus")
	if err == nil || err.Error() != want.Error() {
		t.Fatalf("a bad --kind must refuse in the shared parser's words %q; got %v", want, err)
	}
	if code := deskkit.ExitCodeOf(err); code != deskkit.ExitRefused {
		t.Fatalf("bad --kind exit = %d, want %d", code, deskkit.ExitRefused)
	}
	if fg.untyped != 0 || len(fg.typedKinds) != 0 {
		t.Fatalf("a bad --kind must make no forge call")
	}
}

// On GitHub (no collision) --kind mr is accepted and lands exactly where the untyped reply
// lands: the real GitHub PostCommentTyped is PostComment once the kind is validated.
// regression: #5020
func TestReplyKindMRHarmlessOnGitHub(t *testing.T) {
	work := newBaseFixture(t)
	withEnv(t, work)
	bf := bodyFileWith(t, "a plain reply body")
	if rc := run([]string{"example-org/tracker", "7", "--body-file", bf, "--kind", "mr"}); rc != deskkit.ExitOK {
		t.Fatalf("--kind mr on GitHub rc = %d, want 0", rc)
	}
	if !forgeRec(t).posted() {
		t.Fatalf("--kind mr on GitHub must post the reply as the untyped path does: %v", forgeRec(t).requests)
	}
}
