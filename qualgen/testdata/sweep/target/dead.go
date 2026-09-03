package target

import "fmt"

// Used is called from main and is genuinely live.
func Used() string {
	return fmt.Sprintf("live-%d", 1)
}

// unusedHelper is never referenced anywhere in the tree — the planted dead-code
// suspect. A staticcheck-class U1000 flags it.
func unusedHelper(x int) int {
	return x * 2
}
