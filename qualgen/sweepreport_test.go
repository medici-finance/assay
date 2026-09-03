package main

import (
	"strings"
	"testing"

	"github.com/medici-finance/assay/qualgen/verifier"
)

// TestRenderReport_SectionsByClass proves each verdict class lands in its own
// section: actionable verdicts carry their evidence; false positives are
// suppressed with reasons; could-not-verify items are listed, never dropped.
func TestRenderReport_SectionsByClass(t *testing.T) {
	sConf := verifier.Suspect{Fingerprint: "a", Category: "dead-code", File: "a.go", LineStart: 1, Rule: "U1000"}
	sFP := verifier.Suspect{Fingerprint: "b", Category: "duplication", File: "b.go", LineStart: 2, Rule: "dupl"}
	sCNV := verifier.Suspect{Fingerprint: "c", Category: "swallowed-error", File: "c.go", LineStart: 3, Rule: "errcheck"}

	run := &SweepRun{
		RunDate:    "2026-09-02",
		TargetSHA:  "deadbeef",
		Categories: []CategoryResult{{Category: "dead-code", State: Measured(1)}},
		New:        []verifier.Suspect{sConf, sFP, sCNV},
		Verdicts: map[string]verifier.Verdict{
			"a": {Fingerprint: "a", Class: verifier.ClassConfirmed, EvidencePointer: "a.go:1 dead", Rationale: "no callers"},
			"b": {Fingerprint: "b", Class: verifier.ClassFalsePositive, Rationale: "deliberate"},
			"c": {Fingerprint: "c", Class: verifier.ClassCouldNotVerify, Rationale: "agent timed out"},
		},
		Reclassified: map[string]bool{},
	}
	report := renderSweepReport(run)

	// Actionable section carries the confirmed suspect + its evidence.
	if !strings.Contains(report, "**confirmed**") || !strings.Contains(report, "a.go:1 dead") {
		t.Errorf("confirmed suspect/evidence missing from actionable section:\n%s", report)
	}
	// The false positive is suppressed with its reason, not actionable.
	if !strings.Contains(report, "suppressed: deliberate") {
		t.Errorf("false positive not suppressed with reason:\n%s", report)
	}
	// could-not-verify is listed with its reason.
	if !strings.Contains(report, "could-not-verify: agent timed out") {
		t.Errorf("could-not-verify item not listed:\n%s", report)
	}
	// A false positive must never render as actionable.
	if strings.Contains(report, "b.go") && strings.Contains(report, "**false-positive**") {
		t.Errorf("false positive leaked into an actionable rendering:\n%s", report)
	}
}

func TestMeasureStateCell_ThreeStates(t *testing.T) {
	if got := measureStateCell(Measured(3)); !strings.Contains(got, "measured (3") {
		t.Errorf("measured cell: %q", got)
	}
	if got := measureStateCell(MeasuredZero[int]()); !strings.Contains(got, "measured-zero") {
		t.Errorf("measured-zero cell: %q", got)
	}
	if got := measureStateCell(CouldNotMeasure[int]("no tool")); !strings.Contains(got, "could-not-measure") || !strings.Contains(got, "no tool") {
		t.Errorf("could-not-measure cell: %q", got)
	}
}
