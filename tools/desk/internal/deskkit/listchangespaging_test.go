package deskkit

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// listchangespaging_test.go — #6475: the open+merged changes read behind the represented-PR
// reconciliation (fanoutloop plan, deskdispatch's phantom check) asked for 100 changes WITH
// their bodies per GraphQL page, in a hand-rolled loop. On a repo with a large PR history the
// forge reset the stream partway through such a page ("response body cut off mid-transfer"),
// and one cut-off page failed the whole read every run. ListChanges now walks through the
// shared pager the open-change reads already use: small pages, and a cut-off page is retried
// for the same cursor at half the size. A read that still cannot complete is could-not-check
// and returns NO changes — never a shorter list read as "no PR exists".
//
// The figures are literals, not the package constants, on purpose: the window (500 changes,
// most recently updated first) is the contract consumers reason about, and a test that read
// the constants back would pass whatever they were set to.

// heavyCutoff models a page too heavy for the forge to stream: any page asking for more than
// limit changes is cut off mid-transfer, smaller pages answer.
func heavyCutoff(limit int) func(int, int) string {
	return func(_ int, first int) string {
		if first > limit {
			return "cutoff"
		}
		return ""
	}
}

// regression: #6475
func TestListChangesPagesSmall(t *testing.T) {
	for _, n := range []int{0, 3, 25, 180, 500, 501, 640} {
		fake := &pagedOpenChanges{n: n, body: 4000}
		cl, err := pagedForge(t, fake).ListChanges(pagedRepo, OpenAndMerged())
		if err != nil {
			t.Fatalf("n=%d: %v", n, err)
		}
		want := min(n, 500)
		if len(cl.Changes) != want {
			t.Fatalf("n=%d: %d changes, want every change up to the 500-change window (%d)", n, len(cl.Changes), want)
		}
		if cl.Incomplete != (n > 500) {
			t.Errorf("n=%d: Incomplete=%v — a history past the window must say so, one inside it must not", n, cl.Incomplete)
		}
		for i, c := range cl.Changes {
			if c.Number != n-i || len(c.Body) != 4000 {
				t.Fatalf("n=%d: change %d is #%d with a %d-byte body, want #%d with its whole body", n, i, c.Number, len(c.Body), n-i)
			}
		}
		for _, r := range fake.requests {
			if first, _ := strconv.Atoi(strings.SplitN(r, "/", 2)[0]); first > 25 {
				t.Errorf("n=%d: a page asked for %d changes with their bodies, above the 25 page size", n, first)
			}
		}
		if wantReq := max(1, (want+24)/25); len(fake.requests) != wantReq {
			t.Errorf("n=%d: %d requests, want %d", n, len(fake.requests), wantReq)
		}
	}
}

// regression: #6475
func TestListChangesCutoffShrinks(t *testing.T) {
	fake := &pagedOpenChanges{n: 60, body: 4000, fault: heavyCutoff(12)}
	cl, err := pagedForge(t, fake).ListChanges(pagedRepo, OpenAndMerged())
	if err != nil {
		t.Fatalf("a page cut off mid-transfer must be retried at a smaller page, not fail the read: %v", err)
	}
	if len(cl.Changes) != 60 || cl.Incomplete {
		t.Fatalf("got %d changes incomplete=%v, want all 60", len(cl.Changes), cl.Incomplete)
	}
	want := []string{"25/0", "12/0", "12/12", "12/24", "12/36", "12/48"}
	if !reflect.DeepEqual(fake.requests, want) {
		t.Errorf("requests %v, want %v — the SAME cursor retried at half size, the smaller size kept", fake.requests, want)
	}
}

