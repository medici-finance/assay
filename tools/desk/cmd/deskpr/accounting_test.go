package main

import (
	"errors"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"testing"
)

// failPush makes the push seam fail with an Unverifiable (exit 6) error, the shape an
// ambiguous transport failure takes: the remote may or may not hold the commits.
func failPush(t *testing.T) {
	t.Helper()
	old := pushFn
	pushFn = func(pushSpec) error {
		return deskkit.Unverifiable("push outcome unknown", errors.New("transport ended"))
	}
	t.Cleanup(func() { pushFn = old })
}

// onlyAuditRow returns the single audit row the invocation wrote.
func onlyAuditRow(t *testing.T) deskkit.Entry {
	t.Helper()
	rows, err := deskkit.LoadEntries()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("want exactly one audit row, got %d: %+v", len(rows), rows)
	}
	return rows[0]
}

// The four tests below pin the CALL SITES of ac.writeAttempted (review F1): finalize
// defaults an exit-6 error to unwritten (uncharged), so each verb must set the flag before
// its first potentially mutating call or an ambiguous write would go uncharged. They drive
// the real verb with a failing seam and assert the audit row is unverifiable (charged).
// Deleting the writeAttempted line at the named call site turns the matching test red.

// create: the flag is set before pushFn (deskpr.go, cmdCreate).
func TestCreatePushFailureAuditsUnverifiable(t *testing.T) {
	work := newBaseFixture(t)
	withEnv(t, work)
	failPush(t)
	rc := run([]string{"create", "--title", "new change", "--body-min", "does the thing\nBrief: fixture/01"})
	if rc != deskkit.ExitUnverifiable {
		t.Fatalf("create rc = %d, want %d", rc, deskkit.ExitUnverifiable)
	}
	if row := onlyAuditRow(t); row.Result != deskkit.ResultUnverifiable {
		t.Fatalf("create push failure audited %q, want %q (an attempted write must stay charged)", row.Result, deskkit.ResultUnverifiable)
	}
}

// update: the flag is set before pushFn (deskpr.go, cmdUpdate).
func TestUpdatePushFailureAuditsUnverifiable(t *testing.T) {
	work := newBaseFixture(t)
	withEnv(t, work)
	t.Setenv("FAKEGH_LIST_HAS_PR", "1")
	failPush(t)
	rc := run([]string{"update"})
	if rc != deskkit.ExitUnverifiable {
		t.Fatalf("update rc = %d, want %d", rc, deskkit.ExitUnverifiable)
	}
	if row := onlyAuditRow(t); row.Result != deskkit.ResultUnverifiable {
		t.Fatalf("update push failure audited %q, want %q (an attempted write must stay charged)", row.Result, deskkit.ResultUnverifiable)
	}
}

// edit, changed body: the flag is set before EditChange (edit.go).
func TestEditChangeFailureAuditsUnverifiable(t *testing.T) {
	work := newBaseFixture(t)
	withEnv(t, work)
	t.Setenv("FAKEGH_LIST_HAS_PR", "1")
	t.Setenv("FAKEGH_PR_BODY", "the original body\nBrief: fixture/01\n")
	t.Setenv("FAKEGH_EDIT_FAIL", "1")
	bodyPath := writeTempFile(t, "the original body, corrected\nBrief: fixture/01\n")
	rc := run([]string{"edit", "--body-file", bodyPath})
	if rc != deskkit.ExitUnverifiable {
		t.Fatalf("edit rc = %d, want %d", rc, deskkit.ExitUnverifiable)
	}
	if row := onlyAuditRow(t); row.Result != deskkit.ResultUnverifiable {
		t.Fatalf("EditChange failure audited %q, want %q (an attempted write must stay charged)", row.Result, deskkit.ResultUnverifiable)
	}
}

// edit, unchanged body with --decided: the flag is set before the label-only reconcile
// write (edit.go).
func TestEditLabelReconcileFailureAuditsUnverifiable(t *testing.T) {
	work := newBaseFixture(t)
	withEnv(t, work)
	bodyPath, decidedPath := editDecidedFixture(t)
	old := applyDeskDecidedLabel
	applyDeskDecidedLabel = func(deskkit.Forge, deskkit.ForgeRepo, int) error {
		return deskkit.Unverifiable("HTTP 500", nil)
	}
	t.Cleanup(func() { applyDeskDecidedLabel = old })
	rc := run([]string{"edit", "--body-file", bodyPath, "--decided", decidedPath})
	if rc != deskkit.ExitUnverifiable {
		t.Fatalf("edit rc = %d, want %d", rc, deskkit.ExitUnverifiable)
	}
	if curForge.edited != nil {
		t.Fatalf("an unchanged body reached EditChange: %+v", curForge.edited)
	}
	if row := onlyAuditRow(t); row.Result != deskkit.ResultUnverifiable {
		t.Fatalf("label reconcile failure audited %q, want %q (an attempted write must stay charged)", row.Result, deskkit.ResultUnverifiable)
	}
}

func TestMissingBodyFileAuditsUnwritten(t *testing.T) {
	work := newBaseFixture(t)
	calls := withEnv(t, work)
	rc := run([]string{"create", "--title", "new change", "--body-file", work + "/absent-body.md"})
	if rc != deskkit.ExitUnverifiable {
		t.Fatalf("rc=%d", rc)
	}
	rows, err := deskkit.LoadEntries()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Result != deskkit.ResultUnwritten {
		t.Fatalf("before-write error charged: %+v", rows)
	}
	if len(ghCalls(*calls)) != 0 {
		t.Fatalf("missing body reached forge: %v", ghCalls(*calls))
	}
}

func TestAuditKeepsAmbiguousWritesCharged(t *testing.T) {
	work := newBaseFixture(t)
	withEnv(t, work)
	ac := auditCtx{verb: "create", repo: fixtureRepo, writeAttempted: true}
	ac.finalize(deskkit.Unverifiable("remote outcome unknown", errors.New("transport ended")))
	rows, err := deskkit.LoadEntries()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Result != deskkit.ResultUnverifiable {
		t.Fatalf("attempted write uncharged: %+v", rows)
	}
}
