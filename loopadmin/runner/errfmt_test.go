package runner_test

import (
	"go/ast"
	"go/constant"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// Defect class: an error the contract returns that carries request, result or
// adapter content in its text. The guard type-checks the package and keys
// every decision on types and provenance, never on an identifier's name, so a
// renamed variable cannot slip a payload through.
//
// An error value is TRUSTED when it is nil, a package-level sentinel whose
// initializer is trusted, a call to a concrete function of this package (whose
// own returns this guard checks), a fmt.Errorf that passes the format rules, an
// errors.New of a constant, or a literal of an in-package error type whose
// Error method reads only trusted fields. Everything else is untrusted: a call
// through an interface (an Adapter or a Fence), through a func value, into
// another package, a parameter, a range or type-switch variable, a variable
// whose address is taken, and a struct field. The analysis is per variable and
// flow-insensitive: a variable that ever holds an untrusted error is untrusted
// everywhere, so an adapter's error never shares a variable with a refusal.
//
// Sinks:
//   - every return of an error-typed result, except inside the boundary shims
//     (methods of an in-package type that implements Adapter or Fence, which
//     only forward the adapter's answer to the Client that seals it);
//   - every fmt.Errorf: a constant format with no indexed or starred verb; %w
//     only of a trusted error; %d only of an integer type; %s, %v and %q only of
//     a constant or a fieldPath; no argument without a verb (fmt would print it
//     as %!(EXTRA ...));
//   - every errors.New: a constant argument only;
//   - every literal of, or assignment to a field of, an in-package error type:
//     each field its Error method reads must be trusted;
//   - fieldPath, the one non-constant string type a refusal may format: a
//     conversion to it outside its own methods must be of a constant, its
//     methods take no string parameter, and its address is never taken.
//
// It is a floor over this package's own source, not a proof: reflection and
// unsafe are out of its reach.
type guard struct {
	fset      *token.FileSet
	pkg       *types.Package
	info      *types.Info
	files     []*ast.File
	errType   types.Type
	fieldPath types.Type
	bounds    []*types.Interface
	reads     map[*types.TypeName]map[*types.Var]bool // fields an Error method reads
	sources   map[*types.Var][]source
	tainted   map[*types.Var]bool
	visiting  map[*types.Var]bool
	fpMethods map[*ast.FuncDecl]bool
	bad       map[string]bool
	returns   int // error-typed results checked
}

type source struct {
	expr  ast.Expr
	tuple bool // one result of a multi-value expression
}

var sharedImporter = importer.ForCompiler(token.NewFileSet(), "source", nil)

func loadGuard(srcs map[string][]byte) (*guard, error) {
	g := &guard{
		fset:      token.NewFileSet(),
		errType:   types.Universe.Lookup("error").Type(),
		reads:     map[*types.TypeName]map[*types.Var]bool{},
		sources:   map[*types.Var][]source{},
		tainted:   map[*types.Var]bool{},
		visiting:  map[*types.Var]bool{},
		fpMethods: map[*ast.FuncDecl]bool{},
		bad:       map[string]bool{},
	}
	names := make([]string, 0, len(srcs))
	for n := range srcs {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		f, err := parser.ParseFile(g.fset, n, srcs[n], 0)
		if err != nil {
			return nil, err
		}
		g.files = append(g.files, f)
	}
	g.info = &types.Info{
		Types:      map[ast.Expr]types.TypeAndValue{},
		Defs:       map[*ast.Ident]types.Object{},
		Uses:       map[*ast.Ident]types.Object{},
		Implicits:  map[ast.Node]types.Object{},
		Selections: map[*ast.SelectorExpr]*types.Selection{},
	}
	conf := types.Config{Importer: sharedImporter}
	pkg, err := conf.Check(g.files[0].Name.Name, g.fset, g.files, g.info)
	if err != nil {
		return nil, err
	}
	g.pkg = pkg
	if tn, ok := pkg.Scope().Lookup("fieldPath").(*types.TypeName); ok {
		g.fieldPath = tn.Type()
	}
	for _, name := range []string{"Adapter", "Fence"} {
		if tn, ok := pkg.Scope().Lookup(name).(*types.TypeName); ok {
			if it, ok := tn.Type().Underlying().(*types.Interface); ok {
				g.bounds = append(g.bounds, it)
			}
		}
	}
	return g, nil
}

func (g *guard) flag(at token.Pos, msg string) {
	g.bad[g.fset.Position(at).String()+": "+msg] = true
}

func (g *guard) findings() []string {
	out := make([]string, 0, len(g.bad))
	for b := range g.bad {
		out = append(out, b)
	}
	sort.Strings(out)
	return out
}

func (g *guard) isErr(t types.Type) bool {
	return t != nil && types.Identical(t, g.errType)
}

func (g *guard) isFieldPath(t types.Type) bool {
	return g.fieldPath != nil && t != nil && types.Identical(t, g.fieldPath)
}

func (g *guard) localVar(e ast.Expr) *types.Var {
	id, ok := ast.Unparen(e).(*ast.Ident)
	if !ok {
		return nil
	}
	obj := g.info.Defs[id]
	if obj == nil {
		obj = g.info.Uses[id]
	}
	v, _ := obj.(*types.Var)
	return v
}

func (g *guard) packageLevel(v *types.Var) bool {
	return v.Pkg() == g.pkg && v.Parent() == g.pkg.Scope()
}

// recvNamed returns the named type a method's receiver is declared on.
func recvNamed(t types.Type) *types.Named {
	if p, ok := t.(*types.Pointer); ok {
		t = p.Elem()
	}
	n, _ := t.(*types.Named)
	return n
}

// exempt reports whether fn is a boundary shim: a method of an in-package type
// that implements Adapter or Fence.
func (g *guard) exempt(fn *types.Func) bool {
	sig, ok := fn.Type().(*types.Signature)
	if !ok || sig.Recv() == nil {
		return false
	}
	n := recvNamed(sig.Recv().Type())
	if n == nil || n.Obj().Pkg() != g.pkg {
		return false
	}
	for _, b := range g.bounds {
		if types.Implements(n, b) || types.Implements(types.NewPointer(n), b) {
			return true
		}
	}
	return false
}

// collect records every variable's sources and taints, and the fields each
// in-package Error method reads.
func (g *guard) collect() {
	for _, tn := range g.info.Implicits {
		if v, ok := tn.(*types.Var); ok {
			g.tainted[v] = true // type-switch variables
		}
	}
	taintParams := func(ft *ast.FuncType, recv *ast.FieldList) {
		for _, fl := range []*ast.FieldList{recv, ft.Params} {
			if fl == nil {
				continue
			}
			for _, fld := range fl.List {
				for _, n := range fld.Names {
					if v := g.localVar(n); v != nil {
						g.tainted[v] = true
					}
				}
			}
		}
	}
	// An assignment to a package-level error is itself a finding (check), so
	// it adds no source: only the declaration's initializer counts.
	add := func(lhs []ast.Expr, rhs []ast.Expr, decl bool) {
		for i, l := range lhs {
			v := g.localVar(l)
			if v == nil || (!decl && g.packageLevel(v)) {
				continue
			}
			switch {
			case len(rhs) == len(lhs):
				g.sources[v] = append(g.sources[v], source{expr: rhs[i]})
			case len(rhs) == 1:
				g.sources[v] = append(g.sources[v], source{expr: rhs[0], tuple: true})
			}
		}
	}
	for _, f := range g.files {
		ast.Inspect(f, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.FuncDecl:
				taintParams(x.Type, x.Recv)
				if x.Recv != nil && x.Name.Name == "Error" {
					g.collectReads(x)
				}
				if x.Recv != nil && g.fieldPath != nil {
					if fn, ok := g.info.Defs[x.Name].(*types.Func); ok {
						if n := recvNamed(fn.Type().(*types.Signature).Recv().Type()); n != nil && g.isFieldPath(n) {
							g.fpMethods[x] = true
						}
					}
				}
			case *ast.FuncLit:
				taintParams(x.Type, nil)
			case *ast.AssignStmt:
				add(x.Lhs, x.Rhs, false)
			case *ast.ValueSpec:
				lhs := make([]ast.Expr, len(x.Names))
				for i, n := range x.Names {
					lhs[i] = n
				}
				add(lhs, x.Values, true)
			case *ast.RangeStmt:
				for _, e := range []ast.Expr{x.Key, x.Value} {
					if v := g.localVar(e); v != nil {
						g.tainted[v] = true
					}
				}
			case *ast.UnaryExpr:
				if x.Op == token.AND {
					if v := g.localVar(x.X); v != nil {
						g.tainted[v] = true
					}
				}
			}
			return true
		})
	}
}

