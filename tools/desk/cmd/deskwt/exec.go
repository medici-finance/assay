package main

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

// execCommand is the single seam through which every git invocation flows. Production
// binds it to exec.Command; tests wrap it to RECORD every argv (so the "no --force in
// any git argv" assertion is checked on the real constructed argv) while still
// delegating to a real git process against a scratch fixture. Nothing else in this
// package constructs commands, so there is exactly one place argv is built.
var execCommand = exec.Command

// runGit executes `git <args...>` in dir and returns trimmed stdout. The argv is
// ALWAYS an explicit slice built from literal verbs plus values that have already been
// regex-validated (name/branch/base) or derived from git state — never a shell string
// and never a raw caller flag ("constructed argv only, no caller flag passthrough").
// A `--force` can therefore never reach git through this path.
func runGit(dir string, args ...string) (string, error) {
	cmd := execCommand("git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	err := cmd.Run()
	stdout := strings.TrimSpace(out.String())
	if err != nil {
		return stdout, fmt.Errorf("git %s: %w (%s)", strings.Join(args, " "),
			err, strings.TrimSpace(errb.String()))
	}
	return stdout, nil
}
