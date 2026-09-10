// Cursor packaging: generate the Cursor-native resident-rules delivery artifact
// (plugins/assay/cursor/assay.mdc — a `.cursor/rules/*.mdc` always-apply rule)
// from the single resident-rules source, enforce the same closed coverage rule
// codex packaging uses (every skill packaged or excluded-with-reason, ported from
// SOURCES.yaml), and assert packaging↔binding consistency (every packaged skill
// has a degradation cell in the Cursor binding file). Derived, never hand-ported:
// `cursor --check` regenerates to memory and diffs against the committed artifact,
// so a hand-edit or a source edited without regenerating reddens CI rather than
// shipping a divergent rule set (harness-portability/12).
//
// Cursor is LIGHTER than Codex packaging: Cursor consumes `SKILL.md`, `AGENTS.md`,
// and `.cursor/rules/*.mdc` directly from the repo tree (HP/12 §2.10), so there is
// no per-harness plugin manifest to generate — the packaging IS the instruction
// files. The one generated artifact is the `.mdc` resident-rules rule; the same
// coverage + binding discipline the codex verb runs guards the skill set and the
// Cursor binding file. Cursor also reads the shared Codex `AGENTS.md` fragment
// natively (HP/12 §2.1), so the adopt flow offers either channel. Per Ian's
// 2026-08-26 ruling, both Cursor surfaces are targeted, headless-first.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// cursorPackagingMarker opens the machine-readable coverage roster in the Cursor
// packaging.md.
const cursorPackagingMarker = "<!-- assay:cursor-packaging"

// cursorPaths names, relative to a bundle dir, the resident-rules source, the
// generated `.mdc` artifact, the coverage roster, the Cursor binding file, and the
// skills tree.
type cursorPaths struct {
	bundle      string // the plugins/assay dir (or a --bundle override)
	residentSrc string // resident-rules.md — the single resident-rules source
	outRule     string // cursor/assay.mdc — the generated artifact
	packaging   string // cursor/packaging.md — the coverage roster
	binding     string // references/cursor.md — the per-skill degradation cells
	skillsDir   string // skills/ — the bundle tree
}

func cursorPathsFor(bundle string) cursorPaths {
	return cursorPaths{
		bundle:      bundle,
		residentSrc: filepath.Join(bundle, "resident-rules.md"),
		outRule:     filepath.Join(bundle, "cursor", "assay.mdc"),
		packaging:   filepath.Join(bundle, "cursor", "packaging.md"),
		binding:     filepath.Join(bundle, "references", "cursor.md"),
		skillsDir:   filepath.Join(bundle, "skills"),
	}
}

