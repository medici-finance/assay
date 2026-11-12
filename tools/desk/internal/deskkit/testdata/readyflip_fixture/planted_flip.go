// Package fixture is the #6242 backstop guard's committed POSITIVE CONTROL: a planted ready
// flip with no post-flip backstop. TestReadyFlipBackstopGuard must flag this file; if it stops
// flagging it, the guard's matcher has gone blind and the test fails on that basis.
//
// This file is never compiled: Go tooling skips any directory named "testdata", so it exists
// purely as scan input for the guard's walker.
package fixture

type flipper interface{ MarkReadyForReview(nodeID string) error }

// flipWithoutBackstop is the planted violation: it flips a change ready and reports success
// without asking the forge whether the change is mergeable.
func flipWithoutBackstop(fg flipper, nodeID string) error {
	return fg.MarkReadyForReview(nodeID)
}
