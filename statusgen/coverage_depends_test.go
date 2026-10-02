package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// coverage_depends_test.go — #2026: a witness is judged by whether the paths
// its brief's Verify rows depend on differ between the witness's tree and the
// item's, never by commit ancestry and never by "any file changed" when the
// brief declares its dependencies (`verify-depends:` in `## Context`). Every
// test runs with coverageOptions{} — the production shape, no manifest.

const depVerify = "| # | Command | Expect |\n|---|---------|--------|\n| 1 | `true` | exit 0 |"

// depCtx renders the `files:` value writeCoverageBriefWithFiles expects plus
// an optional verify-depends: line (dependsLine is written verbatim after the
// label; "-" omits the label).
func depCtx(dependsLine string) string {
	if dependsLine == "-" {
		return "`src/a.go`"
	}
	return "`src/a.go`\nverify-depends:" + dependsLine
}

// depScenario commits the brief (context ctxW, plus setup) as the witness tree
// W, then in one later commit records a passing witness at W, rewrites the
// brief's context to ctxNow and applies change. It returns the one claim.
func depScenario(t *testing.T, ctxW, ctxNow string, setup, change func(t *testing.T, root string)) Claim {
	t.Helper()
	s, root := mustCoverageStream(t, "cov")
	writeCoverageBriefWithFiles(t, s.Dir, "01", "cov", ctxW, depVerify, "")
	mustWriteFile(t, root, "src/a.go", "a v1\n")
	if setup != nil {
		setup(t, root)
	}
	w := mustGitInit(t, root)
	ev := coverageEvidenceTable(covWitnessRow("1", "true", statePass, w))
	writeCoverageBriefWithFiles(t, s.Dir, "01", "cov", ctxNow, depVerify, ev)
	if change != nil {
		change(t, root)
	}
	mustGitCommitAll(t, root, "record Evidence and apply the change")
	return soleClaim(t, root, s, coverageOptions{})
}

// squashRepo builds the #2026 cause-1 shape: main at M0; a branch commits the
// work (src/a.go v2) at B, where the witness runs, then its Evidence; main
// moves on (mainChange, may be nil); the branch is squash-merged onto main,
// so B is NOT an ancestor of the item's revision. It returns the root, the
// stream, B and main's branch name.
func squashRepo(t *testing.T, ctx string, setup, mainChange func(t *testing.T, root string)) (string, *Stream, string, string) {
	t.Helper()
	s, root := mustCoverageStream(t, "cov")
	writeCoverageBriefWithFiles(t, s.Dir, "01", "cov", ctx, depVerify, "")
	mustWriteFile(t, root, "src/a.go", "a v1\n")
	if setup != nil {
		setup(t, root)
	}
	mustGitInit(t, root)
	out, err := exec.Command("git", "-C", root, "rev-parse", "--abbrev-ref", "HEAD").Output()
	if err != nil {
		t.Fatal(err)
	}
	mainBranch := strings.TrimSpace(string(out))
	runGit(t, root, "checkout", "-q", "-b", "feat")
	mustWriteFile(t, root, "src/a.go", "a v2\n")
	b := mustGitCommitAll(t, root, "the work")
	ev := coverageEvidenceTable(covWitnessRow("1", "true", statePass, b))
	writeCoverageBriefWithFiles(t, s.Dir, "01", "cov", ctx, depVerify, ev)
	mustGitCommitAll(t, root, "record Evidence on the branch")
	runGit(t, root, "checkout", "-q", mainBranch)
	if mainChange != nil {
		mainChange(t, root)
		mustGitCommitAll(t, root, "main moves on")
	}
	runGit(t, root, "merge", "-q", "--squash", "feat")
	head := mustGitCommitAll(t, root, "squash-merge the branch")
	if exec.Command("git", "-C", root, "merge-base", "--is-ancestor", b, head).Run() == nil {
		t.Fatalf("fixture drift: the witness commit %s must not be an ancestor of the squash commit %s", b, head)
	}
	return root, s, b, mainBranch
}

