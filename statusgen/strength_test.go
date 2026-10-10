package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Tests for verify-integrity/03's strength rules (R11–R13) and the promotion of
// the unfailable-row rules to PROBLEMs for a closure the branch makes. Rule
// names in these tests are the stable tags, never the R-numbers, so a doc
// renumbering cannot silently retarget a test.

// TestRule11TriviallyGreen — R11: a command that exits 0 whatever the tree
// holds. Each positive case is a shape the brief names; each negative case is
// a near miss the rule must leave alone.
func TestRule11TriviallyGreen(t *testing.T) {
	declared := []string{"../assay/docs/new-doc.md", "statusgen/strength.go"}
	cases := []struct {
		name, cell, expect string
		want               bool
	}{
		{"bare true", "`true`", "exit 0", true},
		{"echo ok", "`echo ok`", "output ok", true},
		{"colon", "`:`", "exit 0", true},
		{"exit 0", "`exit 0`", "exit 0", true},
		{"trailing or-true", "`go test ./statusgen \\|\\| true`", "ok", true},
		{"trailing or-echo", "`make check \\|\\| echo failed`", "exit 0", true},
		{"git log grep", "`git log --grep 'verify-integrity/03' --oneline`", "one line", true},
		{"test -e on a declared path", "`test -e docs/new-doc.md`", "exit 0", true},
		{"[ -f ] on a declared path", "`[ -f statusgen/strength.go ]`", "exit 0", true},
		{"R2 sanctioned count fix", "`grep -c foo doc.md \\|\\| true`", "0", false},
		{"existence then content", "`test -e docs/new-doc.md && grep -c R1 docs/new-doc.md`", "13", false},
		{"build then echo", "`go build ./... && echo built`", "exit 0", false},
		{"echo of the real status", "`make check; echo $?`", "output 0", false},
		{"test -e on an undeclared path", "`test -e docs/other.md`", "exit 0", false},
		{"git log piped into a count", "`git log --grep x --oneline \\| wc -l`", "1", false},
		{"or-true mid-command", "`rm -f x.out \\|\\| true; go test ./statusgen`", "exit 0", false},
	}
	for _, c := range cases {
		got := hasRule(rowFindingsCtx(c.cell, c.expect, declared), ruleTriviallyGreen)
		if got != c.want {
			t.Errorf("%s: %s with Expect %q — trivially-green fired=%v, want %v", c.name, c.cell, c.expect, got, c.want)
		}
	}
}

// TestRule12NoOutputAssert — R12: Expect is exactly `exit 0` and the command
// produces output nobody asserts on.
func TestRule12NoOutputAssert(t *testing.T) {
	cases := []struct {
		name, cell, expect string
		want               bool
	}{
		{"go test exit 0", "`go test ./statusgen -run TestX`", "exit 0", true},
		{"backticked exit 0", "`go vet ./...`", "`exit 0`", true},
		{"exit=0 spelling", "`make lint`", "exit=0", true},
		{"bare test", "`test -f docs/x.md`", "exit 0", false},
		{"bracket test", "`[ -d statusgen ]`", "exit 0", false},
		{"quiet grep", "`grep -q needle docs/x.md`", "exit 0", false},
		{"pipeline ending in a quiet grep", "`go test ./x -v \\| grep -q -- '--- PASS'`", "exit 0", false},
		{"sanctioned output form", "`go test ./statusgen`", "exit 0; output contains \"ok\"", false},
		{"sanctioned line count", "`grep -c R1 doc.md`", "exit 0; 13 lines", false},
		{"not exactly exit 0", "`go test ./statusgen`", "exit 0 and the PASS line", false},
		{"trivially green is R11's", "`true`", "exit 0", false},
	}
	for _, c := range cases {
		got := hasRule(rowFindingsCtx(c.cell, c.expect, nil), ruleNoOutputAssert)
		if got != c.want {
			t.Errorf("%s: %s with Expect %q — no-output-assertion fired=%v, want %v", c.name, c.cell, c.expect, got, c.want)
		}
	}
}

