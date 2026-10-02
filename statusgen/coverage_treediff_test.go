package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// coverage_treediff_test.go — #2026 cause 1: a witness is judged by whether
// any path it speaks for differs between the witness's tree and the item's,
// not by commit ancestry. The scope is unchanged (conservative: every path
// outside docs/streams/** and STATUS.md); only the ancestry requirement is
// replaced, by a tree comparison plus a shared-history requirement. Every
// test runs with coverageOptions{} — the production shape, no manifest.

const tdVerify = "| # | Command | Expect |\n|---|---------|--------|\n| 1 | `true` | exit 0 |"

const tdFiles = "`src/a.go`"

// tdScenario commits the brief (plus setup) as the witness tree W, then in
// one later commit records a passing witness at W and applies change. It
// returns the one claim.
func tdScenario(t *testing.T, setup, change func(t *testing.T, root string)) Claim {
	t.Helper()
	s, root := mustCoverageStream(t, "cov")
	writeCoverageBriefWithFiles(t, s.Dir, "01", "cov", tdFiles, tdVerify, "")
	mustWriteFile(t, root, "src/a.go", "a v1\n")
	if setup != nil {
		setup(t, root)
	}
	w := mustGitInit(t, root)
	ev := coverageEvidenceTable(covWitnessRow("1", "true", statePass, w))
	writeCoverageBriefWithFiles(t, s.Dir, "01", "cov", tdFiles, tdVerify, ev)
	if change != nil {
		change(t, root)
	}
	mustGitCommitAll(t, root, "record Evidence and apply the change")
	return soleClaim(t, root, s, coverageOptions{})
}

// tdWitnessAt commits the brief and src/a.go, lets prepare return the witness
// token to record (it may create commits, branches or orphans, and must leave
// the main branch checked out), then records a passing witness at that token
// and returns the one claim.
func tdWitnessAt(t *testing.T, prepare func(t *testing.T, root, mainBranch string) string) Claim {
	t.Helper()
	s, root := mustCoverageStream(t, "cov")
	writeCoverageBriefWithFiles(t, s.Dir, "01", "cov", tdFiles, tdVerify, "")
	mustWriteFile(t, root, "src/a.go", "a v1\n")
	mustGitInit(t, root)
	tok := prepare(t, root, gitBranchName(t, root))
	ev := coverageEvidenceTable(covWitnessRow("1", "true", statePass, tok))
	writeCoverageBriefWithFiles(t, s.Dir, "01", "cov", tdFiles, tdVerify, ev)
	mustGitCommitAll(t, root, "record Evidence")
	return soleClaim(t, root, s, coverageOptions{})
}

func gitBranchName(t *testing.T, root string) string {
	t.Helper()
	out, err := exec.Command("git", "-C", root, "rev-parse", "--abbrev-ref", "HEAD").Output()
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(out))
}

func mustGitOut(t *testing.T, root string, args ...string) string {
	t.Helper()
	out, err := exec.Command("git", append([]string{"-C", root}, args...)...).Output()
	if err != nil {
		t.Fatalf("git %v: %v", args, err)
	}
	return strings.TrimSpace(string(out))
}

