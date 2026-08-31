package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// archivelinkfallback_test.go — regression for the link/backtick ARCHIVE fallback.
//
// #259 taught the EDGE resolver (depends:/unblocks:/affects:) that a stream moved
// off the active board into docs/archive/<stream>/ is still a known target. It did
// NOT teach the link/backtick check the same relocation: a markdown link or a
// backticked path into docs/streams/<stream>/X, valid while the stream was active,
// regressed to a hard "dead link" / "backticked path does not exist" PROBLEM the
// moment the stream's files moved to docs/archive/<stream>/X. archivedStreamFallbackExists
// closes that gap — the link/backtick parallel of #259.
//
// The boundary is the other half: a target present under NEITHER docs/streams/ nor
// docs/archive/ is genuinely broken and must STILL be reported. The fallback adds a
// resolution base; it never blanket-suppresses missing-file detection.

// writeArchiveLinkFixture materializes a tree with:
//   - an active source doc under docs/streams/refs/ carrying (a) a backticked path
//     and a markdown link into a stream that now lives under docs/archive/, and
//     (b) CONTROL references into a path that exists NOWHERE;
//   - an ARCHIVED stream `completed-core` under docs/archive/ holding the real
//     target files (README.md + a brief), so the fallback has something to resolve.
func writeArchiveLinkFixture(t *testing.T) (root, sourceFile string) {
	t.Helper()
	root = t.TempDir()

	// The archived stream's real files live under docs/archive/completed-core/.
	arcDir := filepath.Join(root, "docs", "archive", "completed-core")
	if err := os.MkdirAll(arcDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFixtureFile(t, filepath.Join(arcDir, "README.md"), "# Completed Core\n")
	writeFixtureFile(t, filepath.Join(arcDir, "brief-07-legacy.md"), "# Legacy brief\n")

	// The active source doc that references the (now archived) stream.
	srcDir := filepath.Join(root, "docs", "streams", "refs")
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := "# Refs into an archived stream\n\n" +
		// (a) a BACKTICKED root-relative path into the archived stream — the exact
		// shape of the measured PROBLEM (docs/streams/<s>/README.md moved to archive).
		"Backtick into archived: `docs/streams/completed-core/README.md`.\n" +
		// (a) a MARKDOWN link (../<s>/…) into the archived stream's brief.
		"Link into archived: [legacy](../completed-core/brief-07-legacy.md).\n" +
		// (b) CONTROL: a backticked path that exists in NEITHER tree — still a PROBLEM.
		"Backtick nowhere: `docs/streams/ghost-nowhere/README.md`.\n" +
		// (b) CONTROL: a markdown link that exists in NEITHER tree — still a PROBLEM.
		"Link nowhere: [gone](../ghost-nowhere/brief-01-missing.md).\n"
	sourceFile = writeTemp(t, srcDir, "README.md", content)
	return root, sourceFile
}

// TestArchivedStreamLinkFallbackResolves is the RED-before-GREEN fixture: before
// archivedStreamFallbackExists was wired in, the backtick and the markdown link
// into docs/archive/completed-core/ each produced a PROBLEM (4 total). After, only
// the two genuinely-missing controls remain (2). Running this test against the
// pre-fix code reddens on `got 2 problems, want ...`/the archived refs appearing.
func TestArchivedStreamLinkFallbackResolves(t *testing.T) {
	root, src := writeArchiveLinkFixture(t)
	problems := linkProblems(root, []string{src})

	// The two references into the ARCHIVED stream must NOT be flagged.
	for _, gone := range []string{"completed-core/README.md", "completed-core/brief-07-legacy.md"} {
		for _, p := range problems {
			if strings.Contains(p, gone) {
				t.Errorf("archived target %q should resolve via docs/archive/ fallback, still flagged: %v", gone, p)
			}
		}
	}

	// The two genuinely-missing CONTROLS must STILL be flagged (boundary preserved).
	for _, want := range []string{"ghost-nowhere/README.md", "ghost-nowhere/brief-01-missing.md"} {
		found := false
		for _, p := range problems {
			if strings.Contains(p, want) {
				found = true
			}
		}
		if !found {
			t.Errorf("genuinely-missing control %q must still PROBLEM (fallback must not blanket-suppress): %v", want, problems)
		}
	}

	if len(problems) != 2 {
		t.Fatalf("got %d problems, want exactly 2 (both ghost-nowhere controls, neither archived ref): %v", len(problems), problems)
	}
}

// TestArchivedStreamFallbackExistsUnit exercises the helper directly, including the
// two boundaries: a non-docs/streams path is never rewritten, and a target absent
// from both trees does not resolve.
func TestArchivedStreamFallbackExistsUnit(t *testing.T) {
	root := t.TempDir()
	arc := filepath.Join(root, "docs", "archive", "s")
	if err := os.MkdirAll(arc, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFixtureFile(t, filepath.Join(arc, "file.md"), "x")

	// A docs/streams/s/file.md target relocates to docs/archive/s/file.md — resolves.
	if !archivedStreamFallbackExists(root, filepath.Join(root, "docs", "streams", "s", "file.md")) {
		t.Error("docs/streams/s/file.md should resolve against docs/archive/s/file.md")
	}
	// A docs/streams/s/absent.md target has no archived twin — does not resolve.
	if archivedStreamFallbackExists(root, filepath.Join(root, "docs", "streams", "s", "absent.md")) {
		t.Error("docs/streams/s/absent.md has no archived twin; must not resolve")
	}
	// A path outside docs/streams/ is never rewritten to docs/archive/.
	if archivedStreamFallbackExists(root, filepath.Join(root, "docs", "articles", "x.md")) {
		t.Error("a non-docs/streams path must never be rewritten into docs/archive/")
	}
}
