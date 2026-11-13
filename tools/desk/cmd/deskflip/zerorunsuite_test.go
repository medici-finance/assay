package main

// zerorunsuite_test.go — checks-green against the check SUITES at the head (#6234).
//
// A suite that has not concluded holds the forge's merge gate even when it carries no runs,
// and a run-less suite never appears in the rollup. checks-green therefore reads the suites
// too: an unconcluded suite from an app that reports at the head refuses could-not-check.

import (
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

const actionsApp = "github-actions"

// assertSuiteHold runs the flip and wants could-not-check, no ready mutation, and a refusal
// naming checks-green and every fragment in want.
func assertSuiteHold(t *testing.T, s *stub, want ...string) {
	t.Helper()
	var rc int
	out := captureStderr(t, func() { rc = run([]string{"7", "--repo", privateCIRepo}) })
	if rc != deskkit.ExitUnverifiable {
		t.Fatalf("rc = %d, want %d (could-not-check) — the suite the rollup cannot see was not read\n%s",
			rc, deskkit.ExitUnverifiable, out)
	}
	if s.flipped() {
		t.Fatalf("the ready mutation ran on a head the forge holds pending: %v", s.requests)
	}
	for _, w := range append([]string{condChecksGreen}, want...) {
		if !strings.Contains(out, w) {
			t.Errorf("refusal does not name %q:\n%s", w, out)
		}
	}
}

// assertFlips runs the flip and wants success with the ready mutation issued.
func assertFlips(t *testing.T, s *stub) {
	t.Helper()
	var rc int
	out := captureStderr(t, func() { rc = run([]string{"7", "--repo", privateCIRepo}) })
	if rc != deskkit.ExitOK {
		t.Fatalf("rc = %d, want 0 — the suite hold over-applied\n%s", rc, out)
	}
	if !s.flipped() {
		t.Errorf("the ready mutation never ran: %v", s.requests)
	}
}

// regression: #6234
// The reported shape: every required check is green in the rollup, and an older suite of the
// same app concluded green, but two NEWER suites of that app have no conclusion and no runs.
func TestZeroRunSuiteHolds(t *testing.T) {
	s := newStub()
	s.rollup = []rollupEntry{
		{Name: "changelog", Status: "COMPLETED", Conclusion: "SUCCESS"},
		{Name: "build-test", Status: "COMPLETED", Conclusion: "SUCCESS"},
	}
	s.requiredChecks = []string{"changelog", "build-test"}
	s.suites = []deskkit.CheckSuite{
		{ID: "116700446469", App: actionsApp, Status: "completed", Conclusion: "success", RunCount: 2,
			CreatedAt: "2026-11-11T20:09:57Z"},
		{ID: "116705687855", App: actionsApp, Status: "queued", CreatedAt: "2026-11-11T20:23:15Z"},
		{ID: "116705694588", App: actionsApp, Status: "queued", CreatedAt: "2026-11-11T20:23:16Z"},
	}
	s.install(t)
	s.reviews = approvalAtHead(t, headSHA)

	assertSuiteHold(t, s, "116705687855", "116705694588", "0 run(s)")
}

// regression: #6234
// A second path to the same divergence: the branch's required set reads EMPTY (a 404), and
// the workflow that would report the required check was created but never got a job — its
// suite is queued with zero runs while another check of the same app is green. The empty-set
// arm and the green arm both read the rollup alone; the suite read holds this one too.
func TestEmptyRequiredSuiteHolds(t *testing.T) {
	s := newStub()
	s.rollup = []rollupEntry{{Name: "changelog", Status: "COMPLETED", Conclusion: "SUCCESS"}}
	s.suites = []deskkit.CheckSuite{
		{ID: "201", App: actionsApp, Status: "completed", Conclusion: "success", RunCount: 1,
			CreatedAt: "2026-11-11T20:18:00Z"},
		{ID: "202", App: actionsApp, Status: "queued", CreatedAt: "2026-11-11T20:18:01Z"},
	}
	s.install(t)
	s.reviews = approvalAtHead(t, headSHA)

	assertSuiteHold(t, s, "suite 202 queued")
}

// regression: #6234
// A suite list the forge asserts is longer than it served was not read in full.
func TestSuiteShortReadHolds(t *testing.T) {
	s := newStub()
	s.suites = []deskkit.CheckSuite{
		{ID: "301", App: actionsApp, Status: "completed", Conclusion: "success", RunCount: 1,
			CreatedAt: "2026-11-11T20:00:00Z"},
	}
	s.suiteTotalOverride = 2
	s.install(t)
	s.reviews = approvalAtHead(t, headSHA)

	assertSuiteHold(t, s, "1/2 check suites")
}

// regression: #6234
// A suite read that fails is could-not-check, never green.
func TestSuiteReadErrorHolds(t *testing.T) {
	s := newStub()
	s.failPath = "/check-suites"
	s.install(t)
	s.reviews = approvalAtHead(t, headSHA)

	assertSuiteHold(t, s, "cannot read the check suites")
}

// regression: #6234
// Review finding F1, on the reported head's own shape: the two stuck changelog-check suites are
// followed by a LATER suite that concluded with runs — the 00:40:07Z rerun of that same
// workflow. The forge still read BLOCKED after it, so a later suite releases nothing.
func TestRerunSuiteHolds(t *testing.T) {
	s := newStub()
	s.suites = []deskkit.CheckSuite{
		{ID: "116705687855", App: actionsApp, Status: "queued", CreatedAt: "2026-11-11T20:23:15Z"},
		{ID: "116705694588", App: actionsApp, Status: "queued", CreatedAt: "2026-11-11T20:23:16Z"},
		{ID: "116795245984", App: actionsApp, Status: "completed", Conclusion: "success", RunCount: 1,
			CreatedAt: "2026-11-12T00:40:07Z"},
	}
	s.install(t)
	s.reviews = approvalAtHead(t, headSHA)

	assertSuiteHold(t, s, "116705687855", "116705694588")
}

// Scope: an installed app that never acts leaves a queued, run-less suite on every commit. It
// reports nothing at this head, so nothing the merge gate waits on; it does not hold.
func TestIdleAppSuiteIgnored(t *testing.T) {
	s := newStub()
	s.suites = []deskkit.CheckSuite{
		{ID: "501", App: actionsApp, Status: "completed", Conclusion: "success", RunCount: 1,
			CreatedAt: "2026-11-11T20:00:00Z"},
		{ID: "502", App: "idle-app", Status: "queued", CreatedAt: "2026-11-11T20:00:01Z"},
	}
	s.install(t)
	s.reviews = approvalAtHead(t, headSHA)

	assertFlips(t, s)
}
