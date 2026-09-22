package main

// realexec.go — the production process seam.

import (
	"fmt"
	"os/exec"
	"strings"
)

// RealExec runs the reconcile's declared toolset for real. Like scanloop's lane executor
// it dispatches to a LITERAL-argv exec.Command per binary rather than
// exec.Command(name, …) with a variable, so the forge-CLI ban resolves every launch site
// to a compile-time constant and the toolset is closed by construction: an unknown name is
// refused, never launched. statusgen is the pinned binary resolved from PATH under its bare
// name, exactly as scanloop's executor resolves it — the in-repo tools/statusgen copy is
// frozen and never run. Tests inject a fake statusgen through the Exec seam, not here.
func RealExec(dir, name string, args ...string) (string, error) {
	var cmd *exec.Cmd
	switch name {
	case "git":
		cmd = exec.Command("git", args...)
	case "statusgen":
		cmd = exec.Command("statusgen", args...)
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
