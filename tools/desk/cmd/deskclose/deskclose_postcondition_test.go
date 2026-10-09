package main

// deskclose_postcondition_test.go — pins the write-boundary postcondition (postcondition.go).
//
// The no-op shape: the close call returns without error but the post-close re-read still shows
// the item open. It must FAIL — non-zero exit, never 0 — AND file a repair issue, once. The
// benign replay (the item already reads closed BEFORE any write) is a different thing and stays
// a clean exit-0 no-op with zero writes. The tests drive the CLI end to end through the stub
// forge's closeNoReflect toggle, so they pin the exit code an operator and a manifest loop see.

import (
	"fmt"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

var subjectKey = fmt.Sprintf("%s#%d", testRepo, subjectIssue)

// noopCloseRun runs a review-request close on baseWorld's subject with the close made a no-op.
func noopCloseRun(t *testing.T, s *stubRemote, rul string) (int, string) {
	t.Helper()
	s.closeNoReflect[subjectKey] = true
	return closeRun(modeReviewRequest, "-R", testRepo, fmt.Sprint(subjectIssue), "--rulings", rul)
}

// closeRun dispatches the CLI and returns the exit code the process would report plus the
// stdout and error text together (run() prints the error to stderr, which execCLI drops).
func closeRun(args ...string) (int, string) {
	var out strings.Builder
	err := dispatch(args, &out)
	if err == nil {
		return deskkit.ExitOK, out.String()
	}
	return deskkit.ExitCodeOf(err), out.String() + err.Error()
}

// TestNoopCloseFailsAndFiles: a close that claims success while the item stays open exits
// non-zero (could-not-check, 6) and files exactly one repair issue naming the item.
func TestNoopCloseFailsAndFiles(t *testing.T) {
	s, rul := baseWorld(t)
	code, out := noopCloseRun(t, s, rul)
	if code == deskkit.ExitOK {
		t.Fatalf("a close that did not take exited 0 — the silent no-op this seam exists to kill:\n%s", out)
	}
	if code != deskkit.ExitUnverifiable {
		t.Fatalf("want exit %d (could-not-check), got %d\n%s", deskkit.ExitUnverifiable, code, out)
	}
	for _, want := range []string{"still reads state", "postcondition failed", "filed repair issue #901"} {
		if !strings.Contains(out, want) {
			t.Fatalf("the failure does not report %q:\n%s", want, out)
		}
	}
	if len(s.filed) != 1 || s.filed[0] != repairTitle(subjectIssue, deskkit.TargetIssue) {
		t.Fatalf("want exactly one repair issue titled %q, got %v", repairTitle(subjectIssue, deskkit.TargetIssue), s.filed)
	}
	if !strings.Contains(s.items[subjectKey], `"state":"open"`) {
		t.Fatalf("fixture error: the subject should still read open: %s", s.items[subjectKey])
	}
}

// TestNoopCloseRepairDeduped: an open repair issue with the same title is found, not refiled;
// the run still fails.
func TestNoopCloseRepairDeduped(t *testing.T) {
	s, rul := baseWorld(t)
	s.openIssues = []deskkit.IssueSearchResult{
		{Number: 77, Title: repairTitle(subjectIssue, deskkit.TargetIssue) + " (older)", State: "open"},
		{Number: 78, Title: repairTitle(subjectIssue, deskkit.TargetIssue), State: "closed"},
		{Number: 79, Title: repairTitle(subjectIssue, deskkit.TargetIssue), State: "open"},
	}
	code, out := noopCloseRun(t, s, rul)
	if code != deskkit.ExitUnverifiable {
		t.Fatalf("want exit %d, got %d\n%s", deskkit.ExitUnverifiable, code, out)
	}
	if len(s.filed) != 0 {
		t.Fatalf("an open repair issue already exists; nothing should be filed, got %v", s.filed)
	}
	if !strings.Contains(out, "repair issue already open #79") {
		t.Fatalf("the failure should name the open repair issue (exact title, open state):\n%s", out)
	}
}

// TestNoopCloseFileFailStillFails: when the repair issue cannot be filed (search or filing
// refused) the exit is STILL non-zero and the note says the issue was not filed.
func TestNoopCloseFileFailStillFails(t *testing.T) {
	for _, tc := range []struct {
		name string
		set  func(*stubRemote)
	}{
		{"search fails", func(s *stubRemote) { s.failSearch = true }},
		{"filing fails", func(s *stubRemote) { s.failFile = true }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, rul := baseWorld(t)
			tc.set(s)
			code, out := noopCloseRun(t, s, rul)
			if code != deskkit.ExitUnverifiable {
				t.Fatalf("want exit %d, got %d\n%s", deskkit.ExitUnverifiable, code, out)
			}
			if !strings.Contains(out, "repair issue NOT filed — could-not-check") {
				t.Fatalf("the failure should say the repair issue was not filed:\n%s", out)
			}
			if len(s.filed) != 0 {
				t.Fatalf("nothing should be recorded as filed, got %v", s.filed)
			}
		})
	}
}

