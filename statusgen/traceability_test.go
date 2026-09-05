package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Test names stay under 42 characters (the pre-push secret-scan floor the
// requirements_test.go header explains).

// tracedStream builds a Stream whose Dir is a fresh temp dir, with the given board
// rows, and writes one valid brief-v1 file per row carrying the given satisfies
// line (spelled verbatim, "" = none). The stream's name is the dir basename so a
// brief's Key resolves to <stream>/<NN>.
func tracedStream(t *testing.T, name string, traced bool, rows []Brief, satisfiesByNum map[string]string) *Stream {
	t.Helper()
	dir := filepath.Join(t.TempDir(), name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, r := range rows {
		requirementBrief(t, dir, r.Num, satisfiesByNum[r.Num])
	}
	return &Stream{Name: name, Dir: dir, Traced: traced, Briefs: rows}
}

// fixedNow is a stable clock so age rendering is deterministic in tests.
func traceNow() time.Time { return time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC) }

// ---------- orphan-requirement (NOTICE) ----------

// TestOrphanRequirement: an accepted requirement that no brief's satisfies names
// raises the orphan NOTICE, listed with its impact and age. A proposed one is not
// yet orphan-eligible, and a cited one is not orphan.
func TestOrphanRequirement(t *testing.T) {
	accepted := validRequirementFixture()
	accepted.ID = "REQ-coverage-boundary"
	accepted.Status = "accepted"
	accepted.Impact = "major"
	proposed := validRequirementFixture() // stays proposed → never orphan
	proposed.ID = "REQ-evidence-visible"
	root := requirementRoot(t, accepted, proposed)

	// No brief cites anything.
	empty := tracedStream(t, "sdlc", false, []Brief{{Num: "01", Status: "todo"}}, nil)
	notices := orphanRequirementNotices(root, []*Stream{empty}, traceNow())
	if !containsAll(notices, "orphan-requirement", "REQ-coverage-boundary", "major") {
		t.Errorf("an accepted uncited requirement must raise orphan-requirement by id+impact; got:\n%s", joined(notices))
	}
	if strings.Contains(joined(notices), "REQ-evidence-visible") {
		t.Errorf("a proposed requirement must NOT be orphan; got:\n%s", joined(notices))
	}

	// Now a brief cites the accepted one → no longer orphan.
	cite := tracedStream(t, "sdlc", false,
		[]Brief{{Num: "01", Status: "in-progress"}},
		map[string]string{"01": "satisfies: [\"REQ-coverage-boundary\"]\n"})
	if n := orphanRequirementNotices(root, []*Stream{cite}, traceNow()); len(n) != 0 {
		t.Errorf("a cited accepted requirement must NOT be orphan; got:\n%s", joined(n))
	}
}

// TestOrphanSortedByImpact: two accepted orphans list impact-descending, so the
// buyer's highest-consequence gap is read first (the §3.5 ordered axis).
func TestOrphanSortedByImpact(t *testing.T) {
	crit := validRequirementFixture()
	crit.ID = "REQ-evidence-visible"
	crit.Status = "accepted"
	crit.Impact = "critical"
	minor := validRequirementFixture()
	minor.ID = "REQ-glossary-first"
	minor.Status = "accepted"
	minor.Impact = "minor"
	root := requirementRoot(t, minor, crit) // written minor-first on purpose
	empty := tracedStream(t, "sdlc", false, []Brief{{Num: "01", Status: "todo"}}, nil)
	notices := orphanRequirementNotices(root, []*Stream{empty}, traceNow())
	if len(notices) != 2 {
		t.Fatalf("want 2 orphan notices, got %d:\n%s", len(notices), joined(notices))
	}
	ci := strings.Index(joined(notices), "REQ-evidence-visible")
	mi := strings.Index(joined(notices), "REQ-glossary-first")
	if !(ci >= 0 && mi >= 0 && ci < mi) {
		t.Errorf("critical must sort before minor; got:\n%s", joined(notices))
	}
}

// TestOrphanEmptyRegisterSilent: no register, or no accepted entries, is silent.
func TestOrphanEmptyRegisterSilent(t *testing.T) {
	empty := tracedStream(t, "sdlc", false, []Brief{{Num: "01", Status: "todo"}}, nil)
	if n := orphanRequirementNotices(t.TempDir(), []*Stream{empty}, traceNow()); len(n) != 0 {
		t.Errorf("an absent register must raise no orphan notice; got:\n%s", joined(n))
	}
}

// ---------- untraced-brief (NOTICE) ----------

// TestUntracedBrief: a forward brief in a TRACED stream that cites no requirement
// raises the untraced NOTICE; the same brief in an un-traced stream is silent.
func TestUntracedBrief(t *testing.T) {
	rows := []Brief{{Num: "01", Status: "in-progress"}}

	traced := tracedStream(t, "sdlc", true, rows, nil)
	if n := untracedBriefNotices([]*Stream{traced}); !containsAll(n, "untraced-brief", "sdlc/01") {
		t.Errorf("a forward brief in a traced stream citing nothing must raise untraced-brief; got:\n%s", joined(n))
	}

	untraced := tracedStream(t, "sdlc", false, rows, nil)
	if n := untracedBriefNotices([]*Stream{untraced}); len(n) != 0 {
		t.Errorf("the SAME brief in an un-traced stream must be silent — the opt-in is the point; got:\n%s", joined(n))
	}
}

// TestUntracedOnlyForward: a todo brief has not been dispatched and a satisfies-
// bearing brief is traced — neither raises the notice, even in a traced stream.
func TestUntracedOnlyForward(t *testing.T) {
	todo := tracedStream(t, "sdlc", true, []Brief{{Num: "01", Status: "todo"}}, nil)
	if n := untracedBriefNotices([]*Stream{todo}); len(n) != 0 {
		t.Errorf("a todo brief is not in-progress-or-later; must be silent; got:\n%s", joined(n))
	}
	cited := tracedStream(t, "sdlc", true,
		[]Brief{{Num: "01", Status: "implemented"}},
		map[string]string{"01": "satisfies: [\"REQ-evidence-visible\"]\n"})
	if n := untracedBriefNotices([]*Stream{cited}); len(n) != 0 {
		t.Errorf("a brief that DOES cite a requirement is traced; must be silent; got:\n%s", joined(n))
	}
}

// ---------- dangling-satisfies (PROBLEM) ----------

// TestDanglingSatisfies: a satisfies naming an in-repo REQ id no entry defines is a
// hard PROBLEM naming the offending id; a ref to an existing id is clean.
func TestDanglingSatisfies(t *testing.T) {
	existing := validRequirementFixture()
	existing.ID = "REQ-evidence-visible"
	root := requirementRoot(t, existing)

	dangling := tracedStream(t, "sdlc", false,
		[]Brief{{Num: "01", Status: "in-progress"}},
		map[string]string{"01": "satisfies: [\"REQ-does-not-exist\"]\n"})
	problems := danglingSatisfiesProblems(root, []*Stream{dangling})
	if !containsAll(problems, "dangling-satisfies", "REQ-does-not-exist", "sdlc/01") {
		t.Errorf("a satisfies naming a non-existent in-repo id must be a PROBLEM by id; got:\n%s", joined(problems))
	}

	good := tracedStream(t, "sdlc", false,
		[]Brief{{Num: "01", Status: "in-progress"}},
		map[string]string{"01": "satisfies: [\"REQ-evidence-visible\"]\n"})
	if p := danglingSatisfiesProblems(root, []*Stream{good}); len(p) != 0 {
		t.Errorf("a satisfies naming an EXISTING id must be clean; got:\n%s", joined(p))
	}
}

// TestDanglingCrossRepoNotFlagged: a cross-repo <alias>:REQ-<slug> ref names a
// register this offline check cannot read — could-not-check, never dangling. It
// must NOT be a PROBLEM even when the local register does not define it.
func TestDanglingCrossRepoNotFlagged(t *testing.T) {
	existing := validRequirementFixture()
	existing.ID = "REQ-evidence-visible"
	root := requirementRoot(t, existing)
	// graph-repos registry so the cross-repo ref is grammar-valid at parse time.
	writeFile(t, root, "docs/streams/graph-repos.yaml",
		"schema: graph-repos-v1\ncell: house\naliases:\n  toolkit:\n    cell: house\n    repo: owner/toolkit\n")
	cross := tracedStream(t, "sdlc", false,
		[]Brief{{Num: "01", Status: "in-progress"}},
		map[string]string{"01": "satisfies: [\"toolkit:REQ-evidence-visible\"]\n"})
	if p := danglingSatisfiesProblems(root, []*Stream{cross}); len(p) != 0 {
		t.Errorf("a cross-repo ref must be could-not-check, never dangling; got:\n%s", joined(p))
	}
}
