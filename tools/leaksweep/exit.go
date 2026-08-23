package main

// Exit codes are a three-state instrument, not a two-state one
// (docs/three-state-instrument-rule.md). A sweep that answers only
// "clean / not clean" collapses two opposite situations into one number, and
// one of them — "I could not look" — is the exact state this tool exists to
// stop being reported as "clean":
//
//	0  CLEAN           — every token entry is checked-clean: its control was
//	                     found in the tree AND the token was absent. This is the
//	                     ONLY state that certifies the tree for the one-way copy.
//	2  LEAK            — at least one entry is checked-failed: a withheld token is
//	                     present in the tree. Nothing is certified.
//	3  COULD-NOT-CHECK — at least one entry could not be examined: its control was
//	                     absent (so a zero token count proves nothing), a pattern
//	                     errored, a file was unreadable, or the tree is missing.
//	                     A clean-looking result that was never actually checked.
//	1  ERROR           — anything else (bad flags, unparseable token file).
//
// The distinction that matters is 0 vs 3. A two-state tool reports could-not-check
// as clean, which is precisely how an unexamined tree becomes a signed-off one —
// the defect recorded against the word-boundary grep in this brief's own hazard
// table.

import "fmt"

const (
	exitClean         = 0
	exitError         = 1
	exitLeak          = 2
	exitCouldNotCheck = 3
)

type exitErr struct {
	code int
	err  error
}

func (e *exitErr) Error() string { return e.err.Error() }
func (e *exitErr) Code() int     { return e.code }

func withExit(code int, format string, args ...any) error {
	return &exitErr{code: code, err: fmt.Errorf(format, args...)}
}

func codeOf(err error) int {
	if e, ok := err.(*exitErr); ok {
		return e.code
	}
	return exitError
}
