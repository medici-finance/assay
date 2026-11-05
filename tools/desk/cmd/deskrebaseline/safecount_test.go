package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Fail-first tests for #5529, safe:count. Each goes through loadBrief, gatherRowFacts and
// Classify against a real git history, in the shape of TestSafeRename, so on the base
// (which never sets CountShaped or OnlyAdditions) the positive cases are red.

const countFront = "---\ngate: model\nrisk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}\n---\n\n# B\n\n## Verify\n\n| # | Class | Command | Expect |\n|---|---|---|---|\n"

// countRow is the brief's Verify row 1 for cmd (cell form, pipes escaped) and expect.
func countBrief(cmd, expect string) string {
	return countFront + "| 1 | check | `" + cmd + "` | " + expect + " |\n"
}

// countEvidence is an Evidence table recording row 1 with the given result.
func countEvidence(cmd, result string) string {
	return "\n## Evidence\n\n| # | Command | Result | Output | Date | Runner |\n|---|---|---|---|---|---|\n| 1 | `" + cmd + "` | " + result + " | 2 | 2026-01-01 | example-runner |\n"
}

const (
	fileCmd = "grep -c item data/list.txt"
	dirCmd  = "grep -rh item ./data \\| grep -c item"
)

// countCase builds a repo: commit 1 is the brief's Verify row plus the data, commit 2
// appends the Evidence, then change runs and (when commit is true) is committed. It returns
// the facts and classification for row 1.
func countCase(t *testing.T, brief, evidence string, data map[string]string, change func(root string), commit bool) (RowFacts, Classification, verifyRow) {
	t.Helper()
	root := gitRepo(t)
	writeFile(t, root, "docs/brief.md", brief)
	for rel, c := range data {
		writeFile(t, root, rel, c)
	}
	gitCommit(t, root, "brief and data")
	if evidence != "" {
		writeFile(t, root, "docs/brief.md", brief+evidence)
		gitCommit(t, root, "evidence")
	}
	if change != nil {
		change(root)
		if commit {
			gitCommit(t, root, "change")
		}
	}
	return classifyBrief(t, root)
}

func classifyBrief(t *testing.T, root string) (RowFacts, Classification, verifyRow) {
	t.Helper()
	lb, err := loadBrief("", root, filepath.Join(root, "docs/brief.md"))
	if err != nil {
		t.Fatal(err)
	}
	row, ok := lb.row(1)
	if !ok {
		t.Fatalf("no row 1 in %+v", lb.Rows)
	}
	f := gatherRowFacts(root, row, lb.RiskBearing, lb.RiskReason)
	return f, Classify(f), row
}

func appendTo(t *testing.T, root, rel, s string) {
	t.Helper()
	p := filepath.Join(root, rel)
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, append(b, s...), 0o644); err != nil {
		t.Fatal(err)
	}
}

