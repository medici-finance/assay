//go:build hotfixseam

package main

// Driver for cmd/cellctl/testdata/hotfix-e2e.sh. It exists only in a build carrying the
// test-only hotfixseam tag and is skipped unless the script sets HOTFIX_E2E_BINARYPIN: it runs
// boot step 10 (binary-pin) exactly as boot() does, against the throwaway cell, overlay and
// repo the script built, and prints the step's output and the exit code deskboot would use.

import (
	"fmt"
	"os"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

func TestBinaryPinE2EDriver(t *testing.T) {
	root := os.Getenv("HOTFIX_E2E_BINARYPIN")
	if root == "" {
		t.Skip("driven by cmd/cellctl/testdata/hotfix-e2e.sh")
	}
	err := binaryPinStep(bootOpts{role: "worker-desk", root: root})
	if err != nil {
		fmt.Printf("deskboot: refused: %v\n", err)
		fmt.Printf("deskboot: exit=%d\n", deskkit.ExitCodeOf(err))
		return
	}
	fmt.Println("deskboot: exit=0")
}
