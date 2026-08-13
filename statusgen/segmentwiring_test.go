package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Wiring tests for the Awaiting-board segmentation.
//
// WHY THESE EXIST AS A SEPARATE LAYER. TestSegmentClassifier and
// TestAwaitingSegmentedAssertions build Brief structs BY HAND, so they pin the
// classifier and the renderer while proving nothing about the code that fills
// those structs in. Every line the feature adds to brieffile.go — the
// `blocked-by` parse, the invalid-value PROBLEM, and the Gate/Evidence/
// BlockedBy wiring onto the README row — could be deleted with that suite
// green, which is exactly how a `blocked-by` block nested inside the
// `exec-tier-why` block shipped: the field parsed to "" for every brief that
// did not also carry `exec-tier-why`, and the env-blocked segment was inert.
//
// So these run the PRODUCTION path end to end — write real brief files to a
// temp tree, loadStreams -> checkBriefFiles -> classifyAwaiting — and assert
// the segment the board would actually render.

// segFixtureOpts describes a one-brief fixture repo.
type segFixtureOpts struct {
	status   string // README row status; defaults to "implemented"
	gate     string // brief gate; defaults to "model"
	extra    string // extra frontmatter lines (each newline-terminated)
	evidence string // body of the `## Evidence` section
}

// segFixture writes a one-stream repo with a single brief-v1 brief and returns
// the loaded stream, its brief row, and the hard validation problems.
func segFixture(t *testing.T, o segFixtureOpts) (*Stream, *Brief, []string) {
	t.Helper()
	if o.status == "" {
		o.status = "implemented"
	}
	if o.gate == "" {
		o.gate = "model"
	}
	root := t.TempDir()
	dir := filepath.Join(root, "docs", "streams", "seg")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	readme := fmt.Sprintf(`---
stream: seg
status: active
priority: P1
track: platform
---

# Seg

| # | Brief | Wave | Effort | Status | Verified | Reviewed |
|---|-------|------|--------|--------|----------|----------|
| 01 | [Segment wiring](./brief-01-seg.md) | 0 | S | %s | — | — |
`, o.status)
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte(readme), 0o644); err != nil {
		t.Fatal(err)
	}

	gateWhy := ""
	if o.gate == "human" {
		gateWhy = "gate-why: fixture is human-gated to exercise the wiring\n"
	}
	brief := fmt.Sprintf(`---
brief: seg/01
title: Segment wiring fixture
wave: 0
depends: []
unblocks: []
effort: S
gate: %s
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-20 by fixture
sources: ["fixture: segment wiring"]
why: >-
  Exercises the parse-and-wire path the hand-built classifier tests never touch.
%s%s---

# Brief 01 — segment wiring

## Evidence

%s
`, o.gate, gateWhy, o.extra, o.evidence)
	if err := os.WriteFile(filepath.Join(dir, "brief-01-seg.md"), []byte(brief), 0o644); err != nil {
		t.Fatal(err)
	}

	streams, _, err := loadStreams(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(streams) != 1 || len(streams[0].Briefs) != 1 {
		t.Fatalf("fixture did not load one stream with one brief: %+v", streams)
	}
	problems, _ := checkBriefFiles(streams)
	return streams[0], &streams[0].Briefs[0], problems
}

// TestBlockedByParsedWithoutExecTierWhy is the regression pin for the nesting
// defect: `blocked-by` must parse on an ORDINARY brief, one carrying neither
// `exec-tier` nor `exec-tier-why`. Those keys are only written on
// `exec-tier: strong` briefs, so nesting made the feature inert for the common
// case AND made its validation unreachable — an env-blocked brief failed OPEN
// into the desk-actionable queue the desk tries to drain.
func TestBlockedByParsedWithoutExecTierWhy(t *testing.T) {
	s, br, problems := segFixture(t, segFixtureOpts{
		extra:    "blocked-by: env\n",
		evidence: "Nothing run yet.",
	})
	if br.BlockedBy != "env" {
		t.Fatalf("blocked-by must parse without exec-tier-why; got %q", br.BlockedBy)
	}
	if got := classifyAwaiting(s, br); got != segmentEnvBlocked {
		t.Errorf("classifyAwaiting() = %v, want segmentEnvBlocked", got)
	}
	if hasProblem(problems, "blocked-by") {
		t.Errorf("valid blocked-by must raise no problem; got:\n%s", strings.Join(problems, "\n"))
	}
}

// TestBlockedByParsedAlongsideExecTierWhy proves the fix did not trade one
// nesting for another: the field still parses when exec-tier-why IS present.
func TestBlockedByParsedAlongsideExecTierWhy(t *testing.T) {
	s, br, _ := segFixture(t, segFixtureOpts{
		extra:    "exec-tier: strong\nexec-tier-why: touches the ledger\nblocked-by: env\n",
		evidence: "Nothing run yet.",
	})
	if br.BlockedBy != "env" {
		t.Fatalf("blocked-by must parse alongside exec-tier-why; got %q", br.BlockedBy)
	}
	if got := classifyAwaiting(s, br); got != segmentEnvBlocked {
		t.Errorf("classifyAwaiting() = %v, want segmentEnvBlocked", got)
	}
}

