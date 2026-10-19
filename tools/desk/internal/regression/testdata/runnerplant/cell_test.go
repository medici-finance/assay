package runnerplant

import (
	"os"
	"strings"
	"testing"
)

// TestRunnerCellPlant is the planted row TestFloorRunnerCellEnv hands the floor
// runner. Like the cellctl rows the runner drives, it reads the CELLCTL_* environment
// it inherits, and fails when any such variable reaches it. Only the variable's name
// is reported: CELLCTL_GH_AMBIENT carries a credential.
//
// regression: #1631
func TestRunnerCellPlant(t *testing.T) {
	for _, kv := range os.Environ() {
		if key, _, _ := strings.Cut(kv, "="); strings.HasPrefix(key, "CELLCTL_") {
			t.Errorf("floor row inherited %s from the runner's caller", key)
		}
	}
}
