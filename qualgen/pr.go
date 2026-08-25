package main

import (
	"flag"
	"fmt"
	"io"
	"strings"
)

// runPR is the `pr <n>` mode stub — mode scaffolding only. It accepts the
// leading positional PR number, parses its flags, emits a `not yet implemented`
// NOTICE and exits 0. The per-PR risk features (spec §9.1) are a later brief.
func runPR(args []string, stdout, stderr io.Writer) int {
	// The PR number is a leading positional before any flags (`pr 1 --out x`).
	var prNum string
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		prNum = args[0]
		args = args[1:]
	}
	fs := flag.NewFlagSet("pr", flag.ContinueOnError)
	fs.SetOutput(stderr)
	_ = fs.String("out", "", "tracking root to read artifacts from")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	_ = prNum
	fmt.Fprintln(stdout, "NOTICE: qualgen pr is not yet implemented — mode scaffolding only, filled by a later quality-stream brief")
	return 0
}
