package main

import (
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"testing"
)

func TestCreateAfterMaintenanceWrites(t *testing.T) {
	work := newBaseFixture(t)
	calls := withEnv(t, work)
	seedDeskprAudit(t, 40, func(i int) deskkit.Entry {
		n := 6000 + i
		verb := "update"
		if i%2 == 0 {
			verb = "edit"
		}
		return deskkit.Entry{Repo: fixtureRepo, PR: &n, Verb: verb}
	})
	rc := run([]string{"create", "--title", "new change", "--body-min", "does the thing\nBrief: fixture/01"})
	if rc != deskkit.ExitOK {
		t.Fatalf("40 maintenance writes blocked creation: rc=%d", rc)
	}
	if !anyCall(ghCalls(*calls), "pr", "create", "--draft") {
		t.Fatalf("create never reached draft endpoint: %v", ghCalls(*calls))
	}
}
