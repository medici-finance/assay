package deskkit

// heldchange_test.go — #2254: a change the desk opens must carry its merge-hold marker thread.
//
// Two halves, per the defect-class rule. The behaviour tests pin CreateHeldDraftChange itself
// (the hold is opened on the just-created number; not-applicable is success; a hold failure is
// loud and still names the change). TestDraftChangeOnlyViaHeld is the CLASS guard: an
// allow-list structural walk of every non-test .go file under tools/desk that fails naming any
// CreateDraftChange call outside the choke point, with a committed positive-control fixture so
// a matcher that went blind fails instead of reporting clean.

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// heldFake records the create and hold calls CreateHeldDraftChange makes. The embedded nil
// Forge makes any other method a run-time panic, so a stray call cannot pass unnoticed.
type heldFake struct {
	Forge
	createErr error
	createRef *PullRef
	holdErr   error
	calls     []string
}

func (f *heldFake) CreateDraftChange(_ ForgeRepo, in DraftChangeInput) (*PullRef, error) {
	f.calls = append(f.calls, "create "+in.Head)
	if f.createErr != nil {
		return nil, f.createErr
	}
	if f.createRef != nil {
		return f.createRef, nil
	}
	return &PullRef{Number: 27, URL: "https://forge.example/change/27"}, nil
}

func (f *heldFake) OpenMergeHold(_ ForgeRepo, number int) (string, error) {
	f.calls = append(f.calls, fmt.Sprintf("hold %d", number))
	if f.holdErr != nil {
		return "", f.holdErr
	}
	return "disc-1", nil
}

var heldRepo = ForgeRepo{Owner: "example-org", Name: "tracker"}

func TestHeldChangeOpensHold(t *testing.T) {
	f := &heldFake{}
	ref, err := CreateHeldDraftChange(f, heldRepo, DraftChangeInput{Head: "evidence/x", Base: "main"})
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if ref == nil || ref.Number != 27 {
		t.Fatalf("ref = %+v, want change #27", ref)
	}
	if got := strings.Join(f.calls, ","); got != "create evidence/x,hold 27" {
		t.Fatalf("calls = %q, want the create THEN the hold on the just-created #27", got)
	}
}

func TestHeldChangeNotApplicableOK(t *testing.T) {
	f := &heldFake{holdErr: ErrMergeHoldNotApplicable}
	ref, err := CreateHeldDraftChange(f, heldRepo, DraftChangeInput{Head: "b"})
	if err != nil || ref == nil || ref.Number != 27 {
		t.Fatalf("ref, err = %+v, %v — the typed not-applicable (GitHub) must read as success", ref, err)
	}
}

func TestHeldChangeHoldFailIsLoud(t *testing.T) {
	f := &heldFake{holdErr: errors.New("503 the instance is unavailable")}
	ref, err := CreateHeldDraftChange(f, heldRepo, DraftChangeInput{Head: "b"})
	if err == nil {
		t.Fatal("err = nil — a hold-open failure must never read as a clean create")
	}
	if ref == nil || ref.Number != 27 {
		t.Fatalf("ref = %+v — the change already exists and must still be returned so the caller can name it", ref)
	}
	if !strings.Contains(err.Error(), "https://forge.example/change/27") || !strings.Contains(err.Error(), "merge-hold") {
		t.Fatalf("error %q does not name the change and the missing merge-hold", err)
	}
}

func TestHeldChangeCreateFailNoHold(t *testing.T) {
	f := &heldFake{createErr: errors.New("422 a change already exists")}
	ref, err := CreateHeldDraftChange(f, heldRepo, DraftChangeInput{Head: "b"})
	if err == nil || ref != nil {
		t.Fatalf("ref, err = %+v, %v — want (nil, the create error)", ref, err)
	}
	if got := strings.Join(f.calls, ","); got != "create b" {
		t.Fatalf("calls = %q — no hold may be attempted when the create failed", got)
	}
}

