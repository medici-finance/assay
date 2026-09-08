package deskkit

import (
	"strings"
	"testing"
)

// TestOpenChangesQueryOmitsActionsReadFields is the regression guard the deskboard read carried
// over when fetchOpenPRs migrated onto ListOpenChanges: the bulk open-PR read must NOT request
// the `checkSuite { workflowRun … }` sub-selection gh's built-in statusCheckRollup field
// hardcodes — it is a LINK to the Actions run and requires `actions:read`, which under an App
// holding only `checks:read` 403s and, on a PR-heavy repo, sinks the whole read. Requesting the
// rollup CONTEXTS without that sub-field keeps every conclusion the board reads covered by
// `checks:read` alone.
func TestOpenChangesQueryOmitsActionsReadFields(t *testing.T) {
	for _, bad := range []string{"workflowRun", "checkSuite"} {
		if strings.Contains(ghOpenChangesQuery, bad) {
			t.Errorf("ghOpenChangesQuery requests %q — that needs actions:read, which the board is not "+
				"guaranteed; request the rollup contexts without it\nquery: %s", bad, ghOpenChangesQuery)
		}
	}
	// The enumeration must ask ONLY for OPEN changes — a states-less or merged-inclusive query
	// would feed the board closed/merged PRs it never means to classify.
	if !strings.Contains(ghOpenChangesQuery, "pullRequests(states:OPEN") {
		t.Errorf("ghOpenChangesQuery must enumerate pullRequests(states:OPEN …)\nquery: %s", ghOpenChangesQuery)
	}
}

// balancedDelimiters reports the first delimiter-balance fault in s, if any. A GraphQL query
// held as a never-executed Go string is one brace typo away from a query the server rejects at
// runtime with nothing catching it in review; this guards the hand-authored read constants
// this backend ships.
func balancedDelimiters(s string) (string, bool) {
	pairs := map[rune]rune{')': '(', '}': '{', ']': '['}
	var stack []rune
	for _, r := range s {
		switch r {
		case '(', '{', '[':
			stack = append(stack, r)
		case ')', '}', ']':
			if len(stack) == 0 {
				return "unexpected closing '" + string(r) + "' with no opener", false
			}
			if stack[len(stack)-1] != pairs[r] {
				return "closing '" + string(r) + "' does not match opener '" + string(stack[len(stack)-1]) + "'", false
			}
			stack = stack[:len(stack)-1]
		}
	}
	if len(stack) != 0 {
		return "unclosed opener '" + string(stack[len(stack)-1]) + "'", false
	}
	return "", true
}

// TestForgeGraphQLQueriesBalanced guards the backend's hand-authored GraphQL read constants
// against the brace-typo class — the board's bulk open-PR read (ghOpenChangesQuery, which
// carried this guard over from cmd/deskboard when fetchOpenPRs migrated onto ListOpenChanges)
// and the two trust-gate queries the typed trust ops run.
func TestForgeGraphQLQueriesBalanced(t *testing.T) {
	for name, q := range map[string]string{
		"ghOpenChangesQuery": ghOpenChangesQuery,
		"PRTrustQuery":       PRTrustQuery,
		"IssueTrustQuery":    IssueTrustQuery,
		"ghCommentsQuery":    ghCommentsQuery,
	} {
		if msg, ok := balancedDelimiters(q); !ok {
			t.Errorf("%s has unbalanced delimiters: %s\nquery: %s", name, msg, q)
		}
	}
}
