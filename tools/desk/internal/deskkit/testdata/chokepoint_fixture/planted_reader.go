// Package fixture is the #882 choke-point guard's committed POSITIVE CONTROL — a planted
// direct reader of the verify-outcomes path, outside the allow-list. TestVerifyOutcomesSingleReader
// must flag this file; if it stops flagging it, the guard's own matcher has gone blind and the
// test itself fails on that basis, never a silent green.
//
// This file is never compiled: Go tooling (build, vet, test) skips any directory named
// "testdata" by convention, so it exists purely as scan input for the guard's walker.
package fixture

import "os"

// readSidecarDirectly is the planted violation: a reader that opens the verify-outcomes path
// itself instead of going through the module's one allow-listed choke point.
func readSidecarDirectly() ([]byte, error) {
	return os.ReadFile("docs/streams/verify-outcomes.jsonl")
}
