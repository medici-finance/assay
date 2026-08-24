package main

import (
	"flag"
	"fmt"
	"io"
)

// runReport is the `report` mode stub — mode scaffolding only. It recognizes the
// mode and parses its flags, then emits a `not yet implemented` NOTICE and exits
// 0. The trend view + trend JSONL (spec §9.3) are filled by a later wave-1 brief.
func runReport(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("report", flag.ContinueOnError)
	fs.SetOutput(stderr)
	_ = fs.String("out", "", "tracking root to read artifacts from")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	fmt.Fprintln(stdout, "NOTICE: qualgen report is not yet implemented — mode scaffolding only, filled by a later quality-stream brief")
	return 0
}
