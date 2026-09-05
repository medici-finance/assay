// pairedversions — front-door consistency guard for the plugin↔statusgen pairing.
//
// `assay:install` resolves the statusgen (and desk-tools) binary an adopter gets
// FROM plugins/assay/paired-versions.yaml, on behalf of the plugin version in
// plugins/assay/.claude-plugin/plugin.json. Those two records are edited by
// different changes at different times, and nothing made them disagree loudly —
// so a plugin bump that skipped the re-pin left every clean adopter installing
// the tool the PREVIOUS plugin was paired with, twelve minors behind the skills
// shipping alongside it. This is the check that reddens on that state.
//
// It asserts three things:
//
//	(a) plugin.json `version` == paired-versions.yaml `plugin` — the manifest is
//	    a pairing for the plugin that is actually shipping.
//	(b) each section's paired `tag` is a PUBLISHED release of the `release_home`
//	    it names — not a draft, not a tag that was never cut.
//	(c) every per-platform `sha256` equals that release's own checksums.txt
//	    entry — the authority the manifest header names, because a locally built
//	    binary lacks the release -ldflags stamp and hashes differently.
//
// FAIL-CLOSED, three-state. checked-clean is exit 0; a checked disagreement and
// a could-not-check are both non-zero and are reported as themselves. A release
// home that could not be read has cleared nothing and never renders green
// (docs/three-state-instrument-rule.md).
//
// Usage:
//
//	go run . --root ../..     # from tools/pairedversions/
//	pairedversions --root .   # exit 0 = consistent, 1 = not, 2 = usage error
//
// Network: reads the GitHub REST API (api.github.com, or $GITHUB_API_URL) for
// (b) and (c). $GH_TOKEN / $GITHUB_TOKEN is used when present.
package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	root := flag.String("root", ".", "path to the repository root to check")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: pairedversions [--root DIR]\n\n")
		fmt.Fprintf(os.Stderr, "Asserts that %s and %s agree, that the paired tag is a\npublished release of its release home, and that every pinned sha256 matches that\nrelease's %s.\n\n", pluginJSONPath, manifestPath, checksumsAsset)
		fmt.Fprintf(os.Stderr, "exit 0 = consistent; 1 = a problem or a could-not-check; 2 = usage error.\n")
		flag.PrintDefaults()
	}
	flag.Parse()
	if flag.NArg() != 0 {
		flag.Usage()
		os.Exit(2)
	}
	fi, err := os.Stat(*root)
	if err != nil || !fi.IsDir() {
		fmt.Fprintf(os.Stderr, "pairedversions: --root %q is not a directory\n", *root)
		os.Exit(2)
	}
	if Check(*root, newGHAPI(), os.Stdout) {
		return
	}
	os.Exit(1)
}
