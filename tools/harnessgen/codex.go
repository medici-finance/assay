// Codex packaging: generate plugins/assay/.codex-plugin/plugin.json from the
// single metadata source (plugins/assay/.claude-plugin/plugin.json) plus the
// bundle skills tree, enforce a closed coverage rule (every skill packaged or
// excluded-with-reason, ported from SOURCES.yaml), and assert packaging↔binding
// consistency (every packaged skill has a degradation cell in the Codex binding
// file). Derived, never hand-ported: `codex --check` regenerates to memory and
// diffs against the committed manifest, so version skew or a hand-edit reddens CI
// rather than shipping a divergent second manifest (harness-portability/06).
//
// Manifest shape follows HP/01's measured schema (§3.2) and superpowers 6.2.0:
// a `.codex-plugin/plugin.json` whose `skills` field is the `./skills/` directory
// pointer Codex CLI reads — NOT a per-skill array. Everything else derives from
// the Claude manifest (name, version, description, author, homepage, repository,
// license, keywords) so the two manifests cannot skew on version or metadata.
// HP/03 A1 ruled Codex CLI the v1 target, so the App-marketplace `interface`
// block superpowers carries is deliberately omitted (App is HP/08, out of scope).
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// codexPaths names, relative to a bundle dir, the metadata source, the generated
// manifest, the coverage roster, the binding file, and the skills tree.
type codexPaths struct {
	bundle      string // the plugins/assay dir (or a --bundle override)
	metaSource  string // .claude-plugin/plugin.json — the single metadata source
	outManifest string // .codex-plugin/plugin.json — the generated artifact
	packaging   string // codex/packaging.md — the coverage roster
	binding     string // references/codex.md — the per-skill degradation cells
	skillsDir   string // skills/ — the bundle tree
}

func codexPathsFor(bundle string) codexPaths {
	return codexPaths{
		bundle:      bundle,
		metaSource:  filepath.Join(bundle, ".claude-plugin", "plugin.json"),
		outManifest: filepath.Join(bundle, ".codex-plugin", "plugin.json"),
		packaging:   filepath.Join(bundle, "codex", "packaging.md"),
		binding:     filepath.Join(bundle, "references", "codex.md"),
		skillsDir:   filepath.Join(bundle, "skills"),
	}
}

// packagingMarker opens the machine-readable coverage roster in packaging.md.
const packagingMarker = "<!-- assay:codex-packaging"

// author preserves the Claude manifest's author object, re-marshalled uniformly.
type author struct {
	Name  string `json:"name"`
	Email string `json:"email,omitempty"`
	URL   string `json:"url,omitempty"`
}

// claudeManifest is the subset of the Claude plugin manifest the Codex manifest
// derives from — the single metadata source (row 3 of the brief's facts).
type claudeManifest struct {
	Name        string   `json:"name"`
	Version     string   `json:"version"`
	License     string   `json:"license"`
	Description string   `json:"description"`
	Author      author   `json:"author"`
	Homepage    string   `json:"homepage"`
	Repository  string   `json:"repository"`
	Keywords    []string `json:"keywords"`
}

// codexManifest is the generated .codex-plugin/plugin.json. Field ORDER here is
// the byte order of the committed file (json.MarshalIndent emits struct fields in
// declaration order), so `--check` compares apples to apples.
type codexManifest struct {
	Name        string         `json:"name"`
	Version     string         `json:"version"`
	Description string         `json:"description"`
	Author      author         `json:"author"`
	Homepage    string         `json:"homepage"`
	Repository  string         `json:"repository"`
	License     string         `json:"license"`
	Keywords    []string       `json:"keywords"`
	Skills      string         `json:"skills"`
	Hooks       map[string]any `json:"hooks"`
}

