package target

import "strconv"

// ParseLoose swallows the error from Atoi — the planted swallowed-error
// suspect. An errcheck-class linter flags the unchecked return on line 8.
func ParseLoose(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}