// collectReads records the receiver fields an Error method reads. A receiver
// used any other way (passed on, or a method called on it) reads every field.
func (g *guard) collectReads(fd *ast.FuncDecl) {
	fn, ok := g.info.Defs[fd.Name].(*types.Func)
	if !ok {
		return
	}
	n := recvNamed(fn.Type().(*types.Signature).Recv().Type())
	if n == nil || fd.Body == nil {
		return
	}
	var recv *types.Var
	if names := fd.Recv.List[0].Names; len(names) == 1 {
		recv = g.localVar(names[0])
	}
	read := map[*types.Var]bool{}
	all := false
	selected := map[*ast.Ident]bool{}
	ast.Inspect(fd.Body, func(node ast.Node) bool {
		sel, ok := node.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		if id, ok := sel.X.(*ast.Ident); ok && recv != nil && g.info.Uses[id] == recv {
			selected[id] = true
			if s := g.info.Selections[sel]; s != nil && s.Kind() == types.FieldVal {
				read[s.Obj().(*types.Var)] = true
			} else {
				all = true
			}
		}
		return true
	})
	ast.Inspect(fd.Body, func(node ast.Node) bool {
		if id, ok := node.(*ast.Ident); ok && recv != nil && g.info.Uses[id] == recv && !selected[id] {
			all = true
		}
		return true
	})
	if st, ok := n.Underlying().(*types.Struct); ok && all {
		for i := 0; i < st.NumFields(); i++ {
			read[st.Field(i)] = true
		}
	}
	if _, ok := n.Underlying().(*types.Struct); !ok {
		read[nil] = true // a non-struct error type reads its own value
	}
	g.reads[n.Obj()] = read
}

