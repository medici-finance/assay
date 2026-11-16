package main

// receipt_test.go — the receipt line names the object it acted on the way the forge serving
// it writes that object (#6595).
//
// A receipt is the tab-separated stdout line each lane prints (closed / dry-run / noop /
// proposed / disputed / refired / the manifest's per-row lines). A desk transcribes it as
// evidence, so it must name exactly one object. GitHub numbers issues and pull requests in
// ONE sequence and writes both as `#N`; GitLab numbers them SEPARATELY and writes a merge
// request as `!N`. A receipt that spells a GitLab merge request `#N` names the issue that
// shares its number instead.
//
// Two layers. The behavioural rows run real lanes against the stub on each forge. The source
// guard (TestNoRawReceiptSigil) covers the receipts no row executes: every receipt spells its
// item through itemRef, the one forge-aware choke point, and the guard refuses a receipt
// format that writes the sigil itself. Its own positive control plants a raw receipt so a
// guard that stopped matching fails instead of reporting clean.

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// gitlabCollisionDraft is the reported shape: on a GitLab project the worker App's own open
// draft is merge request !draftNum, and issue #draftNum is a DIFFERENT object, already closed.
func gitlabCollisionDraft(t *testing.T) (*stubRemote, string) {
	t.Helper()
	s, rul := baseWorld(t)
	s.forge = deskkit.ForgeGitLab
	s.host = glHost
	s.items[fmt.Sprintf("%s#%d", testRepo, draftNum)] = issueJSON(draftNum, "closed", nil, "an older issue")
	s.items[changeKey(testRepo, draftNum)] = prIssueJSON(draftNum, "open")
	s.pulls[changeKey(testRepo, draftNum)] = draftPullJSON(draftNum, "open", true, workerLogin, workerBotID)
	s.viewer = workerLogin
	return s, rul
}

// TestSelfWithdrawReceiptSigil pins the self-withdraw receipt on both forges and in all three
// of its shapes (closed, dry-run, noop).
// regression: #6595
func TestSelfWithdrawReceiptSigil(t *testing.T) {
	type world func(*testing.T) (*stubRemote, string)
	github := func(t *testing.T) (*stubRemote, string) { return selfWithdrawWorld(t) }
	for _, tc := range []struct {
		name  string
		world world
		item  string
		extra []string
		close func(*stubRemote)
		want  string
	}{
		{
			name: "github closed keeps #N", world: github, item: fmt.Sprint(draftNum),
			want: fmt.Sprintf("closed\t%s#%d\tself-withdraw\tbecause=superseded\tby=%s#40\n", testRepo, draftNum, testRepo),
		},
		{
			name: "github typed !N still prints #N", world: github, item: fmt.Sprintf("!%d", draftNum),
			want: fmt.Sprintf("closed\t%s#%d\tself-withdraw", testRepo, draftNum),
		},
		{
			name: "github dry-run keeps #N", world: github, item: fmt.Sprint(draftNum), extra: []string{"--dry-run"},
			want: fmt.Sprintf("dry-run\t%s#%d\tself-withdraw", testRepo, draftNum),
		},
		{
			name: "gitlab closed prints !N", world: gitlabCollisionDraft, item: fmt.Sprintf("!%d", draftNum),
			want: fmt.Sprintf("closed\t%s!%d\tself-withdraw\tbecause=superseded\tby=%s#40\n", testRepo, draftNum, testRepo),
		},
		{
			name: "gitlab dry-run prints !N", world: gitlabCollisionDraft, item: fmt.Sprintf("!%d", draftNum),
			extra: []string{"--dry-run"},
			want:  fmt.Sprintf("dry-run\t%s!%d\tself-withdraw", testRepo, draftNum),
		},
		{
			name: "gitlab noop prints !N", world: gitlabCollisionDraft, item: fmt.Sprintf("!%d", draftNum),
			close: func(s *stubRemote) {
				s.pulls[changeKey(testRepo, draftNum)] = draftPullJSON(draftNum, "closed", true, workerLogin, workerBotID)
			},
			want: fmt.Sprintf("noop\t%s!%d\talready closed\n", testRepo, draftNum),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, rul := tc.world(t)
			if tc.close != nil {
				tc.close(s)
			}
			args := append([]string{modeSelfWithdraw, "-R", testRepo, tc.item,
				"--because", becauseSuperseded, "--by", "#40", "--rulings", rul}, tc.extra...)
			code, out := execCLI(args...)
			if code != deskkit.ExitOK {
				t.Fatalf("want exit 0, got %d\n%s", code, out)
			}
			if !strings.Contains(out, tc.want) {
				t.Fatalf("receipt should carry %q, got:\n%s", tc.want, out)
			}
		})
	}
}

// TestGitLabReceiptMatchesBy is the issue's own example: the receipt's target and
// its by= field both carry the merge-request sigil, and the issue sharing the number is
// untouched.
// regression: #6595
func TestGitLabReceiptMatchesBy(t *testing.T) {
	s, rul := gitlabCollisionDraft(t)
	code, out := execCLI(modeSelfWithdraw, "-R", testRepo, fmt.Sprintf("!%d", draftNum),
		"--because", becauseSuperseded, "--by", fmt.Sprintf("!%d", draftNum+1), "--rulings", rul)
	if code != deskkit.ExitOK {
		t.Fatalf("want exit 0, got %d\n%s", code, out)
	}
	want := fmt.Sprintf("closed\t%s!%d\tself-withdraw\tbecause=superseded\tby=%s!%d\n",
		testRepo, draftNum, testRepo, draftNum+1)
	if !strings.Contains(out, want) {
		t.Fatalf("receipt should read %q, got:\n%s", want, out)
	}
	if issue := s.items[fmt.Sprintf("%s#%d", testRepo, draftNum)]; !strings.Contains(issue, `"state":"closed"`) ||
		strings.Contains(issue, "stub pr") {
		t.Fatalf("the issue sharing the number changed: %s", issue)
	}
}

