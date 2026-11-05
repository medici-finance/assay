// Cursor packaging: generate the Cursor delivery artifacts from the single
// sources — the Cursor-native resident-rules rule (plugins/assay/cursor/assay.mdc,
// a `.cursor/rules/*.mdc` always-apply rule) from resident-rules.md, and the
// Cursor plugin manifest (plugins/assay/.cursor-plugin/plugin.json) from the
// Claude manifest — enforce the same closed coverage rule codex packaging uses
// (every skill packaged or excluded-with-reason, ported from SOURCES.yaml), and
// assert packaging↔binding consistency (every packaged skill has a degradation
// cell in the Cursor binding file). Derived, never hand-ported: `cursor --check`
// regenerates to memory and diffs against the committed artifacts, so a
// hand-edit, a planted version skew, or a source edited without regenerating
// reddens CI rather than shipping a divergent rule set or a stale manifest
// (harness-portability/12, #4775).
//
// Two Cursor install routes read this bundle. The copy-skills route
// (`deskinstall --harness cursor`) places SKILL.md skills and the `.mdc` rule
// into an adopter's `.cursor/` tree. The Cursor plugin route (a team
// marketplace import of this repository) reads `.cursor-plugin/marketplace.json`
// at the repository root, follows the `assay` entry's `source` to
// plugins/assay, and reads plugins/assay/.cursor-plugin/plugin.json there
// (https://cursor.com/docs/reference/plugins). The manifest's `version` is the
// one version location for that route: the root marketplace entry carries none,
// so the release stamp has exactly one Cursor file to move and the two cannot
// skew. With `--root` (not `--bundle`) this verb also generates and checks the
// root marketplace, derived from the Claude marketplace catalog.
//
// Cursor also reads the shared Codex `AGENTS.md` fragment natively (HP/12 §2.1),
// so the adopt flow offers either resident-rules channel. Per Ian's 2026-08-26
// ruling, both Cursor surfaces are targeted, headless-first.
package main

import (
	"encoding/json"
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
	outRule     string // cursor/assay.mdc — the generated rule
	metaSource  string // .claude-plugin/plugin.json — the single metadata source
	outManifest string // .cursor-plugin/plugin.json — the generated Cursor plugin manifest
	packaging   string // cursor/packaging.md — the coverage roster
	binding     string // references/cursor.md — the per-skill degradation cells
	skillsDir   string // skills/ — the bundle tree
}

func cursorPathsFor(bundle string) cursorPaths {
	return cursorPaths{
		bundle:      bundle,
		residentSrc: filepath.Join(bundle, "resident-rules.md"),
		outRule:     filepath.Join(bundle, "cursor", "assay.mdc"),
		metaSource:  filepath.Join(bundle, ".claude-plugin", "plugin.json"),
		outManifest: filepath.Join(bundle, ".cursor-plugin", "plugin.json"),
		packaging:   filepath.Join(bundle, "cursor", "packaging.md"),
		binding:     filepath.Join(bundle, "references", "cursor.md"),
		skillsDir:   filepath.Join(bundle, "skills"),
	}
}

// cursorCmd implements the `cursor` verb. Exit codes are the three-state
// instrument (docs/three-state-instrument-rule.md), same contract as `codex`:
//
//	0 — clean: artifacts written, or --check found each identical to what its
//	    source would generate.
//	1 — drift: --check found a committed artifact (the .mdc rule, the Cursor
//	    plugin manifest, or — in --root mode — the root Cursor marketplace)
//	    differs from its source (a hand-edit, a planted version skew, or a source
//	    edited without regenerating).
//	2 — could-not-check: the resident source, the Claude manifest, or (in --root
//	    mode) the Claude marketplace catalog is missing/unparseable, the roster is
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

	// (5) Metadata source — the Claude manifest is the single source for the
	// Cursor manifest AND for the Header's {{VERSION}} token (#730). Missing or
	// unparseable is could-not-check: a Cursor manifest cannot be generated
	// without it.
	meta, err := readClaudeManifest(p.metaSource)
	if err != nil {
		fmt.Fprintf(os.Stderr, "harnessgen cursor: could-not-check: %v\n", err)
		return exitCouldNotCheck
	}

	// Generate the rule bytes from the resident source.
	rule, err := cursorRules(s, meta.Version)
	if err != nil {
		fmt.Fprintf(os.Stderr, "harnessgen cursor: could-not-check: %v\n", err)
		return exitCouldNotCheck
	}

	outs := []cursorOutput{
		{path: p.outRule, want: rule, what: "rule", source: "the resident source"},
		{path: p.outManifest, want: renderCursorManifest(meta), what: "manifest", source: "the metadata source"},
	}

	// (6) Root marketplace — only when the bundle is resolved from --root, since
	// a bare --bundle has no repository root to place it under.
	if *bundle == "" {
		catalog := filepath.Join(*root, ".claude-plugin", "marketplace.json")
		mkt, err := renderCursorMarketplace(catalog, meta.Name, "./"+filepath.ToSlash(filepath.Join("plugins", "assay")))
		if err != nil {
			fmt.Fprintf(os.Stderr, "harnessgen cursor: could-not-check: %v\n", err)
			return exitCouldNotCheck
		}
		outs = append(outs, cursorOutput{
			path: filepath.Join(*root, ".cursor-plugin", "marketplace.json"), want: mkt,
			what: "marketplace", source: "the Claude marketplace catalog",
		})
	}

	if *check {
		rc := exitClean
		for _, o := range outs {
			got, err := os.ReadFile(o.path)
			if err != nil {
				fmt.Fprintf(os.Stderr, "harnessgen cursor --check: could-not-check: cannot read committed %s %s (%v)\n", o.what, o.path, err)
				return exitCouldNotCheck
			}
			if string(got) != o.want {
				fmt.Fprintf(os.Stderr, "harnessgen cursor --check: DRIFT — committed %s %s differs from %s; run `go run ./tools/harnessgen cursor` and commit\n", o.what, o.path, o.source)
				rc = exitDrift
				continue
			}
			fmt.Printf("harnessgen cursor --check: clean — %s matches %s\n", o.path, o.source)
		}
		return rc
	}

	// Write mode.
	for _, o := range outs {
		if err := os.MkdirAll(filepath.Dir(o.path), 0o755); err != nil {
			fmt.Fprintf(os.Stderr, "harnessgen cursor: %v\n", err)
			return exitCouldNotCheck
		}
		if err := os.WriteFile(o.path, []byte(o.want), 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "harnessgen cursor: %v\n", err)
			return exitCouldNotCheck
		}
		fmt.Printf("wrote %s\n", o.path)
	}
	return exitClean
}

