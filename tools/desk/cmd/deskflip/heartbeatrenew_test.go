package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// plantFlipLease writes a HEARTBEAT two hours old (fresh) under the HOME s.install set.
func plantFlipLease(t *testing.T) (string, time.Time) {
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

// Every case drives the real verb (run → cmdFlip → flip → audit) through the fake forge, so
// the production wiring of `flipped` is what decides: only a ready mutation that the
// backstop leaves standing renews the lease. A dry run, an already-ready re-gate (with or
// without a label reconcile) and a flip the backstop reverts all leave it alone.
// regression: #6510
func TestFlipRenewsOnlyRealFlip(t *testing.T) {
	for _, tc := range []struct {
		name   string
		setup  func(s *stub)
		args   []string
		states []string
		rc     int
		renew  bool
	}{
		{"real flip, clean backstop", nil, nil, []string{"clean"}, deskkit.ExitOK, true},
		{"dry run", nil, []string{"--dry-run"}, nil, deskkit.ExitOK, false},
		{"already ready, label correct", func(s *stub) {
			s.pr.IsDraft = false
			s.pr.Labels = []string{labelAfterFlip}
		}, nil, nil, deskkit.ExitOK, false},
		{"already ready, relabel only", func(s *stub) {
			s.pr.IsDraft = false
			s.pr.Labels = []string{labelBeforeFlip}
		}, nil, nil, deskkit.ExitOK, false},
		{"backstop reverts", nil, nil, []string{"blocked"}, deskkit.ExitRefused, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := newStub()
			if tc.setup != nil {
				tc.setup(s)
			}
			fakeBackstopClock(t)
			s.install(t)
			s.reviews = approvalAtHead(t, headSHA)
			s.mergeStates = tc.states
			path, old := plantFlipLease(t)

			var rc int
			out := captureStderr(t, func() {
				rc = run(append([]string{"7", "--repo", privateCIRepo}, tc.args...))
			})
			if rc != tc.rc {
				t.Fatalf("rc = %d, want %d\n%s", rc, tc.rc, out)
			}
			fi, err := os.Stat(path)
			if err != nil {
				t.Fatal(err)
			}
			if renewed := fi.ModTime().After(old); renewed != tc.renew {
				t.Fatalf("lease renewed=%v, want %v (ready mutation ran=%v)", renewed, tc.renew, s.flipped())
			}
		})
	}
}
