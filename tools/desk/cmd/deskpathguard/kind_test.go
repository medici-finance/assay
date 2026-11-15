package main

// kind_test.go — `check --kind` (#5032): on a forge whose issue and merge-request numbers
// collide, the untyped read refuses could-not-check, and --kind mr reads and evaluates the
// merge request only. The issue at the same number is never read.

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// collideText is the shared resolver's refusal for a number that names both objects
// (internal/deskkit/forge_gitlab.go getIssue), reproduced so the fake refuses in its words.
const collideText = "could-not-check: %s carries BOTH issue #%d and merge request !%d"

// collidingForge serves an issue AND a merge request at every number, like GitLab's two
// sequences. The untyped GetIssue refuses as the real resolver does; the typed read hands
// back the object of the stated kind and records which kinds were asked for.
type collidingForge struct {
	fakeForge
	untypedReads int
	typedKinds   []deskkit.TargetKind
	issueObj     *deskkit.Issue
}

func (f *collidingForge) GetIssue(fr deskkit.ForgeRepo, n int) (*deskkit.Issue, error) {
	f.untypedReads++
	return nil, deskkit.Unverifiable(fmt.Sprintf(collideText, fr.Slug(), n, n), nil)
}

func (f *collidingForge) GetIssueTyped(fr deskkit.ForgeRepo, n int, kind deskkit.TargetKind) (*deskkit.Issue, error) {
	f.typedKinds = append(f.typedKinds, kind)
	src := f.issue
	if kind == deskkit.TargetIssue {
		src = f.issueObj
	}
	iss := *src
	iss.Number = n
	return &iss, nil
}

// newCollidingForge: the merge request is worker-authored and touches a protected path
// (so it is labelled); the issue at the same number is desk-authored (exempt). Evaluating
// the wrong object therefore flips the verdict, which is what the tests below watch.
func newCollidingForge() *collidingForge {
	return &collidingForge{
		fakeForge: fakeForge{
			issue: &deskkit.Issue{IsPullRequest: true, Author: deskkit.Account{Login: workerLogin}},
			files: []deskkit.ChangedFile{
				{Filename: "statusgen/unrun.go"},
				{Filename: "docs/streams/verify-integrity/brief-01-protected-verifier-paths.md"},
			},
			diff: verifyTableEditDiff,
		},
		issueObj: &deskkit.Issue{Author: deskkit.Account{Login: deskLogin}},
	}
}

// regression: #5032
func TestCheckKindMRReadsOnlyTheMR(t *testing.T) {
	for _, val := range []string{"mr", "pr", "MR"} {
		t.Run(val, func(t *testing.T) {
			fg := newCollidingForge()
			plantCheckWorld(t, fg)
			var buf bytes.Buffer
			if err := cmdCheck([]string{fixtureRepo, "42", "--kind", val}, &buf); err != nil {
				t.Fatalf("--kind %s on a colliding number: %v (out=%s)", val, err, buf.String())
			}
			if fg.untypedReads != 0 {
				t.Fatalf("--kind %s must not take the untyped read; got %d", val, fg.untypedReads)
			}
			if len(fg.typedKinds) != 1 || fg.typedKinds[0] != deskkit.TargetChange {
				t.Fatalf("--kind %s must read the merge request only; typed reads = %v", val, fg.typedKinds)
			}
			if !strings.Contains(buf.String(), gateForcedLine) {
				t.Fatalf("the merge request's verdict must be evaluated; got: %s", buf.String())
			}
			if len(fg.applied) != 1 {
				t.Fatalf("want one label write on the change, got %+v", fg.applied)
			}
		})
	}
}