func (g *guard) trustedVar(v *types.Var) bool {
	if v.IsField() || g.tainted[v] {
		return false
	}
	if v.Pkg() != g.pkg {
		return false
	}
	if g.visiting[v] {
		return true
	}
	g.visiting[v] = true
	defer delete(g.visiting, v)
	for _, s := range g.sources[v] {
		if s.tuple {
			call, ok := ast.Unparen(s.expr).(*ast.CallExpr)
			if !ok || !g.trustedCall(call) {
				return false
			}
			continue
		}
		if !g.trusted(s.expr) {
			return false
		}
	}
	return true
}

func (g *guard) trusted(e ast.Expr) bool {
	e = ast.Unparen(e)
	tv := g.info.Types[e]
	if tv.IsNil() || tv.Value != nil {
		return true
	}
	switch x := e.(type) {
	case *ast.Ident:
		if v, ok := g.info.Uses[x].(*types.Var); ok {
			return g.trustedVar(v)
		}
	case *ast.SelectorExpr:
		if g.info.Selections[x] == nil { // a package-qualified name
			if v, ok := g.info.Uses[x.Sel].(*types.Var); ok {
				return g.trustedVar(v)
			}
		}
	case *ast.CallExpr:
		return g.trustedCall(x)
	case *ast.UnaryExpr:
		if lit, ok := x.X.(*ast.CompositeLit); ok && x.Op == token.AND {
			return g.trustedLit(lit, false)
		}
	case *ast.CompositeLit:
		return g.trustedLit(x, false)
	}
	return false
}

