package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestFixtureCorpusMarkerRefusedOutsideStreams pins the SCOPE half of the
// fail-closed contract: the marker is honoured ONLY strictly under
// docs/streams/<corpus>/. A marker at the repo root, over docs/, over
// docs/streams/ itself, or anywhere outside docs/ is INERT — it exempts
// nothing. An exemption that covers every stream at once is not an exemption,
// it is a bypass, and the resolver must refuse it on its own (not merely have
// the lint complain), because --corroborate consults the same resolver without
// ever running the lint.
func TestFixtureCorpusMarkerRefusedOutsideStreams(t *testing.T) {
	root := t.TempDir()

	// Four MISPLACED markers, each of which would be a bypass if honoured.
	writeFile(t, root, fixtureCorpusMarkerName, "")                 // repo root
	writeFile(t, root, "docs/"+fixtureCorpusMarkerName, "")         // the whole docs tree
	writeFile(t, root, "docs/streams/"+fixtureCorpusMarkerName, "") // every stream at once
	writeFile(t, root, "elsewhere/corpus/"+fixtureCorpusMarkerName, "")

	// Content each misplaced marker would have exempted.
	writeFile(t, root, "README.md", "x")
	writeFile(t, root, "docs/narrative.md", "x")
	writeFile(t, root, "docs/streams/s/brief-01-live.md", "x")
	writeFile(t, root, "elsewhere/corpus/run.md", "x")

	for _, p := range []string{
		"README.md",
		"docs/narrative.md",
		"docs/streams/s/brief-01-live.md",
		"elsewhere/corpus/run.md",
	} {
		if isFixtureCorpusPath(root, p) {
			t.Errorf("isFixtureCorpusPath(root, %q) = true — a marker outside docs/streams/<corpus>/ must be INERT; honouring it exempts a whole tree at once (a bypass, not an exemption)", p)
		}
	}
}

// TestFixtureCorpusMarkerHonouredInsideStreams is the positive control for the
// scope rule: one directory below docs/streams/ is the shallowest legal corpus
// root, and depth below that is legal too.
func TestFixtureCorpusMarkerHonouredInsideStreams(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "docs/streams/shallow/"+fixtureCorpusMarkerName, "")
	writeFile(t, root, "docs/streams/shallow/run.md", "x")
	writeFile(t, root, "docs/streams/s/evals/round-1/"+fixtureCorpusMarkerName, "")
	writeFile(t, root, "docs/streams/s/evals/round-1/run.md", "x")
	// An unmarked sibling of the deep corpus stays in scope (fail-closed).
	writeFile(t, root, "docs/streams/s/evals/round-2/run.md", "x")

	for _, p := range []string{
		"docs/streams/shallow/run.md",
		"docs/streams/s/evals/round-1/run.md",
	} {
		if !isFixtureCorpusPath(root, p) {
			t.Errorf("isFixtureCorpusPath(root, %q) = false — a marker strictly under docs/streams/ declares its own subtree", p)
		}
	}
	if isFixtureCorpusPath(root, "docs/streams/s/evals/round-2/run.md") {
		t.Errorf("an UNMARKED sibling was excluded — fail-closed: only a declared subtree is exempt")
	}
}

// TestFixtureCorpusScopeOK pins the predicate directly, including the shapes a
// path can arrive in.
func TestFixtureCorpusScopeOK(t *testing.T) {
	ok := []string{
		"docs/streams/s",
		"docs/streams/s/evals",
		"docs/streams/s/evals/deep/deeper",
	}
	for _, d := range ok {
		if !fixtureCorpusScopeOK(d) {
			t.Errorf("fixtureCorpusScopeOK(%q) = false, want true", d)
		}
	}
	bad := []string{
		"", ".", "/", "..",
		"docs", "docs/streams", "docs/streams/",
		"docsx/streams/s",          // lookalike prefix — the slash boundary is load-bearing
		"docs/streamsx/s",          // ditto
		"elsewhere/docs/streams/s", // not anchored at the repo root
		"../docs/streams/s",        // escaping
	}
	for _, d := range bad {
		if fixtureCorpusScopeOK(d) {
			t.Errorf("fixtureCorpusScopeOK(%q) = true, want false — out of scope markers must be inert", d)
		}
	}
}