// TestDependsSquashIdentical — #2026 cause 1: a witness written on a branch
// that was then squash-merged names a commit that is not an ancestor of main.
// When every path it speaks for is byte-identical between its tree and main's,
// it is reused (the receipt keeps the witness's revision). That holds in the
// conservative scope when main did not otherwise move, and in DECLARED mode
// when main moved only outside the declared dependencies.
func TestDependsSquashIdentical(t *testing.T) {
	cases := []struct {
		name, ctx  string
		mainChange func(t *testing.T, root string)
		wantReason string
	}{
		{"conservative, main did not move", depCtx("-"), nil, "no path outside docs/streams/**"},
		{"declared, unrelated main change", depCtx(" `src/a.go`"), writes("src/other.go", "o v2\n"), "verify-depends:"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root, s, b, _ := squashRepo(t, tc.ctx, writes("src/other.go", "o v1\n"), tc.mainChange)
			got := soleClaim(t, root, s, coverageOptions{})
			if got.Result != covPass || got.Revision != b {
				t.Fatalf("a squash-merged witness whose dependencies are byte-identical must be reused at %s, got %+v", b, got)
			}
			if !strings.Contains(got.Reason, "reused") || !strings.Contains(got.Reason, tc.wantReason) {
				t.Fatalf("a reused pass must state its derivation (%q), got %q", tc.wantReason, got.Reason)
			}
		})
	}
}

// TestDependsSquashDepChanged — the other side of cause 1: after a squash
// merge, a declared dependency that differs on main still refuses, as does
// any non-bookkeeping difference in the conservative scope.
func TestDependsSquashDepChanged(t *testing.T) {
	cases := []struct{ name, ctx, path string }{
		{"declared dependency differs", depCtx(" `src/a.go`, `src/dep.go`"), "src/dep.go"},
		{"conservative, any path differs", depCtx("-"), "src/other.go"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			setup := writes("src/dep.go", "d v1\n", "src/other.go", "o v1\n")
			root, s, b, _ := squashRepo(t, tc.ctx, setup, writes(tc.path, "v2\n"))
			got := soleClaim(t, root, s, coverageOptions{})
			if got.Result != covWrongRevision || got.Revision != b {
				t.Fatalf("a dependency that differs from the witness's tree must refuse, got %+v", got)
			}
			if !strings.Contains(got.Reason, tc.path) {
				t.Fatalf("the reason must name %s, got %q", tc.path, got.Reason)
			}
		})
	}
}

// TestDependsUnreachableWitness — the witness commit is not in this clone's
// object store (a squash-merged branch commit that was never fetched, or a
// hand-edited sha). The comparison cannot be made, so the claim is
// could-not-check, naming why — never pass, and never a silent guess.
func TestDependsUnreachableWitness(t *testing.T) {
	t.Run("squash branch not fetched", func(t *testing.T) {
		root, _, b, mainBranch := squashRepo(t, depCtx(" `src/a.go`"), nil, nil)
		clone := filepath.Join(t.TempDir(), "clone")
		runGit(t, filepath.Dir(clone), "clone", "-q", "--no-local", "--single-branch", "--branch", mainBranch, root, clone)
		if exec.Command("git", "-C", clone, "cat-file", "-e", b+"^{commit}").Run() == nil {
			t.Fatalf("fixture drift: the clone must not carry the branch commit %s", b)
		}
		s := &Stream{Name: "cov", Dir: filepath.Join(clone, "docs", "streams", "cov"), Root: clone}
		got := soleClaim(t, clone, s, coverageOptions{})
		if got.Result != covCouldNotCheck || !strings.Contains(got.Reason, "not in this clone") {
			t.Fatalf("an unreachable witness tree must be could-not-check naming it, got %+v", got)
		}
	})
	t.Run("sha absent from the repo", func(t *testing.T) {
		s, root := mustCoverageStream(t, "cov")
		writeCoverageBriefWithFiles(t, s.Dir, "01", "cov", depCtx(" `src/a.go`"), depVerify, "")
		mustWriteFile(t, root, "src/a.go", "a v1\n")
		mustGitInit(t, root)
		ev := coverageEvidenceTable(covWitnessRow("1", "true", statePass, "0123456789ab"))
		writeCoverageBriefWithFiles(t, s.Dir, "01", "cov", depCtx(" `src/a.go`"), depVerify, ev)
		mustGitCommitAll(t, root, "record Evidence at a sha this repo never had")
		got := soleClaim(t, root, s, coverageOptions{})
		if got.Result != covCouldNotCheck || !strings.Contains(got.Reason, "not in this clone") {
			t.Fatalf("an unresolvable witness sha must be could-not-check naming it, got %+v", got)
		}
	})
}

