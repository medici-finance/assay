// Package archtest states deskcore's architecture as checks that its tests run, so a change that
// breaks a rule fails `go test` in this directory. CI does not run this suite yet
// (medici-finance/assay#2208); until it does, a rule holds only where the suite is run.
//
// The checks:
//
//   - Check walks a package graph (the JSON stream "go list -e -json -deps" prints) and reports
//     every pure package that reaches a forbidden package by any transitive path: a process,
//     network or HTTP package from the standard library, an effectful package of the module
//     itself, or any third-party package. It also reports every module package, reachable from a
//     pure one, that directly imports a standard package outside an allow list. The standard
//     library itself reaches os and syscall, so a transitive ban cannot cover them; the allow
//     list does, together with every other standard package that can touch the filesystem
//     (path/filepath, text/template, go/parser, runtime/coverage), without naming any of them.
//     Each violation carries the import chain, so the fix is obvious.
//   - ScanKnobReads parses Go source and reports every read of the process environment, and
//     every hard-coded copy of a knob's earlier literal value, outside the config package. A
//     knob has exactly one reader; a second one would let the resolver's precedence and bounds
//     be bypassed.
//   - ScanRules reports calls a rule forbids outside the functions it allows. An allowed
//     package can still have entry points with an effect (time.Now and time.LoadLocation,
//     fmt.Println and fmt.Scan); the rules forbid those in a pure package, and forbid a second
//     place that decodes untrusted JSON or computes a time difference.
//   - ScanForgeable reports a struct type that has a constructor and an exported field, so a
//     composite literal could build one without the constructor's checks.
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
	// AllowedStd lists the only standard-library packages that a pure package, and every module
	// package it reaches, may import directly; an empty list allows none. The standard library
	// reaches os and syscall itself (fmt imports os), so a transitive ban cannot cover them, and
	// a deny list of direct imports would have to name every package that touches the
	// filesystem (path/filepath, text/template, go/parser, runtime/coverage and more). An allow
	// list needs no such enumeration: a new standard import is refused until it is added here.
	AllowedStd []string
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
			if isInternal(cur, p.Module) {
				for _, imp := range cp.Imports {
					ip := byPath[imp]
					std := (ip != nil && ip.Standard) || (ip == nil && isStdPath(imp))
					// A package the transitive check forbids is reported once, by that check.
					if std && !isInternal(imp, p.Module) && !contains(p.AllowedStd, imp) && forbidden(imp, ip, p) == "" {
						out = append(out, Violation{Pure: root, Bad: imp, Chain: append(chain(parent, cur), imp),
							Why: "imports standard package " + imp + " directly, which is not on the allow list"})
					}
				}
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

func isInternal(imp, module string) bool {
	return imp == module || strings.HasPrefix(imp, module+"/")
}

func contains(list []string, s string) bool {
	for _, e := range list {
		if e == s {
			return true
		}
	}
	return false
}

func forbidden(imp string, pkg *Package, p Policy) string {
	switch {
	case isInternal(imp, p.Module):
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

// Rule forbids a call in the scanned directories except inside the functions it allows.
type Rule struct {
	// Pkg and Funcs name package-level functions, for example "time" and Now. A use is matched
	// under any import name, called or not; a dot-import of Pkg is itself a finding.
	Pkg   string
	Funcs []string
	// Method, set instead of Pkg, matches every call of a method with this name, x.Method(...),
	// where x is not an imported package. Without type information the receiver is not known,
	// so the rule fails closed on every method of that name.
	Method string
	// Allow lists the functions the use may appear in, as "dir/file.go:Func" or
	// "dir/file.go:Recv.Method", relative to the root of the scanned file system.
	Allow []string
	// Why explains the rule in each finding.
	Why string
}

func (r Rule) label(sel string) string {
	if r.Method != "" {
		return "call of method " + sel
	}
	return r.Pkg + "." + sel
}

// ScanRules reports every use a rule forbids in the non-test Go files directly inside dirs
// (slash-separated, relative to the root of fsys). A directory that cannot be read is an error,
// since a lint that silently scans nothing proves nothing, and so is an Allow entry that allowed
// no use, so an allow-list cannot outlive the code it names.
func ScanRules(fsys fs.FS, dirs []string, rules []Rule) ([]Finding, error) {
	var out []Finding
	used := map[string]bool{}
	for _, dir := range dirs {
		files, err := parseDir(fsys, dir)
		if err != nil {
			return nil, err
		}
		for _, pf := range files {
			out = append(out, scanRulesFile(pf, rules, used)...)
		}
	}
	var stale []error
	for _, r := range rules {
		for _, a := range r.Allow {
			if !used[a] {
				stale = append(stale, fmt.Errorf("allow entry %s for %s allowed nothing; remove it", a, r.Why))
			}
		}
	}
	if len(stale) > 0 {
		return nil, errors.Join(stale...)
	}
	sortFindings(out)
	return out, nil
}

func scanRulesFile(pf parsedFile, rules []Rule, used map[string]bool) []Finding {
	var out []Finding
	at := func(pos token.Pos, what string) {
		out = append(out, Finding{File: pf.name, Line: pf.fset.Position(pos).Line, What: what})
	}
	pkgNames := map[string]bool{} // every local name of an imported package
	local := map[string]string{}  // local name -> import path
	for _, imp := range pf.file.Imports {
		path, _ := strconv.Unquote(imp.Path.Value)
		n := path[strings.LastIndex(path, "/")+1:]
		if imp.Name != nil {
			n = imp.Name.Name
		}
		if n == "." {
			for _, r := range rules {
				if r.Pkg == path {
					at(imp.Pos(), fmt.Sprintf("dot-import of %s hides %s", path, r.Why))
				}
			}
			continue
		}
		pkgNames[n] = true
		local[n] = path
	}
	check := func(fn string, body ast.Node) {
		ast.Inspect(body, func(n ast.Node) bool {
			var sel *ast.SelectorExpr
			isCall := false
			switch x := n.(type) {
			case *ast.CallExpr:
				sel, _ = x.Fun.(*ast.SelectorExpr)
				isCall = true
			case *ast.SelectorExpr:
				sel = x
			}
			if sel == nil {
				return true
			}
			id, isIdent := sel.X.(*ast.Ident)
			for _, r := range rules {
				var hit bool
				switch {
				case r.Method != "":
					hit = isCall && sel.Sel.Name == r.Method && !(isIdent && pkgNames[id.Name])
				default:
					hit = !isCall && isIdent && local[id.Name] == r.Pkg && contains(r.Funcs, sel.Sel.Name)
				}
				if !hit {
					continue
				}
				key := pf.name + ":" + fn
				if fn != "" && contains(r.Allow, key) {
					used[key] = true
					continue
				}
				where := "at package level"
				if fn != "" {
					where = "in " + fn
				}
				at(sel.Pos(), fmt.Sprintf("%s %s: %s", r.label(sel.Sel.Name), where, r.Why))
			}
			return true
		})
	}
	for _, d := range pf.file.Decls {
		if fd, ok := d.(*ast.FuncDecl); ok {
			check(funcName(fd), fd)
		} else {
			check("", d)
		}
	}
	return out
}

func funcName(fd *ast.FuncDecl) string {
	if fd.Recv == nil || len(fd.Recv.List) == 0 {
		return fd.Name.Name
	}
	return typeName(fd.Recv.List[0].Type) + "." + fd.Name.Name
}

// typeName returns the name of a (possibly pointer or generic) type expression, or "".
func typeName(e ast.Expr) string {
	switch x := e.(type) {
	case *ast.StarExpr:
		return typeName(x.X)
	case *ast.IndexExpr:
		return typeName(x.X)
	case *ast.IndexListExpr:
		return typeName(x.X)
	case *ast.Ident:
		return x.Name
	}
	return ""
}

// ScanForgeable reports every struct type, in the non-test Go files directly inside dirs, that
// has a constructor (a function named New<Type> whose first result is the type or a pointer to
// it) and an exported field. A composite literal can set such a field and so build a value the
// constructor would have refused.
func ScanForgeable(fsys fs.FS, dirs []string) ([]Finding, error) {
	var out []Finding
	for _, dir := range dirs {
		files, err := parseDir(fsys, dir)
		if err != nil {
			return nil, err
		}
		type open struct {
			file  parsedFile
			pos   token.Pos
			field string
		}
		exported := map[string]open{}
		ctors := map[string]bool{}
		for _, pf := range files {
			for _, d := range pf.file.Decls {
				switch x := d.(type) {
				case *ast.GenDecl:
					for _, sp := range x.Specs {
						ts, ok := sp.(*ast.TypeSpec)
						if !ok {
							continue
						}
						st, ok := ts.Type.(*ast.StructType)
						if !ok {
							continue
						}
						if f := exportedField(st); f != "" {
							exported[ts.Name.Name] = open{file: pf, pos: ts.Pos(), field: f}
						}
					}
				case *ast.FuncDecl:
					if x.Recv == nil && x.Type.Results != nil && len(x.Type.Results.List) > 0 {
						t := typeName(x.Type.Results.List[0].Type)
						if t != "" && x.Name.Name == "New"+t {
							ctors[t] = true
						}
					}
				}
			}
		}
		for t, o := range exported {
			if ctors[t] {
				out = append(out, Finding{File: o.file.name, Line: o.file.fset.Position(o.pos).Line,
					What: fmt.Sprintf("type %s has constructor New%s and exported field %s; a composite literal skips the constructor", t, t, o.field)})
			}
		}
	}
	sortFindings(out)
	return out, nil
}

func exportedField(st *ast.StructType) string {
	for _, f := range st.Fields.List {
		if len(f.Names) == 0 {
			if n := typeName(f.Type); n != "" && ast.IsExported(n) {
				return n
			}
			continue
		}
		for _, n := range f.Names {
			if n.IsExported() {
				return n.Name
			}
		}
	}
	return ""
}

type parsedFile struct {
	name string
	fset *token.FileSet
	file *ast.File
}

// parseDir parses the non-test Go files directly inside dir.
func parseDir(fsys fs.FS, dir string) ([]parsedFile, error) {
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return nil, fmt.Errorf("scan %s: %w", dir, err)
	}
	var out []parsedFile
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || path.Ext(name) != ".go" || strings.HasSuffix(name, "_test.go") {
			continue
		}
		p := path.Join(dir, name)
		src, err := fs.ReadFile(fsys, p)
		if err != nil {
			return nil, err
		}
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, p, src, parser.SkipObjectResolution)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", p, err)
		}
		out = append(out, parsedFile{name: p, fset: fset, file: f})
	}
	return out, nil
}

func sortFindings(out []Finding) {
	sort.Slice(out, func(i, j int) bool {
		if out[i].File != out[j].File {
			return out[i].File < out[j].File
		}
		if out[i].Line != out[j].Line {
			return out[i].Line < out[j].Line
		}
		return out[i].What < out[j].What
	})
}
