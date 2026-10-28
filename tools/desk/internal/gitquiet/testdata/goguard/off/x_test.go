package off

import (
	"os/exec"
	"testing"

	"github.com/medici-finance/assay/gitquiet/goquiet"
)

func TestBuild(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	_ = goquiet.Cmd(exec.Command("go", "build", ".")).Run()
}
