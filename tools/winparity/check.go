package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// makefileRelPath and psRelPath are the two artifacts whose target sets must
// agree, relative to the repository root passed as --root.
const (
	makefileRelPath = "Makefile"
	psRelPath       = "scripts/build-windows.ps1"

	// The PowerShell script fences its declared target set between these two
	// markers so the set is machine-readable without executing PowerShell.
	psBeginMarker = "MAKEFILE-PARITY TARGETS (BEGIN)"
	psEndMarker   = "MAKEFILE-PARITY TARGETS (END)"
)

// phonyTargetRE matches a Makefile target name — the token shape a `.PHONY`
// list and a rule head both use.
var phonyTargetRE = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

// psQuotedRE matches a single-quoted PowerShell string literal.
var psQuotedRE = regexp.MustCompile(`'([^']*)'`)

// Check asserts that the Windows build script's declared target set equals the
// Unix Makefile's `.PHONY` set, so the Windows counterpart cannot silently fall
// behind the Unix target set (or grow a target the Makefile lacks).
//
// FAIL-CLOSED, three-state (docs/three-state-instrument-rule.md). It returns
// true only when the two sets were BOTH read and are EQUAL (checked-clean). A
// disagreement is checked-failed; a file it could not read or parse is
// could-not-check. Both non-clean states are reported AS THEMSELVES and both
// return false — a could-not-check is never rounded up to a pass.
func Check(root string, out io.Writer) bool {
	makeSet, makeErr := makefilePhonyTargets(filepath.Join(root, filepath.FromSlash(makefileRelPath)))
	psSet, psErr := powershellTargets(filepath.Join(root, filepath.FromSlash(psRelPath)))

	// could-not-check: a source we could not read or parse has cleared nothing.
	cnc := false
	if makeErr != nil {
		fmt.Fprintf(out, "winparity: could-not-check: %s: %v\n", makefileRelPath, makeErr)
		cnc = true
	}
	if psErr != nil {
		fmt.Fprintf(out, "winparity: could-not-check: %s: %v\n", psRelPath, psErr)
		cnc = true
	}
	if cnc {
		fmt.Fprintf(out, "winparity: NOT CLEARED — a target set could not be read (could-not-check is not a pass)\n")
		return false
	}

	missing := difference(makeSet, psSet) // in Makefile .PHONY, absent from the ps1
	extra := difference(psSet, makeSet)   // declared in the ps1, absent from Makefile .PHONY

	if len(missing) == 0 && len(extra) == 0 {
		fmt.Fprintf(out, "winparity: OK — %d targets in parity: %s\n", len(makeSet), strings.Join(sortedKeys(makeSet), " "))
		return true
	}

	fmt.Fprintf(out, "winparity: DRIFT — %s and %s declare different target sets:\n", makefileRelPath, psRelPath)
	if len(missing) > 0 {
		fmt.Fprintf(out, "  in Makefile .PHONY but MISSING from %s: %s\n", psRelPath, strings.Join(missing, " "))
	}
	if len(extra) > 0 {
		fmt.Fprintf(out, "  declared in %s but ABSENT from Makefile .PHONY: %s\n", psRelPath, strings.Join(extra, " "))
	}
	fmt.Fprintf(out, "  reconcile the two so the Windows build cannot fall behind the Unix target set.\n")
	return false
}

// makefilePhonyTargets returns the union of every target named on a `.PHONY:`
// line in the Makefile, honouring `\` line continuation. An empty set is an
// error: a Makefile with no phony targets means the parser found nothing to
// compare, which is a could-not-check, not a clean run.
func makefilePhonyTargets(path string) (map[string]bool, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	set := map[string]bool{}
	lines := strings.Split(strings.ReplaceAll(string(raw), "\r\n", "\n"), "\n")
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		if !strings.HasPrefix(strings.TrimSpace(line), ".PHONY:") {
			continue
		}
		// Gather the logical line across `\` continuations.
		acc := line
		for strings.HasSuffix(strings.TrimRight(acc, " \t"), "\\") && i+1 < len(lines) {
			acc = strings.TrimRight(strings.TrimRight(acc, " \t"), "\\")
			i++
			acc += " " + lines[i]
		}
		acc = strings.TrimSpace(acc)
		acc = strings.TrimPrefix(acc, ".PHONY:")
		for _, tok := range strings.Fields(acc) {
			if phonyTargetRE.MatchString(tok) {
				set[tok] = true
			}
		}
	}
	if len(set) == 0 {
		return nil, fmt.Errorf("no .PHONY targets found (parser matched nothing)")
	}
	return set, nil
}

// powershellTargets returns the target names declared between the BEGIN/END
// parity markers in the PowerShell build script — every single-quoted string
// literal in that block. An absent marker block, or an empty one, is an error
// (could-not-check): the guard must not silently pass when it cannot locate the
// declared set.
func powershellTargets(path string) (map[string]bool, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	content := strings.ReplaceAll(string(raw), "\r\n", "\n")
	bi := strings.Index(content, psBeginMarker)
	if bi < 0 {
		return nil, fmt.Errorf("BEGIN marker %q not found", psBeginMarker)
	}
	ei := strings.Index(content, psEndMarker)
	if ei < 0 {
		return nil, fmt.Errorf("END marker %q not found", psEndMarker)
	}
	if ei <= bi {
		return nil, fmt.Errorf("END marker precedes BEGIN marker")
	}
	block := content[bi+len(psBeginMarker) : ei]
	set := map[string]bool{}
	for _, m := range psQuotedRE.FindAllStringSubmatch(block, -1) {
		tok := m[1]
		if phonyTargetRE.MatchString(tok) {
			set[tok] = true
		}
	}
	if len(set) == 0 {
		return nil, fmt.Errorf("no target names found between the parity markers")
	}
	return set, nil
}

// difference returns the sorted keys present in a but not in b.
func difference(a, b map[string]bool) []string {
	var out []string
	for k := range a {
		if !b[k] {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
