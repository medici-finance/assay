package main

import (
	"github.com/medici-finance/assay/deskd/internal/humanid"
	"os"
	"testing"
)

// Existing synthetic fixtures run under an explicit process-start policy.
// The run is bracketed by treeLeakGuard (tree_test.go): a test that leaves a
// deskd state file in the tree fails the package.
//
// regression: #5863
func TestMain(m *testing.M) {
	checkTree := treeLeakGuard(os.Stderr)
	if os.Getenv("DESKD_TEST_POLICY_DISABLED") != "1" {
		login := os.Getenv("DESKD_TEST_POLICY_LOGIN")
		if login == "" {
			login = "operator"
		}
		if err := humanid.Configure(login); err != nil {
			panic(err)
		}
	}
	os.Exit(checkTree(m.Run()))
}
