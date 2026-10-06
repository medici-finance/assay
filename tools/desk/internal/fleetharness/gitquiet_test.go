package fleetharness

import (
	"os"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/gitquiet"
)

// TestMain stops git's automatic background maintenance in every repository these tests
// create, so a detached repack cannot race t.TempDir's cleanup (see package gitquiet).
func TestMain(m *testing.M) { os.Exit(gitquiet.Run(m)) }
