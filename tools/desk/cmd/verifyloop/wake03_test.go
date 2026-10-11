package main

// wake03_test.go — verify-reset/03: the planner holds a non-pass until its blocker moves. The
// hold table (docs/verify-wake.md) is pinned one subtest per row; a forge read that cannot be
// made is could-not-check, never "closed"; a newer explicit recheck overrides a hold; and the
// plan over the checked-in fixture prints one WAIT line per held brief plus the summary line.

import (
	"errors"
	"strconv"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// allIssues answers every blocker read with one state.
type allIssues deskkit.BlockerState

func (a allIssues) IssueState(deskkit.BlockerRef) (deskkit.BlockerState, string) {
	if deskkit.BlockerState(a) == deskkit.BlockerCouldNotCheck {
		return deskkit.BlockerCouldNotCheck, "forge read failed"
	}
	return deskkit.BlockerState(a), ""
}

var openIssues = allIssues(deskkit.BlockerOpen)

// countIssues answers open and counts the reads, so a test can prove a row never reads the forge.
type countIssues struct{ n int }

func (c *countIssues) IssueState(deskkit.BlockerRef) (deskkit.BlockerState, string) {
	c.n++
	return deskkit.BlockerOpen, ""
}

// refIssues answers by "owner/name#N"; an unlisted ref is could-not-check.
type refIssues map[string]deskkit.BlockerState

func (m refIssues) IssueState(ref deskkit.BlockerRef) (deskkit.BlockerState, string) {
	st, ok := m[ref.Repo().Slug()+"#"+strconv.Itoa(ref.Number)]
	if !ok {
		return deskkit.BlockerCouldNotCheck, "not in the fake"
	}
	return st, ""
}

const holdTS = "2026-09-20T00:00:00Z"

// heldReceipt is a complete relevant-input receipt for example-stream/01, held behind #100.
func heldReceipt(ts string) deskkit.WakeReceipt {
	return deskkit.NewWakeReceipt("r-01-"+ts, "medici-finance/assay", "example-stream/01",
		"assay-verifier-app[bot]", "verify-fail", "0000abc", nil, map[string]string{"file:pkg/x.go": "rev1"},
		"desk-tools/v1.0.16", deskkit.BlockerImplementation, "#100",
		deskkit.WakeRelevantInputChanged, "", "", ts)
}

func recheckReceipt(ts, reason string) deskkit.WakeReceipt {
	return deskkit.NewWakeReceipt("rc-01-"+ts, "medici-finance/assay", "example-stream/01",
		"assay-verifier-app[bot]", "verify-fail", "0000abc", nil, nil,
		"desk-tools/v1.0.16", deskkit.BlockerImplementation, "#100",
		deskkit.WakeExplicitRecheck, "", reason, ts)
}

// holdCase plans one brief over the given receipts and returns its disposition, the classifier
// reason, and the wake reason the scan derived.
func holdCase(t *testing.T, reader deskkit.WakeInputs, issues deskkit.IssueStateSource, recs ...deskkit.WakeReceipt) (disposition, string, string) {
	t.Helper()
	root := planFixtureRoot(t, map[string]string{"01": planBrief("model", "no", "no", "no", "no")})
	if len(recs) > 0 {
		writeReceipts(t, root, recs...)
	}
	v := &VerifyLoop{Root: root, WakeReader: reader, Issues: issues, Now: fixedClock("2026-09-20T12:00:00Z")}
	items, err := v.SelectQueue()
	if err != nil {
		t.Fatalf("SelectQueue: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("want one item, got %d", len(items))
	}
	tier, err := v.TierPolicy(items[0])
	if err != nil {
		t.Fatal(err)
	}
	d, r := classifyItem(items[0], tier)
	return d, r, items[0].Payload["wake_reason"]
}

// Verify row 3: one subtest per row of the hold table, first match wins.
func TestWakeHoldTable(t *testing.T) {
	same := fakeWake{revs: map[string]string{"file:pkg/x.go": "rev1"}}

	t.Run("1-recheck-newer-than-hold", func(t *testing.T) {
		c := &countIssues{}
		d, _, why := holdCase(t, same, c, heldReceipt(holdTS), recheckReceipt("2026-09-20T06:00:00Z", "toolchain fixed"))
		if d != dispDispatch || why != "recheck: toolchain fixed" {
			t.Fatalf("got %v %q; want DISPATCH \"recheck: toolchain fixed\"", d, why)
		}
		if c.n != 0 {
			t.Fatalf("the blocker issue was read %d time(s); a recheck row never reads it", c.n)
		}
	})
	t.Run("2-absent-or-incomplete", func(t *testing.T) {
		if d, _, _ := holdCase(t, same, openIssues); d != dispDispatch {
			t.Fatalf("no receipt: got %v, want DISPATCH", d)
		}
		noRef := heldReceipt(holdTS)
		noRef.BlockerRef = ""
		d, _, why := holdCase(t, same, openIssues, noRef)
		if d != dispDispatch || why != "classification pass, writes a receipt" {
			t.Fatalf("incomplete receipt: got %v %q; want one classification pass", d, why)
		}
	})
	t.Run("3-inputs-changed", func(t *testing.T) {
		moved := fakeWake{revs: map[string]string{"file:pkg/x.go": "rev2"}}
		d, _, why := holdCase(t, moved, openIssues, heldReceipt(holdTS))
		if d != dispDispatch || why != "inputs changed: pkg/x.go" {
			t.Fatalf("got %v %q; want DISPATCH \"inputs changed: pkg/x.go\"", d, why)
		}
	})
	t.Run("4-unchanged-blocker-closed", func(t *testing.T) {
		d, _, why := holdCase(t, same, allIssues(deskkit.BlockerClosed), heldReceipt(holdTS))
		if d != dispDispatch || why != "blocker closed: #100" {
			t.Fatalf("got %v %q; want DISPATCH \"blocker closed: #100\"", d, why)
		}
	})
	t.Run("5-unchanged-blocker-open", func(t *testing.T) {
		d, r, _ := holdCase(t, same, openIssues, heldReceipt(holdTS))
		if d != dispWaitReceipt || !strings.Contains(r, "#100") || !strings.Contains(r, "next: worker") {
			t.Fatalf("got %v %q; want WAIT naming #100 and the next actor", d, r)
		}
	})
	t.Run("6-inputs-could-not-check", func(t *testing.T) {
		gone := fakeWake{unreadable: map[string]bool{"file:pkg/x.go": true}}
		d, r, _ := holdCase(t, gone, allIssues(deskkit.BlockerClosed), heldReceipt(holdTS))
		if d != dispCouldNotCheck || !strings.Contains(r, "never rounded to unchanged") {
			t.Fatalf("got %v %q; want could-not-check on the inputs", d, r)
		}
	})
	t.Run("7-blocker-could-not-check", func(t *testing.T) {
		d, r, _ := holdCase(t, same, allIssues(deskkit.BlockerCouldNotCheck), heldReceipt(holdTS))
		if d != dispCouldNotCheck || !strings.Contains(r, "never read as closed") {
			t.Fatalf("got %v %q; want could-not-check on the blocker", d, r)
		}
	})
}

// failForge is a forge whose every issue read fails the way a rejected credential does.
type failForge struct{ deskkit.Forge }

func (failForge) GetIssue(deskkit.ForgeRepo, int) (*deskkit.Issue, error) {
	return nil, errors.New("HTTP 401: Bad credentials")
}

// Verify row 4: no token (the mint refuses) and a rejected token (HTTP 401) are both
// could-not-check — surfaced, held, never read as a closed blocker.
// The repository is planted in the configured set, so each case reaches its forge branch rather
// than stopping at the repository-set check (asserted on the reason).
func TestWakeNoTokenIsCouldNotCheckNotClosed(t *testing.T) {
	plantAllowedRoster(t, "medici-finance/assay")
	same := fakeWake{revs: map[string]string{"file:pkg/x.go": "rev1"}}
	var calls []string
	sources := map[string]*forgeIssueSource{
		"no token": {forgeFor: func(deskkit.ForgeRepo) (deskkit.Forge, error) {
			return nil, deskkit.Unverifiable("could-not-check: no verifier App token for medici-finance/assay", nil)
		}},
		"rejected token": {forgeFor: func(deskkit.ForgeRepo) (deskkit.Forge, error) { return failForge{}, nil }},
		"unknown state": {forgeFor: func(deskkit.ForgeRepo) (deskkit.Forge, error) {
			return stateForge{state: "locked", calls: &calls}, nil
		}},
	}
	for name, src := range sources {
		ref, _ := deskkit.ParseBlockerRef("#100", "medici-finance", "assay")
		if st, why := src.IssueState(ref); st != deskkit.BlockerCouldNotCheck || strings.Contains(why, "outside") {
			t.Errorf("%s: IssueState = %v (%q), want could-not-check from the forge branch", name, st, why)
		}
		d, r, why := holdCase(t, same, src, heldReceipt(holdTS))
		if d != dispCouldNotCheck || strings.Contains(why, "blocker closed") {
			t.Errorf("%s: got %v %q; want could-not-check, never a closed blocker", name, d, r)
		}
	}
}

// Verify row 7: an explicit recheck newer than the hold dispatches with its reason, without
// reading the blocker; an older recheck is superseded by the hold and stays WAIT.
func TestWakeRecheckOverridesHold(t *testing.T) {
	same := fakeWake{revs: map[string]string{"file:pkg/x.go": "rev1"}}
	d, _, why := holdCase(t, same, openIssues, heldReceipt(holdTS), recheckReceipt("2026-09-20T06:00:00Z", "runner image rebuilt"))
	if d != dispDispatch || why != "recheck: runner image rebuilt" {
		t.Fatalf("newer recheck: got %v %q; want DISPATCH \"recheck: runner image rebuilt\"", d, why)
	}
	d, r, _ := holdCase(t, same, openIssues, recheckReceipt("2026-09-19T06:00:00Z", "stale ask"), heldReceipt(holdTS))
	if d != dispWaitReceipt {
		t.Fatalf("older recheck: got %v %q; want the hold to stand", d, r)
	}
}

// Verify row 5: the plan over the checked-in fixture, three blockers open — three WAIT lines
// each naming its ref, and the summary line.
func TestPlanWakeFixture(t *testing.T) {
	old := issueSourceFn
	t.Cleanup(func() { issueSourceFn = old })
	issueSourceFn = func() deskkit.IssueStateSource {
		return refIssues{
			"example-org/fixture#101": deskkit.BlockerOpen,
			"example-org/fixture#102": deskkit.BlockerOpen,
			"example-org/fixture#103": deskkit.BlockerOpen,
		}
	}
	var perr error
	out := captureStdout(t, func() { perr = cmdPlan([]string{"--root", "testdata/wake-fixture-repo"}) })
	if perr != nil {
		t.Fatalf("cmdPlan: %v", perr)
	}
	for _, n := range []string{"1", "2", "3"} {
		want := "   WAIT wake-fixture/0" + n + " — blocker #10" + n + " open"
		if !strings.Contains(out, want) {
			t.Errorf("no WAIT line naming #10%s (%q):\n%s", n, want, out)
		}
	}
	if c := strings.Count(out, "   WAIT "); c != 3 {
		t.Errorf("%d WAIT lines, want 3:\n%s", c, out)
	}
	for _, n := range []string{"1", "2", "3"} {
		if strings.Contains(out, "=== DISPATCH wake-fixture/0"+n+" ") {
			t.Errorf("held brief wake-fixture/0%s has a DISPATCH block:\n%s", n, out)
		}
	}
	if !strings.Contains(out, "=== DISPATCH wake-fixture/04 ") {
		t.Errorf("the legacy record's brief is not dispatched for its classification pass:\n%s", out)
	}
	if !strings.Contains(out, "verify-desk plan: wait=3 dispatchable=1 could-not-check=1\n") {
		t.Errorf("summary line missing:\n%s", out)
	}

	// Row 6's rule below the preflight: --no-forge reads no blocker, so the three held briefs
	// are could-not-check, never WAIT and never "blocker closed".
	out = captureStdout(t, func() { perr = cmdPlan([]string{"--root", "testdata/wake-fixture-repo", "--no-forge"}) })
	if perr != nil {
		t.Fatalf("cmdPlan --no-forge: %v", perr)
	}
	if strings.Contains(out, "   WAIT ") || strings.Contains(out, "blocker closed") ||
		!strings.Contains(out, "verify-desk plan: wait=0 dispatchable=1 could-not-check=4\n") {
		t.Errorf("--no-forge plan is not wait=0 dispatchable=1 could-not-check=4 with no WAIT line:\n%s", out)
	}
}
