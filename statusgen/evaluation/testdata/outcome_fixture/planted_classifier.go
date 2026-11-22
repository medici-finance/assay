package planted

// This file is the committed POSITIVE CONTROL for TestOutcomeProtocolClassGuard: it plants one
// instance of every shape the guard exists to catch, outside the protocol's owner. It is under
// testdata/, so it never compiles into any binary. If the guard stops flagging a rule here, the
// guard is broken, not clean.

import (
	"regexp"
	"strings"
)

// A private copy of the outcome vocabulary (rule: vocabulary-word).
func privateClassify(outcome string) int {
	switch strings.ToLower(strings.TrimSpace(outcome)) {
	case "verified":
		return 1
	case "verify-fail", "fail":
		return 2
	case "blocked", "needs_context", "needs-context":
		return 3
	}
	return 0
}

type row struct{ Outcome string }

// An exact-spelling closure comparison that a case variant walks past (rule: exact-verified).
func privateGuard(r row) bool {
	return r.Outcome != "verified"
}

// The same comparison with the operands reversed (rule: reversed-verified).
func privateGuardReversed(r row) bool {
	return "verified" != r.Outcome
}

// A case-folded copy of the comparison, outside the owner (rule: folded-verified).
func privateGuardFolded(o string) bool {
	return strings.EqualFold(o, "verified")
}

// A second record codec: the stream and number grammar of a brief key (rule: brief-key-grammar).
var (
	privateStreamRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)
	privateNumRe    = regexp.MustCompile(`^[0-9]+$`)
)

// A second row-result vocabulary (rule: row-result-vocabulary).
func privateValidRow(s string) bool {
	switch s {
	case "pass", "fail", "could-not-run", "deferred":
		return true
	}
	return false
}
