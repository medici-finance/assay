package gittest

import (
	"os"
	"testing"

	"github.com/medici-finance/assay/gitquiet"
)

// TestMain stops git's automatic background maintenance in every repository these tests
// create, so a detached repack cannot race t.TempDir's cleanup (see package gitquiet).
func TestMain(m *testing.M) {
	if path := os.Getenv("GITTEST_SIGN_CANARY"); path != "" {
		_ = os.WriteFile(path, []byte("signing invoked\n"), 0o600)
		os.Exit(1)
	}
	os.Exit(gitquiet.Run(m))
}
