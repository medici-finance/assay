package main

// grammar.go — the tick summary-line grammar (plugins/assay/references/tick-contract.md), the Go
// port of plugins/assay/scripts/tick-summary.sh.
//
// A desk role invoked in tick mode prints exactly one machine-readable line, as the LAST line of
// its output:
//
//	tick role=<role> outcome=<outcome> swept=<n> acted=<n> filed=<n> duration=<s>
//
// That line is the sole verdict channel: a one-shot harness call exits 0 for any completed turn,
// and a killed pass prints nothing, so a caller that receives no `tick …` line knows the pass did
// not complete.
//
// THE CROSS-FIELD RULES ARE THE POINT. A shape check alone would accept `outcome=noop swept=-` — a
// blind pass wearing a healthy pass's clothes. So:
//
//	ok               swept numeric, acted numeric and >= 1
//	noop             swept numeric, acted == 0
//	refused          swept == 0, acted == 0
//	could-not-check  swept == -        (the sweep did not complete: its count is unknown)
//
// The published regexp is kept as ONE string, so `regexp` prints exactly what `validate` applies.
// While the script is kept as the oracle, grammar_test.go pins that this string equals the one the
// script prints.

import (
	"fmt"
	"regexp"
	"strings"
)

// grammarVersion is the grammar's own version (the script's VERSION).
const grammarVersion = "1"

// roles and outcomes are the closed sets, in the reference's order.
var (
	roles    = []string{"the-desk", "intake-desk", "worker-desk", "pr-review-desk", "verify-desk"}
	outcomes = []string{"ok", "noop", "refused", "could-not-check"}
)

// count is a non-negative integer OR the literal `-` (unknown). duration is always an integer: a
// pass that printed the line necessarily knows how long it ran.
const count = `([0-9]+|-)`

// Grammar is the published extended regular expression — byte-identical to `tick-summary.sh
// regexp`. It is valid as both POSIX ERE and RE2, so the same text a shell consumer greps with is
// the text this verb compiles.
var Grammar = "^tick role=(" + strings.Join(roles, "|") + ") outcome=(" + strings.Join(outcomes, "|") +
	") swept=" + count + " acted=" + count + " filed=" + count + " duration=[0-9]+$"

var grammarRe = regexp.MustCompile(Grammar)

// ValidationError is a line that does not satisfy the grammar. Reason is the first rule it broke;
// Detail, when set, is the expected-shape help the script prints after a shape failure.
type ValidationError struct {
	Reason string
	Detail []string
}

func (e *ValidationError) Error() string { return e.Reason }

// field returns the value of the first `key=value` token on the line (single-space separated), or "".
func field(line, key string) string {
	for _, tok := range strings.Split(line, " ") {
		if v, ok := strings.CutPrefix(tok, key+"="); ok {
			return v
		}
	}
	return ""
}

// Validate applies the whole grammar to one line: shape first, then the cross-field rules. It
// returns the first rule the line broke, so a producer gets a diagnosis rather than a bare refusal.
func Validate(line string) error {
	if strings.Contains(line, "\t") {
		return &ValidationError{Reason: "the line contains a tab; fields are single-space separated"}
	}
	if strings.ContainsAny(line, "\r\n") {
		return &ValidationError{Reason: "the line contains a line break; the summary is ONE line"}
	}
	if !grammarRe.MatchString(line) {
		return &ValidationError{
			Reason: "does not match the published grammar: " + line,
			Detail: []string{
				"  expected: tick role=<role> outcome=<outcome> swept=<n|-> acted=<n|-> filed=<n|-> duration=<s>",
				"  roles:    " + strings.Join(roles, " "),
				"  outcomes: " + strings.Join(outcomes, " "),
			},
		}
	}
	outcome, swept, acted := field(line, "outcome"), field(line, "swept"), field(line, "acted")
	switch outcome {
	case "could-not-check":
		// A pass whose sweep did not complete has no count to report; a number — 0 above all —
		// claims a measurement it never made.
		if swept != "-" {
			return &ValidationError{Reason: fmt.Sprintf("outcome=could-not-check requires swept=- (the sweep did not complete, so its count is unknown); got swept=%s", swept)}
		}
	case "noop":
		// noop is a positive claim that the queue was read and empty of actionable work.
		if swept == "-" {
			return &ValidationError{Reason: "outcome=noop requires a numeric swept (a blind pass is could-not-check, never noop); got swept=-"}
		}
		if acted != "0" {
			return &ValidationError{Reason: fmt.Sprintf("outcome=noop requires acted=0 (a pass that acted is ok); got acted=%s", acted)}
		}
	case "ok":
		if swept == "-" {
			return &ValidationError{Reason: "outcome=ok requires a numeric swept; got swept=-"}
		}
		if acted == "-" || acted == "0" {
			return &ValidationError{Reason: fmt.Sprintf("outcome=ok requires acted >= 1 (a pass that acted on nothing is noop); got acted=%s", acted)}
		}
	case "refused":
		// A refusal happens before the sweep: there is nothing unknown about it.
		if swept != "0" || acted != "0" {
			return &ValidationError{Reason: fmt.Sprintf("outcome=refused requires swept=0 and acted=0 (the pass declined before sweeping); got swept=%s acted=%s", swept, acted)}
		}
	}
	return nil
}

// errEmptyInput is `check` over no input at all: no summary line, so the pass did not complete.
var errEmptyInput = &ValidationError{Reason: "empty input — no summary line, so the pass did not complete"}

// Check validates the LAST non-blank line of a pass's output. Anything printed after the summary
// line means the pass kept printing, which the contract forbids — so it is the last line or nothing.
func Check(input string) error {
	trimmed := strings.TrimRight(input, "\n")
	if trimmed == "" {
		return errEmptyInput
	}
	last := ""
	for _, l := range strings.Split(trimmed, "\n") {
		if strings.TrimSpace(l) != "" {
			last = l
		}
	}
	return Validate(last)
}