// TestBlockedByInvalidValueIsProblem pins the invalid-value PROBLEM through the
// production path — unreachable while the parse was nested, so an arbitrary
// string was swallowed silently. The bad value must also be left OFF the row
// rather than inventing a segment.
func TestBlockedByInvalidValueIsProblem(t *testing.T) {
	s, br, problems := segFixture(t, segFixtureOpts{
		extra:    "blocked-by: bogus-value-xyz\n",
		evidence: "Nothing run yet.",
	})
	if !hasProblem(problems, "invalid blocked-by", "bogus-value-xyz") {
		t.Errorf("want an invalid blocked-by problem; got:\n%s", strings.Join(problems, "\n"))
	}
	if br.BlockedBy != "" {
		t.Errorf("an invalid blocked-by must not reach the row; got %q", br.BlockedBy)
	}
	if got := classifyAwaiting(s, br); got != segmentDeskActionable {
		t.Errorf("classifyAwaiting() = %v, want segmentDeskActionable", got)
	}
}

// TestGateAndEvidenceWiredOntoRow pins the two other wiring lines: without
// `row.Gate` a human-gated brief with a recorded pass renders as drainable, and
// without `row.Evidence` no verdict is ever visible to the classifier.
func TestGateAndEvidenceWiredOntoRow(t *testing.T) {
	s, br, _ := segFixture(t, segFixtureOpts{
		gate:     "human",
		evidence: "**VERIFY: PASS** · 2026-07-20 · `fixture-verifier`",
	})
	if br.Gate != "human" {
		t.Fatalf("gate must be wired onto the row; got %q", br.Gate)
	}
	if !strings.Contains(br.Evidence, "VERIFY: PASS") {
		t.Fatalf("evidence must be wired onto the row; got %q", br.Evidence)
	}
	if got := classifyAwaiting(s, br); got != segmentHumanGate {
		t.Errorf("classifyAwaiting() = %v, want segmentHumanGate", got)
	}
}

// TestSegmentWiringLiveMarkerForms runs the marker forms actually written in
// this repo's Evidence sections through the production path. Both were filed as
// desk-actionable by the exact-literal `**VERIFY: PASS**` test while genuinely
// waiting on a human — the misfiling the segmentation exists to stop.
func TestSegmentWiringLiveMarkerForms(t *testing.T) {
	for _, tc := range []struct {
		name     string
		evidence string
	}{
		{
			name:     "bold-span form: bold span wraps a longer phrase",
			evidence: "**Non-implementer verifier run — VERIFY: PASS** · 2026-07-20 · `glm-5.2-verifier` · merged main `73d01752`",
		},
		{
			name:     "loop-engine/01 form (trailing prose inside the bold span)",
			evidence: "### Non-implementer verifier run — VERIFY: PASS (stays `implemented`)\n\n**VERIFY: PASS — all 6 rows green.**",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, br, _ := segFixture(t, segFixtureOpts{gate: "human", evidence: tc.evidence})
			if got := classifyAwaiting(s, br); got != segmentHumanGate {
				t.Errorf("classifyAwaiting() = %v, want segmentHumanGate (evidence %q)", got, tc.evidence)
			}
		})
	}
}

// TestSegmentWiringAccumulatedEvidence is the accumulated-evidence shape through the
// production path: a `verified` brief whose Evidence holds superseded FAIL
// records followed by the PASS that promoted it. Any-occurrence matching pinned
// it in rework where the desk would never drain it and the headline would never
// count it.
func TestSegmentWiringAccumulatedEvidence(t *testing.T) {
	evidence := `> earlier VERIFY: FAIL flagged (row 2 re-targeted below).

### Non-implementer verifier run — VERIFY: FAIL (fixture-verifier, 2026-07-16)

**VERIFY: FAIL — row 2 as written.**

### Re-verify run — VERIFY: FAIL (stale command; goal MET) — fixture-verifier, 2026-07-17

### Non-implementer verifier run — VERIFY: PASS — fixture-verifier, 2026-07-20

| # | Command | Exit | Result | Date | Runner |
|---|---------|------|--------|------|--------|
| 1 | go test ./... | 0 | PASS | 2026-07-20 | fixture-verifier |

**VERIFY: PASS — substantive goal MET on current main.**`
	s, br, _ := segFixture(t, segFixtureOpts{status: "verified", evidence: evidence})
	if got := classifyAwaiting(s, br); got != segmentDeskActionable {
		t.Errorf("classifyAwaiting() = %v, want segmentDeskActionable — the last verdict is PASS on a gate:model brief", got)
	}
}