// codexCmd implements the `codex` verb. Exit codes are the three-state instrument
// (docs/three-state-instrument-rule.md), same contract as `resident`:
//
//	0 — clean: manifest written, or --check found it identical to what the source
//	    would generate.
//	1 — drift: --check found the committed manifest differs from the source (a
//	    version skew planted into it, or any hand-edit).
//	2 — could-not-check: the metadata source or roster is missing/unparseable, a
//	    skill on disk is unaccounted-for in the roster (coverage), or a packaged
//	    skill has no degradation cell in the Codex binding (packaging↔binding
//	    skew). None of these is a pass — the tool cannot certify a trustworthy
//	    manifest, so it must surface non-zero, never generate silently.
func codexCmd(args []string) int {
	fs := flag.NewFlagSet("codex", flag.ContinueOnError)
	check := fs.Bool("check", false, "do not write; exit non-zero if the committed manifest differs from what the source would generate")
	root := fs.String("root", ".", "repository root the plugins/assay bundle is resolved against")
	bundle := fs.String("bundle", "", "bundle dir to read/generate against (default <root>/plugins/assay); overrides --root")
	if err := fs.Parse(args); err != nil {
		return exitCouldNotCheck
	}

	bundleDir := *bundle
	if bundleDir == "" {
		bundleDir = filepath.Join(*root, "plugins", "assay")
	}
	p := codexPathsFor(bundleDir)

	// (1) Metadata source — missing/unparseable is could-not-check.
	meta, err := readClaudeManifest(p.metaSource)
	if err != nil {
		fmt.Fprintf(os.Stderr, "harnessgen codex: could-not-check: %v\n", err)
		return exitCouldNotCheck
	}

	// (2) Coverage roster — missing/unparseable/empty is could-not-check.
	packaged, excluded, err := readPackaging(p.packaging, packagingMarker)
	if err != nil {
		fmt.Fprintf(os.Stderr, "harnessgen codex: could-not-check: %v\n", err)
		return exitCouldNotCheck
	}

	// (3) Coverage rule: every skill on disk is packaged or excluded-with-reason;
	// an excluded skill must be absent from disk (the ./skills/ pointer would ship
	// it otherwise); a packaged skill must be present. Unaccounted → exit 2.
	if msgs := coverageViolations(p.skillsDir, packaged, excluded); len(msgs) > 0 {
		fmt.Fprintln(os.Stderr, "harnessgen codex: could-not-check: coverage rule failed — every skills/*/SKILL.md must be packaged or excluded-with-reason:")
		for _, m := range msgs {
			fmt.Fprintf(os.Stderr, "  %s\n", m)
		}
		return exitCouldNotCheck
	}

	// (4) Packaging↔binding consistency: every packaged skill has a degradation
	// cell in the Codex binding file. Skew is a build error, not a doc bug → exit 2.
	if msgs, err := bindingViolations(p.binding, packaged); err != nil {
		fmt.Fprintf(os.Stderr, "harnessgen codex: could-not-check: %v\n", err)
		return exitCouldNotCheck
	} else if len(msgs) > 0 {
		fmt.Fprintln(os.Stderr, "harnessgen codex: could-not-check: packaging↔binding skew — a packaged skill has no degradation cell in the Codex binding file:")
		for _, m := range msgs {
			fmt.Fprintf(os.Stderr, "  %s\n", m)
		}
		return exitCouldNotCheck
	}

	// Generate the manifest bytes from the metadata source.
	want := renderCodexManifest(meta)

	if *check {
		got, err := os.ReadFile(p.outManifest)
		if err != nil {
			fmt.Fprintf(os.Stderr, "harnessgen codex --check: could-not-check: cannot read committed manifest %s (%v)\n", p.outManifest, err)
			return exitCouldNotCheck
		}
		if string(got) != want {
			fmt.Fprintf(os.Stderr, "harnessgen codex --check: DRIFT — committed manifest %s differs from the metadata source; run `go run ./tools/harnessgen codex` and commit\n", p.outManifest)
			return exitDrift
		}
		fmt.Printf("harnessgen codex --check: clean — %s matches the metadata source\n", p.outManifest)
		return exitClean
	}

	// Write mode.
	if err := os.MkdirAll(filepath.Dir(p.outManifest), 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "harnessgen codex: %v\n", err)
		return exitCouldNotCheck
	}
	if err := os.WriteFile(p.outManifest, []byte(want), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "harnessgen codex: %v\n", err)
		return exitCouldNotCheck
	}
	fmt.Printf("wrote %s\n", p.outManifest)
	return exitClean
}

