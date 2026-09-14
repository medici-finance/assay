package main

import (
	_ "embed"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// payloadGolden is the EXACT SessionStart payload the generator currently
// produces from the committed source and the committed plugin manifest
// version. It originally locked the byte-for-byte content the hook heredoc
// carried before the single-source refactor; it was updated (#730) when
// the Header's version went from a hand-typed literal ("v0.1.0", stale
// against plugin.json's "1.0.0") to the {{VERSION}} token generate() resolves
// against the manifest — a deliberate, documented content change, not a
// smuggled one. Rule-content changes are their own PRs, never smuggled into
// plumbing; this locks that guarantee: if the generator's output ever
// diverges from this byte string, the test fails. The golden carries the
// {{VERSION}} token in its Header: the release workflow stamps plugin.json at
// every umbrella cut (#789), so the golden is version-agnostic and locks only
// the CONTENT — the test resolves the token against the committed manifest
// before comparing, exactly as generate() does.
//
//go:embed testdata/payload.golden.txt
var payloadGolden string

// realSource reads the committed single source relative to this module.
func realSource(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile(filepath.FromSlash("../../plugins/assay/resident-rules.md"))
	if err != nil {
		t.Fatalf("reading committed source: %v", err)
	}
	return string(b)
}

// realVersion reads the committed plugin manifest's version relative to this
// module — the value the Header's {{VERSION}} token must resolve to.
func realVersion(t *testing.T) string {
	t.Helper()
	meta, err := readClaudeManifest(filepath.FromSlash("../../plugins/assay/.claude-plugin/plugin.json"))
	if err != nil {
		t.Fatalf("reading committed plugin manifest: %v", err)
	}
	return meta.Version
}

// captureStderr runs fn with os.Stderr redirected to a buffer and returns what
// was written. The check verb reports drifting/unreadable filenames on stderr;
// the tests assert on those names.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	orig := os.Stderr
	os.Stderr = w
	defer func() { os.Stderr = orig }()
	done := make(chan string, 1)
	go func() {
		var sb strings.Builder
		buf := make([]byte, 4096)
		for {
			n, err := r.Read(buf)
			if n > 0 {
				sb.Write(buf[:n])
			}
			if err != nil {
				break
			}
		}
		done <- sb.String()
	}()
	fn()
	w.Close()
	out := <-done
	r.Close()
	return out
}

// setupRoot writes srcContent as the source under a fresh temp root laid out
// like the repo (plugins/assay/resident-rules.md) and returns the root.
// testPluginVersion is the fixture version setupRoot writes into the plugin
// manifest it fabricates — deliberately not the real plugin.json's version,
// so a test asserting on it can't accidentally pass by reading the real file.
const testPluginVersion = "9.9.9"

