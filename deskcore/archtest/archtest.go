// Package archtest enforces deskcore's architecture as tests rather than review comments.
//
// Two checks live here:
//
//   - Check walks a package graph (the JSON stream "go list -e -json -deps" prints) and reports
//     every pure package that reaches a forbidden package by any transitive path: a process,
//     network or HTTP package from the standard library, an effectful package of the module
//     itself, or any third-party package. Each violation carries the import chain that reaches
//     the forbidden package, so the fix is obvious.
//   - ScanKnobReads parses Go source and reports every read of the process environment, and
//     every hard-coded copy of a knob's earlier literal value, outside the config package. A
//     knob has exactly one reader; a second one would let the resolver's precedence and bounds
//     be bypassed.
//
// The package itself runs no process and reads no environment: the tests run "go list" and hand
// the output and a file system to these functions.
package archtest

import (
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"io/fs"
	"path"
	"sort"
	"strconv"
	"strings"
)

// Package is the subset of "go list -json" output the checks use.
type Package struct {
	ImportPath string
	Dir        string
	Standard   bool
	Module     *struct{ Path string }
	Imports    []string
	Error      *struct{ Err string }
	DepsErrors []*struct{ Err string }
}

// Decode reads a stream of "go list -json" package objects.
func Decode(r io.Reader) ([]Package, error) {
	dec := json.NewDecoder(r)
	var out []Package
	for {
		var p Package
		err := dec.Decode(&p)
		if errors.Is(err, io.EOF) {
			return out, nil
		}
		if err != nil {
			return nil, fmt.Errorf("decode go list output: %w", err)
		}
		out = append(out, p)
	}
}

// Policy says which packages are pure and what they may not reach.
type Policy struct {
	// Module is the module path; packages under it are internal, everything else that is not
	// standard library is third-party.
	Module string
	// Pure lists the import paths that must stay pure.
	Pure []string
	// ForbiddenStd lists standard-library packages a pure package may not reach. An entry ending
	// in "/..." also forbids every package below it.
	ForbiddenStd []string
	// ForbiddenInternal lists module-relative directories (for example "adapters") a pure
	// package may not reach, including every package below them.
	ForbiddenInternal []string
}

// Violation is one broken rule.
type Violation struct {
	Pure  string   // the pure package
	Bad   string   // the forbidden package, or "" for a load error or a cycle
	Chain []string // Pure ... Bad, by import
	Why   string
}

func (v Violation) String() string {
	if len(v.Chain) > 0 {
		return fmt.Sprintf("%s: %s via %s", v.Pure, v.Why, strings.Join(v.Chain, " -> "))
	}
	return fmt.Sprintf("%s: %s", v.Pure, v.Why)
}

// Check reports every violation of p in pkgs. A pure package missing from pkgs is itself a
// violation: a check that silently skips its subject proves nothing.
func Check(pkgs []Package, p Policy) []Violation {
	byPath := map[string]*Package{}
	for i := range pkgs {
		byPath[pkgs[i].ImportPath] = &pkgs[i]
	}
	var out []Violation
	for _, root := range p.Pure {
		rp, ok := byPath[root]
		if !ok {
			out = append(out, Violation{Pure: root, Why: "pure package not found in the package graph"})
			continue
		}
		if rp.Error != nil {
			out = append(out, Violation{Pure: root, Why: "load error: " + rp.Error.Err})
		}
		for _, de := range rp.DepsErrors {
			out = append(out, Violation{Pure: root, Why: "dependency load error: " + de.Err})
		}
		if cyc := findCycle(root, byPath); cyc != nil {
			out = append(out, Violation{Pure: root, Chain: cyc, Why: "import cycle"})
		}
		parent := map[string]string{root: ""}
		queue := []string{root}
		for len(queue) > 0 {
			cur := queue[0]
			queue = queue[1:]
			if cur != root {
				if why := forbidden(cur, byPath[cur], p); why != "" {
					out = append(out, Violation{Pure: root, Bad: cur, Chain: chain(parent, cur), Why: why})
					continue // a forbidden package's own imports add nothing new
				}
			}
			cp := byPath[cur]
			if cp == nil {
				continue
			}
			for _, imp := range cp.Imports {
				if _, seen := parent[imp]; !seen {
					parent[imp] = cur
					queue = append(queue, imp)
				}
			}
		}
	}
	return out
}

func forbidden(imp string, pkg *Package, p Policy) string {
	internal := imp == p.Module || strings.HasPrefix(imp, p.Module+"/")
	switch {
	case internal:
		rel := strings.TrimPrefix(strings.TrimPrefix(imp, p.Module), "/")
		for _, f := range p.ForbiddenInternal {
			if rel == f || strings.HasPrefix(rel, f+"/") {
				return "reaches effectful module package " + f
			}
		}
		return ""
	case pkg != nil && pkg.Standard, pkg == nil && isStdPath(imp):
		for _, f := range p.ForbiddenStd {
			if base, ok := strings.CutSuffix(f, "/..."); ok {
				if imp == base || strings.HasPrefix(imp, base+"/") {
					return "reaches forbidden standard package " + f
				}
			} else if imp == f {
				return "reaches forbidden standard package " + f
			}
		}
		return ""
	default:
		return "reaches third-party package " + imp
	}
}

