package testledger_test

import (
	"os"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/gitquiet"
)

// TestMain stops git's automatic background maintenance in every repository these tests
// touch, so a detached repack cannot race a test's cleanup (see package gitquiet).
func TestMain(m *testing.M) { os.Exit(gitquiet.Run(m)) }
