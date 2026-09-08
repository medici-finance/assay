package main

import (
	_ "embed"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// payloadGolden is the EXACT SessionStart payload the hook heredoc carried
// before the single-source refactor (extracted from inject-resident-rules.sh at
// the landing commit). The Claude payload must survive the refactor
// byte-identical in its rules text — rule-content changes are their own PRs,
// never smuggled into plumbing. This locks that guarantee: if the generator's
// output ever diverges from this byte string, the test fails.
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
	return root
}

// --- byte-identity to the pre-refactor payload -----------------------------

func TestClaudePayloadIsByteIdenticalToGolden(t *testing.T) {
	arts, err := generateFromString(realSource(t))
	if err != nil {
		t.Fatalf("generating from committed source: %v", err)
	}
	if arts.ClaudePayload != payloadGolden {
		t.Fatalf("Claude payload diverged from the pre-refactor heredoc.\n"+
			"got %d bytes, golden %d bytes.\nThis is the smuggled-content failure the brief forbids.",
			len(arts.ClaudePayload), len(payloadGolden))
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
