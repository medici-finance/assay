package main

import (
	"errors"
	"fmt"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"
	"time"
)

// historyCase is a compare interval: every commit reachable from head but not from
// the reviewed head, each with the files its first-parent diff lists.
type historyCase struct {
	name    string
	commits []deskkit.RepoCommit
	want    bool
}

func cf(names ...string) []deskkit.ChangedFile {
	out := []deskkit.ChangedFile{}
	for _, n := range names {
		out = append(out, deskkit.ChangedFile{Filename: n})
	}
	return out
}

func rc(sha string, parents []string, files ...string) deskkit.RepoCommit {
	return deskkit.RepoCommit{SHA: sha, Parents: parents, Files: cf(files...), FilesComplete: true}
}

// stubHistory serves c as the compare interval (parents only, as GitHub's compare
// returns them) and each commit's files on GetCommit, recording every commit read.
func stubHistory(t *testing.T, commits []deskkit.RepoCommit) *[]string {
	t.Helper()
	var reads []string
	byID := map[string]deskkit.RepoCommit{}
	for _, c := range commits {
		byID[c.SHA] = c
	}
	stubForgeHooks(t, forgeHookSet{
		compare: func(_, _, _ string) (*deskkit.RefComparison, error) {
			out := &deskkit.RefComparison{CommitsComplete: true, Files: cf("main.go")}
			for _, c := range commits {
				out.Commits = append(out.Commits, deskkit.RepoCommit{SHA: c.SHA, Parents: c.Parents})
			}
			return out, nil
		},
		getCommit: func(_, sha string) (*deskkit.RepoCommit, error) {
			reads = append(reads, sha)
			c, ok := byID[sha]
			if !ok {
				return nil, errors.New("commit outside the interval read: " + sha)
			}
			return &c, nil
		},
	})
	return &reads
}

func TestOwnCommitHistory(t *testing.T) {
	r := func(p ...string) []string { return p }
	for _, tc := range []historyCase{
		{"merge plus own fix", []deskkit.RepoCommit{
			rc("merge", r("review", "main-tip"), "main.go"),
			rc("head", r("merge"), "own.go"),
		}, true},
		// The fix and its revert cancel in the aggregate diff (main.go only); the
		// per-commit history still shows both edits.
		{"fix later reverted", []deskkit.RepoCommit{
			rc("fix", r("review"), "own.go"),
			rc("merge", r("fix", "main-tip"), "main.go"),
			rc("head", r("merge"), "own.go"),
		}, true},
		{"keep current unrelated", []deskkit.RepoCommit{
			rc("merge", r("review", "main-tip"), "main.go"),
			rc("head", r("merge"), "main.go"),
		}, false},
		{"rename away from own", []deskkit.RepoCommit{
			rc("merge", r("review", "main-tip"), "main.go"),
			{SHA: "head", Parents: r("merge"), FilesComplete: true, Files: []deskkit.ChangedFile{{Filename: "new.go", PreviousFilename: "own.go"}}},
		}, true},
		// Another branch's catch-up merge landed on main after the reviewed head. Its
		// first-parent diff re-lists main changes the reviewed head already holds,
		// including own.go; the PR's own keep-current merge changes no own file.
		{"main-side catch-up merge", []deskkit.RepoCommit{
			rc("other-fix", r("main-old"), "own.go"),
			rc("other-merge", r("other-fix", "main-old"), "own.go"),
			rc("main-tip", r("main-old", "other-merge"), "main.go"),
			rc("head", r("review", "main-tip"), "main.go"),
		}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reads := stubHistory(t, tc.commits)
			changed, err := changedFilesBetween("example-org/tracker", "review", "head")
			if err != nil {
				t.Fatal(err)
			}
			got := ownFilesChanged(map[string]bool{"own.go": true}, true, changed)
			if got != tc.want {
				t.Fatalf("own-file edit detected=%v, want %v; union=%v", got, tc.want, changed)
			}
			action, _ := classify(classifyInput{ever: true, ownFilesChanged: got})
			want := actMergeCurr
			if tc.want {
				want = actReReview
			}
			if action != want {
				t.Fatalf("action=%s want %s", action, want)
			}
			for _, sha := range *reads {
				if strings.HasPrefix(sha, "other-") || sha == "main-tip" {
					t.Fatalf("read main-side commit %s off the first-parent chain; reads=%v", sha, *reads)
				}
			}
		})
	}
}