// readClaudeManifest reads and parses the single metadata source.
func readClaudeManifest(path string) (claudeManifest, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return claudeManifest{}, fmt.Errorf("reading metadata source %s: %w", path, err)
	}
	var m claudeManifest
	if err := json.Unmarshal(b, &m); err != nil {
		return claudeManifest{}, fmt.Errorf("metadata source %s is unparseable: %w", path, err)
	}
	if strings.TrimSpace(m.Name) == "" || strings.TrimSpace(m.Version) == "" {
		return claudeManifest{}, fmt.Errorf("metadata source %s has no name/version — refusing to generate a manifest from an incomplete source", path)
	}
	return m, nil
}

// renderCodexManifest turns the metadata source into the exact bytes of the
// committed .codex-plugin/plugin.json: 2-space indent (matching the Claude
// manifest style) plus a single trailing newline.
func renderCodexManifest(meta claudeManifest) string {
	m := codexManifest{
		Name:        meta.Name,
		Version:     meta.Version,
		Description: meta.Description,
		Author:      meta.Author,
		Homepage:    meta.Homepage,
		Repository:  meta.Repository,
		License:     meta.License,
		Keywords:    meta.Keywords,
		// The measured Codex bundling schema (HP/01 §3.2) is a directory pointer,
		// not a per-skill array — Codex CLI loads every SKILL.md under it. The
		// packaged set is proven to equal this tree by the coverage rule above.
		Skills: "./skills/",
		// Assay ships no Codex plugin hooks: resident rules arrive via the
		// AGENTS.md fragment (HP/05), installed by the adopt flow, not a hook.
		Hooks: map[string]any{},
	}
	// encoding/json escapes <, >, & as \u00xx by default; the Claude manifest
	// carries a literal `<name>` in its description, so turn HTML escaping off to
	// reproduce the source verbatim rather than mangling it. Encoder.Encode adds
	// its own trailing newline — the single trailing newline the file carries.
	var buf strings.Builder
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(m); err != nil {
		// A struct of strings/slices/maps cannot fail to marshal; treat an
		// impossible error as a programming fault.
		panic(fmt.Sprintf("marshalling codex manifest: %v", err))
	}
	return buf.String()
}

// readPackaging parses the coverage roster's machine-readable block. Returns the
// packaged set and the excluded set (name→reason). An absent marker, an empty
// roster, or an excluded entry with an empty reason is an error (never a silent
// zero-coverage pass).
func readPackaging(path, marker string) (packaged map[string]bool, excluded map[string]string, err error) {
	raw, rerr := os.ReadFile(path)
	if rerr != nil {
		return nil, nil, fmt.Errorf("reading coverage roster %s: %w", path, rerr)
	}
	lines, ok := blockLinesCodex(string(raw), marker)
	if !ok {
		return nil, nil, fmt.Errorf("coverage roster %s: marker %q not found", path, marker)
	}
	packaged = map[string]bool{}
	excluded = map[string]string{}
	for _, ln := range lines {
		ln = strings.TrimSpace(ln)
		if ln == "" || strings.HasPrefix(ln, "#") {
			continue
		}
		if strings.Contains(ln, " :: ") {
			parts := strings.SplitN(ln, " :: ", 2)
			name := strings.TrimSpace(parts[0])
			rest := strings.TrimSpace(parts[1])
			reason := strings.TrimSpace(strings.TrimPrefix(rest, "EXCLUDED:"))
			if !strings.HasPrefix(rest, "EXCLUDED:") || reason == "" {
				return nil, nil, fmt.Errorf("coverage roster %s: excluded entry %q has an empty reason — an exclusion must cite HP/03's ruling or HP/01's matrix", path, name)
			}
			if name == "" {
				return nil, nil, fmt.Errorf("coverage roster %s: an excluded entry has no skill name", path)
			}
			excluded[name] = reason
			continue
		}
		packaged[ln] = true
	}
	if len(packaged) == 0 && len(excluded) == 0 {
		return nil, nil, fmt.Errorf("coverage roster %s: block parsed to zero entries — refusing to report clean", path)
	}
	return packaged, excluded, nil
}

