package main

import (
	"os"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/cachequiet"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"github.com/medici-finance/assay/tools/desk/internal/gitquiet"
)

// TestMain stops git's automatic background maintenance in every repository these tests
// create, so a detached repack cannot race t.TempDir's cleanup (see package gitquiet). It
// first clears the cell's storage-admission variables, so no test reaches the operator's
// live Go-cache root through an inherited policy (see package cachequiet; #2491).
//
// It also forces the full effective-config echo: some tests here run the built cellctl
// binary with the operator's own HOME, and the default (digest) echo would record a last-seen
// digest in that config home. The forced form reads and writes no state.
func TestMain(m *testing.M) {
	cachequiet.Isolate()
	if err := os.Setenv(deskkit.EnvConfigEcho, "full"); err != nil {
		panic(err)
	}
	os.Exit(gitquiet.Run(m))
}