// callee resolves a call to a concrete function, or nil when the call goes
// through an interface or a func value.
func (g *guard) callee(call *ast.CallExpr) *types.Func {
	switch f := ast.Unparen(call.Fun).(type) {
	case *ast.Ident:
		fn, _ := g.info.Uses[f].(*types.Func)
		return fn
	case *ast.SelectorExpr:
		if s := g.info.Selections[f]; s != nil {
			if s.Kind() == types.FieldVal {
				return nil
			}
			fn, _ := s.Obj().(*types.Func)
			if fn == nil {
				return nil
			}
			if r := fn.Type().(*types.Signature).Recv(); r != nil && types.IsInterface(r.Type()) {
				return nil
			}
			return fn
		}
		fn, _ := g.info.Uses[f.Sel].(*types.Func)
		return fn
	}
	return nil
}

func isFunc(fn *types.Func, pkg, name string) bool {
	return fn != nil && fn.Pkg() != nil && fn.Pkg().Path() == pkg && fn.Name() == name
}

func (g *guard) trustedCall(call *ast.CallExpr) bool {
	if g.info.Types[call.Fun].IsType() { // a conversion
		return len(call.Args) == 1 && g.trusted(call.Args[0])
	}
	fn := g.callee(call)
	switch {
	case fn == nil:
		return false
	case isFunc(fn, "fmt", "Errorf"):
		return len(g.errorfIssues(call)) == 0
	case isFunc(fn, "errors", "New"):
		return len(call.Args) == 1 && g.info.Types[call.Args[0]].Value != nil
	case fn.Pkg() != g.pkg, g.exempt(fn):
		return false
	}
	return true
}

func (g *guard) trustedLit(lit *ast.CompositeLit, report bool) bool {
	t := g.info.Types[lit].Type
	n := recvNamed(t)
	if n == nil {
		return false
	}
	read, ok := g.reads[n.Obj()]
	if !ok {
		return false
	}
	st, _ := n.Underlying().(*types.Struct)
	okAll := true
	for i, el := range lit.Elts {
		var fld *types.Var
		val := el
		if kv, ok := el.(*ast.KeyValueExpr); ok {
			if id, ok := kv.Key.(*ast.Ident); ok {
				fld, _ = g.info.Uses[id].(*types.Var)
			}
			val = kv.Value
		} else if st != nil && i < st.NumFields() {
			fld = st.Field(i)
		}
		if (fld == nil || read[fld]) && !g.trusted(val) {
			okAll = false
			if report {
				g.flag(val.Pos(), "an error literal sets a field its Error method prints from an untrusted value")
			}
		}
	}
	return okAll
}

var verb = regexp.MustCompile(`%([-+# 0]*)(\[[0-9]+\])?(\*?)[0-9]*(?:\.(\*?)[0-9]*)?([a-zA-Z%])`)

// errorfIssues lists what is wrong with one fmt.Errorf call.
func (g *guard) errorfIssues(call *ast.CallExpr) []string {
	if len(call.Args) == 0 {
		return []string{"no format"}
	}
	fv := g.info.Types[call.Args[0]].Value
	if fv == nil || fv.Kind() != constant.String {
		return []string{"format is not a constant"}
	}
	var out []string
	arg := 1
	for _, m := range verb.FindAllStringSubmatch(constant.StringVal(fv), -1) {
		v := m[5]
		if v == "%" {
			continue
		}
		if m[2] != "" || m[3] != "" || m[4] != "" {
			out = append(out, "an indexed or starred verb")
			return out
		}
		if arg >= len(call.Args) {
			return append(out, "more verbs than arguments")
		}
		a := call.Args[arg]
		arg++
		tv := g.info.Types[a]
		switch v {
		case "w":
			if !g.trusted(a) {
				out = append(out, "%w wraps an error the contract does not define")
			}
		case "d":
			if b, ok := tv.Type.Underlying().(*types.Basic); !ok || b.Info()&types.IsInteger == 0 {
				out = append(out, "%d of a non-integer")
			}
		case "s", "v", "q":
			if tv.Value == nil && !g.isFieldPath(tv.Type) {
				out = append(out, "%"+v+" formats a value the contract does not define")
			}
		default:
			out = append(out, "%"+v+" is not allowed in a refusal")
		}
	}
	if arg < len(call.Args) {
		out = append(out, "an argument with no verb")
	}
	return out
}

