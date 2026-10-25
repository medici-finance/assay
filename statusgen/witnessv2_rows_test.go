package main

// witnessv2_rows_test.go — statusgen/36 Verify rows 11 (bounded git calls),
// 21 (the literal-path lower bound through lint and the reader) and the
// `witnessdigest` verb against the reader's own derivation.

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/medici-finance/assay/statusgen/evaluation"
)

// --- row 21 ------------------------------------------------------------------

const gv2OtherFile = "tools/x/other/z.go"

func gv2StreamOf(f *gitV2Fixture) []*Stream {
	return []*Stream{{Name: gv2Stream, Dir: filepath.Join(f.Root, "docs", "streams", gv2Stream), Root: f.Root}}
}

// TestWitnessLiteralPathBound: a Verify row whose Command names a file outside
// `inputs:` is a --lint PROBLEM and the reader refuses it as
// witness-input-undeclared; the same row with the path declared passes lint
// and releases. A command the library cannot read adds no bound.
// regression: #2026
func TestWitnessLiteralPathBound(t *testing.T) {
	mk := func(inputs string) *gitV2Fixture {
		f := newGitV2Fixture(t)
		f.Inputs = inputs
		f.ExtraFiles = map[string]string{gv2OtherFile: "package other\n"}
		f.Rows = []gv2Row{{ID: "1", Class: "check:ci", Command: "cat " + gv2OtherFile, Expect: "exit 0"}}
		f.Wits = []gv2Witness{{Row: "1", Identity: "verifier-app", Tail: " (forge-identity)"}}
		return f.build()
	}

	t.Run("undeclared path", func(t *testing.T) {
		f := mk("`tools/x/pkg/`")
		probs := witnessInputsProblems(f.Root, gv2StreamOf(f))
		if len(probs) != 1 || !strings.Contains(probs[0], gv2OtherFile) || !strings.Contains(probs[0], "witness-input-undeclared") {
			t.Fatalf("lint problems = %q, want one naming %s", probs, gv2OtherFile)
		}
		cl := claimFor(t, f.coverage(), "1")
		if cl.Released || !strings.HasPrefix(cl.Reason, evaluation.GapInputUndeclared) {
			t.Fatalf("reader: released=%v reason=%q, want a refusal starting %s", cl.Released, cl.Reason, evaluation.GapInputUndeclared)
		}
	})

	t.Run("declared path", func(t *testing.T) {
		f := mk("`tools/x/pkg/`, `" + gv2OtherFile + "`")
		if probs := witnessInputsProblems(f.Root, gv2StreamOf(f)); len(probs) != 0 {
			t.Fatalf("lint problems = %q, want none", probs)
		}
		cl := claimFor(t, f.coverage(), "1")
		if !cl.Released {
			t.Fatalf("reader: want a release, got %s: %s", cl.Result, cl.Reason)
		}
	})

	t.Run("unreadable command adds no bound", func(t *testing.T) {
		row := evaluation.VerifyRow{ID: "1", Command: "cat " + gv2OtherFile, Shell: "powershell"}
		exists := func(string) (bool, bool) { return true, false }
		if v := evaluation.InputsBoundViolations([]string{"tools/x/pkg/"}, row, exists); len(v) != 0 {
			t.Fatalf("a command the library cannot read produced a bound: %q", v)
		}
	})

	t.Run("bookkeeping in inputs", func(t *testing.T) {
		f := mk("`tools/x/pkg/`, `docs/streams/wv/README.md`")
		probs := witnessInputsProblems(f.Root, gv2StreamOf(f))
		if len(probs) == 0 || !strings.Contains(strings.Join(probs, "\n"), "bookkeeping") {
			t.Fatalf("lint problems = %q, want a bookkeeping-in-inputs problem", probs)
		}
	})
}

// --- witnessdigest verb ----------------------------------------------------------