// TestDependsUnrelatedReleases — #2026 cause 2: a brief that declares what its
// Verify rows read keeps its witness across a change to a path outside that
// set (the issue's `.claude-plugin/marketplace.json`, `.assay-versions`).
// Without the declaration the conservative scope still holds it.
func TestDependsUnrelatedReleases(t *testing.T) {
	setup := writes(".assay-versions", "v1\n")
	change := writes(".assay-versions", "v2\n")
	got := depScenario(t, depCtx(" `src/a.go`"), depCtx(" `src/a.go`"), setup, change)
	if got.Result != covPass || !strings.Contains(got.Reason, "verify-depends:") {
		t.Fatalf("a change outside the declared dependencies must reuse the witness, got %+v", got)
	}
	got = depScenario(t, depCtx("-"), depCtx("-"), setup, change)
	if got.Result != covWrongRevision || !strings.Contains(got.Reason, ".assay-versions") {
		t.Fatalf("with no declaration the conservative scope must still hold, got %+v", got)
	}
}

// TestDependsChangedRefuses — a stale witness whose declared dependency
// changed refuses, whatever form the change takes: an edit, a rename away, a
// delete, a change under a declared directory, a dependency dropped from the
// line after the run (the union with the line at the witness's commit), and
// a resolving `files:` entry the line does not repeat.
func TestDependsChangedRefuses(t *testing.T) {
	mv := func(from, to string) func(t *testing.T, root string) {
		return func(t *testing.T, root string) {
			t.Helper()
			runGit(t, root, "mv", from, to)
		}
	}
	rm := func(p string) func(t *testing.T, root string) {
		return func(t *testing.T, root string) {
			t.Helper()
			runGit(t, root, "rm", "-q", p)
		}
	}
	dep := depCtx(" `src/dep.go`")
	cases := []struct {
		name, ctxW, ctxNow string
		change             func(t *testing.T, root string)
		path               string
	}{
		{"edited", dep, dep, writes("src/dep.go", "d v2\n"), "src/dep.go"},
		{"renamed away", dep, dep, mv("src/dep.go", "src/moved.go"), "src/dep.go"},
		{"deleted", dep, dep, rm("src/dep.go"), "src/dep.go"},
		{"under a declared dir", depCtx(" `lib/`"), depCtx(" `lib/`"), writes("lib/x.go", "x v2\n"), "lib/x.go"},
		{"line narrowed after run", depCtx(" `src/dep.go`, `lib/`"), dep, writes("lib/x.go", "x v2\n"), "lib/x.go"},
		{"files: entry changed", dep, dep, writes("src/a.go", "a v2\n"), "src/a.go"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := depScenario(t, tc.ctxW, tc.ctxNow, writes("src/dep.go", "d v1\n", "lib/x.go", "x v1\n"), tc.change)
			if got.Result != covWrongRevision {
				t.Fatalf("a changed dependency must refuse as wrong-revision, got %+v", got)
			}
			if !strings.Contains(got.Reason, tc.path) {
				t.Fatalf("the reason must name %s, got %q", tc.path, got.Reason)
			}
		})
	}
}

// TestDependsFailClosed — a verify-depends: line that cannot establish the
// dependency set refuses as could-not-check, naming why, even when nothing it
// could name has changed: an empty line, an unparseable one (unclosed
// backtick), an entry that names no real file, an entry naming only a file no
// witness can speak for, an unsupported form, and a line that was empty at
// the witness's commit. None ever falls back to a release.
func TestDependsFailClosed(t *testing.T) {
	unrelated := writes("src/other.go", "o v2\n")
	cases := []struct {
		name, ctxW, ctxNow, why string
	}{
		{"empty line", depCtx(""), depCtx(""), "names no path"},
		{"unparseable line", depCtx(" `src/a.go"), depCtx(" `src/a.go"), "cannot be parsed"},
		{"entry names nothing", depCtx(" `src/nosuch.go`"), depCtx(" `src/nosuch.go`"), "src/nosuch.go"},
		{"entry is prose", depCtx(" `src/a.go` (new)"), depCtx(" `src/a.go` (new)"), "(new)"},
		{"only an exempt file", depCtx(" `STATUS.md`"), depCtx(" `STATUS.md`"), "STATUS.md"},
		{"brace form", depCtx(" `src/{a,b}.go`"), depCtx(" `src/{a,b}.go`"), "src/{a"},
		{"empty at the witness", depCtx(""), depCtx(" `src/a.go`"), "at the witness's commit"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := depScenario(t, tc.ctxW, tc.ctxNow, writes("src/other.go", "o v1\n", "STATUS.md", "board\n"), unrelated)
			if got.Result != covCouldNotCheck {
				t.Fatalf("an unestablished dependency set must be could-not-check, got %+v", got)
			}
			if !strings.Contains(got.Reason, tc.why) {
				t.Fatalf("the reason must say why (%q), got %q", tc.why, got.Reason)
			}
		})
	}
}

