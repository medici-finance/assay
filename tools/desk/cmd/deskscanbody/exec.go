package main

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// execCommand is the single seam through which every git invocation flows. Production
// binds it to exec.Command; tests replace it so the argv this tool constructs is asserted
// directly rather than inferred from output.
var execCommand = exec.Command

// gitOut runs git with an explicit argv — never a shell string, and never a
// caller-supplied git flag: --base and --dir land in FIXED argv positions after the `--`
// or as an operand, so no external value can inject an option.
//
// Stderr is stripped through deskkit.StripControl before it reaches an error message:
// git echoes ref and path names, and this tool's output is read in terminals and agent
// context.
func gitOut(args ...string) (string, error) {
	cmd := execCommand("git", args...)
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git %s: %w (%s)", strings.Join(args, " "), err,
			deskkit.StripControl(strings.TrimSpace(errb.String())))
	}
	return strings.TrimSpace(out.String()), nil
}
