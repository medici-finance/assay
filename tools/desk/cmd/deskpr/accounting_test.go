package main

import (
	"errors"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"testing"
)

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
