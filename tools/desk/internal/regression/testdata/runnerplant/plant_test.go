// Package runnerplant is the planted row TestFloorRunnerGitIsolation hands the floor
// runner. Like the reused rows the runner drives, it runs git with the environment
// it inherits, so the runner's own scrub is the only thing keeping its writes in its
// temporary repository. It sits under testdata, so `go test ./...` never selects it;
// only the control's one-row manifest names it.
package runnerplant

import (
	"os/exec"
	"testing"
)

func TestRunnerPlant(t *testing.T) {
	dir := t.TempDir()
	for _, args := range [][]string{
		{"init", "-q", dir},
		{"-C", dir, "config", "user.name", "Plant"},
	} {
		if out, err := exec.Command("git", args...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
}