func TestHistoryReadDegrades(t *testing.T) {
	for _, mode := range []string{"short history", "missing parents", "short files", "read failed",
		"rewind", "main as first parent", "root commit"} {
		t.Run(mode, func(t *testing.T) {
			stubForgeHooks(t, forgeHookSet{
				compare: func(_, _, _ string) (*deskkit.RefComparison, error) {
					c := deskkit.RepoCommit{SHA: "head", Parents: []string{"base"}}
					commits := []deskkit.RepoCommit{c}
					switch mode {
					case "missing parents":
						commits[0].Parents = nil
					case "rewind":
						// head is an ancestor of the reviewed sha: the interval is empty
						// and reads complete, but head was never walked to base.
						commits = nil
					case "main as first parent":
						commits = []deskkit.RepoCommit{
							{SHA: "head", Parents: []string{"main-tip", "base"}},
							{SHA: "main-tip", Parents: []string{"main-old"}},
						}
					case "root commit":
						commits[0].Parents = []string{}
					}
					return &deskkit.RefComparison{Commits: commits, CommitsComplete: mode != "short history"}, nil
				},
				getCommit: func(_, sha string) (*deskkit.RepoCommit, error) {
					if mode == "read failed" {
						return nil, errors.New("unreadable")
					}
					return &deskkit.RepoCommit{SHA: sha, FilesComplete: mode != "short files"}, nil
				},
			})
			if _, err := changedFilesBetween("example-org/tracker", "base", "head"); err == nil {
				t.Fatal("incomplete evidence must not prove a benign merge")
			}
		})
	}
}

// Compare consumers form a closed roster. Any new classifier path must use the
// commit-history choke point; aggregate files cannot establish review currency,
// and the interval's commit list is read only through firstParentChain (a raw walk
// over every interval commit replays main-side merges already in the reviewed head).
func compareGuard(src []byte) []string {
	f, err := parser.ParseFile(token.NewFileSet(), "guard.go", src, 0)
	if err != nil {
		return []string{err.Error()}
	}
	var bad []string
	for _, d := range f.Decls {
		fn, ok := d.(*ast.FuncDecl)
		if !ok {
			continue
		}
		comparisons := map[string]bool{}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			if assign, ok := n.(*ast.AssignStmt); ok {
				for _, rhs := range assign.Rhs {
					if call, ok := rhs.(*ast.CallExpr); ok {
						if sel, ok := call.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "CompareRefs" && len(assign.Lhs) > 0 {
							if id, ok := assign.Lhs[0].(*ast.Ident); ok {
								comparisons[id.Name] = true
							}
						}
					}
				}
			}
			return true
		})
		viaChain := map[ast.Node]bool{}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			if call, ok := n.(*ast.CallExpr); ok {
				if id, ok := call.Fun.(*ast.Ident); ok && id.Name == "firstParentChain" {
					for _, a := range call.Args {
						viaChain[a] = true
					}
				}
			}
			if sel, ok := n.(*ast.SelectorExpr); ok {
				if sel.Sel.Name == "Commits" && !viaChain[sel] {
					if id, ok := sel.X.(*ast.Ident); ok && comparisons[id.Name] {
						bad = append(bad, fn.Name.Name+": interval commits off the first-parent chain")
					}
				}
				if sel.Sel.Name == "CompareRefs" && fn.Name.Name != "changedFilesBetween" && fn.Name.Name != "fetchBehindMain" {
					bad = append(bad, fn.Name.Name+": raw compare consumer")
				}
				if sel.Sel.Name == "Files" {
					if id, ok := sel.X.(*ast.Ident); ok && comparisons[id.Name] {
						bad = append(bad, fn.Name.Name+": aggregate files")
					}
				}
			}
			return true
		})
	}
	return bad
}
func TestCompareClassGuard(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		src, err := os.ReadFile(e.Name())
		if err != nil {
			t.Fatal(err)
		}
		if bad := compareGuard(src); len(bad) > 0 {
			t.Fatalf("%s: %v", e.Name(), bad)
		}
	}
	plant := []byte(`package main;func second(f Forge){delta,_:=f.CompareRefs(repo,base,head);_ = delta.Files;for range delta.Commits {};_,_ = firstParentChain(delta.Commits,base,head)}`)
	bad := compareGuard(plant)
	if len(bad) != 3 {
		t.Fatalf("planted second consumer not detected: %v", bad)
	}
	t.Log(fmt.Sprintf("planted second consumer CAUGHT: %v", bad))
}