// TestWitnessDigestVerb: the verb prints exactly the content the reader
// derives (the fixture's record was written by the same library function), the
// row's text digests, dirty_paths for --worktree, and never a releasable
// record.
// regression: #2026
func TestWitnessDigestVerb(t *testing.T) {
	f := newGitV2Fixture(t).build()
	bin := buildStatusgenBinary(t)
	run := func(args ...string) (witnessDigestOut, string) {
		t.Helper()
		cmd := exec.Command(bin, append([]string{"witnessdigest", "--json", "--root", f.Root, "--brief", f.BriefPath}, args...)...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("witnessdigest %v: %v\n%s", args, err, out)
		}
		var d witnessDigestOut
		if err := json.Unmarshal(out, &d); err != nil {
			t.Fatalf("decode: %v\n%s", err, out)
		}
		return d, string(out)
	}

	d, raw := run("--rev", f.Base, "--row", "1")
	repo := coverageRepo{root: f.Root}
	listing, _ := repo.ListTree(f.Base)
	var paths []string
	for _, e := range listing {
		paths = append(paths, e.Path)
	}
	attrs, _ := repo.CheckAttr(f.Base, paths)
	declared, found, _ := evaluation.ParseInputsDeclaration("inputs: " + f.Inputs)
	want := evaluation.ComputeWitnessContent(evaluation.WitnessClaimSpec(declared, found, nil, "1"), listing, attrs)
	if fmt.Sprint(d.Inputs) != fmt.Sprint(want.Inputs) || fmt.Sprint(d.Implicit) != fmt.Sprint(want.Implicit) || fmt.Sprint(d.Attributes) != fmt.Sprint(want.Attributes) || d.Scope != want.Scope {
		t.Fatalf("verb content %+v differs from the library's %+v", d, want)
	}
	vr := f.verifyRow("1")
	if d.CommandSHA != evaluation.WitnessTextDigest(vr.Command) || d.ExpectSHA != evaluation.WitnessTextDigest(vr.Expect) || d.ClassSHA != evaluation.WitnessTextDigest(vr.Class) {
		t.Fatalf("row digests %s/%s/%s do not match the library's", d.CommandSHA, d.ExpectSHA, d.ClassSHA)
	}
	for _, forbidden := range []string{`"runner"`, `"result"`, `"date"`, `"cr"`, `"rec"`, `"output_sha256"`} {
		if strings.Contains(raw, forbidden) {
			t.Errorf("the verb printed %s: it must never print a releasable record", forbidden)
		}
	}
	if len(d.DirtyPaths) != 0 {
		t.Errorf("--rev printed dirty_paths %v", d.DirtyPaths)
	}

	// A dirty edit to a declared input shows in --worktree only.
	gv2Write(t, f.Root, "tools/x/pkg/a.go", "package pkg\n\nfunc A() int { return 99 }\n")
	clean, _ := run("--rev", "HEAD")
	dirty, _ := run("--worktree")
	if got := dirty.DirtyPaths; len(got) != 1 || got[0] != "tools/x/pkg/a.go" {
		t.Fatalf("dirty_paths = %v, want [tools/x/pkg/a.go]", got)
	}
	if dirty.Inputs["tools/x/pkg/a.go"] == clean.Inputs["tools/x/pkg/a.go"] {
		t.Errorf("--worktree digested the committed blob, not the working tree")
	}
	if dirty.Inputs["tools/x/pkg/b.go"] != clean.Inputs["tools/x/pkg/b.go"] {
		t.Errorf("an untouched input's digest changed under --worktree")
	}
	if _, err := os.Stat(f.BriefPath); err != nil {
		t.Fatal(err)
	}
}

// --- row 11 --------------------------------------------------------------------------

type gitCallLog struct {
	mu    sync.Mutex
	root  string
	calls []string
}

func (l *gitCallLog) hook(root string, args []string) {
	if root != l.root {
		return
	}
	l.mu.Lock()
	l.calls = append(l.calls, strings.Join(args, " "))
	l.mu.Unlock()
}

