package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeFile writes content at rel (slash path, relative to root), creating parents.
func writeFile(t *testing.T, root, rel, content string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("mkdir for %s: %v", rel, err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", rel, err)
	}
}

// forwardRefContent is a captured-run body that carries exactly the shapes an eval
// corpus legitimately contains and a live brief would be red for: a dead markdown
// link to a neighbour that does not exist, and a backticked deliverable path (under
// docs/streams/, extension in the link-check set) that does not exist and is not
// marked (planned).
const forwardRefContent = "# captured run\n\n" +
	"See [the next step](./missing-neighbor.md) for details.\n\n" +
	"It will create `docs/streams/somestream/deliverable-to-come.go` as its output.\n"

// TestLinkCheckHonoursDeclaredFixtureMarker proves the link-check side: a DECLARED
// fixture corpus is dropped from the checkable file set AND out of backtick scope,
// while an UNMARKED sibling with identical content is still fully link-checked
// (fail-closed).
func TestLinkCheckHonoursDeclaredFixtureMarker(t *testing.T) {
	root := t.TempDir()

	// Declared fixture corpus + a captured run carrying forward-references.
	if err := os.MkdirAll(filepath.Join(root, "docs/streams/somestream/evalcorpus"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, root, "docs/streams/somestream/evalcorpus/"+fixtureCorpusMarkerName, "")
	writeFile(t, root, "docs/streams/somestream/evalcorpus/run-01.md", forwardRefContent)
	// UNMARKED live brief with the SAME forward-referencing content.
	writeFile(t, root, "docs/streams/somestream/README.md", forwardRefContent)

	markedFile := filepath.Join(root, "docs/streams/somestream/evalcorpus/run-01.md")
	liveFile := filepath.Join(root, "docs/streams/somestream/README.md")

	// (1) docFiles must EXCLUDE the marked corpus file and INCLUDE the live brief.
	docs, walkProblems := docFiles(root)
	if len(walkProblems) != 0 {
		t.Fatalf("unexpected walk problems: %v", walkProblems)
	}
	var sawMarked, sawLive bool
	for _, f := range docs {
		if f == markedFile {
			sawMarked = true
		}
		if f == liveFile {
			sawLive = true
		}
	}
	if sawMarked {
		t.Errorf("docFiles included %s — a file in a DECLARED fixture corpus must be excluded", markedFile)
	}
	if !sawLive {
		t.Errorf("docFiles dropped the UNMARKED live brief %s — fail-closed: only declared corpora are excluded", liveFile)
	}

	// (2) backtickPathScope: false for the marked file, true for the unmarked one.
	if backtickPathScope(root, markedFile) {
		t.Errorf("backtickPathScope true for a declared-corpus file — the strict backtick convention must not apply to it")
	}
	if !backtickPathScope(root, liveFile) {
		t.Errorf("backtickPathScope false for an UNMARKED docs/streams brief — fail-closed: it must stay in scope")
	}

	// (3) End-to-end over docFiles: the live brief's dead link and non-existent
	// backticked path are flagged; the marked corpus's identical content is NOT.
	problems := linkProblems(root, docs)
	var liveFlagged, markedFlagged bool
	for _, p := range problems {
		if strings.Contains(p, "docs/streams/somestream/README.md") {
			liveFlagged = true
		}
		if strings.Contains(p, "evalcorpus") {
			markedFlagged = true
		}
	}
	if !liveFlagged {
		t.Errorf("the UNMARKED live brief's forward-refs were NOT flagged — fail-closed direction broke; problems=%v", problems)
	}
	if markedFlagged {
		t.Errorf("a DECLARED fixture corpus's forward-refs WERE flagged — the exclusion must suppress them; problems=%v", problems)
	}
}

// TestIsFixtureCorpusPath pins the DECLARED + FAIL-CLOSED semantics of the marker
// resolver directly, independent of either scan surface.
func TestIsFixtureCorpusPath(t *testing.T) {
	root := t.TempDir()
	// Declare one corpus.
	if err := os.MkdirAll(filepath.Join(root, "docs/streams/c/corpus/deep"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, root, "docs/streams/c/corpus/"+fixtureCorpusMarkerName, "")

	excluded := []string{
		"docs/streams/c/corpus/run.md",       // directly under the marker
		"docs/streams/c/corpus/deep/run.md",  // nested below the marker
		"docs/streams/c/corpus/sub/dir/x.md", // arbitrary depth
	}
	for _, p := range excluded {
		if !isFixtureCorpusPath(root, p) {
			t.Errorf("isFixtureCorpusPath(root, %q) = false, want true — it is under a declared corpus", p)
		}
	}

	notExcluded := []string{
		"docs/streams/c/README.md",            // ABOVE the marker — fail-closed
		"docs/streams/c/corpus-evil/x.md",     // lookalike sibling, no marker
		"docs/streams/other/brief-01.md",      // unrelated live brief
		"",                                    // empty
		"../escape/x.md",                      // escaping — never a corpus
	}
	for _, p := range notExcluded {
		if isFixtureCorpusPath(root, p) {
			t.Errorf("isFixtureCorpusPath(root, %q) = true, want false — no marker at/above it (fail-closed)", p)
		}
	}

	// Empty root: no filesystem to consult — always false (fail-closed).
	if isFixtureCorpusPath("", "docs/streams/c/corpus/run.md") {
		t.Errorf("isFixtureCorpusPath(\"\", ...) = true, want false — no root means nothing declared")
	}
}
