package main

// zerorunsuite_test.go — `ready` reads the check SUITES at the head, not only the rollups
// (#6234). A suite with no conclusion and no runs adds nothing to either rollup, yet holds the
// forge's merge gate; flipping over it makes ready != mergeable.

import (
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

const suiteApp = "github-actions"

// wantSuiteHold runs ready and wants exit 6, no flip, an unwritten audit, and every fragment.
func wantSuiteHold(t *testing.T, f *fakeGH, errText func() string, want ...string) {
	t.Helper()
	code := run(readyArgs(exampleRepo))
	if code != deskkit.ExitUnverifiable {
		t.Fatalf("ready exit = %d, want %d — the suite the rollups cannot see was not read\n%s",
			code, deskkit.ExitUnverifiable, errText())
	}
	if f.flips != 0 {
		t.Fatal("flipped a head the forge holds pending on an unconcluded suite")
	}
	for _, w := range want {
		if !strings.Contains(errText(), w) {
			t.Errorf("refusal does not name %q:\n%s", w, errText())
		}
	}
}

// regression: #6234
// The reported shape: rollups green, an older suite of the reporting app concluded, a newer
// suite of the same app queued with zero runs.
func TestReadyZeroRunSuiteHolds(t *testing.T) {
	f, errBuf := setupFake(t)
	f.reviews = []reviewInfo{appReview("APPROVED", testHead, okReviewBody)}
	f.status = greenStatus()
	f.suites = []deskkit.CheckSuite{
		{ID: "11", App: suiteApp, Status: "completed", Conclusion: "success", RunCount: 1,
			CreatedAt: "2026-11-11T20:09:57Z"},
		{ID: "12", App: suiteApp, Status: "queued", CreatedAt: "2026-11-11T20:23:15Z"},
	}
	wantSuiteHold(t, f, errBuf.String, "suite 12 queued, 0 run(s)")
	if got := lastAudit(t).Result; got != deskkit.ResultUnwritten {
		t.Fatalf("audit result = %q, want %q (no write was attempted)", got, deskkit.ResultUnwritten)
	}
}

// regression: #6234
// A suite list shorter than the forge's asserted total was not read in full.
func TestReadySuiteShortReadHolds(t *testing.T) {
	f, errBuf := setupFake(t)
	f.reviews = []reviewInfo{appReview("APPROVED", testHead, okReviewBody)}
	f.status = greenStatus()
	f.suites = []deskkit.CheckSuite{{ID: "11", App: suiteApp, Status: "completed", Conclusion: "success",
		RunCount: 1, CreatedAt: "2026-11-11T20:09:57Z"}}
	f.suitesTotal = 2
	wantSuiteHold(t, f, errBuf.String, "could not be read in full")
}

// regression: #6234
// A 403 on the suite read names the permission and fails closed.
func TestReadySuite403Holds(t *testing.T) {
	f, errBuf := setupFake(t)
	f.reviews = []reviewInfo{appReview("APPROVED", testHead, okReviewBody)}
	f.status = greenStatus()
	force403(f, "/check-suites")
	wantSuiteHold(t, f, errBuf.String, "check suites", "checks: read")
}

// Scope: a later suite of the same app that concluded with runs supersedes a stuck one.
func TestReadyRerunSuiteFlips(t *testing.T) {
	f, errBuf := setupFake(t)
	f.reviews = []reviewInfo{appReview("APPROVED", testHead, okReviewBody)}
	f.status = greenStatus()
	f.suites = []deskkit.CheckSuite{
		{ID: "12", App: suiteApp, Status: "queued", CreatedAt: "2026-11-11T20:23:15Z"},
		{ID: "13", App: suiteApp, Status: "completed", Conclusion: "success", RunCount: 1,
			CreatedAt: "2026-11-11T21:00:00Z"},
	}
	if code := run(readyArgs(exampleRepo)); code != 0 || f.flips != 1 {
		t.Fatalf("ready exit = %d flips = %d, want 0 and 1 — the suite hold over-applied\n%s",
			code, f.flips, errBuf.String())
	}
}