// cursorOutput is one generated artifact: where it lives, the bytes its source
// would generate, and the words a drift report names it by.
type cursorOutput struct {
	path, want, what, source string
}

// cursorManifest is the generated .cursor-plugin/plugin.json. Field ORDER is the
// byte order of the committed file. Only fields Cursor's plugin reference
// documents are emitted (https://cursor.com/docs/reference/plugins), and the
// file carries exactly one "version" key so the release stamp's in-place rewrite
// is byte-identical to regenerating.
//
// Component fields are explicit rather than left to folder discovery, because a
// specified field REPLACES discovery for that component:
//   - rules: the generated always-apply resident-rules rule.
//   - skills: the ./skills/ tree — the packaged set, which the coverage rule
//     above proves equals the tree (an excluded skill must be absent from disk).
//   - commands: none. Discovery would otherwise load commands/README.md as a
//     command; the copy-skills route ships no commands either.
//   - hooks: none. The bundle's hooks/hooks.json is the Claude Code
//     SessionStart hook; on Cursor the resident rules arrive via the rule above.
type cursorManifest struct {
	Name        string         `json:"name"`
	Version     string         `json:"version"`
	Description string         `json:"description"`
	Author      author         `json:"author"`
	Homepage    string         `json:"homepage"`
	Repository  string         `json:"repository"`
	License     string         `json:"license"`
	Keywords    []string       `json:"keywords"`
	Rules       string         `json:"rules"`
	Skills      string         `json:"skills"`
	Commands    []string       `json:"commands"`
	Hooks       map[string]any `json:"hooks"`
}

func renderCursorManifest(meta claudeManifest) string {
	return encodeJSON(cursorManifest{
		Name:        meta.Name,
		Version:     meta.Version,
		Description: meta.Description,
		Author:      meta.Author,
		Homepage:    meta.Homepage,
		Repository:  meta.Repository,
		License:     meta.License,
		Keywords:    meta.Keywords,
		Rules:       "./cursor/assay.mdc",
		Skills:      "./skills/",
		Commands:    []string{},
		Hooks:       map[string]any{},
	})
}

// cursorMarketplace is the generated root .cursor-plugin/marketplace.json, the
// file a Cursor team-marketplace import reads first. Its plugin entry carries
// NO version: Cursor merges the per-plugin manifest over the entry ("manifest
// values take precedence"), so the plugin manifest is the one version location
// and the release stamp has nothing here to move.
type cursorMarketplace struct {
	Name    string                   `json:"name"`
	Owner   cursorMarketplaceOwner   `json:"owner"`
	Plugins []cursorMarketplaceEntry `json:"plugins"`
}

type cursorMarketplaceOwner struct {
	Name string `json:"name"`
}

type cursorMarketplaceEntry struct {
	Name   string `json:"name"`
	Source string `json:"source"`
}

// renderCursorMarketplace derives the root Cursor marketplace from the Claude
// marketplace catalog: the catalog's name and owner, and one entry naming the
// plugin and its source directory. The catalog must list the plugin at that
// source, so the two marketplaces cannot point at different trees.
func renderCursorMarketplace(catalogPath, plugin, source string) (string, error) {
	b, err := os.ReadFile(catalogPath)
	if err != nil {
		return "", fmt.Errorf("reading marketplace catalog %s: %w", catalogPath, err)
	}
	var c struct {
		Name  string `json:"name"`
		Owner struct {
			Name string `json:"name"`
		} `json:"owner"`
		Plugins []struct {
			Name   string `json:"name"`
			Source string `json:"source"`
		} `json:"plugins"`
	}
	if err := json.Unmarshal(b, &c); err != nil {
		return "", fmt.Errorf("marketplace catalog %s is unparseable: %w", catalogPath, err)
	}
	if strings.TrimSpace(c.Name) == "" || strings.TrimSpace(c.Owner.Name) == "" {
		return "", fmt.Errorf("marketplace catalog %s has no name/owner.name", catalogPath)
	}
	found := false
	for _, e := range c.Plugins {
		if e.Name == plugin && e.Source == source {
			found = true
		}
	}
	if !found {
		return "", fmt.Errorf("marketplace catalog %s lists no %q entry with source %q", catalogPath, plugin, source)
	}
	return encodeJSON(cursorMarketplace{
		Name:    c.Name,
		Owner:   cursorMarketplaceOwner{Name: c.Owner.Name},
		Plugins: []cursorMarketplaceEntry{{Name: plugin, Source: source}},
	}), nil
}