// TestRule13NoFilesPath — R13: no Verify command references any path the
// brief's `files:` declares. Unparseable `files:` is COULD-NOT-CHECK, never a
// PROBLEM; an absent `files:` is silent.
func TestRule13NoFilesPath(t *testing.T) {
	declared := []string{"../assay/statusgen/verifyrun.go", "../assay/changelog/<slug>.md", "plugins/assay/skills/verify-desk/SKILL.md"}
	cases := []struct {
		name string
		cmds []string
		want bool // true = the rule fires
	}{
		{"package dir of a declared file", []string{"go test ./statusgen -run TestX"}, false},
		{"exact path", []string{"grep -c x statusgen/verifyrun.go"}, false},
		{"glob over the declared file", []string{"grep -c x statusgen/verifyrun*.go"}, false},
		{"placeholder entry's directory", []string{"ls changelog/"}, false},
		{"basename after a cd", []string{"cd plugins/assay/skills/verify-desk && grep -c fail-first SKILL.md"}, false},
		{"go package wildcard", []string{"go vet ./..."}, false},
		{"unrelated paths only", []string{"grep -c x docs/other.md", "go test ./tools/desk -run TestY"}, true},
	}
	for _, c := range cases {
		msg, cnc := tableFilesCheck(c.cmds, declared, true)
		if cnc {
			t.Errorf("%s: a parseable files: must never be could-not-check", c.name)
		}
		if got := msg != ""; got != c.want {
			t.Errorf("%s: %v — table-touches-no-files fired=%v (%q), want %v", c.name, c.cmds, got, msg, c.want)
		}
	}
	// Unparseable: the label is there but yields no path.
	if msg, cnc := tableFilesCheck([]string{"grep -c x docs/other.md"}, nil, true); !cnc || msg != "" {
		t.Errorf("an unparseable files: must be COULD-NOT-CHECK with no finding; got msg=%q cnc=%v", msg, cnc)
	}
	// Absent: nothing declared, nothing to check, nothing said.
	if msg, cnc := tableFilesCheck([]string{"grep -c x docs/other.md"}, nil, false); cnc || msg != "" {
		t.Errorf("an absent files: must be silent; got msg=%q cnc=%v", msg, cnc)
	}
}

// strengthStreamFixture writes one stream whose three briefs each carry the
// same R7 (bre-alternation, #262) row and the same R6 (gorun-exit, #493) row:
// 01 verified, 02 verified, 03 implemented. Which of them is a closure THIS
// branch made is decided by the closedAtBase fake.
func strengthStreamFixture(t *testing.T) (string, []*Stream) {
	t.Helper()
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS("testdata/unfailable-post-base")); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, "docs", "streams", "ufpb")
	src, err := os.ReadFile(filepath.Join(dir, "brief-01-closed.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range []string{"02", "03"} {
		body := strings.Replace(string(src), "brief: ufpb/01", "brief: ufpb/"+n, 1)
		writeTemp(t, dir, "brief-"+n+"-copy.md", body)
	}
	readme := "---\nstream: ufpb\nstatus: active\npriority: P2\ntrack: platform\n---\n\n# ufpb\n\n" +
		"| # | Brief | Wave | Effort | Status | Verified | Reviewed |\n" +
		"|---|-------|------|--------|--------|----------|----------|\n" +
		"| 01 | [Closed](./brief-01-closed.md) | 0 | S | verified | 2026-10-08 | model:x |\n" +
		"| 02 | [Copy](./brief-02-copy.md) | 0 | S | verified | 2026-10-08 | model:x |\n" +
		"| 03 | [Copy](./brief-03-copy.md) | 0 | S | implemented | — | — |\n"
	writeTemp(t, dir, "README.md", readme)
	streams, _, err := loadStreams(root)
	if err != nil {
		t.Fatal(err)
	}
	return root, streams
}

// TestPromotedPostBase — R1–R10 are PROBLEMs for a closure this
// branch made, NOTICEs for a closure already at the merge-base and for an open
// brief, and NOTICEs everywhere when the base cannot be resolved.
func TestPromotedPostBase(t *testing.T) {
	withBase(t, true, "ufpb/02")
	root, streams := strengthStreamFixture(t)
	problems, notices := unfailableRowChecks(root, streams)
	if !containsSub(problems, "brief-01-closed.md: Verify row 1 [bre-alternation]") {
		t.Errorf("a post-base closure's R7 row must be a PROBLEM; problems=%v", problems)
	}
	if containsSub(problems, "brief-02-copy.md") || containsSub(problems, "brief-03-copy.md") {
		t.Errorf("a closure at base and an open brief keep NOTICEs; problems=%v", problems)
	}
	if !containsSub(notices, "brief-02-copy.md: Verify row 1 [bre-alternation]") || !containsSub(notices, "brief-03-copy.md: Verify row 1 [bre-alternation]") {
		t.Errorf("pre-existing and open briefs must still be NOTICEd; notices=%v", notices)
	}

	withBase(t, false)
	problems, notices = unfailableRowChecks(root, streams)
	if len(problems) != 0 {
		t.Errorf("an unresolvable base must grandfather everything; problems=%v", problems)
	}
	if !containsSub(notices, "running degraded") {
		t.Errorf("the degraded run must say so; notices=%v", notices)
	}
}

