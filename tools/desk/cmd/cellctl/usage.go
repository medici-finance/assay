package main

import _ "embed"

// usageText is the help this binary prints, and it is a COPY of the shell oracle's header
// comment — the text `usage()` there produces with
//
//	awk 'NR>1 && /^set -euo/ {exit} NR>1 {print}' "$SELF"
//
// The oracle can read its own source; a compiled binary cannot, so the text is embedded. What
// keeps the copy honest is usage_test.go, which re-derives the header from
// tools/cellctl/cellctl and fails when the two drift — the same "one source, proven equal"
// shape the parity harness applies to the rest of the surface.
//
//go:embed usage.txt
var usageText string