func setupRoot(t *testing.T, srcContent string) string {
	t.Helper()
	root := t.TempDir()
	src := filepath.Join(root, "plugins", "assay", "resident-rules.md")
	if err := os.MkdirAll(filepath.Dir(src), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(src, []byte(srcContent), 0o644); err != nil {
		t.Fatal(err)
	}
	writeFixtureManifest(t, root, testPluginVersion)
	return root
}

// writeFixtureManifest writes a minimal plugin.json fixture under root so
// residentCmd can resolve the Header's {{VERSION}} token — real resident-rules.md
// carries the token (#730), so every setupRoot-based test needs a readable
// manifest even when it isn't itself testing version derivation.
func writeFixtureManifest(t *testing.T, root, version string) {
	t.Helper()
	manifest := filepath.Join(root, "plugins", "assay", ".claude-plugin", "plugin.json")
	if err := os.MkdirAll(filepath.Dir(manifest), 0o755); err != nil {
		t.Fatal(err)
	}
	body := `{"name": "assay", "version": "` + version + `"}` + "\n"
	if err := os.WriteFile(manifest, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// --- byte-identity to the pre-refactor payload -----------------------------

func TestClaudePayloadIsByteIdenticalToGolden(t *testing.T) {
	arts, err := generateFromString(realSource(t), realVersion(t))
	if err != nil {
		t.Fatalf("generating from committed source: %v", err)
	}
	want := strings.Replace(payloadGolden, versionPlaceholder, "v"+realVersion(t), 1)
	if arts.ClaudePayload != want {
		t.Fatalf("Claude payload diverged from the pre-refactor heredoc.\n"+
			"got %d bytes, golden %d bytes.\nThis is the smuggled-content failure the brief forbids.",
			len(arts.ClaudePayload), len(want))
	}
}

// --- committed artifacts are in sync with the committed source --------------

func TestCommittedArtifactsMatchSource(t *testing.T) {
	if code := residentCmd([]string{"--check", "--root", "../.."}); code != exitClean {
		t.Fatalf("resident --check against the repo root returned %d, want %d (clean) — "+
			"the committed artifacts are out of sync with the source; run the generator and commit", code, exitClean)
	}
}

// --- write then check round-trips clean -------------------------------------

func TestWriteThenCheckClean(t *testing.T) {
	root := setupRoot(t, realSource(t))
	if code := residentCmd([]string{"--root", root}); code != exitClean {
		t.Fatalf("write returned %d, want %d", code, exitClean)
	}
	if code := residentCmd([]string{"--check", "--root", root}); code != exitClean {
		t.Fatalf("check-after-write returned %d, want %d (clean)", code, exitClean)
	}
}

// --- a modified committed artifact is caught, and the file is named ---------

func TestCheckDetectsModifiedArtifact(t *testing.T) {
	root := setupRoot(t, realSource(t))
	if code := residentCmd([]string{"--root", root}); code != exitClean {
		t.Fatalf("write returned %d", code)
	}
	frag := filepath.Join(root, "plugins", "assay", "codex", "AGENTS-assay.md")
	b, err := os.ReadFile(frag)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(frag, append(b, []byte("\nDRIFT-PROBE\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
	var code int
	stderr := captureStderr(t, func() { code = residentCmd([]string{"--check", "--root", root}) })
	if code != exitDrift {
		t.Fatalf("check with a tampered fragment returned %d, want %d (drift)", code, exitDrift)
	}
	if !strings.Contains(stderr, "AGENTS-assay.md") {
		t.Fatalf("drift report did not name the tampered file; stderr:\n%s", stderr)
	}
}

// --- a source rule change propagates to BOTH artifacts (single-source) ------

func TestSourceRuleChangeDriftsBothArtifacts(t *testing.T) {
	root := setupRoot(t, realSource(t))
	if code := residentCmd([]string{"--root", root}); code != exitClean {
		t.Fatalf("write returned %d", code)
	}
	// Edit one rule's body in the source; the committed artifacts (unchanged)
	// must now BOTH report drift — the rule reaches every delivery artifact from
	// the one source.
	src := filepath.Join(root, "plugins", "assay", "resident-rules.md")
	raw, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	edited := strings.Replace(string(raw), "REDACTION:", "REDACTION (edited):", 1)
	if edited == string(raw) {
		t.Fatal("test setup: expected to find the REDACTION rule body to edit")
	}
	if err := os.WriteFile(src, []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}
	var code int
	stderr := captureStderr(t, func() { code = residentCmd([]string{"--check", "--root", root}) })
	if code != exitDrift {
		t.Fatalf("check after a source rule edit returned %d, want %d (drift)", code, exitDrift)
	}
	if !strings.Contains(stderr, "resident-rules.payload.txt") || !strings.Contains(stderr, "AGENTS-assay.md") {
		t.Fatalf("a source rule edit must drift BOTH artifacts; stderr named only:\n%s", stderr)
	}
}

// --- unparseable / empty source is could-not-check, never a clean pass ------

func TestUnparseableSourceIsCouldNotCheck(t *testing.T) {
	cases := []struct {
		name string
		src  string
	}{
		{"empty", ""},
		{"whitespace only", "\n\n   \n"},
		{"no header", "## R1 X\nX: body\n"},
		{"nine rules", nineRuleSource(t)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := setupRoot(t, tc.src)
			if code := residentCmd([]string{"--check", "--root", root}); code != exitCouldNotCheck {
				t.Fatalf("check on %q source returned %d, want %d (could-not-check)", tc.name, code, exitCouldNotCheck)
			}
			if code := residentCmd([]string{"--root", root}); code != exitCouldNotCheck {
				t.Fatalf("write on %q source returned %d, want %d (could-not-check)", tc.name, code, exitCouldNotCheck)
			}
		})
	}
}

// --- absent source file is could-not-check ----------------------------------

func TestMissingSourceIsCouldNotCheck(t *testing.T) {
	root := t.TempDir() // no plugins/assay/resident-rules.md at all
	if code := residentCmd([]string{"--check", "--root", root}); code != exitCouldNotCheck {
		t.Fatalf("check with no source returned %d, want %d (could-not-check)", code, exitCouldNotCheck)
	}
}

// --- the Header's {{VERSION}} token derives from the plugin manifest (#730) ---

// TestHeaderVersionDerivedFromPluginManifest proves the fix for #730: the
// generated payload carries the version FROM plugins/assay/.claude-plugin/
// plugin.json, not a literal typed into resident-rules.md. setupRoot's fixture
// manifest deliberately uses a version (testPluginVersion) that never appears
// anywhere in the source text, so this can only pass if the value was actually
// read from the manifest.
func TestHeaderVersionDerivedFromPluginManifest(t *testing.T) {
	root := setupRoot(t, realSource(t))
	if code := residentCmd([]string{"--root", root}); code != exitClean {
		t.Fatalf("write returned %d, want %d", code, exitClean)
	}
	payload, err := os.ReadFile(filepath.Join(root, "plugins", "assay", "hooks", "resident-rules.payload.txt"))
	if err != nil {
		t.Fatal(err)
	}
	want := "assay plugin v" + testPluginVersion
	if !strings.Contains(string(payload), want) {
		t.Fatalf("generated payload does not carry the manifest version %q; head:\n%.120s", want, payload)
	}
	if strings.Contains(string(payload), versionPlaceholder) {
		t.Fatalf("generated payload still carries the unresolved %s token", versionPlaceholder)
	}
	frag, err := os.ReadFile(filepath.Join(root, "plugins", "assay", "codex", "AGENTS-assay.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(frag), want) {
		t.Fatalf("generated codex fragment does not carry the manifest version %q; head:\n%.160s", want, frag)
	}
}

// TestCheckDetectsPluginVersionDrift is the lint the issue asked for: a plugin
// manifest bumped without regenerating must redden `--check`, naming the stale
// artifacts, exactly the way any other source/artifact mismatch does.
func TestCheckDetectsPluginVersionDrift(t *testing.T) {
	root := setupRoot(t, realSource(t))
	if code := residentCmd([]string{"--root", root}); code != exitClean {
		t.Fatalf("write returned %d, want %d", code, exitClean)
	}
	// Bump the manifest version without regenerating — the committed artifacts
	// now name a stale plugin version, same shape as the bug that shipped.
	writeFixtureManifest(t, root, "10.0.0")
	var code int
	stderr := captureStderr(t, func() { code = residentCmd([]string{"--check", "--root", root}) })
	if code != exitDrift {
		t.Fatalf("check after a manifest version bump returned %d, want %d (drift)", code, exitDrift)
	}
	if !strings.Contains(stderr, "resident-rules.payload.txt") || !strings.Contains(stderr, "AGENTS-assay.md") {
		t.Fatalf("a manifest version bump must drift BOTH artifacts; stderr named only:\n%s", stderr)
	}
}

// TestMissingManifestIsCouldNotCheckWhenHeaderNeedsVersion: a Header carrying
// {{VERSION}} with no readable manifest is could-not-check, never a silent pass
// that ships the literal token or a stale guess.
func TestMissingManifestIsCouldNotCheckWhenHeaderNeedsVersion(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, "plugins", "assay", "resident-rules.md")
	if err := os.MkdirAll(filepath.Dir(src), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(src, []byte(realSource(t)), 0o644); err != nil {
		t.Fatal(err)
	}
	// Deliberately no .claude-plugin/plugin.json under root.
	if code := residentCmd([]string{"--root", root}); code != exitCouldNotCheck {
		t.Fatalf("write with no plugin manifest returned %d, want %d (could-not-check)", code, exitCouldNotCheck)
	}
	if code := residentCmd([]string{"--check", "--root", root}); code != exitCouldNotCheck {
		t.Fatalf("check with no plugin manifest returned %d, want %d (could-not-check)", code, exitCouldNotCheck)
	}
}

// --- parser structural guarantees -------------------------------------------

func TestParseSourceRejectsMalformed(t *testing.T) {
	good := realSource(t)
	if _, err := parseSource(good); err != nil {
		t.Fatalf("committed source should parse cleanly: %v", err)
	}

	// Header removed (anchor on the heading line, not the prose mention).
	if _, err := parseSource(strings.Replace(good, "\n## Header\n", "\n## NotHeader\n", 1)); err == nil {
		t.Fatal("parseSource accepted a source with no `## Header` section")
	}
	// Footer removed.
	if _, err := parseSource(strings.Replace(good, "\n## Footer\n", "\n## NotFooter\n", 1)); err == nil {
		t.Fatal("parseSource accepted a source with no `## Footer` section")
	}
	// A rule with an out-of-order number.
	reordered := strings.Replace(good, "## R2 ISOLATION", "## R3 ISOLATION", 1)
	if _, err := parseSource(reordered); err == nil {
		t.Fatal("parseSource accepted rules out of order (R1, R3, ...) — it must require R1..R10 in order")
	}
}

// nineRuleSource returns the committed source with rule R10's section removed,
// so the parser sees only nine rules.
func nineRuleSource(t *testing.T) string {
	t.Helper()
	src := realSource(t)
	idx := strings.Index(src, "\n## R10 ")
	if idx < 0 {
		t.Fatal("committed source has no R10 section to drop")
	}
	foot := strings.Index(src, "\n## Footer\n")
	if foot < 0 || foot < idx {
		t.Fatal("committed source has no Footer after R10")
	}
	return src[:idx] + src[foot:]
}
