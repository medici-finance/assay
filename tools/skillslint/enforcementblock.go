// enforcementblock.go — the derive-or-diff of the generated enforcement-status
// block in the authoring guidance (mistake-proofing/04).
//
// THE SHAPE. Identical in spirit to guardrail.go: one declared source, a copy
// that is REGENERATED from it, and this check as the byte-diff. The difference is
// where the source lives. A shared-guardrail block's source is static text in
// GUARDRAILS.md; THIS block's source is the lint's own rule registry, which lives
// in statusgen and can only be rendered by running it. So the source of the bytes
// is `statusgen enforcement-status`, injected here as a `derive` function: the
// production caller shells out to the emitter, the tests inject a fixture so the
// diff logic is exercised without building statusgen.
//
// WHY THE GATE RIDES HERE. The block lives in a plugins/ SKILL.md, and the ONLY
// tool CI runs over the plugin tree on pull requests is skillslint. Statusgen's
// own tests are not run in CI (build-test does `go build && go vet` only). So the
// byte-diff that binds the guidance copy to the lint registry has to be a
// skillslint check to actually gate a PR — which is why it is folded into the
// default lint here rather than added as a new CI step.
//
// THREE-STATE. A derive that fails, an unreadable site, or a missing/ambiguous
// anchor are could-not-check — reported as FAILURE by the caller, never a pass.
// "Nothing to compare" is never clean.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// enforcementSitePath is the guidance document that carries the generated block.
const enforcementSitePath = "plugins/assay/skills/author-brief/SKILL.md"

// enforcementAnchor is the first line of the generated block — the region anchor
// (first-line-anchor style, matching the shared-guardrail generator). It must be
// byte-identical to what `statusgen enforcement-status` emits as its first line.
const enforcementAnchor = "## Enforcement status of these rules (generated — do not hand-edit)"

// EnforcementReport is the three-state outcome of one check.
type EnforcementReport struct {
	Compared  bool
	Failed    *Issue // the copy exists and drifted from the derived block
	Unchecked *Issue // could-not-check: derive failed, site unreadable, anchor missing/ambiguous
}

// deriveEnforcementBlock is the production source of the block's bytes: it shells
// out to `statusgen enforcement-status`, the emitter that renders the lint's rule
// registry. Running the emitter (rather than re-implementing the render here) is
// what keeps a single source: skillslint never knows the rules, only how to fetch
// their rendered form and diff it.
func deriveEnforcementBlock(root string) (string, error) {
	statusgenDir := filepath.Join(root, "statusgen")
	if _, err := os.Stat(filepath.Join(statusgenDir, "go.mod")); err != nil {
		return "", fmt.Errorf("cannot locate the statusgen module at %s: %w", statusgenDir, err)
	}
	cmd := exec.Command("go", "run", ".", "enforcement-status")
	cmd.Dir = statusgenDir
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("running `statusgen enforcement-status`: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return strings.TrimRight(string(out), "\n"), nil
}

// locateEnforcementRegion finds the generated block in fileLines: it runs from
// the anchor line to (but not including) the next `## ` heading, or end of file.
// hits != 1 is could-not-check — the copy was deleted, or its anchor was edited,
// or it appears twice.
func locateEnforcementRegion(fileLines []string) (start, end, hits int) {
	start = -1
	for i, ln := range fileLines {
		if strings.TrimRight(ln, " \t\r") == enforcementAnchor {
			hits++
			if start < 0 {
				start = i
			}
		}
	}
	if start < 0 {
		return -1, -1, 0
	}
	end = len(fileLines)
	for i := start + 1; i < len(fileLines); i++ {
		if strings.HasPrefix(fileLines[i], "## ") {
			end = i
			break
		}
	}
	return start, end, hits
}

// regionText returns the block's own bytes: the region with trailing blank lines
// (the separator before the next heading) trimmed, so it compares against the
// derived block which carries no trailing newline.
func regionText(fileLines []string, start, end int) string {
	return strings.TrimRight(strings.Join(fileLines[start:end], "\n"), "\n")
}

// CheckEnforcementBlock byte-diffs the copy in the guidance against the derived
// block. It never mutates anything; SyncEnforcementBlock regenerates.
func CheckEnforcementBlock(root string, derive func() (string, error)) EnforcementReport {
	want, err := derive()
	if err != nil {
		return EnforcementReport{Unchecked: &Issue{Path: enforcementSitePath, Msg: fmt.Sprintf("could-not-check: cannot derive the enforcement block: %v", err)}}
	}
	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(enforcementSitePath)))
	if err != nil {
		return EnforcementReport{Unchecked: &Issue{Path: enforcementSitePath, Msg: fmt.Sprintf("could-not-check: cannot read the guidance document: %v", err)}}
	}
	fileLines := strings.Split(strings.ReplaceAll(string(raw), "\r\n", "\n"), "\n")
	start, end, hits := locateEnforcementRegion(fileLines)
	switch {
	case hits == 0:
		return EnforcementReport{Unchecked: &Issue{Path: enforcementSitePath, Msg: fmt.Sprintf("could-not-check: the generated block's anchor line is absent.\n  anchor: %q\n  Either the block was deleted or its first line was hand-edited (run `go run ./tools/skillslint --sync`).", enforcementAnchor)}}
	case hits > 1:
		return EnforcementReport{Unchecked: &Issue{Path: enforcementSitePath, Msg: fmt.Sprintf("could-not-check: the generated block's anchor line occurs %d times, so the block is ambiguous.\n  anchor: %q", hits, enforcementAnchor)}}
	}
	got := regionText(fileLines, start, end)
	if got != want {
		return EnforcementReport{Failed: &Issue{Path: enforcementSitePath, Msg: fmt.Sprintf("line %d: the generated enforcement-status block has drifted from `statusgen enforcement-status`.\n%s\n  Do not hand-edit the block: change the rule registry in statusgen and run `go run ./tools/skillslint --sync`.", start+1, enforcementDiff(want, got))}}
	}
	return EnforcementReport{Compared: true}
}

