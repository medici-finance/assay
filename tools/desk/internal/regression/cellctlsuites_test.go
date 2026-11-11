package regression

import (
	"path"
	"path/filepath"
	"strings"
	"testing"
)

// cellctlSuiteFloor runs one cellctl shell suite against the shell oracle (the suite's default
// CELLCTL), from a throwaway working directory, and requires the suite's own "<name>: OK" line.
// No workflow ran harness.test.sh or provider.test.sh, so their expectations drifted from the
// oracle and stayed red on a development host unnoticed (#6175). The working directory is a
// fresh t.TempDir(), never the package directory: a suite whose scratch-directory setup fails
// must not be able to delete the tree it runs from.
func cellctlSuiteFloor(t *testing.T, relative string) {
	t.Helper()
	skipWindowsShellFixture(t)
	bash := requireBash(t)
	root, err := filepath.Abs("../../../..")
	if err != nil {
		t.Fatal(err)
	}
	suite := filepath.Join(root, filepath.FromSlash(relative))
	t.Chdir(t.TempDir())
	out, err := runShellFixtureWithBash(t, bash, suite, t.TempDir(), shellBudget)
	if want := "\n" + path.Base(relative) + ": OK\n"; err != nil || !strings.Contains(string(out), want) {
		t.Fatalf("fixture suite %s: %v\n%s", relative, err, out)
	}
	t.Log(string(out))
}

// TestReg6175HarnessSuite holds harness.test.sh to the oracle's `check` rows, which read the
// Codex multi_agent state from `codex features list` since ea4f9842d.
//
// regression: #6175
func TestReg6175HarnessSuite(t *testing.T) {
	cellctlSuiteFloor(t, "tools/cellctl/tests/harness.test.sh")
}

// TestReg6175ProviderSuite holds provider.test.sh to the oracle. The suite's "no provider" rows
// must hold on a host whose own shell already runs claude against a provider.
//
// regression: #6175
func TestReg6175ProviderSuite(t *testing.T) {
	cellctlSuiteFloor(t, "tools/cellctl/tests/provider.test.sh")
}
