package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// plantLease writes a HEARTBEAT two hours old under the HOME withEnv set, and returns its
// path and mtime.
func plantLease(t *testing.T) (string, time.Time) {
	t.Helper()
	dir := filepath.Join(os.Getenv("HOME"), ".config", "assay")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "HEARTBEAT")
	if err := os.WriteFile(path, []byte("ok"), 0o600); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-2 * time.Hour).Truncate(time.Second)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatal(err)
	}
	return path, old
}

func leaseMoved(t *testing.T, path string, old time.Time) bool {
	t.Helper()
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	return fi.ModTime().After(old)
}

func lastUpdateRow(t *testing.T) deskkit.Entry {
	t.Helper()
	entries, err := deskkit.LoadEntries()
	if err != nil {
		t.Fatal(err)
	}
	for i := len(entries) - 1; i >= 0; i-- {
		if entries[i].Tool == "deskpr" && entries[i].Verb == "update" {
			return entries[i]
		}
	}
	t.Fatal("no deskpr update audit row")
	return deskkit.Entry{}
}

// An update whose head the remote already holds (pushed by another path, so no update row
// records it) moves no ref: it logs noop and leaves the lease alone. A new head on the
// same PR is real progress and renews it — the positive control that also pins update's
// call site to LogOutwardAct.
// regression: #6510
func TestUpdateUpToDateNoRenew(t *testing.T) {
	work := newBaseFixture(t)
	withEnv(t, work)
	t.Setenv("FAKEGH_LIST_HAS_PR", "1")
	if err := pushBranch(branchSpec(work)); err != nil {
		t.Fatalf("pre-push the head by another path: %v", err)
	}

	path, old := plantLease(t)
	if rc := run([]string{"update"}); rc != deskkit.ExitOK {
		t.Fatalf("update of an already-pushed head rc = %d, want 0", rc)
	}
	if row := lastUpdateRow(t); row.Result != deskkit.ResultNoop {
		t.Fatalf("up-to-date update row result = %q, want %q (detail %q)", row.Result, deskkit.ResultNoop, row.Detail)
	}
	if leaseMoved(t, path, old) {
		t.Fatal("an update that moved no ref renewed the HEARTBEAT lease")
	}

	writeFile(t, filepath.Join(work, "next.txt"), "next\n")
	mustGit(t, work, "add", "next.txt")
	mustGit(t, work, "commit", "-m", "next")
	if rc := run([]string{"update"}); rc != deskkit.ExitOK {
		t.Fatalf("update of a new head rc = %d, want 0", rc)
	}
	if row := lastUpdateRow(t); row.Result != deskkit.ResultOK {
		t.Fatalf("new-head update row result = %q, want %q (detail %q)", row.Result, deskkit.ResultOK, row.Detail)
	}
	if !leaseMoved(t, path, old) {
		t.Fatal("an update that pushed a new head did not renew the lease")
	}
}