// isStdPath reports whether an import path that go list did not describe looks like the
// standard library (no dot in its first element).
func isStdPath(imp string) bool {
	first, _, _ := strings.Cut(imp, "/")
	return !strings.Contains(first, ".")
}

func chain(parent map[string]string, leaf string) []string {
	var c []string
	for cur := leaf; cur != ""; cur = parent[cur] {
		c = append(c, cur)
	}
	for i, j := 0, len(c)-1; i < j; i, j = i+1, j-1 {
		c[i], c[j] = c[j], c[i]
	}
	return c
}

// findCycle returns one import cycle reachable from root, or nil. The go tool refuses a cycle
// when it loads the packages; this catches one in a graph assembled any other way.
func findCycle(root string, byPath map[string]*Package) []string {
	const (
		visiting = 1
		done     = 2
	)
	state := map[string]int{}
	var stack []string
	var walk func(string) []string
	walk = func(n string) []string {
		state[n] = visiting
		stack = append(stack, n)
		if p := byPath[n]; p != nil {
			for _, imp := range p.Imports {
				switch state[imp] {
				case visiting:
					for i, s := range stack {
						if s == imp {
							return append(append([]string(nil), stack[i:]...), imp)
						}
					}
				case 0:
					if c := walk(imp); c != nil {
						return c
					}
				}
			}
		}
		stack = stack[:len(stack)-1]
		state[n] = done
		return nil
	}
	return walk(root)
}

// Knob is what the knob-read lint needs to know about one knob.
type Knob struct {
	Name    string // for example "write.rate.per_pr_hour"
	EnvName string // its environment variable
	V1      int64  // the earlier hard-coded literal, or 0 for none
}

// Finding is one knob-read lint hit.
type Finding struct {
	File string
	Line int
	What string
}

func (f Finding) String() string { return fmt.Sprintf("%s:%d: %s", f.File, f.Line, f.What) }

// envReaders are the process-environment readers, by package.
var envReaders = map[string]map[string]bool{
	"os":      {"Getenv": true, "LookupEnv": true, "Environ": true, "ExpandEnv": true},
	"syscall": {"Getenv": true, "Environ": true},
}

// ScanKnobReads reports, for every non-test Go file in fsys outside the exempt directories
// (slash-separated, relative to the root of fsys), each read of the process environment and
// each integer literal equal to the V1 literal of a knob the file names (by knob name or by
// variable). Directories named testdata, or starting with "." or "_", are skipped, as the go
// tool skips them.
func ScanKnobReads(fsys fs.FS, knobs []Knob, exempt []string) ([]Finding, error) {
	var out []Finding
	err := fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			name := d.Name()
			if p != "." && (name == "testdata" || strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_")) {
				return fs.SkipDir
			}
			for _, e := range exempt {
				if p == e {
					return fs.SkipDir
				}
			}
			return nil
		}
		if path.Ext(p) != ".go" || strings.HasSuffix(p, "_test.go") {
			return nil
		}
		src, err := fs.ReadFile(fsys, p)
		if err != nil {
			return err
		}
		found, err := scanFile(p, src, knobs)
		if err != nil {
			return err
		}
		out = append(out, found...)
		return nil
	})
	sort.Slice(out, func(i, j int) bool {
		if out[i].File != out[j].File {
			return out[i].File < out[j].File
		}
		return out[i].Line < out[j].Line
	})
	return out, err
}

func scanFile(name string, src []byte, knobs []Knob) ([]Finding, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, name, src, parser.SkipObjectResolution)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", name, err)
	}
	var out []Finding
	at := func(pos token.Pos, what string) {
		out = append(out, Finding{File: name, Line: fset.Position(pos).Line, What: what})
	}
	local := map[string]string{} // local name -> package path, for os and syscall
	for _, imp := range f.Imports {
		pkg, _ := strconv.Unquote(imp.Path.Value)
		if envReaders[pkg] == nil {
			continue
		}
		n := pkg
		if imp.Name != nil {
			n = imp.Name.Name
		}
		switch n {
		case "_":
		case ".":
			at(imp.Pos(), "dot-import of "+pkg+" hides environment reads")
		default:
			local[n] = pkg
		}
	}
	text := string(src)
	var named []Knob
	for _, k := range knobs {
		if k.V1 != 0 && (strings.Contains(text, k.Name) || (k.EnvName != "" && strings.Contains(text, k.EnvName))) {
			named = append(named, k)
		}
	}
	ast.Inspect(f, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.SelectorExpr:
			if id, ok := x.X.(*ast.Ident); ok {
				if pkg, ok := local[id.Name]; ok && envReaders[pkg][x.Sel.Name] {
					at(x.Pos(), fmt.Sprintf("environment read %s.%s outside the config package", pkg, x.Sel.Name))
				}
			}
		case *ast.BasicLit:
			if x.Kind != token.INT {
				return true
			}
			v, err := strconv.ParseInt(x.Value, 0, 64)
			if err != nil {
				return true
			}
			for _, k := range named {
				if v == k.V1 {
					at(x.Pos(), fmt.Sprintf("literal %d is knob %s's earlier hard-coded value; read the knob from config", v, k.Name))
				}
			}
		}
		return true
	})
	return out, nil
}
