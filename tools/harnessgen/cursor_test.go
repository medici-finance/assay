package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- committed .mdc rule is in sync with the committed resident source ---------

// TestCursorCommittedRuleMatchesSource is the cursor analogue of
// TestCodexCommittedManifestMatchesSource: the shipped cursor/assay.mdc must be
// exactly what the committed resident-rules.md would generate. If it drifts, this
// fails in the same suite CI runs (Verify: `cursor --check`).
func TestCursorCommittedRuleMatchesSource(t *testing.T) {
	if code := cursorCmd([]string{"--check", "--root", "../.."}); code != exitClean {
		t.Fatalf("cursor --check against the repo root returned %d, want %d (clean) — "+
			"the committed rule is out of sync with the resident source; run `go run ./tools/harnessgen cursor` and commit", code, exitClean)
	}
}

// --- the committed .mdc carries the `.cursor/rules` frontmatter contract -------

// TestCursorRuleHasMdcFrontmatter locks the Cursor-native shape: an `.mdc` rule
// begins with a YAML frontmatter block carrying `alwaysApply: true` (so the rules
// load every session) — the mechanism Cursor reads (HP/12 §2.1). A generator that
// dropped the frontmatter would ship a file Cursor treats as plain prose, not a rule.
func TestCursorRuleHasMdcFrontmatter(t *testing.T) {
	raw, err := os.ReadFile("../../plugins/assay/cursor/assay.mdc")
	if err != nil {
		t.Fatalf("read committed rule: %v", err)
	}
	body := string(raw)
	if !strings.HasPrefix(body, "---\n") {
		t.Fatalf("committed rule does not open with an `.mdc` frontmatter block; head:\n%.80s", body)
	}
	if !strings.Contains(body, "alwaysApply: true") {
		t.Fatalf("committed rule is missing `alwaysApply: true` — Cursor would not load it every session")
	}
}

// --- a planted rule hand-edit is caught as drift, naming the rule --------------

