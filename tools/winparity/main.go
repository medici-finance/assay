// winparity — parity guard for the Windows build script against the Unix Makefile.
//
// The Windows counterpart to the root Makefile is scripts/build-windows.ps1. It
// mirrors the Makefile's `.PHONY` target set by hand, and a hand-kept mirror
// drifts: a target added to one file and not the other ships a Windows build
// that quietly lost (or invented) a target. This is the check that reddens on
// that state, the same way tools/skillslint's guardrail byte-diff and
// tools/pairedversions' front-door check redden on their own drift classes.
//
// It asserts two things about scripts/build-windows.ps1: (1) the set of targets
// between the `MAKEFILE-PARITY TARGETS (BEGIN/END)` markers equals the set of
// targets on the Makefile's `.PHONY:` line; and (2) the script is Windows
// PowerShell 5.1-clean — ASCII-only with no `>>>` in strings — so powershell.exe
// (not just pwsh 7) parses it (#678).
//
// FAIL-CLOSED, three-state. checked-clean is exit 0; a checked disagreement and
// a could-not-check are both non-zero and are reported as themselves. A file
// that could not be read or parsed has cleared nothing and never renders green
// (docs/three-state-instrument-rule.md).
//
// Usage:
//
//	go run . --root ../..     # from tools/winparity/
//	winparity --root .        # exit 0 = in parity, 1 = drift or could-not-check, 2 = usage error
//
// Network: none. It reads only two files under --root.
package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	root := flag.String("root", ".", "path to the repository root to check")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: winparity [--root DIR]\n\n")
		fmt.Fprintf(os.Stderr, "Asserts that %s's declared target set equals %s's .PHONY set.\n\n", psRelPath, makefileRelPath)
		fmt.Fprintf(os.Stderr, "exit 0 = in parity; 1 = drift or a could-not-check; 2 = usage error.\n")
		flag.PrintDefaults()
	}
	flag.Parse()
	if flag.NArg() != 0 {
		flag.Usage()
		os.Exit(2)
	}
	fi, err := os.Stat(*root)
	if err != nil || !fi.IsDir() {
		fmt.Fprintf(os.Stderr, "winparity: --root %q is not a directory\n", *root)
		os.Exit(2)
	}
	if Check(*root, os.Stdout) {
		return
	}
	os.Exit(1)
}
