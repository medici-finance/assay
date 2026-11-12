package deskkit

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAuditLockContendedIsTagged(t *testing.T) {
	err := AuditLockContended("the desk audit lock stayed busy")
	if ExitCodeOf(err) != ExitUnverifiable {
		t.Fatalf("exit = %d, want %d: a lock it cannot take is never permission to proceed",
			ExitCodeOf(err), ExitUnverifiable)
	}
	if err.Error() != "the desk audit lock stayed busy" {
		t.Fatalf("message = %q, want the caller's message unchanged", err)
	}
	if !IsAuditLockContended(fmt.Errorf("wrapped: %w", err)) {
		t.Fatal("a wrapped lock-wait refusal must still be recognised")
	}
	for _, other := range []error{
		nil,
		errors.New("the desk audit lock stayed busy"), // the words alone are not the tag
		Unverifiable("the desk audit lock stayed busy", nil),
		Unverifiable("cannot open the audit lock", nil),
		Refused("refused"),
	} {
		if IsAuditLockContended(other) {
			t.Fatalf("%v was classified as lock contention", other)
		}
	}
}

// regression: #6227
func TestLockStepResultOnlyForContention(t *testing.T) {
	if got := LockStepResult(AuditLockContended("busy"), ResultUnwritten); got != ResultContended {
		t.Fatalf("lock-wait refusal result = %q, want %q", got, ResultContended)
	}
	// A lock that could not be opened is a real fault the caller repeats identically: it keeps
	// the class the caller gave it and keeps counting toward the breaker.
	if got := LockStepResult(Unverifiable("cannot open the audit lock", nil), ResultUnwritten); got != ResultUnwritten {
		t.Fatalf("non-contention lock failure result = %q, want %q", got, ResultUnwritten)
	}
	if got := LockStepResult(Unverifiable("cannot acquire the audit lock", nil), ResultUnverifiable); got != ResultUnverifiable {
		t.Fatalf("non-contention lock failure result = %q, want %q", got, ResultUnverifiable)
	}
}

// The tag must come out of the real choke point, not only the constructor: a lock that stays
// busy for the whole wait is classified as contention, and a lock that cannot be opened at all
// is not. This is the structural half of the class guard: every acquirer reaches the lock through
// AcquireAuditLock (TestAuditLockOnlyViaChokePoint), so one tagged return covers them all, and
// a change that rewrites the busy return without the tag fails here whatever its wording.
//
// regression: class #6227
func TestBusyAuditLockIsContended(t *testing.T) {
	dir := t.TempDir()
	held, err := AcquireAuditLock(dir, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer held.Release()

	_, err = AcquireAuditLock(dir, 30*time.Millisecond)
	if err == nil {
		t.Fatal("second acquire of a held audit lock succeeded")
	}
	if !IsAuditLockContended(err) {
		t.Fatalf("a lock that stayed busy for the whole wait is not tagged as contention: %v", err)
	}
	if ExitCodeOf(err) != ExitUnverifiable {
		t.Fatalf("exit = %d, want %d", ExitCodeOf(err), ExitUnverifiable)
	}
	if got := LockStepResult(err, ResultUnwritten); got != ResultContended {
		t.Fatalf("audit result = %q, want %q", got, ResultContended)
	}

	// A lock file that cannot be opened is a fault, not a queue: it must keep counting.
	broken := t.TempDir()
	if err := os.Mkdir(filepath.Join(broken, AuditLockName), 0o700); err != nil {
		t.Fatal(err)
	}
	_, err = AcquireAuditLock(broken, 30*time.Millisecond)
	if err == nil {
		t.Fatal("acquire succeeded with a directory in place of the lock file")
	}
	if IsAuditLockContended(err) {
		t.Fatalf("an unopenable lock was classified as contention: %v", err)
	}
}

// busyMessageCallers returns the non-test Go files under root, other than the two that own the
// busy-lock refusal, that call AuditBusyMessage. Turning that message into an error anywhere
// else builds a lock-wait refusal by hand: it is untagged, so it is audited as a meter-visible
// result and a queue behind other writers reads as a loop (#6227). The guard keys on the
// message's one producer rather than on its wording, so rewording it cannot blind the guard.
func busyMessageCallers(root string) ([]string, error) {
	var hits []string
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		base := filepath.Base(p)
		if d.IsDir() || !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") ||
			base == "auditlock.go" || base == "auditcontention.go" {
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		if strings.Contains(string(b), "AuditBusyMessage(") {
			hits = append(hits, p)
		}
		return nil
	})
	return hits, err
}

// regression: class #6227
func TestNoHandBuiltLockWaitRefusal(t *testing.T) {
	// Positive control: the matcher must flag a planted second instance, or a matcher that
	// silently stopped matching would report clean forever.
	plant := t.TempDir()
	if err := os.WriteFile(filepath.Join(plant, "planted.go"), []byte(
		"package x\nvar _ = Unverifiable(AuditBusyMessage(nil), nil)\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if hits, err := busyMessageCallers(plant); err != nil || len(hits) != 1 {
		t.Fatalf("positive control: hits=%v err=%v, want the one planted site flagged", hits, err)
	}

	hits, err := busyMessageCallers(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("walk tools/desk: %v", err)
	}
	if len(hits) != 0 {
		t.Fatalf("these files turn the busy-lock message into a refusal by hand instead of going "+
			"through AcquireAuditLock, so the circuit breaker counts it as a loop (#6227): %v", hits)
	}
}
