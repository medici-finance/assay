package main

import (
	"path/filepath"
	"strings"
	"testing"
)

// setStreamCap overrides the cap config for one test and restores it after.
func setStreamCap(t *testing.T, cap int, set bool) {
	t.Helper()
	prev := streamCapConfigFn
	streamCapConfigFn = func() (int, bool) { return cap, set }
	t.Cleanup(func() { streamCapConfigFn = prev })
}

// setBaseActive stubs the base-tree status read: the named repo-relative READMEs
// were `status: active` on the base; every other path was not (an add/flip).
func setBaseActive(t *testing.T, active ...string) {
	t.Helper()
	want := map[string]bool{}
	for _, a := range active {
		want[a] = true
	}
	prev := streamBaseIsActive
	streamBaseIsActive = func(_, rel string) bool { return want[rel] }
	t.Cleanup(func() { streamBaseIsActive = prev })
}

func hasSubstr(list []string, sub string) bool {
	for _, s := range list {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

// TestStreamCapUnsetIsNotice — an adopter with no cap configured sees exactly one
// NOTICE naming the count, and NEVER a PROBLEM. Absent ⇒ inert (Verify row 4).
func TestStreamCapUnsetIsNotice(t *testing.T) {
	setStreamCap(t, 0, false)
	streams := []*Stream{
		mkStream("a", "active", "P1"),
		mkStream("b", "active", "P1"),
		mkStream("c", "active", "P1"),
	}
	problems, notices := streamCapLint(streams, "/repo", nil)
	if len(problems) != 0 {
		t.Fatalf("unset cap must never PROBLEM; got %v", problems)
	}
	if !hasSubstr(notices, "stream-cap") || !hasSubstr(notices, "unset") {
		t.Fatalf("unset cap must NOTICE `stream-cap: unset`; got %v", notices)
	}
	if !hasSubstr(notices, "3 active") {
		t.Fatalf("the unset NOTICE must name the active count; got %v", notices)
	}
}

// TestStreamCapFullLintOverCapIsNoticeNotProblem — a full lint (no --changed) over
// the cap NOTICEs the standing count and cap; it never gates (Verify row 5 / the
// daily-regen surfaces a standing over-cap). The count printed is the live count.
func TestStreamCapFullLintOverCapIsNoticeNotProblem(t *testing.T) {
	setStreamCap(t, 2, true)
	streams := []*Stream{
		mkStream("a", "active", "P1"),
		mkStream("b", "active", "P1"),
		mkStream("c", "active", "P1"),
	}
	problems, notices := streamCapLint(streams, "/repo", nil)
	if len(problems) != 0 {
		t.Fatalf("a full lint must never PROBLEM on a standing over-cap; got %v", problems)
	}
	if !hasSubstr(notices, "stream-cap") || !hasSubstr(notices, "3 active") || !hasSubstr(notices, "cap of 2") {
		t.Fatalf("the full-lint NOTICE must name the count and the cap; got %v", notices)
	}
}

// TestStreamCapAddAtCapIsProblem — the PR-diff gate. A diff that adds an active
// stream README while the tree is over the cap, with no offsetting park, is a
// PROBLEM naming `stream-cap` (Verify row 2, the negative path).
func TestStreamCapAddAtCapIsProblem(t *testing.T) {
	setStreamCap(t, 2, true)
	// Pre-diff at cap (a, b); this diff ADDS c as active → 3 active, over cap.
	streams := []*Stream{
		mkStream("a", "active", "P1"),
		mkStream("b", "active", "P1"),
		mkStream("c", "active", "P1"),
	}
	setBaseActive(t, "docs/streams/a/README.md", "docs/streams/b/README.md") // c is NEW on base
	changed := []string{"docs/streams/c/README.md"}
	problems, _ := streamCapLint(streams, "/repo", changed)
	if !hasSubstr(problems, "stream-cap") {
		t.Fatalf("adding an active stream over the cap must PROBLEM with `stream-cap`; got %v", problems)
	}
	if !hasSubstr(problems, "c") {
		t.Fatalf("the PROBLEM must name the added stream; got %v", problems)
	}
}

// TestStreamCapParkInSameDiffIsClean — a diff that adds one active stream AND parks
// another in the same change nets to the cap, so it passes the gate (Verify row 3,
// the flow path: park-to-make-room clears the cap).
func TestStreamCapParkInSameDiffIsClean(t *testing.T) {
	setStreamCap(t, 2, true)
	// Post-diff: a, c active (2 == cap); b parked. The diff added c and parked b.
	streams := []*Stream{
		mkStream("a", "active", "P1"),
		mkStream("b", "parked", "P1"),
		mkStream("c", "active", "P1"),
	}
	setBaseActive(t, "docs/streams/a/README.md", "docs/streams/b/README.md") // b WAS active, c is new
	changed := []string{"docs/streams/c/README.md", "docs/streams/b/README.md"}
	problems, _ := streamCapLint(streams, "/repo", changed)
	if len(problems) != 0 {
		t.Fatalf("parking a stream to make room must pass the cap gate; got %v", problems)
	}
}

// TestStreamCapUnrelatedPRStandingOverCapNoProblem — a PR that touches no active
// stream README, in a tree that is standing over the cap, is NOT reddened.
func TestStreamCapUnrelatedPRStandingOverCapNoProblem(t *testing.T) {
	setStreamCap(t, 2, true)
	streams := []*Stream{
		mkStream("a", "active", "P1"),
		mkStream("b", "active", "P1"),
		mkStream("c", "active", "P1"),
	}
	setBaseActive(t, "docs/streams/a/README.md", "docs/streams/b/README.md", "docs/streams/c/README.md")
	changed := []string{"statusgen/main.go", "docs/streams/a/brief-01-x.md"}
	problems, notices := streamCapLint(streams, "/repo", changed)
	if len(problems) != 0 {
		t.Fatalf("an unrelated PR must not be reddened by a standing over-cap; got %v", problems)
	}
	if !hasSubstr(notices, "did not introduce") {
		t.Fatalf("the standing over-cap should be surfaced as a NOTICE; got %v", notices)
	}
}

// TestParkedExcludedFromNextUp — a parked stream's briefs are never offered on the
// dispatch board (Verify row 3).
func TestParkedExcludedFromNextUp(t *testing.T) {
	active := mkStream("live", "active", "P1", Brief{Num: "01", Wave: 0, Status: "todo"})
	parked := mkStream("shelved", "parked", "P0", Brief{Num: "01", Wave: 0, Status: "todo"})
	picks := nextUp([]*Stream{active, parked}, ClaimView{}, nil).Picks
	for _, p := range picks {
		if p.Stream.Name == "shelved" {
			t.Fatalf("a parked stream's brief must never be a Next-up pick; got %+v", picks)
		}
	}
	found := false
	for _, p := range picks {
		if p.Stream.Name == "live" {
			found = true
		}
	}
	if !found {
		t.Fatalf("the active stream's brief must still be picked; got %+v", picks)
	}
}

// TestParkedBoardSection — a parked stream renders under its own `## Parked`
// heading, out of the active roll-up (Verify row 1). A tree with no parked stream
// carries no such heading.
func TestParkedBoardSection(t *testing.T) {
	active := mkStream("live", "active", "P1", Brief{Num: "01", Wave: 0, Status: "todo"})
	active.Track = "platform"
	parked := mkStream("shelved", "parked", "P0", Brief{Num: "01", Wave: 0, Status: "todo"})
	parked.Track = "platform"
	streams := []*Stream{active, parked}
	out := emit(streams, nil, nextUp(streams, ClaimView{}, nil), nil, nil, IntakeAlarmResult{}, nil, "")
	if !strings.Contains(out, "## Parked") {
		t.Fatalf("a parked stream must render a `## Parked` board section; board:\n%s", out)
	}
	// The parked stream is listed in the Parked section, not the active roll-up.
	parkedIdx := strings.Index(out, "## Parked")
	rollupIdx := strings.Index(out, "## Roll-up")
	nextupIdx := strings.Index(out, "## Next up")
	if !(rollupIdx < parkedIdx && parkedIdx < nextupIdx) {
		t.Fatalf("the `## Parked` section should sit between Roll-up and Next up; roll-up=%d parked=%d nextup=%d", rollupIdx, parkedIdx, nextupIdx)
	}

	// A board with no parked stream carries no `## Parked` heading and its Totals
	// line has no parked clause — byte-shape unchanged for the common case.
	noParked := []*Stream{active}
	out2 := emit(noParked, nil, nextUp(noParked, ClaimView{}, nil), nil, nil, IntakeAlarmResult{}, nil, "")
	if strings.Contains(out2, "## Parked") {
		t.Fatalf("a board with no parked stream must not render a `## Parked` section")
	}
	if strings.Contains(out2, "parked)") {
		t.Fatalf("a board with no parked stream must not add a parked clause to Totals")
	}
}

// TestParkedTotals — the Totals line counts a parked stream separately from active.
func TestParkedTotals(t *testing.T) {
	streams := []*Stream{
		mkStream("live", "active", "P1"),
		mkStream("shelved", "parked", "P0"),
	}
	out := emit(streams, nil, nextUp(streams, ClaimView{}, nil), nil, nil, IntakeAlarmResult{}, nil, "")
	if !strings.Contains(out, "**1** parked") {
		t.Fatalf("Totals must count the parked stream; board tail:\n%s", out[strings.Index(out, "## Totals"):])
	}
}

// TestStreamSourceRequiresApprovedSpec — the four fixture cases: an active stream
// citing an approved spec is clean; citing a draft or nothing is a PROBLEM; a
// parked stream citing a draft is clean (Verify row 7a).
func TestStreamSourceRequiresApprovedSpec(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "docs/specs/approved.md"), "# Approved spec\n\n**Status:** approved\n\n## Body\n")
	mustWrite(t, filepath.Join(root, "docs/specs/draft.md"), "# Draft spec\n\n**Status:** draft\n\n## Body\n")

	approvedRel := "docs/specs/approved.md"
	draftRel := "docs/specs/draft.md"

	cases := []struct {
		name        string
		status      string
		spec        string
		wantProblem bool
	}{
		{"active-cites-approved", "active", approvedRel, false},
		{"active-cites-draft", "active", draftRel, true},
		{"active-cites-nothing", "active", "", true},
		{"parked-cites-draft", "parked", draftRel, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := &Stream{Name: "s", Status: c.status, Spec: c.spec}
			got := streamSourceProblem(root, s)
			if c.wantProblem && got == "" {
				t.Fatalf("%s: expected a stream-source PROBLEM, got none", c.name)
			}
			if !c.wantProblem && got != "" {
				t.Fatalf("%s: expected clean, got PROBLEM %q", c.name, got)
			}
			if c.wantProblem && !strings.Contains(got, "stream-source") {
				t.Fatalf("%s: PROBLEM must be tagged stream-source; got %q", c.name, got)
			}
		})
	}
}

// TestStreamSourceDifferentialGrandfathersStanding — a full lint (no --changed)
// never fires stream-source, so a standing corpus of active streams with no
// `spec:` is not reddened; the rule binds only diff-added active streams.
func TestStreamSourceDifferentialGrandfathersStanding(t *testing.T) {
	streams := []*Stream{mkStream("legacy", "active", "P1")} // no spec:
	problems, _ := streamSourceLint(streams, "/repo", nil)
	if len(problems) != 0 {
		t.Fatalf("a full lint must grandfather standing active streams; got %v", problems)
	}
	// A diff that ADDS this active stream (new on base) with no spec: → PROBLEM.
	setBaseActive(t /* nothing was active on base */)
	changed := []string{"docs/streams/legacy/README.md"}
	problems2, _ := streamSourceLint(streams, "/repo", changed)
	if !hasSubstr(problems2, "stream-source") {
		t.Fatalf("a diff adding an active stream with no approved spec must PROBLEM; got %v", problems2)
	}
}
