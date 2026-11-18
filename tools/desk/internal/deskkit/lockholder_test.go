package deskkit

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func openLockFixture(t *testing.T, dir string) (*os.Root, *os.File) {
	t.Helper()
	r, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { r.Close() })
	f, err := r.OpenFile("owner.lock", os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	return r, f
}

// A transient holder is waited out with backoff; a probe never waits.
func TestLockWaitsOutSibling(t *testing.T) {
	dir := t.TempDir()
	r, sibling := openLockFixture(t, dir)
	if err := TryLockExclusive(sibling); err != nil {
		t.Fatal(err)
	}
	_, probe := openLockFixture(t, dir)
	if _, err := AcquireHeld(r, "owner.lock", probe, LockSpec{Name: "role lease", Probe: true, Wait: time.Minute}); err != ErrLockBusy {
		t.Fatalf("probe must return ErrLockBusy at once, got %v", err)
	}
	time.AfterFunc(300*time.Millisecond, func() { UnlockFile(sibling); sibling.Close() })
	_, f := openLockFixture(t, dir)
	h, err := AcquireHeld(r, "owner.lock", f, LockSpec{Name: "role lease", Wait: 10 * time.Second})
	if err != nil {
		t.Fatalf("bounded wait did not outlast a 300ms sibling: %v", err)
	}
	rec, err := readLockHolder(r, "owner.lock")
	if err != nil || rec.PID != os.Getpid() || rec.Lock != "role lease" {
		t.Fatalf("held lock wrote no owner record: %+v %v", rec, err)
	}
	if err = h.Release(); err != nil {
		t.Fatal(err)
	}
	if _, err = os.Lstat(filepath.Join(dir, "owner.lock.holder")); !os.IsNotExist(err) {
		t.Fatalf("release left the owner record behind: %v", err)
	}
}

// Every busy path names the lock, its path, the bounded wait, the holder (or
// "unknown"), whether the holder is current, and the action.
func TestBusyErrorNamesAll(t *testing.T) {
	dir := t.TempDir()
	r, sibling := openLockFixture(t, dir)
	if err := TryLockExclusive(sibling); err != nil {
		t.Fatal(err)
	}
	defer sibling.Close()
	_, f := openLockFixture(t, dir)
	_, err := AcquireHeld(r, "owner.lock", f, LockSpec{Name: "role lease", Wait: 50 * time.Millisecond, EndStale: true})
	var busy *BusyError
	if !errors.As(err, &busy) || !errors.Is(err, ErrLockBusy) {
		t.Fatalf("want *BusyError wrapping ErrLockBusy, got %v", err)
	}
	if busy.Diagnosis.State != HolderUnknown || busy.Diagnosis.Eligible {
		t.Fatalf("a holder with no record must be unknown and ineligible: %+v", busy.Diagnosis)
	}
	for _, want := range []string{`"role lease"`, filepath.Join(dir, "owner.lock"), "bounded", "retryable", "holder: unknown", "holder current: unknown", "action: none", "never signalled"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("busy error lacks %q: %v", want, err)
		}
	}
}

// A malformed or oversized record is "no record", never a pid to act on.
func TestMalformedHolderRecord(t *testing.T) {
	dir := t.TempDir()
	for _, body := range []string{"{", `{"schema":"other","pid":1,"command":"cellctl"}`, strings.Repeat("x", lockHolderLimit+1)} {
		if err := os.WriteFile(filepath.Join(dir, "owner.lock.holder"), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		r, f := openLockFixture(t, dir)
		d := Diagnose(r, "owner.lock", f, LockSpec{Name: "role lease", Cell: "fixture"})
		f.Close()
		if d.Record != nil || d.State != HolderUnknown || d.Eligible {
			t.Fatalf("malformed record %.20q diagnosed as %+v", body, d)
		}
	}
}

// fakeWait is a clock that only moves when the wait under test sleeps. It fails
// the test the moment the wait sleeps past its grant, or asks to sleep again
// once the grant is spent, so an unbounded wait reddens instead of hanging.
type fakeWait struct {
	t      *testing.T
	at     time.Time
	grant  time.Duration
	slept  time.Duration
	sleeps int
}

func (c *fakeWait) clock() *waitClock {
	return &waitClock{now: func() time.Time { return c.at }, sleep: c.sleep}
}

func (c *fakeWait) sleep(d time.Duration) {
	c.t.Helper()
	c.sleeps++
	if c.slept >= c.grant {
		c.t.Fatalf("sleep %d (%s) after the %s grant was already spent: the wait has no ceiling", c.sleeps, d, c.grant)
	}
	if c.slept+d > c.grant {
		c.t.Fatalf("sleep %d (%s) takes the wait to %s, past its %s grant", c.sleeps, d, c.slept+d, c.grant)
	}
	c.slept += d
	c.at = c.at.Add(d)
}

// A lock that never frees is waited on for exactly the granted Wait — not one
// backoff step longer — and the caller gets the busy refusal, never a held lock.
//
// regression: F-overwait-ceiling-dropped
func TestLockWaitStopsAtGrant(t *testing.T) {
	dir := t.TempDir()
	r, sibling := openLockFixture(t, dir)
	if err := TryLockExclusive(sibling); err != nil {
		t.Fatal(err)
	}
	defer sibling.Close()
	_, f := openLockFixture(t, dir)
	// Not a sum of backoff steps (10ms doubling to 500ms), so the last sleep has
	// to be cut short for the wait to end at the grant.
	const grant = 1300 * time.Millisecond
	c := &fakeWait{t: t, at: time.Unix(1_700_000_000, 0), grant: grant}
	h, err := AcquireHeld(r, "owner.lock", f, LockSpec{Name: "role lease", Wait: grant, clock: c.clock()})
	if h != nil {
		h.Release()
		t.Fatal("took a lock its sibling never released")
	}
	var busy *BusyError
	if !errors.As(err, &busy) || !errors.Is(err, ErrLockBusy) {
		t.Fatalf("want the busy refusal (*BusyError wrapping ErrLockBusy), got %v", err)
	}
	if c.slept != grant {
		t.Fatalf("slept %s in total, want exactly the %s grant", c.slept, grant)
	}
	if busy.Waited != grant {
		t.Fatalf("busy error reports a %s wait, want the %s grant", busy.Waited, grant)
	}
}