// enforcementDiff renders a compact per-line want/got so the failure says WHICH
// line drifted, not merely that something did.
func enforcementDiff(want, got string) string {
	w := strings.Split(want, "\n")
	g := strings.Split(got, "\n")
	var b strings.Builder
	n := len(w)
	if len(g) > n {
		n = len(g)
	}
	for i := 0; i < n; i++ {
		var wl, gl string
		haveW, haveG := i < len(w), i < len(g)
		if haveW {
			wl = w[i]
		}
		if haveG {
			gl = g[i]
		}
		if haveW && haveG && wl == gl {
			continue
		}
		if haveW {
			fmt.Fprintf(&b, "    want[%d]: %q\n", i+1, wl)
		} else {
			fmt.Fprintf(&b, "    want[%d]: <missing>\n", i+1)
		}
		if haveG {
			fmt.Fprintf(&b, "    got [%d]: %q\n", i+1, gl)
		} else {
			fmt.Fprintf(&b, "    got [%d]: <missing>\n", i+1)
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

// SyncEnforcementBlock REGENERATES the block in the guidance from the derived
// source. Like SyncGuardrails it only rewrites a block it can locate: a missing
// or ambiguous anchor is returned unchanged as an unchecked Issue rather than
// guessing where the block belongs.
func SyncEnforcementBlock(root string, derive func() (string, error)) (changed bool, rep EnforcementReport) {
	want, err := derive()
	if err != nil {
		return false, EnforcementReport{Unchecked: &Issue{Path: enforcementSitePath, Msg: fmt.Sprintf("could-not-check: cannot derive the enforcement block: %v", err)}}
	}
	abs := filepath.Join(root, filepath.FromSlash(enforcementSitePath))
	raw, err := os.ReadFile(abs)
	if err != nil {
		return false, EnforcementReport{Unchecked: &Issue{Path: enforcementSitePath, Msg: fmt.Sprintf("could-not-check: %v", err)}}
	}
	before := strings.ReplaceAll(string(raw), "\r\n", "\n")
	fileLines := strings.Split(before, "\n")
	start, end, hits := locateEnforcementRegion(fileLines)
	if hits != 1 {
		return false, EnforcementReport{Unchecked: &Issue{Path: enforcementSitePath, Msg: fmt.Sprintf("could-not-check: enforcement block anchor found %d times — not rewritten", hits)}}
	}

	out := make([]string, 0, len(fileLines))
	out = append(out, fileLines[:start]...)
	out = append(out, strings.Split(want, "\n")...)
	// Preserve exactly one blank line before the next heading (or nothing at EOF).
	if end < len(fileLines) {
		out = append(out, "")
		out = append(out, fileLines[end:]...)
	}
	after := strings.Join(out, "\n")
	if after == before {
		return false, EnforcementReport{Compared: true}
	}
	info, serr := os.Stat(abs)
	mode := os.FileMode(0o644)
	if serr == nil {
		mode = info.Mode().Perm()
	}
	if werr := os.WriteFile(abs, []byte(after), mode); werr != nil {
		return false, EnforcementReport{Unchecked: &Issue{Path: enforcementSitePath, Msg: fmt.Sprintf("could-not-check: rewriting: %v", werr)}}
	}
	return true, EnforcementReport{Compared: true}
}
