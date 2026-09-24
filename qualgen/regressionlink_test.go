package main

import (
	"os"
	"path/filepath"
	"testing"
)

// writeNestedFile is writeFile (mine_test.go) plus the parent-directory
// creation a brief path under docs/streams/<stream>/ needs — writeFile itself
// assumes the directory already exists.
func writeNestedFile(t *testing.T, dir, name, content string) {
	t.Helper()
	full := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("mkdir for %s: %v", name, err)
	}
	writeFile(t, dir, name, content)
}

// --- BriefRegressionLinkage (the reference RegressionLinkage adapter) fixture
// tests. Reuses szzGit / commitFile / openRepo from szz_test.go (same package)
// so the adapter is dereferenced against a genuine git repository, never a
// mock — the same discipline szz_test.go documents for the B-SZZ engine. ---

const regressionOfBriefContent = `---
brief: assay:assay:quality:99
title: example fix brief
regression-of: "#5"
---

# Brief 99 — example
`

const regressionOfBriefContentNoLink = `---
brief: assay:assay:quality:98
title: example fix brief with no regression-of
---

# Brief 98 — example
`

// TestRegressionLink_BriefTrailerResolves proves the primary path: a fix
// commit carrying a `Brief: quality/99` trailer resolves regression-of from
// THAT brief file, read at the fix commit's own tree.
func TestRegressionLink_BriefTrailerResolves(t *testing.T) {
	requireGit(t)
	dir := t.TempDir()
	szzGit(t, dir, "2020-01-01T00:00:00Z", "init", "-q", "-b", "main")

	briefPath := "docs/streams/quality/brief-99-example.md"
	writeNestedFile(t, dir, briefPath, regressionOfBriefContent)
	szzGit(t, dir, "2020-01-01T00:00:00Z", "add", briefPath)
	fixSHA := commitFile(t, dir, "2020-06-01T00:00:00Z", "fix.go", "package x\n// fix\n", "fix: repair the widget\n\nBrief: quality/99\n")

	repo := openRepo(t, dir)
	linkage := BriefRegressionLinkage{Repo: repo}

	refs, ok, err := linkage.RegressionOf(DefectFix{FixCommitSHA: fixSHA})
	if err != nil {
		t.Fatalf("RegressionOf: unexpected error: %v", err)
	}
	if !ok || len(refs) != 1 {
		t.Fatalf("expected exactly one resolved ref, got ok=%v refs=%+v", ok, refs)
	}
	if refs[0].Issue == nil || refs[0].Issue.Number != 5 {
		t.Fatalf("expected issue #5 resolved from the brief's regression-of:, got %+v", refs[0])
	}
}

// TestRegressionLink_TouchedBriefFallback proves the fallback path: with no
// `Brief:` trailer, a brief file the fix commit itself TOUCHES is read.
func TestRegressionLink_TouchedBriefFallback(t *testing.T) {
	requireGit(t)
	dir := t.TempDir()
	szzGit(t, dir, "2020-01-01T00:00:00Z", "init", "-q", "-b", "main")
	// A prior commit with no brief file at all, so the fallback definitely finds
	// the brief only via the fix commit's own diff.
	commitFile(t, dir, "2020-01-01T00:00:00Z", "seed.txt", "seed\n", "seed")

	briefPath := "docs/streams/quality/brief-99-example.md"
	writeNestedFile(t, dir, briefPath, regressionOfBriefContent)
	writeFile(t, dir, "fix.go", "package x\n// fix\n")
	szzGit(t, dir, "2020-06-01T00:00:00Z", "add", briefPath, "fix.go")
	szzGit(t, dir, "2020-06-01T00:00:00Z", "commit", "-q", "-m", "fix: repair the widget (no trailer)")
	fixSHA := szzGit(t, dir, "2020-06-01T00:00:00Z", "rev-parse", "HEAD")

	repo := openRepo(t, dir)
	linkage := BriefRegressionLinkage{Repo: repo}

	refs, ok, err := linkage.RegressionOf(DefectFix{FixCommitSHA: fixSHA})
	if err != nil {
		t.Fatalf("RegressionOf: unexpected error: %v", err)
	}
	if !ok || len(refs) != 1 || refs[0].Issue == nil || refs[0].Issue.Number != 5 {
		t.Fatalf("expected issue #5 resolved via the touched-file fallback, got ok=%v refs=%+v", ok, refs)
	}
}

