package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// holdAuditLock takes the desk audit lock from a second open file description, so the tool
// under test contends with it exactly as another desk session would. The returned func
// releases it.
func holdAuditLock(t *testing.T) (release func()) {
	t.Helper()
	dir, err := deskDir()
	if err != nil {
		t.Fatalf("state dir: %v", err)
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("mkdir state dir: %v", err)
	}
	holder, err := os.OpenFile(filepath.Join(dir, "audit.lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		t.Fatalf("open lock: %v", err)
	}
	if err := deskkit.TryLockExclusive(holder); err != nil {
		t.Fatalf("hold lock: %v", err)
	}
	released := false
	release = func() {
		if !released {
			released = true
			_ = deskkit.UnlockFile(holder)
			_ = holder.Close()
		}
	}
	t.Cleanup(release)
	return release
}

func shortenLockWait(t *testing.T) {
	t.Helper()
	old := lockWait
	lockWait = 20 * time.Millisecond
	t.Cleanup(func() { lockWait = old })
}

// Waiting on the audit lock is contention (a queue), not a loop. Every `audit lock held`
// refusal happens before any forge read or write, so counting it toward the consecutive
// no-op breaker locked a caller out of a perfectly good post for the breaker's whole
// cooldown after it had merely waited its turn.
//
// regression: #6227
func TestLockWaitNoBreakerCount(t *testing.T) {
	f, _ := setupFake(t)
	shortenLockWait(t)
	release := holdAuditLock(t)

	bf := writeBody(t, "real.md", "the comment that matters\n")
	for i := 0; i < deskkit.BreakerTrip+2; i++ {
		if code := run([]string{"comment", exampleRepo, "1", "--body-file", bf}); code != deskkit.ExitUnverifiable {
			t.Fatalf("attempt %d while the lock is held: exit = %d, want %d (lock refusal)",
				i+1, code, deskkit.ExitUnverifiable)
		}
	}
	if f.postedCmt != 0 {
		t.Fatalf("postedCmt = %d while the lock was held, want 0", f.postedCmt)
	}

	release()
	if code := run([]string{"comment", exampleRepo, "1", "--body-file", bf}); code != deskkit.ExitOK {
		t.Fatalf("exit = %d after the lock freed, want 0 — %d lock refusals must not open the "+
			"circuit breaker", code, deskkit.BreakerTrip+2)
	}
	if f.postedCmt != 1 {
		t.Fatalf("postedCmt = %d, want 1", f.postedCmt)
	}
}

// A genuine no-op loop still trips: the exemption covers the lock step ONLY. Five refusals
// of the same bad input open the breaker; the sixth call is refused by the breaker (exit 4)
// without reaching the forge.
func TestRefusalLoopTripsAtFive(t *testing.T) {
	f, _ := setupFake(t)
	bf := writeBody(t, "rev.md", "no heading and no verdict line\n")
	args := reviewArgs(exampleRepo, "1", "approve", testHead, bf)

	for i := 0; i < deskkit.BreakerTrip; i++ {
		if code := run(args); code != deskkit.ExitRefused {
			t.Fatalf("attempt %d: exit = %d, want %d (refusal)", i+1, code, deskkit.ExitRefused)
		}
	}
	if code := run(args); code != deskkit.ExitRateLimited {
		t.Fatalf("attempt %d: exit = %d, want %d (circuit breaker open)",
			deskkit.BreakerTrip+1, code, deskkit.ExitRateLimited)
	}
	if f.postedReview != 0 {
		t.Fatal("no review may be posted by a refused loop")
	}
}

// Lock refusals interleaved with a genuine loop neither count toward the trip nor reset the
// run: the trip still lands on the fifth REAL refusal. Counting them would trip early;
// treating them as progress would let a looping caller launder its run clean by contending
// for the lock.
//
// regression: #6227
func TestLockWaitKeepsRefusalRun(t *testing.T) {
	setupFake(t)
	shortenLockWait(t)
	bf := writeBody(t, "rev.md", "no heading and no verdict line\n")
	args := reviewArgs(exampleRepo, "1", "approve", testHead, bf)

	for i := 0; i < deskkit.BreakerTrip-1; i++ {
		if code := run(args); code != deskkit.ExitRefused {
			t.Fatalf("refusal %d: exit = %d, want %d", i+1, code, deskkit.ExitRefused)
		}
	}
	release := holdAuditLock(t)
	for i := 0; i < 3; i++ {
		if code := run(args); code != deskkit.ExitUnverifiable {
			t.Fatalf("contended attempt %d: exit = %d, want %d", i+1, code, deskkit.ExitUnverifiable)
		}
	}
	release()
	if code := run(args); code != deskkit.ExitRefused {
		t.Fatalf("fifth real refusal: exit = %d, want %d — contention must not count toward the trip",
			code, deskkit.ExitRefused)
	}
	if code := run(args); code != deskkit.ExitRateLimited {
		t.Fatalf("sixth attempt: exit = %d, want %d — contention must not reset the run",
			code, deskkit.ExitRateLimited)
	}
}
