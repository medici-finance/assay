package main

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// The domain-approach mismatch diagnostic and the Complex-domain measures —
// the second half of the Cynefin lens (the first half, domain classification,
// lives in cynefin.go). Cynefin's money diagnostic is not "which domain is this
// work in" but "which domain is it being MANAGED as": a brief tagged complex
// that is managed with ordered tools — a rigid single-answer Verify, no probe
// or experiment, no exploration — thrashes, because you cannot plan-verify a
// probe-sense-respond problem to a single right answer. This file catches that
// mismatch and, per the same lens, supplies the measures the Complex domain
// actually runs on (probe rate, learning velocity, surprise) instead of a
// single ToC constraint.
//
// Three-state (docs/three-state-instrument-rule.md) applies per measure and to
// the diagnostic itself: a signal with no source reports could-not-check,
// NEVER 0. Two of the three Complex measures (learning velocity via
// decision-latency, and surprise) have no wired source in this build and so
// report could-not-check with the reason named — honest absence, not silence.

// probeMarkerRe is the conservative probe/experiment marker scan. It matches
// the exploration vocabulary the lens documents (probe, experiment, safe-to-fail
// and its spaced variant) so a complex brief that is genuinely probing is not
// misread as ordered-managed. Deliberately tight: "exploration" in prose about
// a past event does not match, and the false-negative direction (a real probe
// missed) is the safe one — it can only ever over-report mismatch, never hide a
// mismatch, and the review gate is what a flagged brief lands on anyway.
var probeMarkerRe = regexp.MustCompile(`(?i)\b(probe|probes|experiment|experiments|safe-to-fail|safe to fail)\b`)

// verifyRowCount counts the DATA rows of the `## Verify` section body — rows
// whose Command and Expect cells are both non-empty, located under the header
// row naming those columns. Same structure rules as verifyTableHasRow in
// brieffile.go; that helper returns presence, this one returns the count, which
// is what the mismatch diagnostic keys on (a single-answer Verify is a
// one-row table).
func verifyRowCount(section string) int {
	cmdIdx, expIdx := -1, -1
	n := 0
	for _, raw := range strings.Split(section, "\n") {
		line := strings.TrimSpace(raw)
		if !strings.HasPrefix(line, "|") {
			cmdIdx, expIdx = -1, -1 // left the table; the next one names its own columns
			continue
		}
		if separatorRowRe.MatchString(strings.Trim(line, "|")) {
			continue
		}
		cells := splitRow(line)
		if cmdIdx < 0 || expIdx < 0 {
			for j, c := range cells {
				switch strings.ToLower(strings.TrimSpace(c)) {
				case "command":
					cmdIdx = j
				case "expect":
					expIdx = j
				}
			}
			continue // the header row itself is not a data row
		}
		if cmdIdx < len(cells) && expIdx < len(cells) {
			cmd := normalizeMark(strings.TrimSpace(cells[cmdIdx]))
			exp := normalizeMark(strings.TrimSpace(cells[expIdx]))
			if cmd != "" && exp != "" {
				n++
			}
		}
	}
	return n
}

// orderedManagementSignals names which ordered-management signals a brief
// carries. The conservative set, chosen so the diagnostic does NOT fire on a
// clean complex brief:
//   - a single-answer Verify — exactly one runnable row, the rigid
//     one-correct-answer shape ordered management demands;
//   - no probe/experiment marker anywhere in the body — nothing that reads as
//     probe-sense-respond exploration.
//
// A complex brief carrying BOTH is managed as if the outcome were knowable in
// advance. A brief with a multi-row Verify or a probe marker is left alone:
// the diagnostic flags the shape, not the tag, and the review gate is where a
// flagged brief is confirmed.
func orderedManagementSignals(bf *BriefFile) []string {
	var sig []string
	if verifyRowCount(bf.Verify) <= 1 {
		sig = append(sig, "single-answer Verify (one runnable row)")
	}
	if !probeMarkerRe.MatchString(bf.Body) {
		sig = append(sig, "no probe/experiment marker in body")
	}
	return sig
}

// cynefinMismatch is one flagged brief: a complex-tagged active brief managed
// with ordered tools.
type cynefinMismatch struct {
	ID      string   `json:"id"`
	Signals []string `json:"signals"`
}

// cynefinMeasure is one three-state measure of the Complex domain: it reports
// checked-clean with a value when a source is wired, or could-not-check with a
// reason when none is — never a fabricated 0.
type cynefinMeasure struct {
	State  string   `json:"state"`            // checked-clean | could-not-check
	Value  *float64 `json:"value,omitempty"`  // present only on checked-clean
	Reason string   `json:"reason,omitempty"` // why, on could-not-check
}

