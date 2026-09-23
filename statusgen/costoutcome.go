package main

// costoutcome.go — the two brief-v1 cost and outcome lines (spec/brief-v1.md §3.2):
//
//   - `budget:` sits beside `effort:` and states what the brief is sized to
//     SPEND, with its unit: `budget: 400k tokens` or `budget: 25 USD`. A number
//     without a unit is a PROBLEM — "400" of what is the question a budget exists
//     to answer. An absent budget means no budget checkpoint applies; it is
//     never flagged.
//   - `outcome:` names what the brief is meant to MOVE: the id of a requirement
//     in the requirement register (registers-v1 §6), or the literal `none` for
//     work that moves no registered outcome. The vocabulary is exactly the set of
//     ids the register defines — this file adds no second vocabulary. A value
//     that is neither `none` nor a requirement reference is a PROBLEM, and so is a
//     well-formed in-repo id the register does not define (the register is
//     append-only, so an undefined in-repo id is a typo or a deleted entry). A
//     cross-repo `<alias>:REQ-<slug>` names a register this offline check cannot
//     read: could-not-check, a NOTICE, never a pass and never a PROBLEM.
//
// Both keys ride the EXISTING brief-v1 schema, for the reason `satisfies:` does:
// brief-schema evolution fails closed, so a new schema value for two optional keys
// would refuse the whole tree on every consumer that had not upgraded yet.
//
// Pure over the tree: no network, no git, the same offline envelope as the rest
// of --lint.

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// outcomeNone is the explicit "this brief moves no registered outcome" value.
// Writing it is a decision; leaving the key off is not, which is why the
// post-cutover absence NOTICE (lifecyclelint.go) is silenced by `none` too.
const outcomeNone = "none"

// budgetRe is the budget grammar: a positive amount, an optional k/M multiplier,
// whitespace, and a unit — the word `tokens`, or an ISO-4217-shaped currency code
// (three upper-case letters). The code is shape-checked, not looked up: a linter
// that shipped a currency table would be wrong the first time one changed.
var budgetRe = regexp.MustCompile(`^([0-9]+(?:\.[0-9]+)?)([kKmM]?)\s+(tokens|[A-Z]{3})$`)

// budgetProblem returns "" when raw is a well-formed budget, else the reason it
// is not, echoing the value so the author can see what the linter read.
func budgetProblem(raw string) string {
	v := strings.TrimSpace(raw)
	if v == "" {
		return `is empty — write the amount with its unit, e.g. "400k tokens" or "25 USD", or drop the key`
	}
	m := budgetRe.FindStringSubmatch(v)
	if m == nil {
		if _, err := strconv.ParseFloat(strings.TrimRight(v, "kKmM"), 64); err == nil {
			return fmt.Sprintf("%q has no unit — a budget states what it is counted in, e.g. \"%s tokens\" or \"%s USD\"", raw, v, v)
		}
		return fmt.Sprintf("%q is not a budget — want <amount>[k|M] <unit>, the unit being `tokens` or a three-letter currency code (e.g. \"400k tokens\", \"25 USD\")", raw)
	}
	if n, err := strconv.ParseFloat(m[1], 64); err != nil || n <= 0 {
		return fmt.Sprintf("%q must be a positive amount", raw)
	}
	return ""
}

// outcomeRegister is the set of in-repo requirement ids an `outcome:` may name,
// read once per root. Unreadable is kept apart from empty (the three-state rule):
// an unreadable register is could-not-check for every in-repo outcome, never a
// wall of false "unknown id" PROBLEMs.
type outcomeRegister struct {
	IDs        map[string]bool
	Unreadable error
}

func loadOutcomeRegister(root string) outcomeRegister {
	entries, err := parseRequirementsDir(root)
	if err != nil {
		return outcomeRegister{Unreadable: err}
	}
	ids := map[string]bool{}
	for _, e := range entries {
		if id := strings.TrimSpace(e.ID); id != "" {
			ids[id] = true
		}
	}
	return outcomeRegister{IDs: ids}
}

// outcomeChecks validates one `outcome:` value. problems are hard PROBLEMs
// (unknown or malformed); notices are could-not-check lines. Both are phrased to
// follow "outcome " in the caller's message.
func outcomeChecks(raw string, graph *graphRepos, reg outcomeRegister) (problems, notices []string) {
	v := strings.TrimSpace(raw)
	if v == outcomeNone {
		return nil, nil
	}
	if v == "" {
		return []string{`is empty — name the requirement id (REQ-<slug>) this brief should move, or write "outcome: none"`}, nil
	}
	if ok, reason := validRequirementRef(v, graph); !ok {
		if requirementRefCrossRepoRe.MatchString(v) {
			return []string{reason}, nil // well-shaped cross-repo ref, unknown alias
		}
		return []string{fmt.Sprintf("%q is not a registered outcome — want a requirement id from docs/streams/requirements/ (REQ-<slug>, or <alias>:REQ-<slug> cross-repo), or none", raw)}, nil
	}
	id, cross := requirementRefKind(v)
	if cross {
		return nil, []string{fmt.Sprintf("%s names a requirement in another repo's register — could-not-check offline (not verified to exist)", v)}
	}
	if reg.Unreadable != nil {
		return nil, []string{fmt.Sprintf("%s: could-not-check — the requirement register is unreadable (%v)", v, reg.Unreadable)}
	}
	if !reg.IDs[id] {
		return []string{fmt.Sprintf("names %s, which no requirement in docs/streams/requirements/ defines — an unknown outcome id (a typo, or a requirement never registered)", id)}, nil
	}
	return nil, nil
}
