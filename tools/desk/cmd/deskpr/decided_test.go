package main

// decided_test.go — `deskpr create|edit --decided` (attention-budget/19).
//
// FAIL-FIRST. Every case here was RED before decided.go and the --decided wiring existed:
// with no --decided flag registered, `run([]string{"create", "--decided", path, …})` failed
// flag parsing (`flag provided but not defined: -decided`) and returned deskkit.ExitRefused
// for the wrong reason — a body/label assertion that reads a create which never happened.
// Observed directly:
//
//	$ go test ./cmd/deskpr/... -run TestCreateDecidedAppliesLabelAndBlock -v
//	--- FAIL: TestCreateDecidedAppliesLabelAndBlock (0.00s)
//	    decided_test.go:NN: create rc = 5, want 0; stderr carried "flag provided but not
//	    defined: -decided"

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

func writeDecidedFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "decided.txt")
	writeFile(t, path, content)
	return path
}

const oneDecidedItem = "1. decision: ship the one-line patch\n" +
	"   alternative: ask first\n" +
	"   cost: one revert\n"

// TestCreateDecidedAppliesLabelAndBlock is Verify row 2: a one-line patch to a settled
// design with one declared decision → the fake forge records the desk-decided label
// (created, since the fixture repo carries none) and the body carries the heading, the
// marker and the three fields.
func TestCreateDecidedAppliesLabelAndBlock(t *testing.T) {
	work := newBaseFixture(t)
	withEnv(t, work)
	decidedPath := writeDecidedFile(t, oneDecidedItem)

	rc := run([]string{"create", "--title", "a one-line patch to a settled design",
		"--body-min", "does the thing\nBrief: fixture/01", "--decided", decidedPath})
	if rc != deskkit.ExitOK {
		t.Fatalf("create rc = %d, want 0", rc)
	}

	if curForge.created == nil {
		t.Fatal("no CreateDraftChange call was recorded")
	}
	body := curForge.created.Body
	if !strings.Contains(body, deskkit.DeskDecidedHeading) {
		t.Errorf("PR body does not carry the %q heading: %q", deskkit.DeskDecidedHeading, body)
	}
	if !strings.Contains(body, deskkit.DeskDecidedMarker) {
		t.Errorf("PR body does not carry the marker: %q", body)
	}
	for _, want := range []string{"decision: ship the one-line patch", "alternative: ask first", "cost: one revert"} {
		if !strings.Contains(body, want) {
			t.Errorf("PR body is missing %q: %q", want, body)
		}
	}

	if len(curForge.labelChanges) != 1 {
		t.Fatalf("ApplyLabels called %d time(s), want exactly 1: %v", len(curForge.labelChanges), curForge.labelChanges)
	}
	lc := curForge.labelChanges[0]
	if lc.Target != deskkit.TargetChange {
		t.Errorf("LabelChange.Target = %v, want TargetChange", lc.Target)
	}
	foundLabel := false
	for _, l := range lc.Add {
		if l.Name == deskkit.DeskDecidedLabel {
			foundLabel = true
		}
	}
	if !foundLabel {
		t.Errorf("LabelChange.Add does not carry %q: %+v", deskkit.DeskDecidedLabel, lc.Add)
	}
	if curForge.labelNums[0] != 101 {
		t.Errorf("label applied to PR #%d, want the just-created #101", curForge.labelNums[0])
	}
}

// TestDecidedEmptyListRefused is Verify row 3: an empty file and an item with no cost: both
// exit 5 and no PR call is made.
func TestDecidedEmptyListRefused(t *testing.T) {
	cases := []struct {
		name    string
		content string
	}{
		{name: "empty file", content: ""},
		{name: "item missing cost", content: "1. decision: ship it\n   alternative: ask first\n"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			work := newBaseFixture(t)
			withEnv(t, work)
			decidedPath := writeDecidedFile(t, c.content)

			rc := run([]string{"create", "--title", "a change", "--body-min",
				"does the thing\nBrief: fixture/01", "--decided", decidedPath})
			if rc != deskkit.ExitRefused {
				t.Fatalf("create rc = %d, want %d (refused)", rc, deskkit.ExitRefused)
			}
			if curForge.createCalls != 0 {
				t.Errorf("a malformed --decided file still called CreateDraftChange %d time(s)", curForge.createCalls)
			}
			if len(curForge.labelChanges) != 0 {
				t.Errorf("a malformed --decided file still applied labels: %v", curForge.labelChanges)
			}
		})
	}
}