// check walks every function for the sinks.
func (g *guard) check() {
	for _, f := range g.files {
		for _, d := range f.Decls {
			fd, ok := d.(*ast.FuncDecl)
			if !ok {
				g.checkNodes(d, nil, false, false)
				continue
			}
			fn, _ := g.info.Defs[fd.Name].(*types.Func)
			if fn == nil || fd.Body == nil {
				continue
			}
			if g.fpMethods[fd] {
				sig := fn.Type().(*types.Signature)
				for i := 0; i < sig.Params().Len(); i++ {
					if b, ok := sig.Params().At(i).Type().Underlying().(*types.Basic); ok && b.Info()&types.IsString != 0 {
						g.flag(fd.Pos(), "a fieldPath method takes a string")
					}
				}
			}
			g.checkNodes(fd.Body, fn.Type().(*types.Signature), g.exempt(fn), g.fpMethods[fd])
		}
	}
}

func (g *guard) checkNodes(root ast.Node, sig *types.Signature, exempt, fpMethod bool) {
	ast.Inspect(root, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.FuncLit:
			g.checkNodes(x.Body, g.info.Types[x].Type.(*types.Signature), false, false)
			return false
		case *ast.ReturnStmt:
			if sig != nil && !exempt {
				g.checkReturn(x, sig)
			}
		case *ast.CallExpr:
			if g.info.Types[x.Fun].IsType() {
				if g.isFieldPath(g.info.Types[x.Fun].Type) && !fpMethod && g.info.Types[x.Args[0]].Value == nil {
					g.flag(x.Pos(), "a conversion to fieldPath of a non-constant")
				}
				return true
			}
			fn := g.callee(x)
			switch {
			case isFunc(fn, "fmt", "Errorf"):
				for _, m := range g.errorfIssues(x) {
					g.flag(x.Pos(), m)
				}
			case isFunc(fn, "errors", "New"):
				if len(x.Args) != 1 || g.info.Types[x.Args[0]].Value == nil {
					g.flag(x.Pos(), "errors.New of a non-constant")
				}
			}
		case *ast.CompositeLit:
			g.trustedLit(x, true)
		case *ast.UnaryExpr:
			if x.Op == token.AND && g.isFieldPath(g.info.Types[x.X].Type) {
				g.flag(x.Pos(), "the address of a fieldPath")
			}
		case *ast.AssignStmt:
			for i, l := range x.Lhs {
				var r ast.Expr
				switch {
				case len(x.Rhs) == len(x.Lhs):
					r = x.Rhs[i]
				case len(x.Rhs) == 1:
					r = x.Rhs[0]
				}
				g.checkAssign(x, l, r, len(x.Rhs) != len(x.Lhs))
			}
		}
		return true
	})
}