// cursorCmd implements the `cursor` verb. Exit codes are the three-state
// instrument (docs/three-state-instrument-rule.md), same contract as `codex`:
//
//	0 — clean: rule written, or --check found it identical to what the source
//	    would generate.
//	1 — drift: --check found the committed .mdc differs from the source (a
//	    hand-edit, or a source edited without regenerating).
//	2 — could-not-check: the resident source is missing/unparseable, the roster is
//	    missing/unparseable, a skill on disk is unaccounted-for in the roster
//	    (coverage), or a packaged skill has no degradation cell in the Cursor
//	    binding (packaging↔binding skew). None of these is a pass.
func cursorCmd(args []string) int {
	fs := flag.NewFlagSet("cursor", flag.ContinueOnError)
	check := fs.Bool("check", false, "do not write; exit non-zero if the committed rule differs from what the source would generate")
	root := fs.String("root", ".", "repository root the plugins/assay bundle is resolved against")
	bundle := fs.String("bundle", "", "bundle dir to read/generate against (default <root>/plugins/assay); overrides --root")
	if err := fs.Parse(args); err != nil {
		return exitCouldNotCheck
	}

	bundleDir := *bundle
	if bundleDir == "" {
		bundleDir = filepath.Join(*root, "plugins", "assay")
	}
	p := cursorPathsFor(bundleDir)

	// (1) Resident-rules source — missing/unparseable is could-not-check.
	srcBytes, err := os.ReadFile(p.residentSrc)
	if err != nil {
		fmt.Fprintf(os.Stderr, "harnessgen cursor: could-not-check: reading resident source %s: %v\n", p.residentSrc, err)
		return exitCouldNotCheck
	}
	s, err := parseSource(string(srcBytes))
	if err != nil {
		fmt.Fprintf(os.Stderr, "harnessgen cursor: could-not-check: %s is unparseable: %v\n", p.residentSrc, err)
		return exitCouldNotCheck
	}

	// (2) Coverage roster — missing/unparseable/empty is could-not-check.
	packaged, excluded, err := readPackaging(p.packaging, cursorPackagingMarker)
	if err != nil {
		fmt.Fprintf(os.Stderr, "harnessgen cursor: could-not-check: %v\n", err)
		return exitCouldNotCheck
	}

	// (3) Coverage rule: every skill on disk is packaged or excluded-with-reason;
	// a packaged skill must be present; an excluded skill must be absent. Unlike
	// the codex directory pointer, the Cursor `.mdc` rule ships no skills itself,
	// but the closed coverage rule is the same SOURCES.yaml discipline — a skill on
	// disk that is neither packaged nor excluded slips into the bundle unaccounted.
	if msgs := coverageViolations(p.skillsDir, packaged, excluded); len(msgs) > 0 {
		fmt.Fprintln(os.Stderr, "harnessgen cursor: could-not-check: coverage rule failed — every skills/*/SKILL.md must be packaged or excluded-with-reason:")
		for _, m := range msgs {
			fmt.Fprintf(os.Stderr, "  %s\n", m)
		}
		return exitCouldNotCheck
	}

	// (4) Packaging↔binding consistency: every packaged skill has a degradation
	// cell in the Cursor binding file. Skew is a build error, not a doc bug → exit 2.
	if msgs, err := bindingViolations(p.binding, packaged); err != nil {
		fmt.Fprintf(os.Stderr, "harnessgen cursor: could-not-check: %v\n", err)
		return exitCouldNotCheck
	} else if len(msgs) > 0 {
		fmt.Fprintln(os.Stderr, "harnessgen cursor: could-not-check: packaging↔binding skew — a packaged skill has no degradation cell in the Cursor binding file:")
		for _, m := range msgs {
			fmt.Fprintf(os.Stderr, "  %s\n", m)
		}
		return exitCouldNotCheck
	}

	// The Header's {{VERSION}} token derives from the plugin manifest, same
	// rule the `resident` verb applies (assay#730) — read it only when the
	// Header actually carries the placeholder.
	var version string
	if strings.Contains(s.Header, versionPlaceholder) {
		meta, err := readClaudeManifest(filepath.Join(bundleDir, ".claude-plugin", "plugin.json"))
		if err != nil {
			fmt.Fprintf(os.Stderr, "harnessgen cursor: could-not-check: %v\n", err)
			return exitCouldNotCheck
		}
		version = meta.Version
	}

	// Generate the rule bytes from the resident source.
	want, err := cursorRules(s, version)
	if err != nil {
		fmt.Fprintf(os.Stderr, "harnessgen cursor: could-not-check: %v\n", err)
		return exitCouldNotCheck
	}

	if *check {
		got, err := os.ReadFile(p.outRule)
		if err != nil {
			fmt.Fprintf(os.Stderr, "harnessgen cursor --check: could-not-check: cannot read committed rule %s (%v)\n", p.outRule, err)
			return exitCouldNotCheck
		}
		if string(got) != want {
			fmt.Fprintf(os.Stderr, "harnessgen cursor --check: DRIFT — committed rule %s differs from the resident source; run `go run ./tools/harnessgen cursor` and commit\n", p.outRule)
			return exitDrift
		}
		fmt.Printf("harnessgen cursor --check: clean — %s matches the resident source\n", p.outRule)
		return exitClean
	}

	// Write mode.
	if err := os.MkdirAll(filepath.Dir(p.outRule), 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "harnessgen cursor: %v\n", err)
		return exitCouldNotCheck
	}
	if err := os.WriteFile(p.outRule, []byte(want), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "harnessgen cursor: %v\n", err)
		return exitCouldNotCheck
	}
	fmt.Printf("wrote %s\n", p.outRule)
	return exitClean
}