// TestCreateWithoutDecidedCarriesNeither is Verify row 4: the transcribe-only fixture gets
// no label and no heading — today's behaviour byte-for-byte. Every existing create test
// already proves the body is unaffected; this pins the ADDITIONAL claim that no label call
// is made either, now that --decided exists as a flag ApplyLabels could have been reached
// from unconditionally.
func TestCreateWithoutDecidedCarriesNeither(t *testing.T) {
	work := newBaseFixture(t)
	withEnv(t, work)

	rc := run([]string{"create", "--title", "transcribes a recorded ruling",
		"--body-min", "does the thing\nBrief: fixture/01"})
	if rc != deskkit.ExitOK {
		t.Fatalf("create rc = %d, want 0", rc)
	}
	if curForge.created == nil {
		t.Fatal("no CreateDraftChange call was recorded")
	}
	if strings.Contains(curForge.created.Body, deskkit.DeskDecidedHeading) {
		t.Errorf("body carries the Desk-decided heading with no --decided given: %q", curForge.created.Body)
	}
	if len(curForge.labelChanges) != 0 {
		t.Errorf("ApplyLabels was called with no --decided given: %v", curForge.labelChanges)
	}
}

// TestCreateRefusesAHandWrittenDeskDecidedHeading pins the create-only refusal: a
// caller-supplied body that already carries a hand-written `## Desk-decided` heading and
// --decided together are refused, before any network call.
func TestCreateRefusesAHandWrittenDeskDecidedHeading(t *testing.T) {
	work := newBaseFixture(t)
	withEnv(t, work)
	decidedPath := writeDecidedFile(t, oneDecidedItem)
	bodyPath := writeTempFile(t, "does the thing\nBrief: fixture/01\n\n## Desk-decided\n\nsomeone typed this by hand\n")

	rc := run([]string{"create", "--title", "a change", "--body-file", bodyPath, "--decided", decidedPath})
	if rc != deskkit.ExitRefused {
		t.Fatalf("create rc = %d, want %d (refused)", rc, deskkit.ExitRefused)
	}
	if curForge.createCalls != 0 {
		t.Errorf("a hand-written heading + --decided still called CreateDraftChange %d time(s)", curForge.createCalls)
	}
}

// TestEditDecidedReplacesTheBlockInPlace proves edit's asymmetry with create: a replacement
// body that already carries the tool's own prior block gets it REPLACED in place by
// --decided, not refused and not duplicated.
func TestEditDecidedReplacesTheBlockInPlace(t *testing.T) {
	work := newBaseFixture(t)
	withEnv(t, work)
	t.Setenv("FAKEGH_LIST_HAS_PR", "1")
	priorBody := "the original body\nBrief: fixture/01\n\n" + deskkit.RenderDecidedBlock([]deskkit.DecidedItem{
		{Decision: "old choice", Alternative: "old alt", Cost: "old cost"},
	})
	t.Setenv("FAKEGH_PR_BODY", priorBody)
	newDecidedPath := writeDecidedFile(t, "1. decision: new choice\n   alternative: new alt\n   cost: new cost\n")
	bodyPath := writeTempFile(t, priorBody)

	rc := run([]string{"edit", "--body-file", bodyPath, "--decided", newDecidedPath})
	if rc != deskkit.ExitOK {
		t.Fatalf("edit rc = %d, want 0", rc)
	}
	if curForge.edited == nil {
		t.Fatal("no EditChange call was recorded")
	}
	body := curForge.edited.Body
	if strings.Contains(body, "old choice") {
		t.Errorf("the prior block's content survived the replace: %q", body)
	}
	if !strings.Contains(body, "new choice") {
		t.Errorf("the new block's content is missing: %q", body)
	}
	if n := strings.Count(body, deskkit.DeskDecidedHeading); n != 1 {
		t.Errorf("got %d Desk-decided headings, want exactly 1: %q", n, body)
	}
	if len(curForge.labelChanges) != 1 {
		t.Fatalf("ApplyLabels called %d time(s), want exactly 1", len(curForge.labelChanges))
	}
}

// TestCreateDecidedLabelFailureIsLoud: a label-write failure that follows an already-landed
// create reports the failure AS ITSELF (exit 6) — the PR (and its block) already exists.
func TestCreateDecidedLabelFailureIsLoud(t *testing.T) {
	work := newBaseFixture(t)
	withEnv(t, work)
	decidedPath := writeDecidedFile(t, oneDecidedItem)
	old := applyDeskDecidedLabel
	applyDeskDecidedLabel = func(fg deskkit.Forge, fr deskkit.ForgeRepo, number int) error {
		return deskkit.Unverifiable("HTTP 500", nil)
	}
	t.Cleanup(func() { applyDeskDecidedLabel = old })

	rc := run([]string{"create", "--title", "a change", "--body-min",
		"does the thing\nBrief: fixture/01", "--decided", decidedPath})
	if rc != deskkit.ExitUnverifiable {
		t.Fatalf("create rc = %d, want %d (unverifiable — the PR landed, only the label failed)", rc, deskkit.ExitUnverifiable)
	}
	if curForge.createCalls != 1 {
		t.Errorf("CreateDraftChange called %d time(s), want exactly 1 — the create must still have landed", curForge.createCalls)
	}
}