// coverageViolations enforces closure between the disk skills tree and the
// roster: every skills/*/SKILL.md is packaged or excluded; a packaged skill is
// present on disk; an excluded skill is absent from disk (the ./skills/ pointer
// would ship it otherwise). Returns human-readable violation lines (empty ==
// clean).
func coverageViolations(skillsDir string, packaged map[string]bool, excluded map[string]string) []string {
	disk := diskSkillSet(skillsDir)
	var msgs []string

	// Every disk skill must be accounted for.
	for _, name := range sortedKeysBool(disk) {
		if !packaged[name] && excluded[name] == "" {
			msgs = append(msgs, fmt.Sprintf("skill %q is on disk but appears in neither the packaged roster nor the excluded list (add it to the per-harness packaging.md coverage roster)", name))
		}
	}
	// A packaged skill must exist on disk (a stale roster entry is a skew too).
	for _, name := range sortedKeysBool(packaged) {
		if !disk[name] {
			msgs = append(msgs, fmt.Sprintf("packaged skill %q has no skills/%s/SKILL.md on disk — the roster is stale", name, name))
		}
	}
	// An excluded skill must be absent from disk: the manifest ships the whole
	// ./skills/ directory, so a present-but-excluded skill would be shipped anyway.
	for _, name := range sortedKeysStr(excluded) {
		if disk[name] {
			msgs = append(msgs, fmt.Sprintf("skill %q is marked EXCLUDED but is still present under skills/ — the ./skills/ pointer would ship it; remove it from disk or from the excluded list", name))
		}
	}
	return msgs
}

// bindingViolations asserts every packaged skill has a degradation cell in the
// Codex binding file (a backticked `<skill>` token, the same convention
// tools/harnesslint bindings uses). A missing binding file is could-not-check.
func bindingViolations(bindingPath string, packaged map[string]bool) ([]string, error) {
	b, err := os.ReadFile(bindingPath)
	if err != nil {
		return nil, fmt.Errorf("reading Codex binding %s: %w", bindingPath, err)
	}
	body := string(b)
	var msgs []string
	for _, name := range sortedKeysBool(packaged) {
		if !strings.Contains(body, "`"+name+"`") {
			msgs = append(msgs, fmt.Sprintf("packaged skill %q has no degradation cell (`%s`) in %s", name, name, bindingPath))
		}
	}
	return msgs, nil
}

// diskSkillSet returns the set of skill directory names carrying a SKILL.md.
func diskSkillSet(skillsDir string) map[string]bool {
	set := map[string]bool{}
	matches, err := filepath.Glob(filepath.Join(skillsDir, "*", "SKILL.md"))
	if err != nil {
		return set
	}
	for _, f := range matches {
		set[filepath.Base(filepath.Dir(f))] = true
	}
	return set
}

// blockLinesCodex returns the lines strictly between an opening marker and the
// next `-->`. Mirrors harnesslint's blockLines; duplicated here so harnessgen
// carries no dependency on a sibling module.
func blockLinesCodex(text, marker string) ([]string, bool) {
	i := strings.Index(text, marker)
	if i < 0 {
		return nil, false
	}
	rest := text[i+len(marker):]
	if nl := strings.IndexByte(rest, '\n'); nl >= 0 {
		rest = rest[nl+1:]
	} else {
		rest = ""
	}
	if end := strings.Index(rest, "-->"); end >= 0 {
		rest = rest[:end]
	}
	return strings.Split(rest, "\n"), true
}

func sortedKeysBool(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func sortedKeysStr(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
