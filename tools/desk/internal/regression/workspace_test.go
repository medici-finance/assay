package regression

import (
	"os/exec"
	"testing"

	"github.com/medici-finance/assay/gitquiet/goquiet"
)

// TestWorkspaceModuleSuite runs the shared workspace module's own suite from the desk
// suite's always-on CI entrypoint. ci.yml only builds and vets sibling modules, so
// without this the contract fixtures, the schema coverage and the real-tree caller
// census (which must go red when a new checkout creator or remover lands anywhere in
// the tree, not only under workspace/) would run nowhere.
// regression: F-cell-workspace-census
func TestWorkspaceModuleSuite(t *testing.T) {
	cmd := exec.Command("go", "test", "-count=1", "-timeout=120s", "./...")
	cmd.Dir = "../../../../workspace"
	cmd.Env = FixtureEnv("GOWORK=off", "GOPROXY=off", "GOTOOLCHAIN=local", "GOSUMDB=off")
	if output, err := goquiet.Cmd(cmd).CombinedOutput(); err != nil {
		t.Fatalf("workspace module suite: %v\n%s", err, output)
	}
}
