package main

import "testing"

// balancedDelimiters reports the first imbalance in s across (), {} and [],
// or ("", true) when every opener is matched by the correct closer in order.
// GraphQL query documents are brace-structured; an unbalanced or mis-nested
// delimiter makes `gh api graphql` reject the whole document at parse time
// (e.g. `actual: RCURLY ("}")`), which fails every read that uses the query.
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

// TestFlipPRGraphQLBalanced catches the class of defect that shipped a
// 13-open / 14-close `flipPRGraphQL` and took deskflip down fleet-wide: the
// query constant is never executed by the gh-stubbing unit suite, so a brace
// typo passed every test. This runs a hermetic structural check on the
// constant itself so the imbalance is caught at test time.
func TestFlipPRGraphQLBalanced(t *testing.T) {
	if msg, ok := balancedDelimiters(flipPRGraphQL); !ok {
		t.Fatalf("flipPRGraphQL has unbalanced delimiters: %s\nquery: %s", msg, flipPRGraphQL)
	}
}
