package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// coverage_inputs_test.go — #2026: a witness is stale when a path its row's
// command READS changed since it ran (cause 2), and only when the tree it ran
// on landed on the item's history (cause 1). Both refusal directions, and every
// fail-closed path, run with coverageOptions{} — the production shape, no
// dependency manifest.

// dsCmd is a derivable row: it reads src/a.go and nothing else.
const dsCmd = "grep -c a src/a.go"

func dsVerify(cmd string) string {
	return "| # | Command | Expect |\n|---|---------|--------|\n| 1 | `" + cmd + "` | exit 0 |"
}

// dsScenario commits the brief with row cmd (plus setup) as the witness tree
// W, then in one later commit records a passing witness at W and applies
// change. It returns the one claim.
func dsScenario(t *testing.T, cmd string, setup, change func(t *testing.T, root string)) Claim {
	t.Helper()
	root, s := dsRepo(t, cmd, setup, change)
	return soleClaim(t, root, s, coverageOptions{})
}

// dsRepo is dsScenario without the evaluation, for tests that alter the repo
// (a clone, a deleted object) before evaluating.
func dsRepo(t *testing.T, cmd string, setup, change func(t *testing.T, root string)) (string, *Stream) {
	t.Helper()
	s, root := mustCoverageStream(t, "cov")
	writeCoverageBriefWithFiles(t, s.Dir, "01", "cov", tdFiles, dsVerify(cmd), "")
	mustWriteFile(t, root, "src/a.go", "a v1\n")
	if setup != nil {
		setup(t, root)
	}
	w := mustGitInit(t, root)
	ev := coverageEvidenceTable(covWitnessRow("1", cmd, statePass, w))
	writeCoverageBriefWithFiles(t, s.Dir, "01", "cov", tdFiles, dsVerify(cmd), ev)
	if change != nil {
		change(t, root)
	}
	mustGitCommitAll(t, root, "record Evidence and apply the change")
	return root, s
}

// dsSquash is squashRepo with the derivable row: main at M0 (src/a.go v1); a
// branch writes src/a.go v2 and runs the witness at B; main
// may move (mainChange) before the branch is squash-merged; after may land on
// main once the squash is in. It returns the root, the stream and B.
func dsSquash(t *testing.T, mainChange, after func(t *testing.T, root string)) (string, *Stream, string) {
	t.Helper()
	s, root := mustCoverageStream(t, "cov")
	writeCoverageBriefWithFiles(t, s.Dir, "01", "cov", tdFiles, dsVerify(dsCmd), "")
	mustWriteFile(t, root, "src/a.go", "a v1\n")
	mustWriteFile(t, root, ".assay-versions", "v1\n")
	mustGitInit(t, root)
	mainBranch := gitBranchName(t, root)
	runGit(t, root, "checkout", "-q", "-b", "feat")
	mustWriteFile(t, root, "src/a.go", "a v2\n")
	b := mustGitCommitAll(t, root, "the work")
	ev := coverageEvidenceTable(covWitnessRow("1", dsCmd, statePass, b))
	writeCoverageBriefWithFiles(t, s.Dir, "01", "cov", tdFiles, dsVerify(dsCmd), ev)
	mustGitCommitAll(t, root, "record Evidence on the branch")
	runGit(t, root, "checkout", "-q", mainBranch)
	if mainChange != nil {
		mainChange(t, root)
		mustGitCommitAll(t, root, "main moves on")
	}
	runGit(t, root, "merge", "-q", "--squash", "feat")
	head := mustGitCommitAll(t, root, "squash-merge the branch")
	if after != nil {
		after(t, root)
		mustGitCommitAll(t, root, "main moves on after the squash")
	}
	if exec.Command("git", "-C", root, "merge-base", "--is-ancestor", b, head).Run() == nil {
		t.Fatalf("fixture drift: the witness commit %s must not be an ancestor of %s", b, head)
	}
	return root, s, b
}

func wantClaim(t *testing.T, got Claim, result string, reasonHas ...string) {
	t.Helper()
	if got.Result != result {
		t.Fatalf("want %s, got %+v", result, got)
	}
	for _, r := range reasonHas {
		if !strings.Contains(got.Reason, r) {
			t.Fatalf("the reason must contain %q, got %q", r, got.Reason)
		}
	}
}