// squashRepo builds the #2026 cause-1 shape: main at M0; a branch commits the
// work (src/a.go v2) at B, where the witness runs, then its Evidence; main
// moves on (mainChange, may be nil); the branch is squash-merged onto main,
// so B is NOT an ancestor of the item's revision. It returns the root, the
// stream, B and main's branch name.
func squashRepo(t *testing.T, setup, mainChange func(t *testing.T, root string)) (string, *Stream, string, string) {
	t.Helper()
	s, root := mustCoverageStream(t, "cov")
	writeCoverageBriefWithFiles(t, s.Dir, "01", "cov", tdFiles, tdVerify, "")
	mustWriteFile(t, root, "src/a.go", "a v1\n")
	if setup != nil {
		setup(t, root)
	}
	mustGitInit(t, root)
	mainBranch := gitBranchName(t, root)
	runGit(t, root, "checkout", "-q", "-b", "feat")
	mustWriteFile(t, root, "src/a.go", "a v2\n")
	b := mustGitCommitAll(t, root, "the work")
	ev := coverageEvidenceTable(covWitnessRow("1", "true", statePass, b))
	writeCoverageBriefWithFiles(t, s.Dir, "01", "cov", tdFiles, tdVerify, ev)
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

// TestTreeDiffSquashIdentical — #2026 cause 1: a witness written on a branch
// that was then squash-merged names a commit that is not an ancestor of main.
// When no path it speaks for differs between its tree and main's, it is
// reused (the receipt keeps the witness's revision and states the derivation).
func TestTreeDiffSquashIdentical(t *testing.T) {
	root, s, b, _ := squashRepo(t, writes("src/other.go", "o v1\n"), nil)
	got := soleClaim(t, root, s, coverageOptions{})
	if got.Result != covPass || got.Revision != b {
		t.Fatalf("a squash-merged witness whose scope is byte-identical must be reused at %s, got %+v", b, got)
	}
	if !strings.Contains(got.Reason, "reused") || !strings.Contains(got.Reason, "no path outside docs/streams/**") {
		t.Fatalf("a reused pass must state its derivation, got %q", got.Reason)
	}
}

// TestTreeDiffSquashChanged — the other side of cause 1: after a squash
// merge, any path in the conservative scope that differs still refuses,
// naming the path.
func TestTreeDiffSquashChanged(t *testing.T) {
	root, s, b, _ := squashRepo(t, writes("src/other.go", "o v1\n"), writes("src/other.go", "o v2\n"))
	got := soleClaim(t, root, s, coverageOptions{})
	if got.Result != covWrongRevision || got.Revision != b {
		t.Fatalf("a path that differs from the witness's tree must refuse, got %+v", got)
	}
	if !strings.Contains(got.Reason, "src/other.go") {
		t.Fatalf("the reason must name src/other.go, got %q", got.Reason)
	}
}

// TestTreeDiffConservativeHolds — #2026 cause 2 is NOT fixed here: with no
// dependency manifest the witness still speaks for every path outside the
// board's bookkeeping, so an unrelated release-bookkeeping change holds it.
func TestTreeDiffConservativeHolds(t *testing.T) {
	got := tdScenario(t, writes(".assay-versions", "v1\n"), writes(".assay-versions", "v2\n"))
	if got.Result != covWrongRevision || !strings.Contains(got.Reason, ".assay-versions") {
		t.Fatalf("the conservative scope must still hold on any non-bookkeeping change, got %+v", got)
	}
}

// TestTreeDiffUnreachableWitness — the witness commit is not in this clone's
// object store (a squash-merged branch commit that was never fetched, or a
// hand-edited sha). The comparison cannot be made, so the claim is
// could-not-check, naming why — never pass, and never a silent guess.
func TestTreeDiffUnreachableWitness(t *testing.T) {
	t.Run("squash branch not fetched", func(t *testing.T) {
		root, _, b, mainBranch := squashRepo(t, nil, nil)
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
		got := tdWitnessAt(t, func(t *testing.T, root, _ string) string { return "0123456789ab" })
		if got.Result != covCouldNotCheck || !strings.Contains(got.Reason, "not in this clone") {
			t.Fatalf("an unresolvable witness sha must be could-not-check naming it, got %+v", got)
		}
	})
}

// TestTreeDiffUnrelatedRoot — a witness commit that shares no history with
// the item's revision is refused even when its tree is identical in scope
// (security review, release-path row 10): an unrelated root is not a witness
// for this work.
func TestTreeDiffUnrelatedRoot(t *testing.T) {
	got := tdWitnessAt(t, func(t *testing.T, root, mainBranch string) string {
		runGit(t, root, "checkout", "-q", "--orphan", "stray")
		o := mustGitCommitAll(t, root, "unrelated root, same tree")
		runGit(t, root, "checkout", "-q", mainBranch)
		if exec.Command("git", "-C", root, "merge-base", o, mainBranch).Run() == nil {
			t.Fatalf("fixture drift: %s must share no history with %s", o, mainBranch)
		}
		return o
	})
	if got.Result != covWrongRevision || !strings.Contains(got.Reason, "shares no history") {
		t.Fatalf("an unrelated-root witness must be wrong-revision naming why, got %+v", got)
	}
}

// TestTreeDiffTokenShapes — a witness token is used only as the object id of
// a commit: a tree id is could-not-check ("not a commit", not "not in this
// clone"), and a hex-shaped ref NAME is could-not-check rather than resolved
// to whatever the ref points at.
func TestTreeDiffTokenShapes(t *testing.T) {
	t.Run("tree id", func(t *testing.T) {
		got := tdWitnessAt(t, func(t *testing.T, root, _ string) string {
			return mustGitOut(t, root, "rev-parse", "HEAD^{tree}")[:12]
		})
		if got.Result != covCouldNotCheck || !strings.Contains(got.Reason, "not a commit") {
			t.Fatalf("a tree id must be could-not-check as not a commit, got %+v", got)
		}
	})
	t.Run("ref name", func(t *testing.T) {
		const name = "abcdef012345"
		got := tdWitnessAt(t, func(t *testing.T, root, _ string) string {
			runGit(t, root, "branch", name, "HEAD")
			return name
		})
		if got.Result != covCouldNotCheck || !strings.Contains(got.Reason, "ref name") {
			t.Fatalf("a hex-shaped ref name must be could-not-check, got %+v", got)
		}
	})
}

// TestTreeDiffItemUnresolvable — the item's own revision names no commit in
// this clone: the comparison has no footing, so could-not-check, naming it.
func TestTreeDiffItemUnresolvable(t *testing.T) {
	s, root := mustCoverageStream(t, "cov")
	writeCoverageBriefWithFiles(t, s.Dir, "01", "cov", tdFiles, tdVerify, "")
	mustWriteFile(t, root, "src/a.go", "a v1\n")
	w := mustGitInit(t, root)
	ev := coverageEvidenceTable(covWitnessRow("1", "true", statePass, w))
	writeCoverageBriefWithFiles(t, s.Dir, "01", "cov", tdFiles, tdVerify, ev)
	mustGitCommitAll(t, root, "record Evidence")
	got := soleClaim(t, root, s, coverageOptions{Revision: "fedcba987654"})
	if got.Result != covCouldNotCheck || !strings.Contains(got.Reason, "item's revision fedcba987654 is not in this clone") {
		t.Fatalf("an unresolvable item revision must be could-not-check naming it, got %+v", got)
	}
}

// TestNoAncestryWitnessJudge is the class guard for #2026. The defect class:
// a witness's applicability judged by COMMIT ANCESTRY (`merge-base
// --is-ancestor`) rather than by the content of the paths it speaks for — a
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
				t.Errorf("%s asks git for commit ancestry (\"--is-ancestor\"); judge a witness by the content of what it speaks for (witnessTreeApplies), never by ancestry — #2026", f)
			}
		}
	}
}
