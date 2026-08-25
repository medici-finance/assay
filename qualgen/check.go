package main

import (
	"flag"
	"fmt"
	"io"
	"strings"
)

// runCheck is the `check <paths>` mode stub — mode scaffolding only. It accepts
// leading positional paths, parses its flags, emits a `not yet implemented`
// NOTICE and exits 0. The brittleness screen (spec §9.2) is a later brief.
func runCheck(args []string, stdout, stderr io.Writer) int {
	// Leading positionals are the file set to screen (`check a.go b.go --out x`).
	var paths []string
	for len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		paths = append(paths, args[0])
		args = args[1:]
	}
	fs := flag.NewFlagSet("check", flag.ContinueOnError)
	fs.SetOutput(stderr)
	_ = fs.String("out", "", "tracking root to read artifacts from")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	_ = paths
	fmt.Fprintln(stdout, "NOTICE: qualgen check is not yet implemented — mode scaffolding only, filled by a later quality-stream brief")
	return 0
}