// editDecidedFixture is the edit-side shape review finding F4 on this brief's PR is about: an
// open PR whose CURRENT body already carries the tool's own block for oneDecidedItem, and a
// --decided file with that same item.
func editDecidedFixture(t *testing.T) (bodyPath, decidedPath string) {
	t.Helper()
	body := "the original body\nBrief: fixture/01\n\n" + deskkit.RenderDecidedBlock([]deskkit.DecidedItem{
		{Decision: "ship the one-line patch", Alternative: "ask first", Cost: "one revert"},
	})
	t.Setenv("FAKEGH_LIST_HAS_PR", "1")
	t.Setenv("FAKEGH_PR_BODY", body)
	return writeTempFile(t, body), writeDecidedFile(t, oneDecidedItem)
}

// TestEditDecidedLabelFailureStillPostsNotice pins review finding F4's first half: when the
// label write fails after an edit LANDED, the re-review notice — the only event a body edit
// produces for the review loop — is still posted, and the failure is still loud (exit 6).
//
// FAIL-FIRST: the label failure returned before the notice, so no comment was posted.
func TestEditDecidedLabelFailureStillPostsNotice(t *testing.T) {
	work := newBaseFixture(t)
	withEnv(t, work)
	t.Setenv("FAKEGH_LIST_HAS_PR", "1")
	t.Setenv("FAKEGH_PR_BODY", "the original body\nBrief: fixture/01\n")
	bodyPath := writeTempFile(t, "the original body, corrected\nBrief: fixture/01\n")
	decidedPath := writeDecidedFile(t, oneDecidedItem)
	old := applyDeskDecidedLabel
	applyDeskDecidedLabel = func(fg deskkit.Forge, fr deskkit.ForgeRepo, number int) error {
		return deskkit.Unverifiable("HTTP 500", nil)
	}
	t.Cleanup(func() { applyDeskDecidedLabel = old })

	rc := run([]string{"edit", "--body-file", bodyPath, "--decided", decidedPath})
	if rc != deskkit.ExitUnverifiable {
		t.Fatalf("edit rc = %d, want %d (unverifiable — the edit landed, only the label failed)", rc, deskkit.ExitUnverifiable)
	}
	if curForge.edited == nil {
		t.Fatal("no EditChange call was recorded — the edit itself must still land")
	}
	if len(curForge.comments) != 1 {
		t.Fatalf("posted %d re-review notice(s), want exactly 1 — a failed label write must not swallow "+
			"the only event that tells the review loop the body changed", len(curForge.comments))
	}
}

// TestEditDecidedNoopReconcilesMissingLabel pins review finding F4's second half: the remedy
// the label-failure message gives — re-run the same edit — must actually work. The body
// already matches, so the edit itself is a no-op, but with --decided given and the label
// missing, the no-op path applies the label rather than exiting 0 with the PR still
// block-without-label (a shape deskflip refuses).
//
// FAIL-FIRST: the idempotency no-op returned before the label apply — ApplyLabels was never
// called and the PR stayed block-without-label.
func TestEditDecidedNoopReconcilesMissingLabel(t *testing.T) {
	work := newBaseFixture(t)
	withEnv(t, work)
	bodyPath, decidedPath := editDecidedFixture(t)

	rc := run([]string{"edit", "--body-file", bodyPath, "--decided", decidedPath})
	if rc != deskkit.ExitOK {
		t.Fatalf("edit rc = %d, want 0", rc)
	}
	if curForge.edited != nil {
		t.Errorf("an unchanged body was re-sent to EditChange: %+v", curForge.edited)
	}
	if len(curForge.labelChanges) != 1 {
		t.Fatalf("ApplyLabels called %d time(s), want exactly 1 — the missing label must be reconciled", len(curForge.labelChanges))
	}
	if len(curForge.comments) != 0 {
		t.Errorf("a label-only reconcile posted %d re-review notice(s), want 0 — the body did not change", len(curForge.comments))
	}
}

// TestEditDecidedNoopWithLabelPresentStaysNoop is the negative control: body unchanged and the
// label already present — a true no-op, no label write, no notice.
func TestEditDecidedNoopWithLabelPresentStaysNoop(t *testing.T) {
	work := newBaseFixture(t)
	withEnv(t, work)
	bodyPath, decidedPath := editDecidedFixture(t)
	t.Setenv("FAKEGH_PR_LABELS", "some-other-label,"+deskkit.DeskDecidedLabel)

	if rc := run([]string{"edit", "--body-file", bodyPath, "--decided", decidedPath}); rc != deskkit.ExitOK {
		t.Fatalf("edit rc = %d, want 0", rc)
	}
	if len(curForge.labelChanges) != 0 || curForge.edited != nil || len(curForge.comments) != 0 {
		t.Errorf("a true no-op wrote something: labels=%d edited=%v comments=%d",
			len(curForge.labelChanges), curForge.edited != nil, len(curForge.comments))
	}
}