// TestRowInputsGrammar — the closed grammar: each derivable row yields
// exactly the paths it reads; everything else is refused with a reason,
// never approximated.
func TestRowInputsGrammar(t *testing.T) {
	ok := []struct {
		cmd  string
		want []string
	}{
		{"grep -c a src/a.go", []string{"src/a.go"}},
		{"grep -q -e foo -e bar src/a.go src/b.go", []string{"src/a.go", "src/b.go"}},
		{"grep -qE 'a|b' -- src/a.go", []string{"src/a.go"}},
		{"cd tools && grep -c x a.go", []string{"tools/a.go"}},
		{"cd tools && cd sub && cat a.go", []string{"tools/sub/a.go"}},
		{"sed -n '1,5p' src/a.go \\| grep -c x", []string{"src/a.go"}},
		{"sed -n '/^x/,$p' src/a.go | wc -l", []string{"src/a.go"}},
		{"test -f src/a.go && echo ok", []string{"src/a.go"}},
		{"head -n 3 src/a.go | tail -1 | tr -d x", []string{"src/a.go"}},
		{"cat src/a.go | sort -u | uniq -c | cut -d, -f1", []string{"src/a.go"}},
		{"set -o pipefail; grep -c x \"src/a b.go\"", []string{"src/a b.go"}},
		{"grep -c x src/a.go || true", []string{"src/a.go"}},
		{"wc -l src && cat ./src/a.go", []string{"src", "src/a.go"}},
	}
	for _, c := range ok {
		got, err := deriveRowInputs(verifyRow{Command: c.cmd})
		if err != nil {
			t.Errorf("%q: want %v, got the refusal %v", c.cmd, c.want, err)
			continue
		}
		if !reflect.DeepEqual(got.paths, c.want) {
			t.Errorf("%q: want %v, got %v", c.cmd, c.want, got.paths)
		}
	}
	refused := []struct{ cmd, shell, why string }{
		{"go test ./statusgen", "", "outside the set"},
		{"bash scripts/x.sh", "", "outside the set"},
		{"git diff --stat", "", "outside the set"},
		{"grep -r x src", "", "grep option -r"},
		{"grep -c x src/a.go -r", "", "grep option -r"},
		{"grep -f pats src/a.go", "", "grep option -f"},
		{"grep --include=x a src", "", "grep option"},
		{"grep -c x $(ls)", "", "metacharacter"},
		{"grep -c x \"$F\"", "", "inside double quotes"},
		{"cat src/*.go", "", "metacharacter"},
		{"cat src/a.go > out", "", "metacharacter"},
		{"cat < src/a.go", "", "metacharacter"},
		{"cat src/a.go &", "", "background"},
		{"cat ~/x", "", "metacharacter"},
		{"cat ../x", "", "climbs"},
		{"cat src/../../x", "", "climbs"},
		{"cat /etc/hosts", "", "plain relative"},
		{"cat .git/config", "", ".git"},
		{"cat .", "", "repository root"},
		{"cat $F", "", "metacharacter \"$\""},
		{"grep -c a src/a.go && sh x.sh", "", "outside the set"},
		{"cd src ; cat a.go", "", "joined by"},
		{"cd src && cat a.go ; cat b.go", "", "joined by"},
		{"true || cd src && cat a.go", "", "joined by"},
		{"cd src || cat a.go", "", "joined by"},
		{"cd src | cat a.go", "", "stage of its own"},
		{"cd src a && cat a.go", "", "stage of its own"},
		{"FOO=1 cat src/a.go", "", "assignment"},
		{"sed 's/a/b/' src/a.go", "", "sed other than"},
		{"sed -n '1w out' src/a.go", "", "sed other than"},
		{"sort -o out src/a.go", "", "sort option -o"},
		{"grep -c x", "", "standard input"},
		{"echo hi | cat src/a.go", "", "filter cat names a file"},
		{"cat src/a.go | uniq -c out", "", "uniq with an operand"},
		{"echo ok", "", "reads no repository path"},
		{"test -d src", "", "test other than"},
		{"set -x; cat src/a.go", "", "set other than"},
		{"cat 'src/a  b.go'", "", "repeated space"},
		{"cat 'src/a.go", "", "unterminated"},
		{"cat src/a.go && ", "", "empty command"},
		{"cat src/a.go", "pwsh", "runs under pwsh"},
	}
	for _, c := range refused {
		_, err := deriveRowInputs(verifyRow{Command: c.cmd, Shell: c.shell})
		if err == nil || !errors.Is(err, errUnderivable) || !strings.Contains(err.Error(), c.why) {
			t.Errorf("%q: want a refusal naming %q, got %v", c.cmd, c.why, err)
		}
	}
}

