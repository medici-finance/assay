package main

// numberspace_test.go — the detector's own proof-it-can-fail, on synthetic
// content only. Nothing here reads docs/brief-rules.md off disk: a test in the
// statusgen module that reads a path outside it is a cross-module reader and
// would need a citrigger registry entry plus a widened `paths:` filter, and the
// detector is a pure function over a string, so there is nothing that read would
// establish. The real file is exercised by `statusgen --lint` in CI, whose
// trigger already covers docs/**.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNumberSpaceCollisionsFindsADuplicate(t *testing.T) {
	content := "" +
		"# Brief rules\n\n" +
		"25. **The `## Evidence` section is a LOG OF RUNS.** Body.\n\n" +
		"26. **The witness result is THREE-STATE.** Body.\n\n" +
		"## Row-runner discipline\n\n" +
		"25. **Never assert a specific non-zero exit code.** Body.\n\n"
	got := numberSpaceCollisions(content)
	if len(got) != 1 {
		t.Fatalf("want exactly one collision, got %d: %v", len(got), got)
	}
	if !strings.Contains(got[0], "rule number 25 is allocated 2 times") {
		t.Errorf("finding must name the number and the count: %q", got[0])
	}
	// Cite by expression, not by line: the finding carries the rule's heading text
	// so a reader can identify which rule is meant after the file moves.
	if !strings.Contains(got[0], "LOG OF RUNS") || !strings.Contains(got[0], "Never assert") {
		t.Errorf("finding must name BOTH allocations by their heading expression: %q", got[0])
	}
}

func TestNumberSpaceCollisionsCleanFileIsEmpty(t *testing.T) {
	// The mutation of the fixture above: the second 25 becomes a free number.
	content := "" +
		"# Brief rules\n\n" +
		"25. **The `## Evidence` section is a LOG OF RUNS.** Body.\n\n" +
		"26. **The witness result is THREE-STATE.** Body.\n\n" +
		"27. **Never assert a specific non-zero exit code.** Body.\n\n"
	if got := numberSpaceCollisions(content); len(got) != 0 {
		t.Fatalf("a correctly-numbered file must produce nothing, got %v", got)
	}
}

func TestNumberSpaceCollisionsIgnoresFencedExamples(t *testing.T) {
	// A numbered line inside a fenced block is a sample, not an allocation. Without
	// this the detector would fire on every doc that shows an example rule.
	content := "" +
		"7. **A real rule.** Body.\n\n" +
		"```\n" +
		"7. **An example pasted into a fence.**\n" +
		"```\n\n"
	if got := numberSpaceCollisions(content); len(got) != 0 {
		t.Fatalf("fenced content must not allocate a number, got %v", got)
	}
}

func TestNumberSpaceCollisionsIgnoresIndentedSublists(t *testing.T) {
	// Rules indent their continuation prose and nested lists. An indented `1.`
	// inside rule 9's body is not rule 1.
	content := "" +
		"9. **A rule with a nested list.** Body.\n" +
		"    1. **A nested item.**\n" +
		"    1. **Another nested item.**\n\n" +
		"10. **The next rule.** Body.\n\n"
	if got := numberSpaceCollisions(content); len(got) != 0 {
		t.Fatalf("indented sub-items must not allocate numbers, got %v", got)
	}
}

func TestBriefRuleNumbersReportsLineAndTitle(t *testing.T) {
	content := "# Head\n\n12. **Some rule.** Body.\n"
	got := briefRuleNumbers(content)
	if len(got) != 1 {
		t.Fatalf("want 1 rule, got %d", len(got))
	}
	if got[0].Num != 12 {
		t.Errorf("num: want 12, got %d", got[0].Num)
	}
	if got[0].Line != 3 {
		t.Errorf("line: want 3, got %d", got[0].Line)
	}
	if !strings.HasPrefix(got[0].Title, "Some rule.") {
		t.Errorf("title: want the heading expression, got %q", got[0].Title)
	}
}

func TestBriefRuleNumberNoticesMissingFileIsSilent(t *testing.T) {
	// Most repos adopting statusgen have no brief-rules.md. A NOTICE for each of
	// them is alarm flooding, not a check.
	if got := briefRuleNumberNotices(t.TempDir()); len(got) != 0 {
		t.Fatalf("a missing brief-rules.md must produce nothing, got %v", got)
	}
}

func TestBriefRuleNumberNoticesUnreadableFileIsCouldNotCheck(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root; mode 0000 is still readable")
	}
	root := t.TempDir()
	p := filepath.Join(root, filepath.FromSlash(briefRulesRelPath))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("1. **x** y\n"), 0o000); err != nil {
		t.Fatal(err)
	}
	got := briefRuleNumberNotices(root)
	if len(got) != 1 {
		t.Fatalf("want exactly one could-not-check notice, got %d: %v", len(got), got)
	}
	if !strings.HasPrefix(got[0], "could-not-check:") {
		t.Errorf("an unreadable file must report could-not-check, never clean: %q", got[0])
	}
}

func TestBriefRuleNumberNoticesSaysItCannotSeeAcrossBranches(t *testing.T) {
	// The branch-local reading must not be mistaken for the merge-time one. If this
	// caveat is ever dropped the NOTICE reads as a complete answer, which it is not.
	root := t.TempDir()
	p := filepath.Join(root, filepath.FromSlash(briefRulesRelPath))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	content := "1. **A.** x\n\n1. **B.** y\n"
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	got := briefRuleNumberNotices(root)
	if len(got) != 2 {
		t.Fatalf("want the finding plus its scope caveat, got %d: %v", len(got), got)
	}
	if !strings.Contains(got[1], "mergecheck") || !strings.Contains(got[1], "WORKING TREE") {
		t.Errorf("the caveat must say which tree was read and which tool reads the other: %q", got[1])
	}
}
