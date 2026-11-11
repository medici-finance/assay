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
	err := AuditLockContended(60 * time.Second)
	if ExitCodeOf(err) != ExitUnverifiable {
		t.Fatalf("exit = %d, want %d: a lock it cannot take is never permission to proceed",
			ExitCodeOf(err), ExitUnverifiable)
	}
	if !strings.Contains(err.Error(), "audit lock held >60s") {
		t.Fatalf("message = %q, want the documented 'audit lock held >60s' wording", err)
	}
	if !IsAuditLockContended(fmt.Errorf("wrapped: %w", err)) {
		t.Fatal("a wrapped lock-wait refusal must still be recognised")
	}
	for _, other := range []error{
		nil,
		errors.New("audit lock held >60s"), // the words alone are not the tag
		Unverifiable("audit lock held >60s (stale lock?)", nil),
		Unverifiable("cannot open audit lock", nil),
		Refused("refused"),
	} {
		if IsAuditLockContended(other) {
			t.Fatalf("%v was classified as lock contention", other)
		}
	}
}

// regression: #6227
func TestLockStepResultOnlyForContention(t *testing.T) {
	if got := LockStepResult(AuditLockContended(time.Second), ResultUnwritten); got != ResultContended {
		t.Fatalf("lock-wait refusal result = %q, want %q", got, ResultContended)
	}
	// A lock that could not be opened is a real fault the caller repeats identically: it keeps
	// the class the caller gave it and keeps counting toward the breaker.
	if got := LockStepResult(Unverifiable("cannot open audit lock", nil), ResultUnwritten); got != ResultUnwritten {
		t.Fatalf("non-contention lock failure result = %q, want %q", got, ResultUnwritten)
	}
	if got := LockStepResult(Unverifiable("cannot acquire audit lock", nil), ResultUnverifiable); got != ResultUnverifiable {
		t.Fatalf("non-contention lock failure result = %q, want %q", got, ResultUnverifiable)
	}
}

// auditLockWaitLiterals returns the non-test Go files under root that spell the lock-wait
// refusal message themselves instead of building it with AuditLockContended. A site that does
// is the defect class of #6227: its refusal is unmarked, so it is audited as a meter-visible
// result and a queue behind other writers reads as a loop.
func auditLockWaitLiterals(root string) ([]string, error) {
	var hits []string
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") ||
			filepath.Base(p) == "auditcontention.go" {
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		if strings.Contains(string(b), "audit lock held >") {
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
		"package x\nvar _ = Unverifiable(\"audit lock held >60s (stale lock?)\", nil)\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if hits, err := auditLockWaitLiterals(plant); err != nil || len(hits) != 1 {
		t.Fatalf("positive control: hits=%v err=%v, want the one planted site flagged", hits, err)
	}

	hits, err := auditLockWaitLiterals(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("walk tools/desk: %v", err)
	}
	if len(hits) != 0 {
		t.Fatalf("these files build the lock-wait refusal by hand instead of AuditLockContended, so "+
			"the circuit breaker counts it as a loop (#6227): %v", hits)
	}
}
