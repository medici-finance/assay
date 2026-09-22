package main

// realexec.go — the production process seam.

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// RealExec runs the reconcile's declared toolset for real. Like scanloop's lane executor
// it dispatches to a LITERAL-argv exec.Command per binary rather than
// exec.Command(name, …) with a variable, so the forge-CLI ban resolves every launch site
// to a compile-time constant and the toolset is closed by construction: an unknown name is
// refused, never launched. statusgen resolves through the pinned-binary env override the
// rest of the desk uses (STATUSGEN_BIN, else PATH) — never the frozen in-repo tree.
func RealExec(dir, name string, args ...string) (string, error) {
	var cmd *exec.Cmd
	switch name {
	case "git":
		cmd = exec.Command("git", args...)
	case "statusgen":
		bin := strings.TrimSpace(os.Getenv(deskkit.StatusgenBinEnv))
		if bin == "" {
			bin = "statusgen"
		}
		cmd = exec.Command(bin, args...)
	case "deskpr":
		cmd = exec.Command("deskpr", args...)
	default:
		return "", fmt.Errorf("deskreconcile: refusing to launch %q — the reconcile executor launches only its declared toolset (git, statusgen, deskpr)", name)
	}
	cmd.Dir = dir
	b, err := cmd.CombinedOutput()
	if err != nil {
		return string(b), fmt.Errorf("%s %s: %w: %s", name, strings.Join(args, " "), err, strings.TrimSpace(string(b)))
	}
	return string(b), nil
}
