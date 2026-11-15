package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// A cut that waits out the audit lock never reaches the forge, so it is contention, not a
// loop: the audit line says `contended`, which neither the budget nor the breaker counts.
// Pins this binary's call site of the classifier (#6227): reverting runOutward's lock-error
// result to a plain unverifiable leaves every other test green.
//
// regression: #6227
func TestLockWaitIsAuditedContended(t *testing.T) {
	h := newHarness(t)
	old := lockWait
	lockWait = 30 * time.Millisecond
	t.Cleanup(func() { lockWait = old })

	dir, err := deskDir()
	if err != nil {
		t.Fatalf("state dir: %v", err)
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("mkdir state dir: %v", err)
	}
	holder, err := os.OpenFile(filepath.Join(dir, deskkit.AuditLockName), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		t.Fatalf("open lock: %v", err)
	}
	if err := deskkit.TryLockExclusive(holder); err != nil {
		t.Fatalf("hold lock: %v", err)
	}
	t.Cleanup(func() { _ = deskkit.UnlockFile(holder); _ = holder.Close() })

	if code := run([]string{"cut", goodTag}); code != deskkit.ExitUnverifiable {
		t.Fatalf("exit = %d, want %d (a lock it could not take is not permission to write)",
			code, deskkit.ExitUnverifiable)
	}
	if len(h.gh.mutatingCalls()) != 0 {
		t.Fatalf("wrote while the lock was held: %v", h.gh.calls())
	}
	lines := h.auditLines(t)
	if len(lines) != 1 {
		t.Fatalf("wrote %d audit lines, want exactly 1", len(lines))
	}
	if got := lines[0].Result; got != deskkit.ResultContended {
		t.Fatalf("audit result = %q, want %q", got, deskkit.ResultContended)
	}
}