// TestDepScopeUnrelatedReleases — #2026 cause 2, the release direction: a
// changelog fragment, release stamps and a README the row does not read
// leave its witness standing.
func TestDepScopeUnrelatedReleases(t *testing.T) {
	setup := writes(".assay-versions", "v1\n", ".claude-plugin/marketplace.json", "{}\n", "README.md", "r1\n", "src/b.go", "b1\n")
	got := dsScenario(t, dsCmd, setup, writes(
		".assay-versions", "v2\n",
		".claude-plugin/marketplace.json", "{\"v\":2}\n",
		"changelog/fix-x.md", "- fixed\n",
		"README.md", "r2\n",
		"src/b.go", "b2\n",
	))
	wantClaim(t, got, covPass, "src/a.go", "derived from its text", "landed at")
}

// TestDepScopeInputChangedHolds — the refusal direction: the file the row's
// command reads changed after the witness ran, so the witness is stale.
func TestDepScopeInputChangedHolds(t *testing.T) {
	t.Run("the file read", func(t *testing.T) {
		got := dsScenario(t, dsCmd, nil, writes("src/a.go", "a v2\n"))
		wantClaim(t, got, covWrongRevision, "src/a.go differs", "derived from its text")
	})
	t.Run("a file under a directory read", func(t *testing.T) {
		got := dsScenario(t, "wc -l src", nil, writes("src/new.go", "n\n"))
		wantClaim(t, got, covWrongRevision, "src/new.go differs")
	})
	t.Run("a file read after cd", func(t *testing.T) {
		got := dsScenario(t, "cd src && cat a.go", nil, writes("src/a.go", "a v2\n"))
		wantClaim(t, got, covWrongRevision, "src/a.go differs")
	})
	t.Run("the input deleted", func(t *testing.T) {
		got := dsScenario(t, dsCmd, nil, func(t *testing.T, root string) {
			if err := os.Remove(filepath.Join(root, "src", "a.go")); err != nil {
				t.Fatal(err)
			}
		})
		wantClaim(t, got, covWrongRevision, "src/a.go differs")
	})
}

// TestDepScopeAttributesHold — a `.gitattributes` on the way to an input can
// change the bytes the command reads without changing the input's blob, so a
// change to one refuses the witness.
func TestDepScopeAttributesHold(t *testing.T) {
	for _, ga := range []string{".gitattributes", "src/.gitattributes"} {
		t.Run(ga, func(t *testing.T) {
			got := dsScenario(t, dsCmd, nil, writes(ga, "*.go eol=crlf\n"))
			wantClaim(t, got, covWrongRevision, ga+" differs")
		})
	}
}

// TestDepScopeUnderivableHolds — a row whose inputs the grammar cannot
// establish keeps the conservative scope: an unrelated release stamp still
// refuses it, and the reason says why the inputs could not be derived.
func TestDepScopeUnderivableHolds(t *testing.T) {
	for _, cmd := range []string{"grep -c a $(echo src/a.go)", "go test ./src", "cat src/a.go > /dev/null"} {
		t.Run(cmd, func(t *testing.T) {
			got := dsScenario(t, cmd, writes(".assay-versions", "v1\n"), writes(".assay-versions", "v2\n"))
			wantClaim(t, got, covWrongRevision, ".assay-versions differs", "cannot be derived from its command")
		})
	}
}

// TestDepScopeLinkWidens — an input that is, or sits under, a symbolic link
// reads the link's target, which its path does not establish: the scope
// widens to conservative and an unrelated change refuses.
func TestDepScopeLinkWidens(t *testing.T) {
	cases := []struct{ name, cmd, link, target string }{
		{"the input is a link", "cat src/l.go", "src/l.go", "a.go"},
		{"a parent is a link", "cat lib/a.go", "lib", "src"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			setup := func(t *testing.T, root string) {
				mustWriteFile(t, root, ".assay-versions", "v1\n")
				if err := os.Symlink(c.target, filepath.Join(root, filepath.FromSlash(c.link))); err != nil {
					t.Fatal(err)
				}
			}
			got := dsScenario(t, c.cmd, setup, writes(".assay-versions", "v2\n"))
			wantClaim(t, got, covWrongRevision, ".assay-versions differs", "symbolic link or submodule "+c.link)
		})
	}
}

