// Package fixture is the #2254 merge-hold choke-point guard's committed POSITIVE CONTROL — a
// planted verb that opens a change through the raw CreateDraftChange seam instead of
// deskkit.CreateHeldDraftChange, so the change carries no merge-hold marker thread.
// TestDraftChangeOnlyViaHeld must flag this file; if it stops flagging it, the guard's matcher
// has gone blind and the test fails on that basis, never a silent green.
//
// This file is never compiled: Go tooling skips any directory named "testdata", so it exists
// purely as scan input for the guard's walker.
package fixture

type forge interface {
	CreateDraftChange(repo string, in string) (int, error)
}

// openUnheldChange is the planted violation: it opens a change and never opens the hold.
func openUnheldChange(fg forge) (int, error) {
	return fg.CreateDraftChange("example-org/tracker", "side-branch")
}

// openViaMethodValue is the second planted violation: it reaches the raw seam through a method
// value rather than a direct call, which a call-only matcher would miss.
func openViaMethodValue(fg forge) (int, error) {
	create := fg.CreateDraftChange
	return create("example-org/tracker", "side-branch")
}
