package main

// check_test.go — end-to-end wiring: cmdCheck against a RECORDING fake forge, so "run the
// check against a fixture PR" (Verify rows 1-4's own wording) exercises the real read →
// Evaluate → write pipeline, not just the pure function protected_test.go already covers.

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

const fixtureRepo = "medici-finance/assay"

type fakeForge struct {
	deskkit.Forge
	issue     *deskkit.Issue
	issueErr  error
	files     []deskkit.ChangedFile
	filesErr  error
	diff      string
	diffErr   error
	applyErr  error
	diffCalls int
	applied   []deskkit.LabelChange
}

func (f *fakeForge) GetIssue(fr deskkit.ForgeRepo, n int) (*deskkit.Issue, error) {
	if f.issueErr != nil {
		return nil, f.issueErr
	}
	iss := *f.issue
	iss.Number = n
	return &iss, nil
}

func (f *fakeForge) ListChangedFiles(fr deskkit.ForgeRepo, n int) ([]deskkit.ChangedFile, error) {
	if f.filesErr != nil {
		return nil, f.filesErr
	}
	return f.files, nil
}

func (f *fakeForge) ChangeDiff(fr deskkit.ForgeRepo, n int) (string, error) {
	f.diffCalls++
	if f.diffErr != nil {
		return "", f.diffErr
	}
	return f.diff, nil
}

func (f *fakeForge) ApplyLabels(fr deskkit.ForgeRepo, n int, change deskkit.LabelChange) (*deskkit.LabelOutcome, error) {
	f.applied = append(f.applied, change)
	if f.applyErr != nil {
		return nil, f.applyErr
	}
	out := &deskkit.LabelOutcome{}
	for _, l := range change.Add {
		out.Added = append(out.Added, l.Name)
	}
	return out, nil
}

func plantCheckWorld(t *testing.T, fg deskkit.Forge) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	plantFixtureRoster(t, home)
	t.Setenv("DESK_TOOLS_DISABLED", "")
	t.Setenv("CLAUDE_SESSION_ID", "deskpathguard-test")
	t.Setenv("DESK_LOOP", "")

	oldRole, oldForge, oldMinted, oldToken := roleFn, forgeForFn, mintedRole, ghToken
	roleFn = func() (string, error) { return "reviewer", nil }
	forgeForFn = func(repo string) (deskkit.Forge, deskkit.ForgeRepo, error) {
		owner, name, _ := strings.Cut(repo, "/")
		return fg, deskkit.ForgeRepo{Owner: owner, Name: name}, nil
	}
	mintedRole, ghToken = "", ""
	t.Cleanup(func() {
		roleFn, forgeForFn, mintedRole, ghToken = oldRole, oldForge, oldMinted, oldToken
	})
}

func TestCmdCheckRow1LabelsWorkerAuthoredPathAndCodeDiff(t *testing.T) {
	fg := &fakeForge{
		issue: &deskkit.Issue{IsPullRequest: true, Author: deskkit.Account{Login: workerLogin}},
		files: []deskkit.ChangedFile{
			{Filename: "statusgen/unrun.go"},
			{Filename: "docs/streams/verify-integrity/brief-01-protected-verifier-paths.md"},
		},
		diff: verifyTableEditDiff,
	}
	plantCheckWorld(t, fg)

	var buf bytes.Buffer
	err := cmdCheck([]string{fixtureRepo, "42"}, &buf)
	if err != nil {
		t.Fatalf("cmdCheck: unexpected error: %v (out=%s)", err, buf.String())
	}
	if !strings.Contains(buf.String(), gateForcedLine) {
		t.Fatalf("stdout must carry the gate-forced line; got: %s", buf.String())
	}
	if fg.diffCalls != 1 {
		t.Fatalf("want exactly one ChangeDiff call (a brief file was touched), got %d", fg.diffCalls)
	}
	if len(fg.applied) != 1 || len(fg.applied[0].Add) != 1 || fg.applied[0].Add[0].Name != wroteToTheTestLabel {
		t.Fatalf("want one ApplyLabels call adding %q, got %+v", wroteToTheTestLabel, fg.applied)
	}
}

func TestCmdCheckDeskIdentityExemptNoForgeWrite(t *testing.T) {
	fg := &fakeForge{
		issue: &deskkit.Issue{IsPullRequest: true, Author: deskkit.Account{Login: deskLogin}},
		files: []deskkit.ChangedFile{
			{Filename: "statusgen/unrun.go"},
			{Filename: "docs/streams/verify-integrity/brief-01-protected-verifier-paths.md"},
		},
		diff: verifyTableEditDiff,
	}
	plantCheckWorld(t, fg)

	var buf bytes.Buffer
	if err := cmdCheck([]string{fixtureRepo, "42"}, &buf); err != nil {
		t.Fatalf("cmdCheck: unexpected error: %v", err)
	}
	if len(fg.applied) != 0 {
		t.Fatalf("the desk identity must never be labelled: applied=%+v", fg.applied)
	}
	if !strings.Contains(buf.String(), "label: none") {
		t.Fatalf("want an explicit 'label: none' line, got: %s", buf.String())
	}
}

// TestCmdCheckRow4UnreadableDiffIsCouldNotCheck is Verify row 4: a PR whose changed-files
// read fails (a deleted head) is could-not-check, exits non-OK, and never applies a label.
func TestCmdCheckRow4UnreadableDiffIsCouldNotCheck(t *testing.T) {
	fg := &fakeForge{
		issue:    &deskkit.Issue{IsPullRequest: true, Author: deskkit.Account{Login: workerLogin}},
		filesErr: errors.New("404: head ref deleted"),
	}
	plantCheckWorld(t, fg)

	var buf bytes.Buffer
	err := cmdCheck([]string{fixtureRepo, "42"}, &buf)
	if err == nil {
		t.Fatal("want a could-not-check error, got nil")
	}
	if deskkit.ExitCodeOf(err) != deskkit.ExitUnverifiable {
		t.Fatalf("want exit %d (unverifiable), got %d: %v", deskkit.ExitUnverifiable, deskkit.ExitCodeOf(err), err)
	}
	if strings.Contains(buf.String(), "label: none") || strings.Contains(buf.String(), "checked-clean") {
		t.Fatalf("could-not-check must never read as clean: %s", buf.String())
	}
	if len(fg.applied) != 0 {
		t.Fatalf("a could-not-check diff must never apply a label: %+v", fg.applied)
	}
}