func gitIn(t *testing.T, root string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

var listData = map[string]string{"data/list.txt": "item a\nitem b\nother\n"}
var dirData = map[string]string{"data/a.txt": "item a\n", "data/b.txt": "item b\n"}

// TestSafeCount — the pinned count grew and the one target file only gained lines since the
// Evidence commit → safe:count, and --open's edit is the measured count.
//
// regression: #5529
func TestSafeCount(t *testing.T) {
	f, got, row := countCase(t, countBrief(fileCmd, "output is `2`"), countEvidence(fileCmd, "pass exit=0"), listData,
		func(root string) { appendTo(t, root, "data/list.txt", "item c\n") }, true)
	if got.Verdict != SafeCount {
		t.Fatalf("additions-only count drift: got %s (%q), want %s\nfacts=%+v", got.Verdict, got.Reason, SafeCount, f)
	}
	if f.CountNow != 3 || f.CountEvidence == "" {
		t.Fatalf("CountNow=%d CountEvidence=%q, want 3 and a record", f.CountNow, f.CountEvidence)
	}
	cmd, exp, changed := computeNewRow("", row, f, got)
	if !changed || cmd != row.Command || exp != "output is `3`" {
		t.Fatalf("computeNewRow = %q, %q, %t; want the command unchanged and output is `3`", cmd, exp, changed)
	}
}

// TestSafeCountDirAdds — a directory target that only gained a new file is additions-only.
//
// regression: #5529
func TestSafeCountDirAdds(t *testing.T) {
	f, got, _ := countCase(t, countBrief(dirCmd, "output is `2`"), countEvidence(dirCmd, "pass exit=0"), dirData,
		func(root string) { writeFile(t, root, "data/d.txt", "item d\n") }, true)
	if got.Verdict != SafeCount || f.CountNow != 3 {
		t.Fatalf("new file in target dir: got %s (%q) CountNow=%d, want %s\nfacts=%+v", got.Verdict, got.Reason, f.CountNow, SafeCount, f)
	}
}

// refusedByHistory asserts the row is the count drift the shape test accepts, refused only
// because the history proof did not hold.
func refusedByHistory(t *testing.T, what string, f RowFacts, got Classification) {
	t.Helper()
	if got.Verdict.IsSafe() {
		t.Fatalf("%s: got %s (%q), want a refusal\nfacts=%+v", what, got.Verdict, got.Reason, f)
	}
	if !f.CountShaped || f.OnlyAdditions {
		t.Fatalf("%s: CountShaped=%t OnlyAdditions=%t, want a count drift refused by the history proof\nfacts=%+v", what, f.CountShaped, f.OnlyAdditions, f)
	}
}

// TestCountDeletionMixedIn — a deleted line beside the additions is not pure growth.
//
// regression: #5529
func TestCountDeletionMixedIn(t *testing.T) {
	f, got, _ := countCase(t, countBrief(fileCmd, "output is `2`"), countEvidence(fileCmd, "pass exit=0"), listData,
		func(root string) { writeFile(t, root, "data/list.txt", "item a\nitem b\nitem c\n") }, true)
	refusedByHistory(t, "deletion mixed in", f, got)
}

// TestCountRenameInTarget — a rename inside the target directory refuses.
//
// regression: #5529
func TestCountRenameInTarget(t *testing.T) {
	f, got, _ := countCase(t, countBrief(dirCmd, "output is `2`"), countEvidence(dirCmd, "pass exit=0"), dirData,
		func(root string) {
			gitIn(t, root, "mv", "data/b.txt", "data/c.txt")
			writeFile(t, root, "data/d.txt", "item d\n")
		}, true)
	refusedByHistory(t, "rename in target", f, got)
}

// TestCountBinaryInTarget — a binary file in the change set (numstat `-`) refuses.
//
// regression: #5529
func TestCountBinaryInTarget(t *testing.T) {
	f, got, _ := countCase(t, countBrief(dirCmd, "output is `2`"), countEvidence(dirCmd, "pass exit=0"), dirData,
		func(root string) {
			writeFile(t, root, "data/d.txt", "item d\n")
			writeFile(t, root, "data/blob.bin", "\x00\x01\x02\x00")
		}, true)
	refusedByHistory(t, "binary in target", f, got)
}

// TestCountModeChange — a mode change on the target refuses.
//
// regression: #5529
func TestCountModeChange(t *testing.T) {
	f, got, _ := countCase(t, countBrief(fileCmd, "output is `2`"), countEvidence(fileCmd, "pass exit=0"), listData,
		func(root string) {
			appendTo(t, root, "data/list.txt", "item c\n")
			if err := os.Chmod(filepath.Join(root, "data/list.txt"), 0o755); err != nil {
				t.Fatal(err)
			}
		}, true)
	refusedByHistory(t, "mode change", f, got)
}

// TestCountNoEvidenceCommit — with no Evidence line, a failing one, or one for another
// command, there is no Evidence commit to diff from.
//
// regression: #5529
func TestCountNoEvidenceCommit(t *testing.T) {
	grow := func(root string) { appendTo(t, root, "data/list.txt", "item c\n") }
	for name, ev := range map[string]string{
		"no evidence":      "",
		"failing evidence": countEvidence(fileCmd, "fail exit=1"),
		"other command":    countEvidence("grep -c item data/other.txt", "pass exit=0"),
	} {
		f, got, _ := countCase(t, countBrief(fileCmd, "output is `2`"), ev, listData, grow, true)
		refusedByHistory(t, name, f, got)
	}
}

// TestCountShallowHistory — in a shallow clone the Evidence commit is not resolvable.
//
// regression: #5529
func TestCountShallowHistory(t *testing.T) {
	root := gitRepo(t)
	brief := countBrief(fileCmd, "output is `2`")
	writeFile(t, root, "docs/brief.md", brief)
	for rel, c := range listData {
		writeFile(t, root, rel, c)
	}
	gitCommit(t, root, "brief and data")
	writeFile(t, root, "docs/brief.md", brief+countEvidence(fileCmd, "pass exit=0"))
	gitCommit(t, root, "evidence")
	appendTo(t, root, "data/list.txt", "item c\n")
	gitCommit(t, root, "grow")
	clone := filepath.Join(t.TempDir(), "c")
	gitIn(t, root, "clone", "-q", "--depth=1", "file://"+root, clone)
	f, got, _ := classifyBrief(t, clone)
	refusedByHistory(t, "shallow clone", f, got)
}

// TestCountRowEditedAfter — a Verify row rewritten after its Evidence has no Evidence of
// its own.
//
// regression: #5529
func TestCountRowEditedAfter(t *testing.T) {
	brief := countBrief(fileCmd, "output is `2`")
	f, got, _ := countCase(t, brief, countEvidence(fileCmd, "pass exit=0"), listData,
		func(root string) {
			edited := strings.Replace(brief, "| output is `2` |", "| output is `2`  |", 1)
			writeFile(t, root, "docs/brief.md", edited+countEvidence(fileCmd, "pass exit=0"))
			appendTo(t, root, "data/list.txt", "item c\n")
		}, true)
	refusedByHistory(t, "row edited after evidence", f, got)
}

// TestCountDirtyTarget — an uncommitted addition is not history.
//
// regression: #5529
func TestCountDirtyTarget(t *testing.T) {
	f, got, _ := countCase(t, countBrief(fileCmd, "output is `2`"), countEvidence(fileCmd, "pass exit=0"), listData,
		func(root string) { appendTo(t, root, "data/list.txt", "item c\n") }, false)
	refusedByHistory(t, "dirty target", f, got)
}

// TestCountNotDrift — a lower count, or an Expect naming two numbers, is not a count drift.
//
// regression: #5529
func TestCountNotDrift(t *testing.T) {
	for name, tc := range map[string]struct {
		expect string
		data   string
	}{
		"count fell":  {"output is `2`", "item a\n"},
		"two numbers": {"exit 0 and output is `2`", "item a\nitem b\nitem c\n"},
	} {
		f, got, _ := countCase(t, countBrief(fileCmd, tc.expect), countEvidence(fileCmd, "pass exit=0"), listData,
			func(root string) { writeFile(t, root, "data/list.txt", tc.data) }, true)
		if got.Verdict.IsSafe() || f.CountShaped {
			t.Fatalf("%s: got %s CountShaped=%t, want a refusal with no count shape\nfacts=%+v", name, got.Verdict, f.CountShaped, f)
		}
	}
}
