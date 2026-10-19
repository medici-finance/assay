package main

import (
	"os"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/cachequiet"
	"github.com/medici-finance/assay/tools/desk/internal/gitquiet"
)

// TestMain stops git's automatic background maintenance in every repository these tests
// create, so a detached repack cannot race t.TempDir's cleanup (see package gitquiet). It
// first clears the cell's storage-admission variables, so no test reaches the operator's
// live Go-cache root through an inherited policy (see package cachequiet; #2491). A re-executed
// helper process (the admission fixture's claim tool or dispatcher) runs its role and exits first.
func TestMain(m *testing.M) {
	if code, ok := admitHelperMain(); ok {
		os.Exit(code)
	}
	cachequiet.Isolate()
	os.Exit(gitquiet.Run(m))
}