func TestMergeFixRoutesReview(t *testing.T) {
	for _, ownChanged := range []bool{true, false} {
		t.Run(fmt.Sprint(ownChanged), func(t *testing.T) {
			installBoardRoster(t, boardRosterGitLabReviewer)
			t.Setenv("DESKBOARD_GH_PRFILES_JSON", `[{"filename":"own.go"}]`)
			t.Setenv("DESKBOARD_GH_PRSTATE_JSON", `{"state":"open","changed_files":1}`)
			stubForgeHooks(t, forgeHookSet{
				reviews: func(string, int) ([]deskkit.Review, error) {
					r := approvalWithNoSHA()
					r[0].CommitID = "review"
					return r, nil
				},
				compare: func(_, _, _ string) (*deskkit.RefComparison, error) {
					return &deskkit.RefComparison{CommitsComplete: true, Commits: []deskkit.RepoCommit{{SHA: "merge", Parents: []string{"review", "main"}}, {SHA: "head", Parents: []string{"merge"}}}}, nil
				},
				getCommit: func(_, sha string) (*deskkit.RepoCommit, error) {
					file := "elsewhere.go"
					if ownChanged {
						file = "own.go"
					}
					return &deskkit.RepoCommit{SHA: sha, Files: []deskkit.ChangedFile{{Filename: file}}, FilesComplete: true}, nil
				},
			})
			out, err := classifyPR(unpinnedHeadRepo, greenRollupPR(5, "head"), false, nil, nil, nil, time.Now())
			if err != nil {
				t.Fatal(err)
			}
			want := actMergeCurr
			if ownChanged {
				want = actReReview
			}
			if out.row == nil || out.row.Action != want {
				t.Fatalf("row=%+v want %s", out.row, want)
			}
		})
	}
}

func TestMergeOwnEditStillReviews(t *testing.T) {
	stubForgeHooks(t, forgeHookSet{
		compare: func(_, _, _ string) (*deskkit.RefComparison, error) {
			return &deskkit.RefComparison{CommitsComplete: true, Commits: []deskkit.RepoCommit{{SHA: "head", Parents: []string{"review", "main"}}}, Files: []deskkit.ChangedFile{{Filename: "own.go"}}}, nil
		},
		getCommit: func(_, sha string) (*deskkit.RepoCommit, error) {
			return &deskkit.RepoCommit{SHA: sha, FilesComplete: true, Files: []deskkit.ChangedFile{{Filename: "own.go"}}}, nil
		},
	})
	changed, err := changedFilesBetween("example-org/tracker", "review", "head")
	if err != nil {
		t.Fatal(err)
	}
	if !ownFilesChanged(map[string]bool{"own.go": true}, true, changed) {
		t.Fatal("own-file conflict resolution in a merge must retain RE-REVIEW")
	}
}
