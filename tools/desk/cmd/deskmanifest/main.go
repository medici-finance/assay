// Command deskmanifest is the component-manifest lint for the component model
// defined by the component-manifest spec (§2). It discovers every
// `component.yaml` in a tree, parses it against §2, resolves every `inject`
// key to a `provides`, checks each `range` against the provider's `version`,
// and reports cycles among `inject.required` — all from the declarations alone,
// so an undeclared dependency is a CI failure instead of an outage.
//
// It is a READ-ONLY reader. It reads `component.yaml` files under --root and
// writes nothing; it contacts no cluster, no forge, and no roster.
//
// THREE STATES, ONE EXIT CODE EACH (brief-v1 §8, this tool's own contract — it
// deliberately does NOT use deskkit's canonical 0/3/4/5/6 map, because a lint's
// three states are clean / problems / could-not-check, not the write-tool set):
//
//	checked-clean       → exit 0  — every required inject resolves, in range, acyclic.
//	checked-failed      → exit 1  — one or more problems; each is named.
//	could-not-check     → exit 2  — the lint could not run (root missing/unreadable).
//	                                Never rounded up to clean, never down to a problem
//	                                it did not observe.
//
// Providers MAY carry attributes (a `flavour`, an `evidence` marker path) and
// inject entries MAY constrain a required key's flavour; a key the catalogue
// (components/KEYS.md) marks `exclusive: true` MUST have at most one ACTIVE
// provider — a second is a lint PROBLEM (component-model.md §9). `--activation`
// additionally reports every component's computed ACTIVE/INACTIVE state.
//
// USAGE:
//
//	deskmanifest lint [--root <dir>] [--activation]
//	deskmanifest --version
package main

import (
	"fmt"
	"io"
	"os"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

const usage = `deskmanifest — the component-manifest lint.

USAGE:
  deskmanifest lint [--root <dir>] [--activation]
  deskmanifest --version

lint discovers every component.yaml under --root (default "."), resolves every
inject key to a provides, checks version ranges (and flavour constraints), and
reports cycles and exclusive-key violations (a key components/KEYS.md marks
exclusive: true with more than one ACTIVE provider). --activation additionally
prints every component's computed ACTIVE/INACTIVE state and, when INACTIVE,
why.

Exit codes:
  0  checked-clean    — every required inject resolves, in range, acyclic
  1  checked-failed   — problems found (each is named)
  2  could-not-check  — the lint could not run (root missing/unreadable)`

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, usage)
		return exitCouldNotCheck
	}
	switch args[0] {
	case "lint":
		return lintCmd(args[1:], stdout, stderr)
	case "--version", "version":
		fmt.Fprintln(stdout, "deskmanifest — component-manifest lint")
		return exitClean
	case "-h", "--help", "help":
		fmt.Fprintln(stdout, usage)
		return exitClean
	default:
		fmt.Fprintf(stderr, "deskmanifest: unknown command %q\n\n%s\n", args[0], usage)
		return exitCouldNotCheck
	}
}