// TestDepScopeBriefReadWidens — a row that reads a file verify and regen
// write (its own brief, a stream README), directly or through a directory,
// keeps the conservative scope.
func TestDepScopeBriefReadWidens(t *testing.T) {
	cmds := []string{
		"grep -c Verify docs/streams/cov/brief-01.md",
		"cat docs/streams/cov/README.md",
		"wc -l docs/streams/cov",
	}
	for _, cmd := range cmds {
		t.Run(cmd, func(t *testing.T) {
			setup := writes(".assay-versions", "v1\n", "docs/streams/cov/README.md", "r\n")
			got := dsScenario(t, cmd, setup, writes(".assay-versions", "v2\n"))
			wantClaim(t, got, covWrongRevision, ".assay-versions differs", "verify and regen write")
		})
	}
}

// TestLandSquashReleases — #2026 cause 1, the release direction: a witness on
// a branch head is honoured after a squash merge whose tree equals the
// witness's (outside the board's bookkeeping), and stays honoured when an
// unrelated release stamp lands on main afterwards.
func TestLandSquashReleases(t *testing.T) {
	t.Run("squash equal", func(t *testing.T) {
		root, s, b := dsSquash(t, nil, nil)
		got := soleClaim(t, root, s, coverageOptions{})
		wantClaim(t, got, covPass, "landed at")
		if got.Revision != b {
			t.Fatalf("the receipt must keep the witness's revision %s, got %+v", b, got)
		}
	})
	t.Run("main moved only in bookkeeping", func(t *testing.T) {
		root, s, _ := dsSquash(t, writes("docs/streams/other/README.md", "x\n", "STATUS.md", "s\n"), nil)
		wantClaim(t, soleClaim(t, root, s, coverageOptions{}), covPass, "landed at")
	})
	t.Run("unrelated stamp after the squash", func(t *testing.T) {
		root, s, _ := dsSquash(t, nil, writes(".assay-versions", "v2\n", "changelog/x.md", "- x\n"))
		wantClaim(t, soleClaim(t, root, s, coverageOptions{}), covPass, "landed at")
	})
}

// TestLandNeverLandedRefused — the refusal direction: the witness's tree
// never existed on the item's history, so the check ran against code main
// never carried as a whole — even when nothing the row reads differs.
func TestLandNeverLandedRefused(t *testing.T) {
	t.Run("main moved before the squash", func(t *testing.T) {
		root, s, _ := dsSquash(t, writes(".assay-versions", "v2\n"), nil)
		wantClaim(t, soleClaim(t, root, s, coverageOptions{}), covWrongRevision, "never landed")
	})
	t.Run("never merged", func(t *testing.T) {
		s, root := mustCoverageStream(t, "cov")
		writeCoverageBriefWithFiles(t, s.Dir, "01", "cov", tdFiles, dsVerify(dsCmd), "")
		mustWriteFile(t, root, "src/a.go", "a v1\n")
		mustGitInit(t, root)
		mainBranch := gitBranchName(t, root)
		runGit(t, root, "checkout", "-q", "-b", "stray")
		mustWriteFile(t, root, "src/other.go", "o\n")
		b := mustGitCommitAll(t, root, "unmerged work the row does not read")
		runGit(t, root, "checkout", "-q", mainBranch)
		ev := coverageEvidenceTable(covWitnessRow("1", dsCmd, statePass, b))
		writeCoverageBriefWithFiles(t, s.Dir, "01", "cov", tdFiles, dsVerify(dsCmd), ev)
		mustGitCommitAll(t, root, "record Evidence")
		wantClaim(t, soleClaim(t, root, s, coverageOptions{}), covWrongRevision, "never landed")
	})
}

