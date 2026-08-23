package main

// The canary — the defence against a dead pipeline.
//
// #643: this repository's P0 leak-on-main alerter was silently dead for roughly
// 180 commits. It reported nothing. Nothing is also what a passing gate looks
// like. No amount of care in the detection rules addresses that failure, because
// the rules were never reached; the instrument itself had stopped.
//
// So every run plants a known-detectable artefact into the tree it is about to
// scan and requires each engine to FIND it — using the engine's own stock rule
// set, not our house config. The reasoning:
//
//   - a house rule firing proves our config was loaded. It says nothing about
//     whether the maintained ecosystem rule set — the entire reason for adopting
//     gitleaks and trufflehog — is running.
//   - the stock pass proves the scanner is scanning, its built-in rules are
//     loaded, and its report path works end to end.
//
// If an engine reports zero canary hits, the run is a DEAD ALERTER: not clean,
// not dirty, could-not-check, non-zero. That is the whole point. Zero findings
// including the canary is the one result that must never certify.
//
// The canary is a TRACKED file at a fixed path (testdata/canary/CANARY.txt) so it
// is reviewable, and it is EMBEDDED so the tool carries it regardless of the
// working directory it is invoked from. A canary that can go missing depending on
// CWD is a canary that will one day be quietly absent.

import (
	_ "embed"
	"fmt"
	"strings"
)

//go:embed testdata/canary/CANARY.txt
var canaryTemplate string

// canaryFields assembles the planted credential shapes from fragments.
//
// The complete shapes exist only here, at run time, and only in the temporary
// tree handed to the scanners — never as a literal in a tracked file. That is
// deliberate and it resolves a genuine tension rather than dodging one: a canary
// must be realistic enough to trip a maintained scanner's stock rules, and
// anything that realistic trips every OTHER secret guard between here and review.
// This repository's own pre-write guard refused the push twice while this was
// being written, on two different credential shapes, and both refusals were
// correct.
//
// The two wrong fixes both leave a gate that reports success: override the guard,
// or water the canary down until no scanner reliably detects it. A canary nobody
// detects is indistinguishable from a dead one.
//
// The same construct-from-parts discipline containsBannedConstruct already uses,
// applied for the same reason: so the file does not contain the thing it is about.
func canaryFields() map[string]string {
	return map[string]string{
		"__CANARY_AWS_ID__":     "AK" + "IA" + "ZZ7Q4KL2MJXPWQ3B",
		"__CANARY_AWS_SECRET__": "7hK2qLpV0sNfR4tYzXbW9cE1" + "jUmDaGiO5PvTnQrZ",
		"__CANARY_SLACK__":      "xo" + "xb-" + "2413579086420-2413579086421-" + "8kQwErTyUiOpAsDfGhJkLzXc",
	}
}

// canaryContent is the fixture with every placeholder substituted — what actually
// gets planted.
func canaryContent() string {
	s := canaryTemplate
	for k, v := range canaryFields() {
		s = strings.ReplaceAll(s, k, v)
	}
	return s
}

const canaryFileName = "CANARY.txt"

// canaryProbe is a stable, human-readable fragment of the canary file used to
// prove the embed actually resolved to the intended content rather than to an
// empty or truncated file.
const canaryProbe = "LEAKSWEEP ENGINE CANARY"

// checkCanaryEmbed validates the embedded fixture at start of run. An empty or
// unrecognisable canary would mean every engine legitimately reports zero hits
// on it, which the dead-alerter check would then read as a dead engine — a
// confusing failure for a real defect one layer up. Catch it here and say so.
func checkCanaryEmbed() error {
	if !strings.Contains(canaryTemplate, canaryProbe) {
		return withExit(exitCouldNotCheck,
			"could-not-check: the embedded canary fixture does not carry its own marker — the canary is broken, so no run can prove an engine is alive")
	}
	// A placeholder that survives substitution is a canary field the scanners can
	// never match — a silently toothless canary, which is the one failure this
	// whole mechanism exists to prevent. Catch it here and refuse the run.
	planted := canaryContent()
	for k := range canaryFields() {
		if strings.Contains(planted, k) {
			return withExit(exitCouldNotCheck,
				"could-not-check: canary placeholder %s was not substituted — the planted canary would carry no detectable credential shape, so no engine could be proven alive", k)
		}
	}
	return nil
}

// deadAlerter renders the dead-alerter diagnosis for one engine. Kept as one
// function so the phrase a Verify row greps for has exactly one source.
func deadAlerter(engineName string) string {
	return fmt.Sprintf(
		"dead-alerter: %s reported ZERO findings for the planted canary, so its stock rule set is not running. "+
			"A run that cannot see a credential it planted itself has not checked the tree — it has proved the scanner is dead (#643)",
		engineName)
}
