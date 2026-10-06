package desk_test

// This is the SHORT-ENGINE-DEADLINE GUARD. It is a tripwire for a defect class in this
// module's tests: a wall-clock deadline on a wait for loopengine.Run (to dispatch, to return,
// to exit after a STOP flag) written as a literal sized to unloaded speed. Every Run
// iteration calls deskkit.Guard, which spawns a git subprocess, so an iteration's wall cost
// grows with machine load. Under the CPU saturation of a whole-module `go test ./...` a
// 2-5s deadline is missed although nothing is wedged, and the suite turns into a coin flip.
// cmd/fanoutloop hit it first (#738); internal/loopengine and cmd/commsloop repeated it.
//
// The fix is one named, load-tolerant ceiling per package (engineTestTimeout, 60s). The
// deadline is a wedge safety net, never a measurement, so a long ceiling weakens nothing: a
// real wedge still fails, just later.
//
// SCOPE: every _test.go file under internal/loopengine/, plus every _test.go file elsewhere in
// the module that calls loopengine.Run. In scope, a time.After or time.NewTimer whose argument
// is a constant duration literal (`5 * time.Second`, `time.Second`, `500 * time.Millisecond`)
// below shortEngineDeadlineFloor fails. A named constant or variable passes; the guard is
// AST-based but does not evaluate identifiers, so a clean scan is evidence, not proof. The
// allow-list is empty and may only stay so. A positive control proves the matcher still flags
// the shape, so a broken matcher fails here rather than reporting clean.

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"
)

// shortEngineDeadlineFloor is the smallest literal deadline a test driving the engine may
// carry. Half the packages' 60s ceiling: anything shorter has already been seen to redden
// under whole-suite load.
const shortEngineDeadlineFloor = 30 * time.Second

var durationUnits = map[string]time.Duration{
	"Nanosecond":  time.Nanosecond,
	"Microsecond": time.Microsecond,
	"Millisecond": time.Millisecond,
	"Second":      time.Second,
	"Minute":      time.Minute,
	"Hour":        time.Hour,
}

// literalDuration evaluates `time.<Unit>` and `<int> * time.<Unit>` (either operand order).
// ok is false for anything else — an identifier, a call, arithmetic on variables.
func literalDuration(e ast.Expr) (time.Duration, bool) {
	switch x := e.(type) {
	case *ast.ParenExpr:
		return literalDuration(x.X)
	case *ast.SelectorExpr:
		if id, isID := x.X.(*ast.Ident); isID && id.Name == "time" {
			if u, known := durationUnits[x.Sel.Name]; known {
				return u, true
			}
		}
	case *ast.BinaryExpr:
		if x.Op != token.MUL {
			return 0, false
		}
		for _, pair := range [][2]ast.Expr{{x.X, x.Y}, {x.Y, x.X}} {
			lit, isLit := pair[0].(*ast.BasicLit)
			if !isLit || lit.Kind != token.INT {
				continue
			}
			n, err := strconv.ParseInt(lit.Value, 0, 64)
			if err != nil {
				continue
			}
			if u, ok := literalDuration(pair[1]); ok {
				return time.Duration(n) * u, true
			}
		}
	}
	return 0, false
}

// shortEngineDeadlines returns every time.After / time.NewTimer call in f whose argument is
// a literal duration below the floor, as "line N: time.After(<d>)".
func shortEngineDeadlines(fset *token.FileSet, f *ast.File) []string {
	var out []string
	ast.Inspect(f, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || len(call.Args) != 1 {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		if pkg, isID := sel.X.(*ast.Ident); !isID || pkg.Name != "time" || (sel.Sel.Name != "After" && sel.Sel.Name != "NewTimer") {
			return true
		}
		if d, lit := literalDuration(call.Args[0]); lit && d < shortEngineDeadlineFloor {
			out = append(out, fmt.Sprintf("line %d: time.%s(%s)", fset.Position(call.Pos()).Line, sel.Sel.Name, d))
		}
		return true
	})
	return out
}

// drivesEngine reports whether a test file is in the guard's scope.
func drivesEngine(path string, src []byte) bool {
	return strings.HasPrefix(path, "internal/loopengine/") || strings.Contains(string(src), "loopengine.Run(")
}

func TestNoShortEngineDeadline(t *testing.T) {
	// Positive control: a planted engine-driving file carrying each spelling must be flagged
	// three times, and the named ceiling must not be. Built by concatenation so this file
	// does not carry the shape itself.
	plant := "package p\nfunc f() {\n" +
		"\tgo loopengine.Run(cfg, loop)\n" +
		"\t<-time." + "After(5 * time.Second)\n" +
		"\t<-time." + "After(time.Second)\n" +
		"\t_ = time." + "NewTimer(500 * time.Millisecond)\n" +
		"\t<-time." + "After(engineTestTimeout)\n" +
		"\t<-time." + "After(60 * time.Second)\n" +
		"}\n"
	pfset := token.NewFileSet()
	pf, err := parser.ParseFile(pfset, "plant_test.go", plant, 0)
	if err != nil {
		t.Fatalf("short-engine-deadline guard: positive control does not parse: %v", err)
	}
	if !drivesEngine("cmd/x/plant_test.go", []byte(plant)) {
		t.Fatal("short-engine-deadline guard: the scope test no longer recognises a file that calls loopengine.Run")
	}
	if got := shortEngineDeadlines(pfset, pf); len(got) != 3 {
		t.Fatalf("short-engine-deadline guard: positive control flagged %d sites %v, want exactly 3 — the scan below would misreport", len(got), got)
	}

	var problems []string
	err = filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == "testdata" || d.Name() == "vendor" || (path != "." && strings.HasPrefix(d.Name(), ".")) {
				return filepath.SkipDir
			}
			return nil
		}
		path = filepath.ToSlash(path)
		if !strings.HasSuffix(path, "_test.go") {
			return nil
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if !drivesEngine(path, src) {
			return nil
		}
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, path, src, 0)
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		for _, site := range shortEngineDeadlines(fset, f) {
			problems = append(problems, path+" "+site+" — a literal deadline under "+shortEngineDeadlineFloor.String()+
				" on a test that drives the engine reddens under whole-suite load; use the package's engineTestTimeout (60s) instead")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("short-engine-deadline guard: could not walk the module: %v", err)
	}
	sort.Strings(problems)
	for _, p := range problems {
		t.Error("short-engine-deadline guard: " + p)
	}
}
