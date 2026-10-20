package main

import (
	"os"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/gitquiet"
)

// TestMain stops git's automatic background maintenance in every repository these tests
// create, so a detached repack cannot race t.TempDir's cleanup (see package gitquiet).
// This package creates no repository (its stub `git` on PATH is the only git it runs), but
// the class guard treats every package whose tests run git the same way.
func TestMain(m *testing.M) { os.Exit(gitquiet.Run(m)) }
