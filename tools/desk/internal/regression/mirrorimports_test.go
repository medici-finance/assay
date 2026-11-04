package regression

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/medici-finance/assay/gitquiet/goquiet"
)

// TestMirrorImportRows gives the standalone source checker the same always-on
// CI entrypoint as the other cross-module regression suites in this package.
// regression: #2743
func TestMirrorImportRows(t *testing.T) {
	cmd := exec.Command("go", "test", "-count=1", "-timeout=90s", "-v",
		"check_env_imports.go", "check_env_imports_test.go")
	cmd.Dir = "../../../../tools/forge-import-check"
	cmd.Env = FixtureEnv("GOWORK=off", "GOPROXY=off", "GOTOOLCHAIN=local", "GOSUMDB=off")
	if output, err := goquiet.Cmd(cmd).CombinedOutput(); err != nil {
		t.Fatalf("mirror/event import row fixtures: %v\n%s", err, output)
	}

	// Invoke the actual new-package suites from this existing unconditional CI
	// entrypoint: the generic module sweep otherwise builds/vets forgeread only.
	cmd = exec.Command("go", "test", "-count=1", "-timeout=90s", "-v", "-run",
		"^Test(ForgeServiceContractCompatibility|ForgeServiceClientFlow|ServicePackageBoundaries|ReviewServiceMapping)$", "./service/...", "./client/...")
	cmd.Dir = "../../../../forgeread"
	cmd.Env = FixtureEnv("GOWORK=off", "GOPROXY=off", "GOTOOLCHAIN=local", "GOSUMDB=off")
	output, err := goquiet.Cmd(cmd).CombinedOutput()
	if err != nil {
		t.Fatalf("service/client named suites: %v\n%s", err, output)
	}
	for _, name := range []string{"TestForgeServiceContractCompatibility", "TestForgeServiceClientFlow", "TestServicePackageBoundaries", "TestReviewServiceMapping"} {
		if !strings.Contains(string(output), "--- PASS: "+name+" ") {
			t.Fatalf("missing named service/client PASS %s: %s", name, output)
		}
	}
}

// regression: #5399
func TestRefreshImportBoundary(t *testing.T) {
	cmd := exec.Command("go", "run", "tools/forge-import-check/check_env_imports.go", "forgeread/refresh")
	cmd.Dir = "../../../.."
	cmd.Env = FixtureEnv("GOWORK=off", "GOPROXY=off", "GOTOOLCHAIN=local", "GOSUMDB=off")
	out, err := goquiet.Cmd(cmd).CombinedOutput()
	if err != nil || string(out) != "0\n" {
		t.Fatalf("actual refresh source boundary: %v %s", err, out)
	}
}