// TestItemRefPerForge pins the choke point every receipt spells its item through, against the
// forge the stub serves. The ruled lanes cannot run end to end on GitLab yet (their sign-off
// is a GitHub comment permalink), so their receipts are held here and by TestNoRawReceiptSigil.
// regression: #6595
func TestItemRefPerForge(t *testing.T) {
	for _, tc := range []struct {
		forge deskkit.ForgeKind
		kind  deskkit.TargetKind
		want  string
	}{
		{deskkit.ForgeGitLab, deskkit.TargetChange, testRepo + "!37"},
		{deskkit.ForgeGitLab, deskkit.TargetIssue, testRepo + "#37"},
		{deskkit.ForgeGitLab, "", testRepo + "#37"},
		{deskkit.ForgeGitHub, deskkit.TargetChange, testRepo + "#37"},
		{deskkit.ForgeGitHub, deskkit.TargetIssue, testRepo + "#37"},
	} {
		s, _ := baseWorld(t)
		s.forge = tc.forge
		if got := itemRef(testRepo, 37, tc.kind); got != tc.want {
			t.Errorf("%s %q: got %q, want %q", tc.forge, tc.kind, got, tc.want)
		}
	}
	previous := forgeKindFn
	t.Cleanup(func() { forgeKindFn = previous })
	forgeKindFn = func(deskkit.ForgeRepo) (deskkit.ForgeKind, error) {
		return "", deskkit.Unverifiable("no forge", nil)
	}
	if got := itemRef(testRepo, 37, deskkit.TargetChange); got != testRepo+"!37" {
		t.Errorf("an unnamed forge must keep the kind-stating !N, got %q", got)
	}
}

// TestIssueReceiptHashOnGitLab is the over-application check: an ISSUE on GitLab is
// still `#N`. A formatter that printed `!` for everything on GitLab would pass the rows above.
// regression: #6595
func TestIssueReceiptHashOnGitLab(t *testing.T) {
	s, rul := gitlabWorld(t)
	plantNote(s, testRepo, triageIssue, "8301", blessLogin, blessID)
	code, out := humanDecidedRun(rul, noteURL(glHost, testRepo, triageIssue, "8301"))
	if code != deskkit.ExitOK {
		t.Fatalf("want exit 0, got %d\n%s", code, out)
	}
	if want := fmt.Sprintf("closed\t%s#%d\ttriage", testRepo, triageIssue); !strings.Contains(out, want) {
		t.Fatalf("receipt should carry %q, got:\n%s", want, out)
	}
}

// rawReceiptSites reports every receipt in one file that writes an item sigil itself: a
// fmt.Fprintf to the lane's `out` writer whose format literal carries `#%d` or `!%d`. A
// receipt names its item through itemRef and prints the result with %s.
func rawReceiptSites(fset *token.FileSet, f *ast.File) []string {
	var hits []string
	ast.Inspect(f, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || len(call.Args) < 2 {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "Fprintf" {
			return true
		}
		if pkg, ok := sel.X.(*ast.Ident); !ok || pkg.Name != "fmt" {
			return true
		}
		if w, ok := call.Args[0].(*ast.Ident); !ok || w.Name != "out" {
			return true
		}
		lit, ok := call.Args[1].(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return true
		}
		format, err := strconv.Unquote(lit.Value)
		if err != nil {
			return true
		}
		if strings.Contains(format, "#%d") || strings.Contains(format, "!%d") {
			hits = append(hits, fset.Position(lit.Pos()).String())
		}
		return true
	})
	return hits
}

// TestNoRawReceiptSigil is the class guard: no non-test file in this package prints a receipt
// whose sigil it wrote itself. The positive control parses a planted raw receipt first, so a
// matcher that silently stopped matching fails here rather than passing the package clean.
// regression: #6595
func TestNoRawReceiptSigil(t *testing.T) {
	const planted = `package main
import ("fmt"; "io")
func plantedReceipt(out io.Writer, repo string, n int) {
	fmt.Fprintf(out, "closed\t%s#%d\tplanted\n", repo, n)
}`
	fset := token.NewFileSet()
	pf, err := parser.ParseFile(fset, "planted.go", planted, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got := rawReceiptSites(fset, pf); len(got) != 1 {
		t.Fatalf("positive control: the guard must flag the planted raw receipt, flagged %v", got)
	}

	files, err := filepath.Glob("*.go")
	if err != nil || len(files) == 0 {
		t.Fatalf("could-not-check: no package sources found (%v)", err)
	}
	var hits []string
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		src, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("could-not-check: %v", err)
		}
		f, err := parser.ParseFile(fset, name, src, 0)
		if err != nil {
			t.Fatalf("could-not-check: %s: %v", name, err)
		}
		hits = append(hits, rawReceiptSites(fset, f)...)
	}
	if len(hits) != 0 {
		t.Fatalf("receipts that write their own item sigil (spell the item with itemRef instead):\n  %s",
			strings.Join(hits, "\n  "))
	}
}
