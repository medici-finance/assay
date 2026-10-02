package main

import _ "embed"

// usageText is the help this binary prints, and it is a COPY of the shell oracle's header
// comment — the text `usage()` there produces with
//
//	awk 'NR>1 && /^set -euo/ {exit} NR>1 {print}' "$SELF"
//
// The oracle can read its own source; a compiled binary cannot, so the text is embedded. What
// keeps the copy honest is usage_test.go, which re-derives the header from
// tools/cellctl/testdata/cellctl-shell-oracle.sh and fails when the two drift — the same "one
// source, proven equal" shape the parity harness applies to the rest of the surface.
//
//go:embed usage.txt
var usageText string

// cadenceUsage describes the Go-only supervisor; it is not a shell oracle verb.
const cadenceUsage = `
# Host role cadence (Go supervisor, independent of cockpit/harness wake tools):
#   cellctl up <cell> --cockpit herdr --harness codex --cadence 30m --tick-budget 20m
#   cellctl up <cell> --cockpit orca --harness cursor --cadence 30m --tick-budget 20m
#   cellctl desk <cell> <role> --cadence 30m [--tick-budget 20m]
#   cellctl cadence <cell> status|stop|resume [role]
#   cellctl cadence <cell> recover <role> --confirm-stopped
#   cellctl --cells-root <absolute-path> <command> ...
# CELL_CADENCE / CELL_TICK_BUDGET persist the same settings; cadence off disables it.
# The foreground Go process survives model turns. Restart resumes its checkpoint;
# an unfinished prior child refuses until inspected and explicitly recovered.
# No OS startup service is installed. Keep/restart the cockpit supervisor after reboot.
`