// bulkV2Fixture lays down `briefs` briefs of `rows` rows, every row carrying a
// v2 witness, landed by `groups` change requests (so `groups` distinct
// recorded commits). It returns the repository root.
func bulkV2Fixture(t *testing.T, briefs, rows, groups int) string {
	t.Helper()
	root := t.TempDir()
	gv2Git(t, root, "init", "-q", "-b", "main")
	gv2Git(t, root, "config", "commit.gpgsign", "false")
	gv2Write(t, root, "docs/streams/"+gv2Stream+"/README.md", gv2README())
	gv2Write(t, root, evaluation.WitnessPinFile, gv2Pins)
	gv2Write(t, root, "tools/x/go.mod", "module example.test/x\n\ngo 1.25\n")
	gv2Write(t, root, "tools/x/pkg/a.go", "package pkg\n\nfunc A() int { return 1 }\n")
	gv2Write(t, root, "README.md", "fixture\n")
	var vrows []gv2Row
	for r := 1; r <= rows; r++ {
		vrows = append(vrows, gv2Row{ID: fmt.Sprint(r), Class: "check:ci", Command: "cat tools/x/pkg/a.go", Expect: fmt.Sprintf("exit 0; row %d", r)})
	}
	briefRel := func(n int) string {
		return fmt.Sprintf("docs/streams/%s/brief-%02d.md", gv2Stream, n)
	}
	briefText := func(n int, evidence string) string {
		s := gv2Brief("`tools/x/pkg/`", vrows, evidence)
		return strings.Replace(s, "brief: "+gv2BriefID, fmt.Sprintf("brief: %s/%02d", gv2Stream, n), 1)
	}
	for n := 1; n <= briefs; n++ {
		gv2Write(t, root, briefRel(n), briefText(n, ""))
	}
	gv2Git(t, root, "add", "-A")
	gv2Git(t, root, "commit", "-q", "-m", "fixture: base")
	declared, found, _ := evaluation.ParseInputsDeclaration("inputs: `tools/x/pkg/`")
	repo := coverageRepo{root: root}
	for g := 0; g < groups; g++ {
		base := gv2Git(t, root, "rev-parse", "HEAD")
		cr := fmt.Sprint(10 + g)
		gv2Git(t, root, "checkout", "-q", "-b", "fixture/g"+cr)
		listing, _ := repo.ListTree(base)
		var paths []string
		for _, e := range listing {
			paths = append(paths, e.Path)
		}
		attrs, _ := repo.CheckAttr(base, paths)
		for n := 1; n <= briefs; n++ {
			if (n-1)%groups != g {
				continue
			}
			id := fmt.Sprintf("%s/%02d", gv2Stream, n)
			var cells []string
			for _, vr := range vrows {
				content := evaluation.ComputeWitnessContent(evaluation.WitnessClaimSpec(declared, found, nil, vr.ID), listing, attrs)
				rec := evaluation.WitnessRecord{
					Schema: 2, Brief: id, Row: vr.ID,
					CommandSHA256: evaluation.WitnessTextDigest(vr.Command), ExpectSHA256: evaluation.WitnessTextDigest(vr.Expect), ClassSHA256: evaluation.WitnessTextDigest(vr.Class),
					Runner: "verifier-app", RunnerSource: "app-token", RunnerClass: evaluation.WitnessRunnerContainer, Network: "none",
					Tools:       map[string]string{"statusgen": "statusgen/v1.0.59", "desk-tools": "desk-tools/v1.0.59", "harness": gv2Harness, "go": "go1.25.3"},
					ToolsSource: "recorded", Origin: "run", Result: "pass exit=0", Exit: "0", OutputSHA256: gv2Output, Date: gv2Date,
					CR: cr, Scope: content.Scope, Inputs: content.Inputs, Implicit: content.Implicit, Attributes: content.Attributes,
					DirtyPaths: []string{}, Informational: evaluation.WitnessInformational{Commit: base},
				}
				canonical, rid, err := evaluation.EncodeWitnessRecord(rec)
				if err != nil {
					t.Fatal(err)
				}
				gv2Write(t, root, evaluation.WitnessRecordPath(id, rid), string(canonical))
				cells = append(cells, "| "+vr.ID+" | `"+vr.Command+"` | pass exit=0 | "+gv2Output+" | "+gv2Date+" | verifier-app @ [w2] cr:"+cr+" rec:"+rid+" (forge-identity) |")
			}
			gv2Write(t, root, briefRel(n), briefText(n, witnessHeader+"\n"+strings.Join(cells, "\n")))
		}
		gv2Git(t, root, "add", "-A")
		gv2GitAs(t, root, fixtureVerifierName, fixtureVerifierEmail, "commit", "-q", "-m", "Evidence: witness rows "+cr)
		gv2Git(t, root, "checkout", "-q", "main")
		gv2Git(t, root, "merge", "-q", "--no-ff", "-m", "Merge pull request #"+cr+" from example-org/g"+cr, "fixture/g"+cr)
	}
	return root
}

func countCoverageGitCalls(t *testing.T, briefs, rows, groups int) (calls []string, released, claims int) {
	t.Helper()
	root := bulkV2Fixture(t, briefs, rows, groups)
	log := &gitCallLog{root: root}
	coverageGitHook = log.hook
	t.Cleanup(func() { coverageGitHook = nil })
	s := &Stream{Name: gv2Stream, Dir: filepath.Join(root, "docs", "streams", gv2Stream), Root: root}
	cov := evaluateCoverage(root, []*Stream{s}, coverageOptions{})
	coverageGitHook = nil
	for _, c := range cov {
		for _, cl := range c.Claims {
			claims++
			if cl.Released {
				released++
			}
		}
	}
	return log.calls, released, claims
}

// TestCoverageGitCallsBounded: over 20 briefs x 10 rows landed by 3 change
// requests (three recorded commits, plus the item's revision), the git
// invocations the coverage port makes are at most 3 per distinct revision plus
// a constant, and do not grow with the row or brief count.
// regression: #2026
func TestCoverageGitCallsBounded(t *testing.T) {
	const (
		perRevision = 3
		constant    = 4
	)
	smallCalls, smallRel, smallClaims := countCoverageGitCalls(t, 3, 1, 3)
	bigCalls, bigRel, bigClaims := countCoverageGitCalls(t, 20, 10, 3)
	if bigClaims < 200 || bigRel != bigClaims {
		t.Fatalf("non-vacuity: %d of %d claims released over the 20x10 fixture, want all 200", bigRel, bigClaims)
	}
	if smallRel != smallClaims || smallClaims == 0 {
		t.Fatalf("non-vacuity: %d of %d claims released over the small fixture", smallRel, smallClaims)
	}
	t.Logf("git invocations: 3x1 -> %d, 20x10 -> %d", len(smallCalls), len(bigCalls))
	distinct := 3 + 1
	bound := perRevision*distinct + constant
	if len(bigCalls) > bound {
		t.Errorf("%d git invocations over 20x10x%d revisions, want at most %d:\n%s", len(bigCalls), distinct, bound, strings.Join(bigCalls, "\n"))
	}
	if len(bigCalls) > len(smallCalls)+constant {
		t.Errorf("git invocations grew with the corpus: %d for 3x1 vs %d for 20x10", len(smallCalls), len(bigCalls))
	}
}