func TestHeldChangeNoNumberIsLoud(t *testing.T) {
	f := &heldFake{createRef: &PullRef{URL: "https://forge.example/change/?"}}
	_, err := CreateHeldDraftChange(f, heldRepo, DraftChangeInput{Head: "b"})
	if err == nil {
		t.Fatal("err = nil — a change with no number cannot carry a hold and must not read as clean")
	}
}

// heldChangeAllowList is every (file, enclosing func) under tools/desk that may call
// CreateDraftChange directly. Anything else opening a change must go through
// CreateHeldDraftChange. A stale entry (no longer calling it) is ALSO a failure.
var heldChangeAllowList = map[string]string{
	"internal/deskkit/heldchange.go:CreateHeldDraftChange": "the choke point: the create paired with OpenMergeHold",
	"internal/deskkit/outboundforge.go:CreateDraftChange":  "the checking decorator delegating to its backend",
}

// draftChangeCallSites walks walkRoot's non-test .go files and returns every selector call
// `<x>.CreateDraftChange(...)` as "<path relative to relRoot>:<enclosing func>".
func draftChangeCallSites(walkRoot, relRoot string) ([]string, error) {
	var sites []string
	err := filepath.WalkDir(walkRoot, func(p string, d os.DirEntry, werr error) error {
		if werr != nil {
			return werr
		}
		if d.IsDir() {
			if p != walkRoot && (d.Name() == "testdata" || d.Name() == "vendor" || d.Name() == ".git") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
			return nil
		}
		fset := token.NewFileSet()
		f, perr := parser.ParseFile(fset, p, nil, 0)
		if perr != nil {
			return fmt.Errorf("parsing %s: %w", p, perr)
		}
		rel, rerr := filepath.Rel(relRoot, p)
		if rerr != nil {
			return rerr
		}
		rel = filepath.ToSlash(rel)
		for _, decl := range f.Decls {
			fd, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}
			// Match every selector naming CreateDraftChange, not only call expressions: a method
			// value (f := fg.CreateDraftChange; f(...)) or a method expression reaches the raw seam
			// just as a direct call does.
			ast.Inspect(fd, func(n ast.Node) bool {
				if sel, ok := n.(*ast.SelectorExpr); ok && sel.Sel.Name == "CreateDraftChange" {
					sites = append(sites, rel+":"+fd.Name.Name)
				}
				return true
			})
		}
		return nil
	})
	return sites, err
}

func TestDraftChangeOnlyViaHeld(t *testing.T) {
	moduleRoot, err := filepath.Abs(deskTreeRoot)
	if err != nil {
		t.Fatal(err)
	}
	sites, err := draftChangeCallSites(moduleRoot, moduleRoot)
	if err != nil {
		t.Fatalf("could not walk %s: %v — this is could-not-check, not a clean tree", moduleRoot, err)
	}
	seen := map[string]bool{}
	var offenders []string
	for _, s := range sites {
		seen[s] = true
		if _, ok := heldChangeAllowList[s]; !ok {
			offenders = append(offenders, s)
		}
	}
	if len(offenders) > 0 {
		sort.Strings(offenders)
		t.Fatalf("CreateDraftChange called outside the merge-hold choke point — use "+
			"deskkit.CreateHeldDraftChange so the change carries its merge-hold marker thread:\n%s",
			strings.Join(offenders, "\n"))
	}
	for s := range heldChangeAllowList {
		if !seen[s] {
			t.Fatalf("allow-list entry %s no longer calls CreateDraftChange — remove the stale entry", s)
		}
	}

	// Positive control: the committed fixture plants ONE raw call outside the allow-list.
	fixtureRoot, err := filepath.Abs("testdata/heldchange_fixture")
	if err != nil {
		t.Fatal(err)
	}
	planted, err := draftChangeCallSites(fixtureRoot, fixtureRoot)
	if err != nil {
		t.Fatalf("fixture scan: %v", err)
	}
	sort.Strings(planted)
	want := []string{"planted_change.go:openUnheldChange", "planted_change.go:openViaMethodValue"}
	if strings.Join(planted, ",") != strings.Join(want, ",") {
		t.Fatalf("positive control: fixture sites = %v, want exactly %v — the matcher is not live "+
			"(both the direct call and the method-value reach must be flagged)", planted, want)
	}
}