// regression: #5032
func TestCheckKindIssueRefusedTouchesNothing(t *testing.T) {
	fg := newCollidingForge()
	plantCheckWorld(t, fg)
	var buf bytes.Buffer
	err := cmdCheck([]string{fixtureRepo, "42", "--kind", "issue"}, &buf)
	if code := deskkit.ExitCodeOf(err); code != deskkit.ExitRefused {
		t.Fatalf("--kind issue must refuse exit %d; got %d (%v)", deskkit.ExitRefused, code, err)
	}
	if !strings.Contains(err.Error(), "an issue has no diff") {
		t.Fatalf("refusal must say why: %v", err)
	}
	if fg.untypedReads != 0 || len(fg.typedKinds) != 0 || fg.diffCalls != 0 || len(fg.applied) != 0 {
		t.Fatalf("--kind issue must make no forge call; untyped=%d typed=%v diff=%d applied=%v",
			fg.untypedReads, fg.typedKinds, fg.diffCalls, fg.applied)
	}
	if buf.Len() != 0 {
		t.Fatalf("a refusal prints no state= line; got %s", buf.String())
	}
}

// regression: #5032
func TestCheckNoKindStillRefusesCollision(t *testing.T) {
	fg := newCollidingForge()
	plantCheckWorld(t, fg)
	var buf bytes.Buffer
	err := cmdCheck([]string{fixtureRepo, "42"}, &buf)
	if code := deskkit.ExitCodeOf(err); code != deskkit.ExitUnverifiable {
		t.Fatalf("no --kind on a colliding number must stay exit %d; got %d (%v)", deskkit.ExitUnverifiable, code, err)
	}
	if !strings.Contains(err.Error(), "carries BOTH issue #42 and merge request !42") {
		t.Fatalf("the could-not-check text must be unchanged; got %v", err)
	}
	if len(fg.typedKinds) != 0 || len(fg.applied) != 0 || strings.Contains(buf.String(), "state=") {
		t.Fatalf("no typed read, no label, no state= line; typed=%v applied=%v out=%s",
			fg.typedKinds, fg.applied, buf.String())
	}
}

// regression: #5032
func TestCheckBadKindRefusedLikeSiblings(t *testing.T) {
	fg := newCollidingForge()
	plantCheckWorld(t, fg)
	var buf bytes.Buffer
	err := cmdCheck([]string{fixtureRepo, "42", "--kind", "bogus"}, &buf)
	_, want := deskkit.ParseTargetKind("bogus")
	if err == nil || err.Error() != want.Error() {
		t.Fatalf("a bad --kind must refuse in the shared parser's words %q; got %v", want, err)
	}
	if code := deskkit.ExitCodeOf(err); code != deskkit.ExitRefused {
		t.Fatalf("bad --kind exit = %d, want %d", code, deskkit.ExitRefused)
	}
	if fg.untypedReads != 0 || len(fg.typedKinds) != 0 {
		t.Fatalf("a bad --kind must make no forge call")
	}
}

// On a forge where kinds do not collide (GitHub), --kind mr changes nothing: same verdict,
// same state= line, same label write, same exit — including under --dry-run.
// regression: #5032
func TestCheckKindHarmlessWithoutCollision(t *testing.T) {
	for _, dry := range []bool{false, true} {
		run := func(extra ...string) (string, int, int) {
			fg := &plainTypedForge{fakeForge: *newCollidingForge().fakeForge.clone()}
			plantCheckWorld(t, fg)
			args := append([]string{fixtureRepo, "42"}, extra...)
			if dry {
				args = append(args, "--dry-run")
			}
			var buf bytes.Buffer
			err := cmdCheck(args, &buf)
			return buf.String(), deskkit.ExitCodeOf(err), len(fg.applied)
		}
		outA, codeA, appA := run()
		outB, codeB, appB := run("--kind", "mr")
		if outA != outB || codeA != codeB || appA != appB {
			t.Fatalf("dry=%v: --kind mr must not change a non-colliding check:\n untyped: %q exit %d writes %d\n typed:   %q exit %d writes %d",
				dry, outA, codeA, appA, outB, codeB, appB)
		}
	}
}

// plainTypedForge answers the typed read with the same object as the untyped one, as
// GitHub's GetIssueTyped does for a pull request.
type plainTypedForge struct{ fakeForge }

func (f *plainTypedForge) GetIssueTyped(fr deskkit.ForgeRepo, n int, kind deskkit.TargetKind) (*deskkit.Issue, error) {
	return f.GetIssue(fr, n)
}

func (f fakeForge) clone() *fakeForge { c := f; c.applied = nil; return &c }
