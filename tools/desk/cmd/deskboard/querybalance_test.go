package main

import "testing"

// balancedDelimiters — see the deskflip copy. Duplicated per package because
// each command owns its own GraphQL query constant and is a separate `main`.
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

// TestOpenPRsGraphQLBalanced guards the board's open-PR read constant against
// the same never-executed-string brace-typo class as flipPRGraphQL.
func TestOpenPRsGraphQLBalanced(t *testing.T) {
	if msg, ok := balancedDelimiters(openPRsGraphQL); !ok {
		t.Fatalf("openPRsGraphQL has unbalanced delimiters: %s\nquery: %s", msg, openPRsGraphQL)
	}
}