// TestLandShallowCannotCheck — a shallow clone cuts the history the landing
// search reads: could-not-check, naming it, never a pass.
func TestLandShallowCannotCheck(t *testing.T) {
	root, _ := dsRepo(t, dsCmd, nil, writes("changelog/x.md", "- x\n"))
	clone := filepath.Join(t.TempDir(), "clone")
	runGit(t, filepath.Dir(clone), "clone", "-q", "--depth", "2", "file://"+root, clone)
	if mustGitOut(t, clone, "rev-parse", "--is-shallow-repository") != "true" {
		t.Fatal("fixture drift: the clone must be shallow")
	}
	s := &Stream{Name: "cov", Dir: filepath.Join(clone, "docs", "streams", "cov"), Root: clone}
	wantClaim(t, soleClaim(t, clone, s, coverageOptions{}), covCouldNotCheck, "shallow")
}

// TestLandBoundCannotCheck — more commits to compare than the search's bound:
// could-not-check, never assumed landed.
func TestLandBoundCannotCheck(t *testing.T) {
	old := maxLandingCandidates
	maxLandingCandidates = 0
	t.Cleanup(func() { maxLandingCandidates = old })
	root, s, _ := dsSquash(t, nil, nil)
	wantClaim(t, soleClaim(t, root, s, coverageOptions{}), covCouldNotCheck, "past the bound")
}

// dsDropTree deletes the loose root-tree object of rev, so git can no longer
// read that commit's tree.
func dsDropTree(t *testing.T, root, rev string) {
	t.Helper()
	id := mustGitOut(t, root, "rev-parse", rev+"^{tree}")
	if err := os.Remove(filepath.Join(root, ".git", "objects", id[:2], id[2:])); err != nil {
		t.Fatalf("fixture drift: the tree %s must be a loose object: %v", id, err)
	}
}

// TestLandGitErrorCannotCheck — git cannot read a tree the judgement needs:
// could-not-check, naming which step failed, never a pass.
func TestLandGitErrorCannotCheck(t *testing.T) {
	t.Run("the witness's tree", func(t *testing.T) {
		root, s := dsRepo(t, dsCmd, nil, writes("changelog/x.md", "- x\n"))
		dsDropTree(t, root, "HEAD~1")
		wantClaim(t, soleClaim(t, root, s, coverageOptions{}), covCouldNotCheck, "ls-tree failed")
	})
	t.Run("a landing candidate's tree", func(t *testing.T) {
		root, s, b := dsSquash(t, nil, nil)
		base := mustGitOut(t, root, "merge-base", b, "HEAD")
		dsDropTree(t, root, base)
		wantClaim(t, soleClaim(t, root, s, coverageOptions{}), covCouldNotCheck, "git diff failed")
	})
}

// TestNoFixedPathExemption is the class guard for #2026 cause 2. The defect
// class: deciding a witness's staleness by a FIXED list of paths instead of
// by what the row reads — whether every path (the defect) or a hand-kept
// list of exempt paths (the tempting "fix"). A changelog fragment or release
// stamp must stop staling a witness only because the row does not read it,
// never because the path is named as exempt: a row that DOES read it must
// still be refused. So the bookkeeping exemptions stay docs/streams/** and
// STATUS.md, and no non-test coverage file names a release-stamp path.
func TestNoFixedPathExemption(t *testing.T) {
	stamps := []string{"changelog/x.md", ".claude-plugin/marketplace.json", ".assay-versions", "README.md", "CHANGELOG.md"}
	for _, p := range stamps {
		if isBoardBookkeepingPath(p) || isVerifyWrittenPath(p) {
			t.Errorf("%s is exempt by name; a witness stops being stale only because its row does not read a path (#2026)", p)
		}
	}
	files, err := filepath.Glob("coverage*.go")
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
		for _, lit := range []string{`"changelog/`, `"CHANGELOG`, `marketplace.json"`, `".assay-versions"`, `"README.md"`} {
			if strings.Contains(string(b), lit) {
				t.Errorf("%s names the release path %s; judge staleness by the row's derived inputs, never by a fixed exemption (#2026)", f, lit)
			}
		}
	}
	// And the direction the exemption would break: a row that reads a
	// release stamp is still refused when the stamp changes.
	got := dsScenario(t, "grep -c v .assay-versions", writes(".assay-versions", "v1\n"), writes(".assay-versions", "v2\n"))
	wantClaim(t, got, covWrongRevision, ".assay-versions differs")
}