func (g *guard) checkAssign(st *ast.AssignStmt, l, r ast.Expr, tuple bool) {
	trusted := func() bool {
		if r == nil {
			return false
		}
		if tuple {
			call, ok := ast.Unparen(r).(*ast.CallExpr)
			return ok && g.trustedCall(call)
		}
		return g.trusted(r)
	}
	if v := g.localVar(l); v != nil && g.packageLevel(v) && g.isErr(v.Type()) {
		g.flag(st.Pos(), "assigns a package-level error")
		return
	}
	sel, ok := ast.Unparen(l).(*ast.SelectorExpr)
	if !ok {
		return
	}
	s := g.info.Selections[sel]
	if s == nil || s.Kind() != types.FieldVal {
		return
	}
	n := recvNamed(s.Recv())
	if n == nil {
		return
	}
	if read, ok := g.reads[n.Obj()]; ok && read[s.Obj().(*types.Var)] && !trusted() {
		g.flag(st.Pos(), "assigns an untrusted value to a field an Error method prints")
	}
}

func (g *guard) checkReturn(ret *ast.ReturnStmt, sig *types.Signature) {
	res := sig.Results()
	switch {
	case len(ret.Results) == 0:
		for i := 0; i < res.Len(); i++ {
			if g.isErr(res.At(i).Type()) {
				g.returns++
				if !g.trustedVar(res.At(i)) {
					g.flag(ret.Pos(), "returns an error the contract does not define")
				}
			}
		}
	case len(ret.Results) == 1 && res.Len() > 1:
		for i := 0; i < res.Len(); i++ {
			if g.isErr(res.At(i).Type()) {
				g.returns++
				call, ok := ast.Unparen(ret.Results[0]).(*ast.CallExpr)
				if !ok || !g.trustedCall(call) {
					g.flag(ret.Pos(), "returns an error the contract does not define")
				}
			}
		}
	default:
		for i, e := range ret.Results {
			if i < res.Len() && g.isErr(res.At(i).Type()) {
				g.returns++
				if !g.trusted(e) {
					g.flag(e.Pos(), "returns an error the contract does not define")
				}
			}
		}
	}
}

func runGuard(srcs map[string][]byte) (*guard, error) {
	g, err := loadGuard(srcs)
	if err != nil {
		return nil, err
	}
	g.collect()
	g.check()
	return g, nil
}