// TestDependsLineParses pins the verify-depends: reader's four states and that
// it never swallows a neighbouring `files:` value (or the reverse).
func TestDependsLineParses(t *testing.T) {
	cases := []struct {
		name, ctx string
		want      []string
		st        labelState
	}{
		{"absent", "files: `a/x.go`", nil, labelAbsent},
		{"empty", "files: `a/x.go`\nverify-depends:", nil, labelEmpty},
		{"unclosed", "verify-depends: `a/x.go", nil, labelUnparseable},
		{"inline", "files: `a/x.go`\nverify-depends: `b/y.go`, c/", []string{"b/y.go", "c/"}, labelEntries},
		{"bulleted", "verify-depends:\n- `b/y.go`\n- c/\nfiles: `a/x.go`", []string{"b/y.go", "c/"}, labelEntries},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body := "## Context\n" + tc.ctx + "\n\n## Task\nx\n"
			got, st := extractContextLabelEntries(body, contextVerifyDependsLabelRe)
			if st != tc.st || strings.Join(got, "|") != strings.Join(tc.want, "|") {
				t.Fatalf("want %q (state %d), got %q (state %d)", tc.want, tc.st, got, st)
			}
			if files, found := extractContextDeclaredEntriesRaw(body); found && strings.Join(files, "|") != "a/x.go" {
				t.Fatalf("files: must read only its own value, got %q", files)
			}
		})
	}
}

// TestNoAncestryWitnessJudge is the class guard for #2026. The defect class:
// a witness's applicability judged by COMMIT ANCESTRY (`merge-base
// --is-ancestor`) rather than by the content of the paths it depends on — a
// squash merge breaks ancestry without changing a byte the check read. No
// non-test file in this package may ask git for ancestry; witnessTreeApplies
// compares trees instead. A future need for ancestry that is NOT a witness
// judgement must be argued in review and allowlisted here by file, with why.
func TestNoAncestryWitnessJudge(t *testing.T) {
	allow := map[string]string{} // file -> why; empty by design
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(b), `"--is-ancestor"`) {
			if _, ok := allow[f]; !ok {
				t.Errorf("%s asks git for commit ancestry (\"--is-ancestor\"); judge a witness by the content of what it depends on (witnessTreeApplies), never by ancestry — #2026", f)
			}
		}
	}
}

// TestDependsUnknownAtWitness — what the Verify rows depended on cannot be
// read at the witness's commit (the brief did not exist there), or the item's
// own revision names no commit in this clone: either way the comparison has
// no footing, so the claim is could-not-check, naming why.
func TestDependsUnknownAtWitness(t *testing.T) {
	t.Run("brief absent at witness", func(t *testing.T) {
		s, root := mustCoverageStream(t, "cov")
		mustWriteFile(t, root, "src/a.go", "a v1\n")
		w := mustGitInit(t, root)
		ev := coverageEvidenceTable(covWitnessRow("1", "true", statePass, w))
		writeCoverageBriefWithFiles(t, s.Dir, "01", "cov", depCtx(" `src/a.go`"), depVerify, ev)
		mustGitCommitAll(t, root, "the brief arrives after its witness ran")
		got := soleClaim(t, root, s, coverageOptions{})
		if got.Result != covCouldNotCheck || !strings.Contains(got.Reason, "cannot be read at the witness's commit") {
			t.Fatalf("a brief unreadable at the witness's commit must be could-not-check naming it, got %+v", got)
		}
	})
	t.Run("item revision unresolvable", func(t *testing.T) {
		s, root := mustCoverageStream(t, "cov")
		writeCoverageBriefWithFiles(t, s.Dir, "01", "cov", depCtx(" `src/a.go`"), depVerify, "")
		mustWriteFile(t, root, "src/a.go", "a v1\n")
		w := mustGitInit(t, root)
		ev := coverageEvidenceTable(covWitnessRow("1", "true", statePass, w))
		writeCoverageBriefWithFiles(t, s.Dir, "01", "cov", depCtx(" `src/a.go`"), depVerify, ev)
		mustGitCommitAll(t, root, "record Evidence")
		got := soleClaim(t, root, s, coverageOptions{Revision: "fedcba987654"})
		if got.Result != covCouldNotCheck || !strings.Contains(got.Reason, "does not resolve to a commit") {
			t.Fatalf("an unresolvable item revision must be could-not-check naming it, got %+v", got)
		}
	})
}
