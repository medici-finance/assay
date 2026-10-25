//go:build unix

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// regression: #4405
// TestLaunchExportsDeskToolsBin: a cell whose DESK_TOOLS_BIN is set only in its own config
// (cell.env, never the launching shell) gets shims aimed at that directory AND a window whose
// environment names the same directory. deskboot's binary-pin step resolves the pinned
// directory from the window's environment, so the two must be one resolution: without the
// export the step reads the default directory and refuses every ordinary shim as aimed outside
// the pinned and hotfix dirs, in a cell that has no hotfix at all.
func TestLaunchExportsDeskToolsBin(t *testing.T) {
	f := newPolicyFixture(t, "2.1.278")
	f.prepareLocalLaunch(t)
	custom := filepath.Join(t.TempDir(), "custom", "bin")
	must(t, os.MkdirAll(custom, 0o755))
	must(t, os.WriteFile(filepath.Join(custom, "deskboard"), []byte("#!/bin/sh\nexit 0\n"), 0o755))
	script := "#!/bin/sh\ncase \"$1\" in\n--version) echo '2.1.278 (stub)'; exit 0;;\nplugin) exit 0;;\nesac\n" +
		"printf 'DESK_TOOLS_BIN=%s\\n' \"${DESK_TOOLS_BIN-}\"\nexit 0\n"
	must(t, os.WriteFile(filepath.Join(f.binDir, "claude"), []byte(script), 0o755))
	envPath := filepath.Join(f.cellDir, "cell.env")
	base, err := os.ReadFile(envPath)
	must(t, err)
	must(t, os.WriteFile(envPath, append(base, []byte("\nDESK_TOOLS_BIN="+custom+"\n")...), 0o600))

	// The fixture's run composes the child environment from scratch, so DESK_TOOLS_BIN is not
	// in the launching shell: the cell config is its only source.
	r := f.run(t, []string{"CELLCTL_DESKWT=0"}, "desk", "example", "worker-desk", "--cadence", "off")
	if r.code != 0 {
		t.Fatalf("launch exit=%d\nstdout=%s\nstderr=%s", r.code, r.stdout, r.stderr)
	}
	if !strings.Contains(r.stdout, deskkit.DeskToolsBinEnv+"="+custom+"\n") {
		t.Errorf("the launched window's %s is not the cell config's %s:\n%s", deskkit.DeskToolsBinEnv, custom, r.stdout)
	}
	shim, err := os.ReadFile(filepath.Join(f.cellDir, "shim", "deskboard"))
	must(t, err)
	if !strings.Contains(string(shim), `"`+filepath.Join(custom, "deskboard")+`" "$@"`) {
		t.Errorf("the shim does not exec the cell config's directory %s:\n%s", custom, shim)
	}
}
