package main

import (
	"os"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/gitquiet"
)

// TestMain stops git's automatic background maintenance in every repository these tests
// create, so a detached repack cannot race t.TempDir's cleanup (see package gitquiet),
// and removes the shared tool binary's temp dir once every test has run.
func TestMain(m *testing.M) {
	code := gitquiet.Run(m)
	if binDir != "" {
		_ = os.RemoveAll(binDir)
	}
	os.Exit(code)
}