// TestRegressionLink_NoRegressionOf_LegitimatelyAbsent proves a brief with no
// `regression-of:` field returns ok=false, err=nil — a legitimate absence, not
// an error.
func TestRegressionLink_NoRegressionOf_LegitimatelyAbsent(t *testing.T) {
	requireGit(t)
	dir := t.TempDir()
	szzGit(t, dir, "2020-01-01T00:00:00Z", "init", "-q", "-b", "main")

	briefPath := "docs/streams/quality/brief-98-example.md"
	writeNestedFile(t, dir, briefPath, regressionOfBriefContentNoLink)
	szzGit(t, dir, "2020-01-01T00:00:00Z", "add", briefPath)
	fixSHA := commitFile(t, dir, "2020-06-01T00:00:00Z", "fix.go", "package x\n// fix\n", "fix: repair another widget\n\nBrief: quality/98\n")

	repo := openRepo(t, dir)
	linkage := BriefRegressionLinkage{Repo: repo}

	refs, ok, err := linkage.RegressionOf(DefectFix{FixCommitSHA: fixSHA})
	if err != nil {
		t.Fatalf("RegressionOf: unexpected error: %v", err)
	}
	if ok || len(refs) != 0 {
		t.Fatalf("expected a legitimate absence (ok=false, no refs), got ok=%v refs=%+v", ok, refs)
	}
}

// TestRegressionLink_DefectClass_UnconfiguredIsError proves the adapter-level
// enforcement of fact 3: an unconfigured class-label prefix is ALWAYS an error
// (could-not-measure upstream), never a silent "no class".
func TestRegressionLink_DefectClass_UnconfiguredIsError(t *testing.T) {
	linkage := BriefRegressionLinkage{Labels: stubIssueLabelSource{labels: map[string][]string{"5": {"class:widget-nil-deref"}}}}
	_, ok, err := linkage.DefectClass(IssueRef{Number: 5})
	if err == nil {
		t.Fatalf("expected an error for an unconfigured class-label prefix, got ok=%v", ok)
	}
	if ok {
		t.Fatalf("must not report ok=true alongside an error")
	}
}

// TestRegressionLink_DefectClass_ConfiguredReadsPrefixedLabel proves the
// configured path reads the class key off the matching prefixed label.
func TestRegressionLink_DefectClass_ConfiguredReadsPrefixedLabel(t *testing.T) {
	source := stubIssueLabelSource{labels: map[string][]string{
		"5": {"bug", "class:widget-nil-deref"},
		"6": {"enhancement"},
	}}
	linkage := BriefRegressionLinkage{Labels: source, ClassLabelPrefix: "class:"}

	class, ok, err := linkage.DefectClass(IssueRef{Number: 5})
	if err != nil || !ok || class != "widget-nil-deref" {
		t.Fatalf("expected class widget-nil-deref, got class=%q ok=%v err=%v", class, ok, err)
	}

	_, ok, err = linkage.DefectClass(IssueRef{Number: 6})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Fatalf("expected no class-prefixed label on issue #6 to resolve ok=false")
	}
}

// TestRegressionLink_DefectClass_LabelSourceErrorPropagates proves an
// IssueLabelSource error is surfaced as-is (could-not-measure upstream), never
// swallowed into a silent no-class.
func TestRegressionLink_DefectClass_LabelSourceErrorPropagates(t *testing.T) {
	source := stubIssueLabelSource{unresolvable: map[int]bool{7: true}}
	linkage := BriefRegressionLinkage{Labels: source, ClassLabelPrefix: "class:"}
	_, ok, err := linkage.DefectClass(IssueRef{Number: 7})
	if err == nil || ok {
		t.Fatalf("expected the label source's error to propagate, got ok=%v err=%v", ok, err)
	}
}
