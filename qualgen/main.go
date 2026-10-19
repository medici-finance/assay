// Command qualgen is a repo-agnostic git-history quality miner — a
// statusgen-sibling Go binary (spec: docs/streams/quality/spec.md). It reads a
// git repository's full history into diffable, auditable, append-only artifacts
// that every later quality metric ("are we getting better / where is it brittle /
// how do we compare") is computed from.
//
// This is the wave-0 skeleton: the `mine` mode extracts commits + diffs through
// the three-state instrument wrapper and records its horizon. `report` (M1
// trend view), `check` (brittleness screen), and `pr` (per-PR risk-feature
// feed) are filled in by later briefs.
//
// The tool is repo-agnostic and OSS: no house specifics are hard-coded here;
// they arrive as configuration in later briefs.
package main

import "os"

func main() {
	os.Exit(dispatch(os.Args[1:], os.Stdout, os.Stderr))
}

// The command line — the verb tree, help, version and the declared settings —
// lives in cli.go.