// regression: #6475
func TestListChangesCutoffHolds(t *testing.T) {
	fake := &pagedOpenChanges{n: 60, body: 4000, fault: heavyCutoff(0)}
	f := pagedForge(t, fake)
	cl, err := f.ListChanges(pagedRepo, OpenAndMerged())
	if ExitCodeOf(err) != ExitUnverifiable || cl != nil {
		t.Fatalf("a read cut off at every size must be could-not-check with no list: cl=%v exit=%d err=%v", cl, ExitCodeOf(err), err)
	}
	if !strings.Contains(err.Error(), "cut off mid-transfer") {
		t.Errorf("the forge's own failure must survive in the refusal: %v", err)
	}
	if want := []string{"25/0", "12/0", "6/0"}; !reflect.DeepEqual(fake.requests, want) {
		t.Errorf("requests under a sustained cut-off %v, want exactly %v", fake.requests, want)
	}
	// The reconciliation's reduction must carry the error, never an empty answer.
	fake.requests = nil
	refs, incomplete, rerr := RepresentedPRRefs(f, pagedRepo)
	if rerr == nil || refs != nil || incomplete {
		t.Fatalf("an unreadable list must stay could-not-check, never \"no PR exists\": refs=%v incomplete=%v err=%v", refs, incomplete, rerr)
	}
	// A refusal on a later page is final too, and the pages already read are not handed back.
	fake = &pagedOpenChanges{n: 60, fault: func(i, _ int) string {
		if i == 2 {
			return "403"
		}
		return ""
	}}
	cl, err = pagedForge(t, fake).ListChanges(pagedRepo, OpenAndMerged())
	if err == nil || cl != nil || len(fake.requests) != 2 {
		t.Fatalf("a mid-walk refusal must fail the whole read: cl=%v err=%v requests=%v", cl, err, fake.requests)
	}
}

// regression: #6475
func TestPRListReadsShrinkOnCutoff(t *testing.T) {
	readers := map[string]func(*GitHubForge) (int, error){
		"ghOpenChangesQuery": func(f *GitHubForge) (int, error) {
			o, err := f.ListOpenChanges(pagedRepo)
			if err != nil {
				return 0, err
			}
			return len(o.Changes), nil
		},
		"ghReviewQueueQuery": func(f *GitHubForge) (int, error) {
			q, err := f.ReviewQueueSnapshot(pagedRepo)
			if err != nil {
				return 0, err
			}
			return len(q.Changes), nil
		},
		"ghListChangesQuery": func(f *GitHubForge) (int, error) {
			c, err := f.ListChanges(pagedRepo, OpenAndMerged())
			if err != nil {
				return 0, err
			}
			return len(c.Changes), nil
		},
	}
	// Every package-level GraphQL document that pages a pullRequests connection by $first must
	// be driven here, so a new bulk PR-list read cannot ship with a fixed page and no backoff.
	found := prListQueryConsts(t)
	var names []string
	for name := range readers {
		names = append(names, name)
	}
	sort.Strings(names)
	if !reflect.DeepEqual(found, names) {
		t.Fatalf("paged pullRequests documents %v, readers registered for %v — "+
			"walk the new read through the shared pager and register it here", found, names)
	}
	for _, name := range names {
		n, err := readers[name](pagedForge(t, &pagedOpenChanges{n: 60, body: 4000, fault: heavyCutoff(12)}))
		if err != nil || n != 60 {
			t.Errorf("%s: a page cut off mid-transfer must shrink and complete: %d changes, err=%v", name, n, err)
		}
	}
}

// prListQueryConsts names every non-test constant in this package whose string pages a
// pullRequests connection by a $first variable.
func prListQueryConsts(t *testing.T) []string {
	t.Helper()
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	var out []string
	for _, p := range files {
		if strings.HasSuffix(p, "_test.go") {
			continue
		}
		src, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		f, err := parser.ParseFile(fset, p, src, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, d := range f.Decls {
			g, ok := d.(*ast.GenDecl)
			if !ok || g.Tok != token.CONST {
				continue
			}
			for _, s := range g.Specs {
				vs := s.(*ast.ValueSpec)
				for i, v := range vs.Values {
					lit, ok := v.(*ast.BasicLit)
					if !ok || lit.Kind != token.STRING {
						continue
					}
					if strings.Contains(lit.Value, "pullRequests(") && strings.Contains(lit.Value, "first:$first") {
						out = append(out, vs.Names[i].Name)
					}
				}
			}
		}
	}
	sort.Strings(out)
	if len(out) == 0 {
		t.Fatal("found no paged pullRequests document at all — the scan itself is broken")
	}
	return out
}
