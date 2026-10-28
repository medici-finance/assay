package treated

import (
	"os/exec"
	"testing"

	"github.com/medici-finance/assay/gitquiet/goquiet"
)

func TestBuild(t *testing.T) {
	t.Setenv("HOME", goquiet.Home(t))
	_ = exec.Command("go", "build", ".").Run()
}
