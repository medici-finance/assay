package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- committed manifest is in sync with the committed metadata source ---------

// TestCodexCommittedManifestMatchesSource is the codex analogue of
// TestCommittedArtifactsMatchSource: the shipped .codex-plugin/plugin.json must
// be exactly what the committed .claude-plugin/plugin.json would generate. If it
// drifts, this fails in the same suite CI runs (Verify row 4).
func TestCodexCommittedManifestMatchesSource(t *testing.T) {
	if code := codexCmd([]string{"--check", "--root", "../.."}); code != exitClean {
		t.Fatalf("codex --check against the repo root returned %d, want %d (clean) — "+
			"the committed manifest is out of sync with the metadata source; run `go run ./tools/harnessgen codex` and commit", code, exitClean)
	}
}

// --- version skew is impossible: the two manifests carry the same version ------

func TestCodexManifestVersionEqualsClaude(t *testing.T) {
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
	codex := readVersion("../../plugins/assay/.codex-plugin/plugin.json")
	if claude != codex {
		t.Fatalf("version skew: Claude manifest %q != Codex manifest %q — the Codex manifest must be generated from the Claude one", claude, codex)
	}
}

// --- a planted version skew (a hand-edit) is caught, naming the manifest -------

func TestCodexCheckDetectsVersionSkew(t *testing.T) {
	bundle := writeMinimalBundle(t)
	// Generate the manifest clean.
	if code := codexCmd([]string{"--bundle", bundle}); code != exitClean {
		t.Fatalf("write returned %d, want %d", code, exitClean)
	}
	manifest := filepath.Join(bundle, ".codex-plugin", "plugin.json")
	raw, err := os.ReadFile(manifest)
	if err != nil {
		t.Fatal(err)
	}
	skewed := strings.Replace(string(raw), `"version": "9.9.9"`, "", 1) // sanity
	if skewed != string(raw) {
		t.Fatal("test setup: minimal bundle unexpectedly already had version 9.9.9")
	}
	// Plant a skew directly (mirrors Verify row 3a's jq edit).
	skewed = strings.Replace(string(raw), `"version": "1.2.3"`, `"version": "9.9.9"`, 1)
	if skewed == string(raw) {
		t.Fatal("test setup: expected to find the source version 1.2.3 to skew")
	}
	if err := os.WriteFile(manifest, []byte(skewed), 0o644); err != nil {
		t.Fatal(err)
	}
	var code int
	stderr := captureStderr(t, func() { code = codexCmd([]string{"--check", "--bundle", bundle}) })
	if code != exitDrift {
		t.Fatalf("check with a skewed manifest version returned %d, want %d (drift)", code, exitDrift)
	}
	if !strings.Contains(stderr, "plugin.json") {
		t.Fatalf("drift report did not name the manifest; stderr:\n%s", stderr)
	}
	// After restoring, --check passes again (row 3a's second half).
	if err := os.WriteFile(manifest, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	if code := codexCmd([]string{"--check", "--bundle", bundle}); code != exitClean {
		t.Fatalf("check after restore returned %d, want %d (clean)", code, exitClean)
	}
}

// --- coverage: a skill on disk with no roster/exclusion entry is exit 2 --------

func TestCodexCoverageCatchesUnaccountedSkill(t *testing.T) {
	bundle := writeMinimalBundle(t)
	if code := codexCmd([]string{"--check", "--bundle", bundle}); code != exitClean {
		t.Fatalf("baseline check returned %d, want %d", code, exitClean)
	}
	// Add a skill to the tree without a roster/exclusion entry (Verify row 5).
	probe := filepath.Join(bundle, "skills", "probe-skill")
	if err := os.MkdirAll(probe, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(probe, "SKILL.md"), []byte("---\nname: probe-skill\ndescription: probe\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var code int
	stderr := captureStderr(t, func() { code = codexCmd([]string{"--check", "--bundle", bundle}) })
	if code != exitCouldNotCheck {
		t.Fatalf("check with an unaccounted skill returned %d, want %d (could-not-check)", code, exitCouldNotCheck)
	}
	if !strings.Contains(stderr, "probe-skill") {
		t.Fatalf("coverage failure did not name the unaccounted skill; stderr:\n%s", stderr)
	}
}

// --- packaging↔binding skew: a packaged skill with no degradation cell is caught

func TestCodexBindingSkewCaught(t *testing.T) {
	bundle := writeMinimalBundle(t)
	if code := codexCmd([]string{"--check", "--bundle", bundle}); code != exitClean {
		t.Fatalf("baseline check returned %d, want %d", code, exitClean)
	}
	// Remove one packaged skill's degradation cell from the binding file (the
	// property Verify row 6 exercises — its literal token `batch-fanout` is stale
	// since the methodology/46 rename to `worker-desk`, so this test uses a real
	// packaged skill name instead).
	binding := filepath.Join(bundle, "references", "codex.md")
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
	stderr := captureStderr(t, func() { code = codexCmd([]string{"--check", "--bundle", bundle}) })
	if code != exitCouldNotCheck {
		t.Fatalf("check with a missing binding cell returned %d, want %d (could-not-check)", code, exitCouldNotCheck)
	}
	if !strings.Contains(stderr, "the-desk") {
		t.Fatalf("binding-skew failure did not name the skill; stderr:\n%s", stderr)
	}
}

// --- an excluded entry with an empty reason is a parse error (could-not-check) -

func TestCodexExcludedEmptyReasonIsParseError(t *testing.T) {
	bundle := writeMinimalBundle(t)
	packaging := filepath.Join(bundle, "codex", "packaging.md")
	raw, err := os.ReadFile(packaging)
	if err != nil {
		t.Fatal(err)
	}
	// Turn a packaged line into an excluded one with an empty reason.
	edited := strings.Replace(string(raw), "author-brief\n", "author-brief :: EXCLUDED:\n", 1)
	if edited == string(raw) {
		t.Fatal("test setup: expected to find author-brief in the roster")
	}
	if err := os.WriteFile(packaging, []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}
	var code int
	stderr := captureStderr(t, func() { code = codexCmd([]string{"--check", "--bundle", bundle}) })
	if code != exitCouldNotCheck {
		t.Fatalf("check with an empty-reason exclusion returned %d, want %d (could-not-check)", code, exitCouldNotCheck)
	}
	if !strings.Contains(stderr, "empty reason") {
		t.Fatalf("parse error did not explain the empty reason; stderr:\n%s", stderr)
	}
}

// --- an excluded skill still on disk is incoherent (the pointer would ship it) -

func TestCodexExcludedSkillStillOnDiskIsCaught(t *testing.T) {
	bundle := writeMinimalBundle(t)
	packaging := filepath.Join(bundle, "codex", "packaging.md")
	raw, err := os.ReadFile(packaging)
	if err != nil {
		t.Fatal(err)
	}
	// Exclude author-brief WITH a reason, but leave its SKILL.md on disk.
	edited := strings.Replace(string(raw), "author-brief\n", "author-brief :: EXCLUDED: HP/03 says it cannot exist\n", 1)
	if edited == string(raw) {
		t.Fatal("test setup: expected to find author-brief in the roster")
	}
	if err := os.WriteFile(packaging, []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}
	var code int
	stderr := captureStderr(t, func() { code = codexCmd([]string{"--check", "--bundle", bundle}) })
	if code != exitCouldNotCheck {
		t.Fatalf("check with an excluded-but-present skill returned %d, want %d (could-not-check)", code, exitCouldNotCheck)
	}
	if !strings.Contains(stderr, "author-brief") || !strings.Contains(stderr, "pointer") {
		t.Fatalf("coherence failure did not explain the pointer contradiction; stderr:\n%s", stderr)
	}
}

// --- write then check round-trips clean ---------------------------------------

func TestCodexWriteThenCheckClean(t *testing.T) {
	bundle := writeMinimalBundle(t)
	if code := codexCmd([]string{"--bundle", bundle}); code != exitClean {
		t.Fatalf("write returned %d, want %d", code, exitClean)
	}
	if code := codexCmd([]string{"--check", "--bundle", bundle}); code != exitClean {
		t.Fatalf("check-after-write returned %d, want %d (clean)", code, exitClean)
	}
}

// --- a missing/incomplete metadata source is could-not-check ------------------

func TestCodexMissingMetadataIsCouldNotCheck(t *testing.T) {
	bundle := t.TempDir() // no .claude-plugin/plugin.json at all
	if code := codexCmd([]string{"--check", "--bundle", bundle}); code != exitCouldNotCheck {
		t.Fatalf("check with no metadata source returned %d, want %d (could-not-check)", code, exitCouldNotCheck)
	}
}

// --- an empty coverage roster is could-not-check, never a clean pass -----------

func TestCodexEmptyRosterIsCouldNotCheck(t *testing.T) {
	bundle := writeMinimalBundle(t)
	packaging := filepath.Join(bundle, "codex", "packaging.md")
	// Marker present but block empty.
	if err := os.WriteFile(packaging, []byte("<!-- assay:codex-packaging\n-->\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if code := codexCmd([]string{"--check", "--bundle", bundle}); code != exitCouldNotCheck {
		t.Fatalf("check with an empty roster returned %d, want %d (could-not-check)", code, exitCouldNotCheck)
	}
}

// writeMinimalBundle builds a self-contained plugins/assay-shaped bundle under a
// temp dir: metadata source (version 1.2.3), three skills, a binding file with a
// degradation cell per skill, a full coverage roster, and a generated manifest.
// Fully controlled — the tests mutate exactly one thing at a time.
func writeMinimalBundle(t *testing.T) string {
	t.Helper()
	bundle := t.TempDir()

	mustWrite := func(rel, content string) {
		p := filepath.Join(bundle, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

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

	mustWrite("references/codex.md", "# Codex bindings\n\n| Skill | Codex CLI |\n|---|---|\n"+
		"| `adopt` | runs |\n| `author-brief` | runs |\n| `the-desk` | runs |\n")

	mustWrite("codex/packaging.md", "<!-- assay:codex-packaging\n"+
		"adopt\nauthor-brief\nthe-desk\n"+
		"# excluded entries: <name> :: EXCLUDED: <reason>\n-->\n")

	// Generate the manifest so a baseline --check is clean before any mutation.
	if code := codexCmd([]string{"--bundle", bundle}); code != exitClean {
		t.Fatalf("writeMinimalBundle: generating the baseline manifest returned %d, want %d", code, exitClean)
	}

	return bundle
}
