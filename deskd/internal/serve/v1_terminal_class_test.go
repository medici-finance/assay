package serve_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// The defect class (the authorization contract, the service contract r7 finding R7-2): a
// `terminal.state` frame reaches the wire stamped with a cursor that does not
// yet cover the bytes the state was derived from. The frame's id is the
// Last-Event-ID a client resumes from, so such a frame tells the client "you
// hold everything up to here" while the bytes that set the title are still in
// the ring, unsent.
//
// The instance was the handler's poll tick: it emitted a new title before the
// pending wake had drained the bytes that carried it. The guard below allows
// state frames in exactly ONE function and checks source order there:
// follower snapshot, ring drain, frame construction. A second emit site, a
// missing drain, or a snapshot moved after the drain fails it.

// stateFrameOwner is the one function allowed to build a terminal.state frame.
const stateFrameOwner = "(*terminalStream).state"

// stateFrameMarkers are the identifiers that only a state frame uses.
var stateFrameMarkers = map[string]bool{"EventTerminalState": true, "TerminalStateData": true}

// stateFrameViolations reports every place in files that builds a
// terminal.state frame outside [stateFrameOwner], or inside it without a
// preceding drain call after its follower snapshot.
func stateFrameViolations(fset *token.FileSet, files []*ast.File) []string {
	var out []string
	for _, file := range files {
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				// A package-level var or func literal that builds a state frame
				// is outside the owner by definition.
				ast.Inspect(decl, func(n ast.Node) bool {
					if sel, ok := n.(*ast.SelectorExpr); ok && stateFrameMarkers[sel.Sel.Name] {
						at := fset.Position(sel.Pos())
						out = append(out, filepath.Base(at.Filename)+":"+strconv.Itoa(at.Line)+": a package-level declaration builds a terminal.state frame — only "+stateFrameOwner+" may")
					}
					return true
				})
				continue
			}
			name := funcName(fn)
			var firstDrain, lastSnapshot token.Pos
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				if call, ok := n.(*ast.CallExpr); ok {
					if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
						if sel.Sel.Name == "drain" && firstDrain == token.NoPos {
							firstDrain = call.Pos()
						}
						if sel.Sel.Name == "State" {
							lastSnapshot = call.Pos()
						}
					}
				}
				return true
			})
			seen := map[string]bool{}
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				sel, ok := n.(*ast.SelectorExpr)
				if !ok || !stateFrameMarkers[sel.Sel.Name] {
					return true
				}
				at := fset.Position(sel.Pos())
				where := filepath.Base(at.Filename) + ":" + strconv.Itoa(at.Line)
				if seen[where] { // the event type and its payload sit on one line
					return true
				}
				seen[where] = true
				switch {
				case name != stateFrameOwner:
					out = append(out, where+": "+name+" builds a terminal.state frame — only "+stateFrameOwner+" may, so the drain-first rule has one home")
				case firstDrain == token.NoPos || firstDrain > sel.Pos():
					out = append(out, where+": "+name+" builds a terminal.state frame without draining the ring first — the frame's id would not cover the bytes the state came from")
				case lastSnapshot == token.NoPos || lastSnapshot > firstDrain:
					out = append(out, where+": "+name+" builds a terminal.state frame without taking the state snapshot before draining — ingestion between drain and snapshot could leave the frame's id behind its bytes")
				}
				return true
			})
		}
	}
	sort.Strings(out)
	return out
}

func funcName(fn *ast.FuncDecl) string {
	if fn.Recv == nil || len(fn.Recv.List) == 0 {
		return fn.Name.Name
	}
	recv := fn.Recv.List[0].Type
	star := ""
	if s, ok := recv.(*ast.StarExpr); ok {
		star, recv = "*", s.X
	}
	if id, ok := recv.(*ast.Ident); ok {
		return "(" + star + id.Name + ")." + fn.Name.Name
	}
	return fn.Name.Name
}

func parseGoFiles(t *testing.T, fset *token.FileSet, paths []string) []*ast.File {
	t.Helper()
	var files []*ast.File
	for _, p := range paths {
		src, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		f, err := parser.ParseFile(fset, p, src, parser.SkipObjectResolution)
		if err != nil {
			t.Fatalf("parse %s: %v", p, err)
		}
		files = append(files, f)
	}
	return files
}

// TestTerminalStateFrameClassGuard runs the guard over every non-test source in
// this package and requires zero violations; then over a planted second
// instance (testdata) and requires the guard to catch it — a guard that cannot
// go red is not a guard.
func TestTerminalStateFrameClassGuard(t *testing.T) {
	all, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	var sources []string
	for _, p := range all {
		if !strings.HasSuffix(p, "_test.go") {
			sources = append(sources, p)
		}
	}
	if len(sources) == 0 {
		t.Fatal("no package sources found — the guard would pass vacuously (could-not-check)")
	}
	fset := token.NewFileSet()
	files := parseGoFiles(t, fset, sources)
	owners := 0
	for _, f := range files {
		for _, d := range f.Decls {
			if fn, ok := d.(*ast.FuncDecl); ok && funcName(fn) == stateFrameOwner {
				owners++
			}
		}
	}
	if owners != 1 {
		t.Fatalf("found %d definitions of %s, want 1 — the guard's allow-list names a function that is not there (could-not-check)", owners, stateFrameOwner)
	}
	if v := stateFrameViolations(fset, files); len(v) > 0 {
		t.Errorf("terminal.state frame built outside the drain-first owner:\n  %s", strings.Join(v, "\n  "))
	}

	// Positive control: the planted file carries a second emit site in a poll
	// case, a state step with its drain removed, and a drain before the
	// snapshot. Every variant must be caught.
	pfset := token.NewFileSet()
	planted := stateFrameViolations(pfset, parseGoFiles(t, pfset, []string{filepath.Join("testdata", "terminal_state_planted.go.txt")}))
	var secondSite, noDrain, drainBeforeState bool
	for _, v := range planted {
		secondSite = secondSite || strings.Contains(v, "pollCase builds")
		noDrain = noDrain || strings.Contains(v, "without draining")
		drainBeforeState = drainBeforeState || strings.Contains(v, "without taking the state snapshot before draining")
	}
	if !secondSite || !noDrain || !drainBeforeState {
		t.Fatalf("the guard missed a planted instance (second emit site caught=%v, drainless state step caught=%v, drain-before-snapshot caught=%v); it reported:\n  %s",
			secondSite, noDrain, drainBeforeState, strings.Join(planted, "\n  "))
	}
}