// TestReplayIncidentRows — Verify row 5: the #262 `grep -c "a\|b\|c"` row and
// the #493 `go run` non-zero-exit row, replayed as a post-base closure, are
// both PROBLEMs under the promoted rules.
func TestReplayIncidentRows(t *testing.T) {
	withBase(t, true)
	root, streams := strengthStreamFixture(t)
	problems, _ := unfailableRowChecks(root, streams)
	for _, want := range []string{
		"brief-01-closed.md: Verify row 1 [bre-alternation]",
		"brief-01-closed.md: Verify row 2 [gorun-exit]",
	} {
		if !containsSub(problems, want) {
			t.Errorf("incident replay %q must be a PROBLEM; problems=%v", want, problems)
		}
	}
}

// ufpbRepo builds a git repo from the unfailable-post-base fixture. With
// closedAtBase false the merge-base (refs/remotes/origin/main) carries brief 01
// as implemented and HEAD flips it to verified — a closure THIS branch makes.
// With closedAtBase true, origin/main is HEAD: the closure is already on main.
func ufpbRepo(t *testing.T, closedAtBase bool) string {
	t.Helper()
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS("testdata/unfailable-post-base")); err != nil {
		t.Fatal(err)
	}
	readme := filepath.Join(root, "docs", "streams", "ufpb", "README.md")
	closed, err := os.ReadFile(readme)
	if err != nil {
		t.Fatal(err)
	}
	open := strings.Replace(string(closed), "| verified | 2026-10-08 fixture-verifier | 2026-10-08 model:fixture |", "| implemented | — | — |", 1)
	if open == string(closed) {
		t.Fatal("fixture README row shape changed; ufpbRepo cannot open the brief")
	}
	runGit(t, root, "init", "-q", "-b", "main")
	if !closedAtBase {
		if err := os.WriteFile(readme, []byte(open), 0o644); err != nil {
			t.Fatal(err)
		}
		runGit(t, root, "add", "-A")
		runGitEnv(t, root, nil, "commit", "-q", "-m", "base: brief open")
		runGit(t, root, "update-ref", "refs/remotes/origin/main", "HEAD")
		if err := os.WriteFile(readme, closed, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	runGit(t, root, "add", "-A")
	runGitEnv(t, root, nil, "commit", "-q", "-m", "head: brief verified")
	if closedAtBase {
		runGit(t, root, "update-ref", "refs/remotes/origin/main", "HEAD")
	}
	return root
}

// TestUnfailablePostBase — Verify row 4, end to end through `--lint`: the
// fixture's R7 (bre-alternation) row on a closure this branch makes is a
// PROBLEM and the lint exits 1; the same fixture with the closure already at
// the merge-base keeps the NOTICE and exits 0. (The brief's literal row cannot
// run against the bare testdata directory: it has no git history, so no
// merge-base, so the gate runs degraded and exits 0 by design.)
func TestUnfailablePostBase(t *testing.T) {
	var code int
	root := ufpbRepo(t, false)
	stderr := captureStderr(t, func() { code = run(root, "lint", nil, nil, "") })
	if code != 1 || !strings.Contains(stderr, "Verify row 1 [bre-alternation]") || !strings.Contains(stderr, "this branch closes the brief") {
		t.Errorf("post-base closure: exit %d, want 1 with the R7 PROBLEM; stderr:\n%s", code, stderr)
	}

	root = ufpbRepo(t, true)
	stderr = captureStderr(t, func() { code = run(root, "lint", nil, nil, "") })
	if code != 0 || strings.Contains(stderr, "this branch closes the brief") || !strings.Contains(stderr, "[bre-alternation]") {
		t.Errorf("closure at base: exit %d, want 0 with the R7 NOTICE only; stderr:\n%s", code, stderr)
	}
}
