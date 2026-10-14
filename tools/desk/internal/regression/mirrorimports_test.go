package regression

import (
	"os"
	"os/exec"
	"testing"
)

// TestMirrorImportRows gives the standalone source checker the same always-on
// CI entrypoint as the other cross-module regression suites in this package.
// regression: #2743
func TestMirrorImportRows(t *testing.T) {
	cmd := exec.Command("go", "test", "-count=1", "-timeout=90s", "-v",
		"check_env_imports.go", "check_env_imports_test.go")
	cmd.Dir = "../../../../tools/forge-import-check"
	cmd.Env = append(os.Environ(), "GOWORK=off", "GOPROXY=off", "GOTOOLCHAIN=local", "GOSUMDB=off")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("mirror/event import row fixtures: %v\n%s", err, output)
	}
}
