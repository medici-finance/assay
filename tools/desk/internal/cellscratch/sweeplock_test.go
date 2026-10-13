package cellscratch

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// A sibling launch mid-sweep holds the cell-wide sweep lock for a moment. Open,
// Begin and Sweep must wait their turn instead of failing at once.
//
// regression: #2709
func TestSweepLockWaits(t *testing.T) {
	s := scratchStore(t)
	for _, step := range []struct {
		name string
		run  func() error
	}{
		{"open", func() error {
			o, err := Open(s.Path)
			if err == nil {
				o.Close()
			}
			return err
		}},
		{"begin", func() error {
			r, err := s.Begin("session", "worker", "revision")
			if err == nil {
				r.Close()
			}
			return err
		}},
		{"sweep", func() error { _, err := s.Sweep(DefaultPolicy(), false); return err }},
	} {
		lock, err := s.lock(sweepLockName)
		if err != nil {
			t.Fatal(err)
		}
		time.AfterFunc(300*time.Millisecond, func() { lock.Close() })
		if err = step.run(); err != nil {
			t.Fatalf("%s failed while a sibling briefly held the sweep lock: %v", step.name, err)
		}
	}
}

// When the wait runs out, the error names the lock, its path, the bounded wait,
// the holder and the (absent) action — and stays ErrLockBusy for callers.
func TestSweepLockBusyNamed(t *testing.T) {
	s := scratchStore(t)
	held, err := s.sweepLock()
	if err != nil {
		t.Fatal(err)
	}
	defer held.Release()
	s.lockWait = 50 * time.Millisecond
	_, err = s.Sweep(DefaultPolicy(), false)
	var busy *deskkit.BusyError
	if !errors.As(err, &busy) || !errors.Is(err, deskkit.ErrLockBusy) {
		t.Fatalf("busy sweep lock not reported as a named BusyError: %v", err)
	}
	for _, want := range []string{`"cell scratch sweep"`, s.Path, "sweep.lock", "bounded", "retryable", "holder:", "holder current:", "action:"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("busy error lacks %q: %v", want, err)
		}
	}
}