// planted is the guard's positive and negative control. Every line marked
// "// want" must be flagged and no other line may be: the shapes the reviews
// found (an adapter error passed through, wrapped with %v or %w, an untrusted
// value under a name the old name-keyed guard allowed), plus the bypasses a
// name-keyed check missed.
const planted = `package p

import (
	"context"
	"errors"
	"fmt"
	"reflect"
)

var errX = errors.New("p: refused")

type Adapter interface {
	Start(ctx context.Context, s string) (int, error)
}

type Fence interface {
	Current(ctx context.Context) (uint64, error)
}

type FenceFunc func(ctx context.Context) (uint64, error)

func (f FenceFunc) Current(ctx context.Context) (uint64, error) { return f(ctx) }

type fieldPath string

func (p fieldPath) field(t reflect.Type, i int) fieldPath { return p + "." + fieldPath(t.Field(i).Name) }

func (p fieldPath) named(s string) fieldPath { return p + fieldPath(s) } // want

type opaque struct{ sentinel, cause error }

func (e *opaque) Error() string   { return e.sentinel.Error() }
func (e *opaque) Unwrap() []error { return []error{e.sentinel, e.cause} }

type C struct {
	a Adapter
	f Fence
	q func() error
}

type R struct {
	Outcome, Name string
	N             int
}

func static() error { return errX }

func (c C) formats(ctx context.Context, r R) {
	_, ae := c.a.Start(ctx, r.Name)
	path := r.Outcome
	err := r.Name
	_ = fmt.Errorf("%w: %s", errX, path)        // want
	_ = fmt.Errorf("%w: %s", errX, err)         // want
	_ = fmt.Errorf("%w: %v", errX, ae)          // want
	_ = fmt.Errorf("%w: %w", errX, ae)          // want
	_ = fmt.Errorf("%w: %d", errX, r.Name)      // want
	_ = fmt.Errorf("%w", errX, r.Name)          // want
	_ = fmt.Errorf("%[2]d %[1]w", errX, 1)      // want
	_ = errors.New(fmt.Sprintf("x %d", r.N))    // want
	_ = fmt.Errorf(r.Name)                      // want
	_ = fieldPath(r.Name)                       // want
	_ = &opaque{sentinel: ae, cause: errX}      // want
	o := &opaque{sentinel: errX}
	o.sentinel = ae                             // want
	errX = ae                                   // want
	var fp fieldPath
	_ = &fp                                     // want
}

func (c C) passthrough(ctx context.Context) error {
	_, err := c.a.Start(ctx, "")
	return err // want
}

func (c C) renamed(ctx context.Context) error {
	_, path := c.a.Start(ctx, "")
	return path // want
}

func (c C) reused(ctx context.Context) error {
	err := static()
	if err != nil {
		return err // want
	}
	_, err = c.a.Start(ctx, "")
	return nil
}

func (c C) funcField() error { return c.q() } // want

type lister interface{ list() error }

func (c C) viaIface(l lister) error { return l.list() } // want

func (c C) fenced(ctx context.Context) (uint64, error) {
	return c.f.Current(ctx) // want
}

func (c C) param(err error) error { return err } // want

func (c C) asTarget(err error) error {
	var t *opaque
	if errors.As(err, &t) {
		return t // want
	}
	return nil
}

func (c C) typeSwitch(x any) error {
	switch e := x.(type) {
	case error:
		return e // want
	}
	return nil
}

func (c C) shim(ctx context.Context, f FenceFunc) error {
	_, err := f.Current(ctx)
	return err // want
}

func (c C) closure(ctx context.Context) func() error {
	return func() error {
		_, err := c.a.Start(ctx, "")
		return err // want
	}
}

func (c C) clean(ctx context.Context, r R, t reflect.Type) error {
	_, ae := c.a.Start(ctx, r.Name)
	if ae != nil {
		return &opaque{sentinel: errX, cause: ae}
	}
	for name, v := range map[fieldPath]string{"caller": r.Name} {
		if v == "" {
			return fmt.Errorf("%w: %s is required", errX, name)
		}
	}
	var p fieldPath = "request"
	if r.N > 0 {
		return fmt.Errorf("%w: %s at %d of %s", errX, p.field(t, 0), r.N, "v1")
	}
	if err := static(); err != nil {
		return err
	}
	if err := static(); err != nil {
		return fmt.Errorf("%w: %w", errX, err)
	}
	return nil
}
`

// TestNoPayloadInErrors runs the class guard over every non-test source file
// of the contract package, after the planted control shows it flags exactly
// the marked shapes.
func TestNoPayloadInErrors(t *testing.T) {
	g, err := runGuard(map[string][]byte{"planted.go": []byte(planted)})
	if err != nil {
		t.Fatal(err)
	}
	want := map[int]bool{}
	for i, line := range strings.Split(planted, "\n") {
		if strings.Contains(line, "// want") {
			want[i+1] = true
		}
	}
	got := map[int]bool{}
	for _, b := range g.findings() {
		parts := strings.Split(b, ":")
		n, err := strconv.Atoi(parts[1])
		if err != nil {
			t.Fatalf("finding %q has no line", b)
		}
		got[n] = true
		if !want[n] {
			t.Errorf("control: flagged a clean line: %s", b)
		}
	}
	for n := range want {
		if !got[n] {
			t.Errorf("control: did not flag planted.go:%d", n)
		}
	}
	if len(want) < 20 {
		t.Fatalf("control plants only %d shapes", len(want))
	}
	if t.Failed() {
		t.FailNow()
	}

	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	srcs := map[string][]byte{}
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		b, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		srcs[file] = b
	}
	if len(srcs) < 5 {
		t.Fatalf("class guard checked only %d source files", len(srcs))
	}
	g, err = runGuard(srcs)
	if err != nil {
		t.Fatal(err)
	}
	for _, b := range g.findings() {
		t.Error(b)
	}
	if g.returns < 40 {
		t.Fatalf("class guard checked only %d error returns", g.returns)
	}
}
