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

func TestOwnCommitHistory(t *testing.T) {
	for _, tc := range []struct {
		name  string
		files []deskkit.ChangedFile
		want  bool
	}{
		{"merge plus own fix", []deskkit.ChangedFile{{Filename: "own.go"}}, true},
		{"fix later reverted", []deskkit.ChangedFile{{Filename: "own.go"}}, true},
		{"keep current unrelated", []deskkit.ChangedFile{{Filename: "main.go"}}, false},
		{"rename away from own", []deskkit.ChangedFile{{Filename: "new.go", PreviousFilename: "own.go"}}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stubForgeHooks(t, forgeHookSet{
				compare: func(_, _, _ string) (*deskkit.RefComparison, error) {
					return &deskkit.RefComparison{
						Files: []deskkit.ChangedFile{{Filename: "main.go"}}, CommitsComplete: true,
						Commits: []deskkit.RepoCommit{{SHA: "merge", Parents: []string{"review", "main"}}, {SHA: "fix", Parents: []string{"merge"}}},
					}, nil
				},
				getCommit: func(_, sha string) (*deskkit.RepoCommit, error) {
					if sha == "merge" {
						return &deskkit.RepoCommit{SHA: sha, FilesComplete: true, Files: []deskkit.ChangedFile{{Filename: "main.go"}}}, nil
					}
					return &deskkit.RepoCommit{SHA: sha, Parents: []string{"merge"}, Files: tc.files, FilesComplete: true}, nil
				},
			})
			changed, err := changedFilesBetween("example-org/tracker", "review", "head")
			if err != nil {
				t.Fatal(err)
			}
			got := ownFilesChanged(map[string]bool{"own.go": true}, true, changed)
			if got != tc.want {
				t.Fatalf("own-file edit detected=%v, want %v; aggregate diff=%v", got, tc.want, changed)
			}
			action, _ := classify(classifyInput{ever: true, ownFilesChanged: got})
			want := actMergeCurr
			if tc.want {
				want = actReReview
			}
			if action != want {
				t.Fatalf("action=%s want %s", action, want)
			}
		})
	}
}

func TestHistoryReadDegrades(t *testing.T) {
	for _, mode := range []string{"short history", "missing parents", "short files", "read failed"} {
		t.Run(mode, func(t *testing.T) {
			stubForgeHooks(t, forgeHookSet{
				compare: func(_, _, _ string) (*deskkit.RefComparison, error) {
					c := deskkit.RepoCommit{SHA: "fix", Parents: []string{"base"}}
					if mode == "missing parents" {
						c.Parents = nil
					}
					return &deskkit.RefComparison{Commits: []deskkit.RepoCommit{c}, CommitsComplete: mode != "short history"}, nil
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
// commit-history choke point; aggregate files cannot establish review currency.
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
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			if sel, ok := n.(*ast.SelectorExpr); ok {
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
	plant := []byte(`package main;func second(f Forge){delta,_:=f.CompareRefs(repo,base,head);_ = delta.Files}`)
	bad := compareGuard(plant)
	if len(bad) != 2 {
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
					return &deskkit.RefComparison{CommitsComplete: true, Commits: []deskkit.RepoCommit{{SHA: "merge", Parents: []string{"review", "main"}}, {SHA: "fix", Parents: []string{"merge"}}}}, nil
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
			return &deskkit.RefComparison{CommitsComplete: true, Commits: []deskkit.RepoCommit{{SHA: "merge", Parents: []string{"review", "main"}}}, Files: []deskkit.ChangedFile{{Filename: "own.go"}}}, nil
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
