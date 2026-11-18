//go:build linux || darwin

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// A busy audit lock is an audit-read failure of its own: exit 6 like any could-not-check, but its
// text says AUDIT-READ FAILURE, names the lock file, and says the poll was not attempted. The
// poll must not run, and the failure must not read as a blind poll.
//
// regression: #6199
func TestAuditReadFailureIsDistinct(t *testing.T) {
	state := isolatedDeskHome(t)
	if err := os.MkdirAll(state, 0o700); err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile(filepath.Join(state, deskkit.AuditLockName), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if err := deskkit.TryLockExclusive(f); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = deskkit.UnlockFile(f); _ = f.Close() })
	old := exitLockWait
	exitLockWait = 100 * time.Millisecond
	t.Cleanup(func() { exitLockWait = old })

	polled := false
	monitor := retainingMonitor(t.TempDir(), []string{"medici-finance/assay"}, func() (*MonitorReport, error) {
		polled = true
		return &MonitorReport{}, nil
	})
	_, err = monitor()
	if err == nil || polled {
		t.Fatalf("err=%v polled=%v", err, polled)
	}
	if got := deskkit.ExitCodeOf(err); got != deskkit.ExitUnverifiable {
		t.Fatalf("exit = %d, want 6", got)
	}
	msg := err.Error()
	for _, want := range []string{"AUDIT-READ FAILURE", "not a blind poll", filepath.Join(state, deskkit.AuditLockName), "Action:"} {
		if !strings.Contains(msg, want) {
			t.Errorf("message lacks %q:\n%s", want, msg)
		}
	}
	if strings.Contains(msg, "stale lock?") || strings.Contains(msg, "another desk write may be stuck") {
		t.Errorf("message keeps the unattributed wording:\n%s", msg)
	}

	var out bytes.Buffer
	if !renderAuditReadFailure(&out, err) || !strings.Contains(out.String(), "AUDIT READ: FAILED") {
		t.Fatalf("the pass report does not name the audit-read failure: %q", out.String())
	}
	// A blind-poll error keeps its own text and is not reported as an audit-read failure.
	blind := deskkit.Unverifiable("scanloop run: the inbound surface was not fully readable this pass", nil)
	if renderAuditReadFailure(&out, blind) {
		t.Fatal("a blind poll was reported as an audit-read failure")
	}
}
