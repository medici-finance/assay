package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// rowRenewsLease writes this verb's terminal audit row with the given result against a fresh
// two-hour-old HEARTBEAT and reports whether the lease moved.
func rowRenewsLease(t *testing.T, result string) bool {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := filepath.Join(home, ".config", "assay")
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
	(&auditCtx{repo: "o/r"}).log(result, "replied")
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	return fi.ModTime().After(old)
}

// Pins this verb's call site to LogOutwardAct: an ok row renews a fresh lease, a noop does
// not. Reverting the call site to plain deskkit.Log reddens the ok case.
// regression: #6510
func TestReplyRowRenewsLease(t *testing.T) {
	if !rowRenewsLease(t, deskkit.ResultOK) {
		t.Fatal("an ok deskreply row did not renew the HEARTBEAT lease")
	}
	if rowRenewsLease(t, deskkit.ResultNoop) {
		t.Fatal("a noop deskreply row renewed the HEARTBEAT lease")
	}
}