// TestNoopReadBackFailNoFile: a re-read that FAILS is could-not-check — the close is
// unconfirmed, not known to have failed — so it exits 6 and files nothing.
func TestNoopReadBackFailNoFile(t *testing.T) {
	s, rul := baseWorld(t)
	inner := forgeForFn
	forgeForFn = func(repo string) (deskkit.Forge, deskkit.ForgeRepo, error) {
		fg, fr, err := inner(repo)
		return readBackFails{Forge: fg, s: s}, fr, err
	}
	t.Cleanup(func() { forgeForFn = inner })
	code, out := closeRun(modeReviewRequest, "-R", testRepo, fmt.Sprint(subjectIssue), "--rulings", rul)
	if code != deskkit.ExitUnverifiable || !strings.Contains(out, "could not be read back") {
		t.Fatalf("want exit %d with a could-not-check read-back, got %d\n%s", deskkit.ExitUnverifiable, code, out)
	}
	if len(s.filed) != 0 {
		t.Fatalf("an unconfirmed close must not file a repair issue, got %v", s.filed)
	}
}

// readBackFails fails every typed read issued AFTER the close call.
type readBackFails struct {
	deskkit.Forge
	s *stubRemote
}

func (r readBackFails) GetIssueTyped(fr deskkit.ForgeRepo, n int, kind deskkit.TargetKind) (*deskkit.Issue, error) {
	if len(r.s.typedCloses) > 0 {
		return nil, deskkit.Unverifiable("HTTP 502: bad gateway", nil)
	}
	return r.Forge.GetIssueTyped(fr, n, kind)
}

// TestNoopReplayIsBenign: the benign replay — the item already reads closed on the pre-write
// fetch — is an idempotent no-op: exit 0, zero writes, nothing filed.
func TestNoopReplayIsBenign(t *testing.T) {
	s, rul := baseWorld(t)
	s.items[subjectKey] = issueJSON(subjectIssue, "closed", nil, "")
	code, out := execCLI(modeReviewRequest, "-R", testRepo, fmt.Sprint(subjectIssue), "--rulings", rul)
	if code != deskkit.ExitOK || !strings.Contains(out, "noop") {
		t.Fatalf("an already-closed item is a benign replay (exit 0 noop), got %d\n%s", code, out)
	}
	assertNoWrites(t, s)
	if len(s.filed) != 0 {
		t.Fatalf("a benign replay filed a repair issue: %v", s.filed)
	}
}

// TestNoopCloseTriageLane: the postcondition sits at the shared close choke point, so a lane
// other than the ruled verbs (triage) fails the same way on a no-op close.
func TestNoopCloseTriageLane(t *testing.T) {
	s, rul := triageWorld(t)
	plantTrustedMarker(s, triageIssue)
	s.closeNoReflect[fmt.Sprintf("%s#%d", testRepo, triageIssue)] = true
	code, out := closeRun(modeTriage, "-R", testRepo, fmt.Sprint(triageIssue),
		"--disposition", dispositionNotPlanned, "--rulings", rul)
	if code == deskkit.ExitOK {
		t.Fatalf("a triage close that did not take exited 0:\n%s", out)
	}
	if len(s.filed) != 1 || s.filed[0] != repairTitle(triageIssue, deskkit.TargetIssue) {
		t.Fatalf("want one repair issue for the triage subject, got %v", s.filed)
	}
}