func TestCursorCheckDetectsDrift(t *testing.T) {
	bundle := writeMinimalCursorBundle(t)
	rule := filepath.Join(bundle, "cursor", "assay.mdc")
	raw, err := os.ReadFile(rule)
	if err != nil {
		t.Fatal(err)
	}
	// Hand-edit the generated rule (mirrors a stray edit that bypasses the generator).
	edited := string(raw) + "\nHAND-EDITED LINE\n"
	if err := os.WriteFile(rule, []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}
	var code int
	stderr := captureStderr(t, func() { code = cursorCmd([]string{"--check", "--bundle", bundle}) })
	if code != exitDrift {
		t.Fatalf("check with a hand-edited rule returned %d, want %d (drift)", code, exitDrift)
	}
	if !strings.Contains(stderr, "assay.mdc") {
		t.Fatalf("drift report did not name the rule; stderr:\n%s", stderr)
	}
	// After restoring, --check passes again.
	if err := os.WriteFile(rule, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	if code := cursorCmd([]string{"--check", "--bundle", bundle}); code != exitClean {
		t.Fatalf("check after restore returned %d, want %d (clean)", code, exitClean)
	}
}

// --- coverage: a skill on disk with no roster/exclusion entry is exit 2 --------

func TestCursorCoverageCatchesUnaccountedSkill(t *testing.T) {
	bundle := writeMinimalCursorBundle(t)
	if code := cursorCmd([]string{"--check", "--bundle", bundle}); code != exitClean {
		t.Fatalf("baseline check returned %d, want %d", code, exitClean)
	}
	probe := filepath.Join(bundle, "skills", "probe-skill")
	if err := os.MkdirAll(probe, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(probe, "SKILL.md"), []byte("---\nname: probe-skill\ndescription: probe\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var code int
	stderr := captureStderr(t, func() { code = cursorCmd([]string{"--check", "--bundle", bundle}) })
	if code != exitCouldNotCheck {
		t.Fatalf("check with an unaccounted skill returned %d, want %d (could-not-check)", code, exitCouldNotCheck)
	}
	if !strings.Contains(stderr, "probe-skill") {
		t.Fatalf("coverage failure did not name the unaccounted skill; stderr:\n%s", stderr)
	}
}

// --- packaging↔binding skew: a packaged skill with no degradation cell is caught

func TestCursorBindingSkewCaught(t *testing.T) {
	bundle := writeMinimalCursorBundle(t)
	if code := cursorCmd([]string{"--check", "--bundle", bundle}); code != exitClean {
		t.Fatalf("baseline check returned %d, want %d", code, exitClean)
	}
	binding := filepath.Join(bundle, "references", "cursor.md")
	raw, err := os.ReadFile(binding)
	if err != nil {
		t.Fatal(err)
	}
	stripped := strings.ReplaceAll(string(raw), "`the-desk`", "the-desk")
	if stripped == string(raw) {
		t.Fatal("test setup: expected a `the-desk` degradation cell to strip")
	}
	if err := os.WriteFile(binding, []byte(stripped), 0o644); err != nil {
		t.Fatal(err)
	}
	var code int
	stderr := captureStderr(t, func() { code = cursorCmd([]string{"--check", "--bundle", bundle}) })
	if code != exitCouldNotCheck {
		t.Fatalf("check with a missing binding cell returned %d, want %d (could-not-check)", code, exitCouldNotCheck)
	}
	if !strings.Contains(stderr, "the-desk") {
		t.Fatalf("binding-skew failure did not name the skill; stderr:\n%s", stderr)
	}
}

// --- an excluded entry with an empty reason is a parse error (could-not-check) -

func TestCursorExcludedEmptyReasonIsParseError(t *testing.T) {
	bundle := writeMinimalCursorBundle(t)
	packaging := filepath.Join(bundle, "cursor", "packaging.md")
	raw, err := os.ReadFile(packaging)
	if err != nil {
		t.Fatal(err)
	}
	edited := strings.Replace(string(raw), "author-brief\n", "author-brief :: EXCLUDED:\n", 1)
	if edited == string(raw) {
		t.Fatal("test setup: expected to find author-brief in the roster")
	}
	if err := os.WriteFile(packaging, []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}
	var code int
	stderr := captureStderr(t, func() { code = cursorCmd([]string{"--check", "--bundle", bundle}) })
	if code != exitCouldNotCheck {
		t.Fatalf("check with an empty-reason exclusion returned %d, want %d (could-not-check)", code, exitCouldNotCheck)
	}
	if !strings.Contains(stderr, "empty reason") {
		t.Fatalf("parse error did not explain the empty reason; stderr:\n%s", stderr)
	}
}

// --- the Cursor plugin manifest (#4775) ----------------------------------------

// TestCursorManifestVersionEqualsClaude: the committed Cursor plugin manifest
// carries the Claude manifest's version — the version a Cursor plugin install
// reports. (TestCursorCommittedRuleMatchesSource's root-mode --check also diffs
// the whole manifest and the root marketplace; this names the version failure.)
func TestCursorManifestVersionEqualsClaude(t *testing.T) {
	readVersion := func(path string) string {
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		var m struct {
			Version string `json:"version"`
		}
		if err := json.Unmarshal(b, &m); err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		return m.Version
	}
	claude := readVersion("../../plugins/assay/.claude-plugin/plugin.json")
	cursor := readVersion("../../plugins/assay/.cursor-plugin/plugin.json")
	if claude != cursor {
		t.Fatalf("version skew: Claude manifest %q != Cursor manifest %q — the Cursor manifest must be generated from the Claude one", claude, cursor)
	}
}

// TestCursorManifestShape locks the documented contract the manifest relies on:
// exactly one "version" key (so the release stamp's in-place rewrite equals a
// regenerate), relative component paths with no `..` that resolve inside the
// bundle, and explicit empty commands/hooks so folder discovery cannot pick up
// commands/README.md or the Claude-format hooks/hooks.json.
func TestCursorManifestShape(t *testing.T) {
	bundle := writeMinimalCursorBundle(t)
	raw, err := os.ReadFile(filepath.Join(bundle, ".cursor-plugin", "plugin.json"))
	if err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(string(raw), `"version"`); n != 1 {
		t.Fatalf("manifest carries %d \"version\" keys, want exactly 1", n)
	}
	var m struct {
		Name     string         `json:"name"`
		Version  string         `json:"version"`
		Rules    string         `json:"rules"`
		Skills   string         `json:"skills"`
		Commands []string       `json:"commands"`
		Hooks    map[string]any `json:"hooks"`
	}
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("manifest is not JSON: %v", err)
	}
	if m.Name != "assay" || m.Version != "1.2.3" {
		t.Fatalf("name/version = %q/%q, want assay/1.2.3 (derived from the Claude manifest)", m.Name, m.Version)
	}
	for _, rel := range []string{m.Rules, m.Skills} {
		if !strings.HasPrefix(rel, "./") || strings.Contains(rel, "..") || filepath.IsAbs(rel) {
			t.Fatalf("component path %q is not a relative in-bundle path", rel)
		}
		if _, err := os.Stat(filepath.Join(bundle, rel)); err != nil {
			t.Fatalf("component path %q does not resolve inside the bundle: %v", rel, err)
		}
	}
	if m.Commands == nil || len(m.Commands) != 0 {
		t.Fatalf("commands = %v, want an explicit empty array", m.Commands)
	}
	if m.Hooks == nil || len(m.Hooks) != 0 {
		t.Fatalf("hooks = %v, want an explicit empty object", m.Hooks)
	}
}

// TestCursorCheckDetectsManifestVersionSkew: a version planted into the Cursor
// manifest (a hand-edit, or a stamp that skipped it) is drift, naming the file.
func TestCursorCheckDetectsManifestVersionSkew(t *testing.T) {
	bundle := writeMinimalCursorBundle(t)
	manifest := filepath.Join(bundle, ".cursor-plugin", "plugin.json")
	raw, err := os.ReadFile(manifest)
	if err != nil {
		t.Fatal(err)
	}
	skewed := strings.Replace(string(raw), `"version": "1.2.3"`, `"version": "9.9.9"`, 1)
	if skewed == string(raw) {
		t.Fatal("test setup: expected to find the source version 1.2.3 to skew")
	}
	if err := os.WriteFile(manifest, []byte(skewed), 0o644); err != nil {
		t.Fatal(err)
	}
	var code int
	stderr := captureStderr(t, func() { code = cursorCmd([]string{"--check", "--bundle", bundle}) })
	if code != exitDrift {
		t.Fatalf("check with a skewed Cursor manifest version returned %d, want %d (drift)", code, exitDrift)
	}
	if !strings.Contains(stderr, filepath.Join(".cursor-plugin", "plugin.json")) {
		t.Fatalf("drift report did not name the Cursor manifest; stderr:\n%s", stderr)
	}
	if err := os.WriteFile(manifest, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	if code := cursorCmd([]string{"--check", "--bundle", bundle}); code != exitClean {
		t.Fatalf("check after restore returned %d, want %d (clean)", code, exitClean)
	}
}

// TestCursorMissingManifestIsCouldNotCheck: a committed rule with no committed
// Cursor manifest is could-not-check, never a clean pass on the rule alone.
func TestCursorMissingManifestIsCouldNotCheck(t *testing.T) {
	bundle := writeMinimalCursorBundle(t)
	if err := os.Remove(filepath.Join(bundle, ".cursor-plugin", "plugin.json")); err != nil {
		t.Fatal(err)
	}
	if code := cursorCmd([]string{"--check", "--bundle", bundle}); code != exitCouldNotCheck {
		t.Fatalf("check with no Cursor manifest returned %d, want %d (could-not-check)", code, exitCouldNotCheck)
	}
}

// TestCursorMissingMetadataIsCouldNotCheck: without the Claude manifest there is
// no source to derive the Cursor manifest from.
func TestCursorMissingMetadataIsCouldNotCheck(t *testing.T) {
	bundle := writeMinimalCursorBundle(t)
	if err := os.Remove(filepath.Join(bundle, ".claude-plugin", "plugin.json")); err != nil {
		t.Fatal(err)
	}
	if code := cursorCmd([]string{"--check", "--bundle", bundle}); code != exitCouldNotCheck {
		t.Fatalf("check with no Claude manifest returned %d, want %d (could-not-check)", code, exitCouldNotCheck)
	}
}

// TestCursorRootMarketplace: in --root mode the verb also generates and checks
// the root .cursor-plugin/marketplace.json. Its entry carries no version (the
// plugin manifest is the one version location), a planted version is drift,
// and a catalog that does not list the plugin at plugins/assay is could-not-check.
func TestCursorRootMarketplace(t *testing.T) {
	root := t.TempDir()
	writeCursorBundleAt(t, filepath.Join(root, "plugins", "assay"))
	catalog := filepath.Join(root, ".claude-plugin", "marketplace.json")
	if err := os.MkdirAll(filepath.Dir(catalog), 0o755); err != nil {
		t.Fatal(err)
	}
	catalogBody := `{
  "name": "assay",
  "owner": { "name": "example-owner" },
  "plugins": [
    { "name": "assay", "source": "./plugins/assay", "version": "1.2.3" }
  ]
}
`
	if err := os.WriteFile(catalog, []byte(catalogBody), 0o644); err != nil {
		t.Fatal(err)
	}
	if code := cursorCmd([]string{"--root", root}); code != exitClean {
		t.Fatalf("root-mode write returned %d, want %d", code, exitClean)
	}
	if code := cursorCmd([]string{"--check", "--root", root}); code != exitClean {
		t.Fatalf("root-mode check-after-write returned %d, want %d", code, exitClean)
	}
	mkt := filepath.Join(root, ".cursor-plugin", "marketplace.json")
	raw, err := os.ReadFile(mkt)
	if err != nil {
		t.Fatal(err)
	}
	var m struct {
		Name  string `json:"name"`
		Owner struct {
			Name string `json:"name"`
		} `json:"owner"`
		Plugins []map[string]any `json:"plugins"`
	}
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("marketplace is not JSON: %v", err)
	}
	if m.Name != "assay" || m.Owner.Name != "example-owner" || len(m.Plugins) != 1 {
		t.Fatalf("marketplace = %s, want name assay, owner example-owner, one plugin", raw)
	}
	if m.Plugins[0]["name"] != "assay" || m.Plugins[0]["source"] != "./plugins/assay" {
		t.Fatalf("marketplace entry = %v, want assay at ./plugins/assay", m.Plugins[0])
	}
	if _, has := m.Plugins[0]["version"]; has {
		t.Fatalf("marketplace entry carries a version — the plugin manifest must be the one Cursor version location")
	}

	// A planted version in the entry is drift, naming the marketplace.
	planted := strings.Replace(string(raw), `"source": "./plugins/assay"`, `"source": "./plugins/assay",
      "version": "0.0.1"`, 1)
	if err := os.WriteFile(mkt, []byte(planted), 0o644); err != nil {
		t.Fatal(err)
	}
	var code int
	stderr := captureStderr(t, func() { code = cursorCmd([]string{"--check", "--root", root}) })
	if code != exitDrift || !strings.Contains(stderr, "marketplace.json") {
		t.Fatalf("planted marketplace version: code %d (want %d), stderr:\n%s", code, exitDrift, stderr)
	}

	// A catalog with no assay entry at that source is could-not-check.
	if err := os.WriteFile(catalog, []byte(strings.Replace(catalogBody, "./plugins/assay", "./plugins/other", 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	if code := cursorCmd([]string{"--check", "--root", root}); code != exitCouldNotCheck {
		t.Fatalf("catalog without the plugin entry returned %d, want %d (could-not-check)", code, exitCouldNotCheck)
	}
}

// --- a missing resident source is could-not-check ------------------------------

func TestCursorMissingSourceIsCouldNotCheck(t *testing.T) {
	bundle := t.TempDir() // no resident-rules.md at all
	if code := cursorCmd([]string{"--check", "--bundle", bundle}); code != exitCouldNotCheck {
		t.Fatalf("check with no resident source returned %d, want %d (could-not-check)", code, exitCouldNotCheck)
	}
}

// --- an empty coverage roster is could-not-check, never a clean pass -----------

func TestCursorEmptyRosterIsCouldNotCheck(t *testing.T) {
	bundle := writeMinimalCursorBundle(t)
	packaging := filepath.Join(bundle, "cursor", "packaging.md")
	if err := os.WriteFile(packaging, []byte("<!-- assay:cursor-packaging\n-->\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if code := cursorCmd([]string{"--check", "--bundle", bundle}); code != exitCouldNotCheck {
		t.Fatalf("check with an empty roster returned %d, want %d (could-not-check)", code, exitCouldNotCheck)
	}
}

// --- write then check round-trips clean ---------------------------------------

func TestCursorWriteThenCheckClean(t *testing.T) {
	bundle := writeMinimalCursorBundle(t)
	if code := cursorCmd([]string{"--bundle", bundle}); code != exitClean {
		t.Fatalf("write returned %d, want %d", code, exitClean)
	}
	if code := cursorCmd([]string{"--check", "--bundle", bundle}); code != exitClean {
		t.Fatalf("check-after-write returned %d, want %d (clean)", code, exitClean)
	}
}

// writeMinimalCursorBundle builds a self-contained plugins/assay-shaped bundle
// under a temp dir: a valid resident-rules source (header, R1..R10, footer), three
// skills, a Cursor binding file with a degradation cell per skill, a full coverage
// roster, and a generated .mdc rule. Fully controlled — the tests mutate exactly
// one thing at a time.
func writeMinimalCursorBundle(t *testing.T) string {
	t.Helper()
	bundle := t.TempDir()
	writeCursorBundleAt(t, bundle)
	if code := cursorCmd([]string{"--bundle", bundle}); code != exitClean {
		t.Fatalf("writeMinimalCursorBundle: generating the baseline artifacts returned %d, want %d", code, exitClean)
	}
	return bundle
}

// writeCursorBundleAt writes the minimal bundle's sources into bundle without
// generating anything (the root-mode tests place it under <root>/plugins/assay).
func writeCursorBundleAt(t *testing.T, bundle string) {
	t.Helper()

	mustWrite := func(rel, content string) {
		p := filepath.Join(bundle, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	// Build a valid source: `## Header`, `## R1 TITLE` .. `## R10 TITLE` (the
	// heading form ruleHeadingRE `^## R(\d+) ` matches), and `## Footer`.
	var src strings.Builder
	src.WriteString("## Header\n\nRESIDENT OPERATING RULES (test).\n\n")
	for i := 1; i <= 10; i++ {
		src.WriteString("## R")
		src.WriteString(itoa(i))
		src.WriteString(" RULE")
		src.WriteString(itoa(i))
		src.WriteString("\n\nRULE")
		src.WriteString(itoa(i))
		src.WriteString(": body line for rule ")
		src.WriteString(itoa(i))
		src.WriteString(".\n\n")
	}
	src.WriteString("## Footer\n\nSee the skill bodies.\n")
	mustWrite("resident-rules.md", src.String())

	mustWrite(".claude-plugin/plugin.json", `{
  "name": "assay",
  "version": "1.2.3",
  "license": "Apache-2.0",
  "description": "test bundle <name>",
  "author": { "name": "medici-finance" },
  "homepage": "https://example.invalid/assay",
  "repository": "https://example.invalid/assay",
  "keywords": ["a", "b"]
}
`)

	skills := []string{"adopt", "author-brief", "the-desk"}
	for _, s := range skills {
		mustWrite(filepath.Join("skills", s, "SKILL.md"),
			"---\nname: "+s+"\ndescription: "+s+" test skill\n---\n# "+s+"\n")
	}

	mustWrite("references/cursor.md", "# Cursor bindings\n\n| Skill | Cursor |\n|---|---|\n"+
		"| `adopt` | runs |\n| `author-brief` | runs |\n| `the-desk` | runs |\n")

	mustWrite("cursor/packaging.md", "<!-- assay:cursor-packaging\n"+
		"adopt\nauthor-brief\nthe-desk\n"+
		"# excluded entries: <name> :: EXCLUDED: <reason>\n-->\n")
}

// itoa is a tiny int→string helper so the fixture builder needs no strconv import
// churn beyond the standard test set (kept local, single-digit + ten only).
func itoa(i int) string {
	if i < 10 {
		return string(rune('0' + i))
	}
	return string(rune('0'+i/10)) + string(rune('0'+i%10))
}