// TestFixtureCorpusChecksNoticesAndProblems proves the exemption is never
// SILENT: every honoured corpus produces a NOTICE naming it and its file count,
// and every misplaced marker produces a PROBLEM that says it is inert.
func TestFixtureCorpusChecksNoticesAndProblems(t *testing.T) {
	root := t.TempDir()
	// One honoured corpus: marker + three files (one nested).
	writeFile(t, root, "docs/streams/s/evals/"+fixtureCorpusMarkerName, "")
	writeFile(t, root, "docs/streams/s/evals/run-01.md", "x")
	writeFile(t, root, "docs/streams/s/evals/run-02.md", "x")
	writeFile(t, root, "docs/streams/s/evals/deep/run-03.md", "x")
	// One misplaced marker.
	writeFile(t, root, "docs/"+fixtureCorpusMarkerName, "")
	// Live content that must not be counted into the corpus.
	writeFile(t, root, "docs/streams/s/README.md", "x")

	problems, notices := fixtureCorpusChecks(root)

	var sawNotice bool
	for _, n := range notices {
		if strings.Contains(n, "fixture corpus exempted: docs/streams/s/evals (3 files)") {
			sawNotice = true
		}
	}
	if !sawNotice {
		t.Errorf("no NOTICE naming the exempted subtree and its file count; notices=%v", notices)
	}

	var sawProblem bool
	for _, p := range problems {
		if strings.Contains(p, "docs/"+fixtureCorpusMarkerName) && strings.Contains(p, "INERT") {
			sawProblem = true
		}
	}
	if !sawProblem {
		t.Errorf("a marker over the whole docs/ tree was not refused with a PROBLEM; problems=%v", problems)
	}
	// The honoured corpus must NOT also be reported as misplaced.
	for _, p := range problems {
		if strings.Contains(p, "docs/streams/s/evals/") {
			t.Errorf("a legal corpus was refused: %s", p)
		}
	}
}

// TestFixtureCorpusChecksSilentWithoutMarkers: a tree that declares nothing
// produces no notice and no problem — the mechanism is invisible until used.
func TestFixtureCorpusChecksSilentWithoutMarkers(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "docs/streams/s/README.md", "x")
	problems, notices := fixtureCorpusChecks(root)
	if len(problems) != 0 || len(notices) != 0 {
		t.Errorf("undeclared tree produced output: problems=%v notices=%v", problems, notices)
	}
}

// TestFixtureCorpusLintRedGreenRed walks the whole story at the --lint surface,
// on ONE corpus body, in the three states the mechanism has to distinguish:
//
//	(1) no marker            -> RED   (the forward-references are link problems)
//	(2) marker in the corpus -> GREEN (declared: the subtree is skipped, with a NOTICE)
//	(3) marker moved to docs/ -> RED  (out of scope: inert, and refused as a PROBLEM)
func TestFixtureCorpusLintRedGreenRed(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "docs/streams/s/evalcorpus/run-01.md", forwardRefContent)

	// linkRed reports whether the corpus body is flagged by the link check.
	linkRed := func() bool {
		docs, walkProblems := docFiles(root)
		if len(walkProblems) != 0 {
			t.Fatalf("unexpected walk problems: %v", walkProblems)
		}
		for _, p := range linkProblems(root, docs) {
			if strings.Contains(p, "evalcorpus") {
				return true
			}
		}
		return false
	}

	// (1) No marker: the corpus is scanned as a live brief and reds.
	if !linkRed() {
		t.Fatalf("state 1 (no marker): expected the corpus's forward-references to red the link check")
	}
	if p, n := fixtureCorpusChecks(root); len(p) != 0 || len(n) != 0 {
		t.Fatalf("state 1: expected no marker output; problems=%v notices=%v", p, n)
	}

	// (2) Marker AT the corpus root: green, and announced.
	marker := filepath.Join(root, "docs/streams/s/evalcorpus", fixtureCorpusMarkerName)
	writeFile(t, root, "docs/streams/s/evalcorpus/"+fixtureCorpusMarkerName, "")
	if linkRed() {
		t.Fatalf("state 2 (declared): the corpus must be skipped by the link check")
	}
	p, n := fixtureCorpusChecks(root)
	if len(p) != 0 {
		t.Fatalf("state 2: a legal corpus must raise no PROBLEM; got %v", p)
	}
	if len(n) != 1 || !strings.Contains(n[0], "docs/streams/s/evalcorpus") {
		t.Fatalf("state 2: expected exactly one NOTICE naming the corpus; got %v", n)
	}

	// (3) Marker moved OUT of docs/streams/**: inert, so the corpus reds again,
	//     and the misplaced marker is itself refused.
	if err := os.Remove(marker); err != nil {
		t.Fatal(err)
	}
	writeFile(t, root, "docs/"+fixtureCorpusMarkerName, "")
	if !linkRed() {
		t.Fatalf("state 3 (marker outside docs/streams): the corpus must be scanned again — an out-of-scope marker exempts nothing")
	}
	p, _ = fixtureCorpusChecks(root)
	if len(p) != 1 || !strings.Contains(p[0], "INERT") {
		t.Fatalf("state 3: expected exactly one PROBLEM refusing the misplaced marker; got %v", p)
	}
}