// cynefinComplexMeasures is the measure set the Complex domain runs on. Two of
// the three are source-less in this build (see the field comments) and degrade
// to could-not-check by design — a measure with no source is never reported as
// zero.
type cynefinComplexMeasures struct {
	// LearningVelocity reuses the decision-latency instrumentation when wired.
	// It is not wired in this build, so it reports could-not-check — the honest
	// absence a caller can distinguish from "velocity is zero".
	LearningVelocity cynefinMeasure `json:"learning-velocity"`
	// ProbeRate is computable here: the share of active complex briefs that
	// carry a probe/experiment marker — complex work that is actually probing.
	ProbeRate cynefinMeasure `json:"probe-rate"`
	// Surprise (outcome != prediction) needs a prediction record to compare
	// against; none exists in this build, so it reports could-not-check.
	Surprise cynefinMeasure `json:"surprise"`
}

// computeCynefinMismatch scans active complex briefs for ordered-management
// signals. Scope is ACTIVE work only, exactly like the distribution: done has
// left the pipeline and a completed brief's management shape is retrospective,
// not a live mismatch.
func computeCynefinMismatch(streams []*Stream) []cynefinMismatch {
	var out []cynefinMismatch
	for _, s := range streams {
		rowStatus := map[string]string{}
		for i := range s.Briefs {
			rowStatus[s.Briefs[i].Num] = s.Briefs[i].Status
		}
		for _, path := range briefFilePaths(s) {
			bf, ok, err := parseBriefFile(path)
			if err != nil || !ok {
				continue
			}
			if displayDomain(bf.Domain) != "complex" {
				continue
			}
			id, num, okName := expectedBriefID(path)
			if !okName {
				continue
			}
			if status, hasRow := rowStatus[num]; !hasRow || status == "done" {
				continue
			}
			if sig := orderedManagementSignals(bf); len(sig) == 2 {
				out = append(out, cynefinMismatch{ID: id, Signals: sig})
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// computeCynefinComplexMeasures builds the three Complex-domain measures from
// the same active-work scan. probeRate is the one measure with a wired source;
// the other two name their absent source and report could-not-check.
func computeCynefinComplexMeasures(streams []*Stream) cynefinComplexMeasures {
	m := cynefinComplexMeasures{
		LearningVelocity: cynefinMeasure{
			State:  cynefinUnknown,
			Reason: "no decision-latency source wired in this build (learning velocity composes with the decision-latency instrumentation; measure degrades to could-not-check by design)",
		},
		Surprise: cynefinMeasure{
			State:  cynefinUnknown,
			Reason: "no prediction record exists to compare outcomes against (surprise = outcome != prediction; nothing records predictions in this build)",
		},
	}

	total := 0
	probing := 0
	for _, s := range streams {
		rowStatus := map[string]string{}
		for i := range s.Briefs {
			rowStatus[s.Briefs[i].Num] = s.Briefs[i].Status
		}
		for _, path := range briefFilePaths(s) {
			bf, ok, err := parseBriefFile(path)
			if err != nil || !ok {
				continue
			}
			if displayDomain(bf.Domain) != "complex" {
				continue
			}
			num := ""
			if _, n, okName := expectedBriefID(path); okName {
				num = n
			}
			if status, hasRow := rowStatus[num]; !hasRow || status == "done" {
				continue
			}
			total++
			if probeMarkerRe.MatchString(bf.Body) {
				probing++
			}
		}
	}

	if total == 0 {
		m.ProbeRate = cynefinMeasure{
			State:  cynefinUnknown,
			Reason: "no active complex briefs to measure",
		}
		return m
	}
	rate := float64(probing) / float64(total)
	m.ProbeRate = cynefinMeasure{State: cynefinClean, Value: &rate}
	return m
}

// renderMeasure writes one Complex-domain measure line in the text view:
//
//	probe-rate: 0.33 (checked-clean)
//	learning-velocity: could-not-check — <reason>
//
// A measure never renders a bare number without its three-state verdict.
func renderMeasure(b *strings.Builder, name string, m cynefinMeasure) {
	if m.State == cynefinClean && m.Value != nil {
		fmt.Fprintf(b, "  %s: %.2f (checked-clean)\n", name, *m.Value)
		return
	}
	fmt.Fprintf(b, "  %s: %s", name, m.State)
	if m.Reason != "" {
		fmt.Fprintf(b, " — %s", m.Reason)
	}
	b.WriteString("\n")
}
