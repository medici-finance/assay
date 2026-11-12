//go:build linux || darwin

package deskkit

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"
)

func rawHold(t *testing.T, dir string) {
	t.Helper()
	f, err := os.OpenFile(filepath.Join(dir, AuditLockName), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if err := TryLockExclusive(f); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = UnlockFile(f); _ = f.Close() })
}

func deadPID(t *testing.T) int {
	t.Helper()
	cmd := exec.Command("true")
	if err := cmd.Run(); err != nil {
		t.Skipf("cannot spawn a short-lived process: %v", err)
	}
	return cmd.ProcessState.Pid()
}

func plantRecord(t *testing.T, dir string, rec LockHolder) {
	t.Helper()
	b, _ := json.Marshal(rec)
	if err := os.WriteFile(filepath.Join(dir, AuditLockName+lockHolderSuffix), b, 0o600); err != nil {
		t.Fatal(err)
	}
}

func busyMessage(t *testing.T, dir string) string {
	t.Helper()
	_, err := AcquireAuditLock(dir, 100*time.Millisecond)
	if err == nil {
		t.Fatal("a held audit lock was acquired")
	}
	if got := ExitCodeOf(err); got != ExitUnverifiable {
		t.Fatalf("exit = %d, want %d (unverifiable)", got, ExitUnverifiable)
	}
	return err.Error()
}

// The acquirer records itself beside the lock and removes the record on release.
//
// regression: #6199
func TestAuditLockRecordsHolder(t *testing.T) {
	dir := t.TempDir()
	l, err := AcquireAuditLock(dir, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	r, _ := os.OpenRoot(dir)
	defer r.Close()
	rec, err := ReadLockHolder(r, AuditLockName)
	if err != nil || rec.PID != os.Getpid() || rec.Start == "" || rec.Acquired.IsZero() {
		t.Fatalf("no usable owner record: %+v %v", rec, err)
	}
	l.Release()
	if _, err := os.Lstat(filepath.Join(dir, AuditLockName+lockHolderSuffix)); !os.IsNotExist(err) {
		t.Fatalf("release left the owner record: %v", err)
	}
}

// A lock that stays busy names the lock file, the holder's pid, process start and age, and the
// action; it does not use the old "stale lock?" wording that named none of them.
//
// regression: #6199
func TestAuditBusyNamesLockAndHolder(t *testing.T) {
	dir := t.TempDir()
	l, err := AcquireAuditLock(dir, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Release()
	msg := busyMessage(t, dir)
	for _, want := range []string{
		filepath.Join(dir, AuditLockName),
		"pid " + strconv.Itoa(os.Getpid()),
		"process start",
		"holding since",
		"age ",
		"Action:",
	} {
		if !strings.Contains(msg, want) {
			t.Errorf("message lacks %q:\n%s", want, msg)
		}
	}
	if strings.Contains(msg, "stale lock?") {
		t.Errorf("message still uses the unattributed stale-lock wording:\n%s", msg)
	}
}

// A holder that is gone leaves nothing to clear: the kernel released its lock, and the next
// acquirer takes it and overwrites the leftover owner record.
//
// regression: #6199
func TestAuditLockDeadHolderReclaimed(t *testing.T) {
	dir := t.TempDir()
	plantRecord(t, dir, LockHolder{Schema: lockHolderSchema, PID: deadPID(t), Start: "1.000000", Command: "scanloop", Lock: auditLockLabel, Acquired: time.Now().Add(-time.Hour).UTC()})
	l, err := AcquireAuditLock(dir, time.Second)
	if err != nil {
		t.Fatalf("a lock whose recorded holder is gone was not reclaimed: %v", err)
	}
	r, _ := os.OpenRoot(dir)
	defer r.Close()
	if rec, err := ReadLockHolder(r, AuditLockName); err != nil || rec.PID != os.Getpid() {
		t.Fatalf("leftover record not replaced: %+v %v", rec, err)
	}
	l.Release()
}

// The lock is busy but the record names a dead pid: the real holder is unrecorded. The message
// says so, says no stale file needs deleting, and gives the command that finds the holder.
//
// regression: #6199
func TestAuditBusyDeadRecordedHolder(t *testing.T) {
	dir := t.TempDir()
	rawHold(t, dir)
	dead := deadPID(t)
	plantRecord(t, dir, LockHolder{Schema: lockHolderSchema, PID: dead, Start: "1.000000", Command: "scanloop", Lock: auditLockLabel, Acquired: time.Now().Add(-time.Hour).UTC()})
	msg := busyMessage(t, dir)
	for _, want := range []string{"pid " + strconv.Itoa(dead), "does not check out", "unrecorded process", "never needs deleting", "lsof " + filepath.Join(dir, AuditLockName)} {
		if !strings.Contains(msg, want) {
			t.Errorf("message lacks %q:\n%s", want, msg)
		}
	}
}

// No owner record at all (a writer from a build that records none): holder unknown, same action.
//
// regression: #6199
func TestAuditBusyNoRecord(t *testing.T) {
	dir := t.TempDir()
	rawHold(t, dir)
	msg := busyMessage(t, dir)
	for _, want := range []string{"no owner record is readable", "holder is unknown", "lsof " + filepath.Join(dir, AuditLockName)} {
		if !strings.Contains(msg, want) {
			t.Errorf("message lacks %q:\n%s", want, msg)
		}
	}
}

// Rotation's single attempt never waits and reports a busy lock as "do nothing".
func TestTryLockAuditBusy(t *testing.T) {
	dir := t.TempDir()
	unlock, ok := tryLockAudit(dir)
	if !ok {
		t.Fatal("a free lock was not taken")
	}
	if _, ok2 := tryLockAudit(dir); ok2 {
		t.Fatal("a held lock was taken a second time")
	}
	unlock()
	if u, ok3 := tryLockAudit(dir); !ok3 {
		t.Fatal("the released lock was not retaken")
	} else {
		u()
	}
}

// Defect class: a desk write that takes the shared audit lock by hand (open audit.lock, flock it,
// bare "held" refusal) instead of through AcquireAuditLock, so a busy lock names no holder. The
// lock file's name may be spelled in exactly one non-test file.
func auditLockLiterals(name string, src any) ([]string, error) {
	f, err := parser.ParseFile(token.NewFileSet(), name, src, 0)
	if err != nil {
		return nil, err
	}
	var hits []string
	ast.Inspect(f, func(n ast.Node) bool {
		if lit, ok := n.(*ast.BasicLit); ok && lit.Kind == token.STRING {
			if s, err := strconv.Unquote(lit.Value); err == nil && s == "audit.lock" {
				hits = append(hits, filepath.ToSlash(name))
			}
		}
		return true
	})
	return hits, nil
}

func TestAuditLockOnlyViaChokePoint(t *testing.T) {
	var bad []string
	err := filepath.WalkDir("../..", func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") || filepath.Base(p) == "auditlock.go" {
			return nil
		}
		src, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		hits, err := auditLockLiterals(p, src)
		bad = append(bad, hits...)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(bad)
	if len(bad) > 0 {
		t.Fatalf("audit.lock opened outside deskkit.AcquireAuditLock (a busy lock would name no holder): %v", bad)
	}
}

// Positive control: a planted second instance is caught.
func TestAuditLockGuardBites(t *testing.T) {
	planted := `package main
import "os"
func lock(dir string) { os.OpenFile(dir + "/" + "audit.lock", 0, 0) }`
	hits, err := auditLockLiterals("cmd/planted/main.go", planted)
	if err != nil || len(hits) != 1 {
		t.Fatalf("planted raw audit.lock not reported: %v %v", hits, err)
	}
}
