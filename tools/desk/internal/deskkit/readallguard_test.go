package deskkit

// readallguard_test.go — the io.ReadAll half of the forge-client class guard
// (forgetimeout_test.go, issue #6718).
//
// Once a per-request deadline can cut a forge response body short, io.ReadAll returns the
// prefix it read together with an error. Code that drops that error decodes a truncated page
// (`[]`, say) as a complete answer. The guard reports every unchecked io.ReadAll error in the
// scanned trees, in these shapes:
//
//   - the error bound to `_`: `x, _ := io.ReadAll(r)`, `_, _ = ...`, `var x, _ = ...`;
//   - the whole result dropped: `io.ReadAll(r)` as a statement;
//   - the error bound to a name that is never read afterwards: no later reference, only
//     `_ = err`, or overwritten (`err = ...`) before any read;
//   - each of the above through io/ioutil.ReadAll, or through a function in the same package
//     whose body returns io.ReadAll(...) directly (a one-level wrapper);
//   - each of the above inside a function declaration or inside any function literal,
//     including one outside every function declaration (a package-level func value, a func
//     field in a package-level composite literal).
//
// It is a syntactic scan, not data-flow analysis: a read of the error anywhere after the call
// in the enclosing function (a closure included) counts as checked, and a wrapper more than
// one call deep is not followed. Known misses: `defer`/`go io.ReadAll(r)`, a method wrapper, a
// bare return inside a nested literal (read as the enclosing named error result), and an error
// bound to a field or index. ReadForgeBody is the control; this guard is defense in depth.

import (
	"go/ast"
	"go/token"
	"sort"
)

// readAllNames resolves the local names file f gives io and io/ioutil.
func readAllNames(f *ast.File) (ioName, ioutilName string) {
	return importName(f, "io", "io"), importName(f, "io/ioutil", "ioutil")
}

// isReadAllCall reports whether e is a call of io.ReadAll, ioutil.ReadAll or one of the
// package's one-level wrappers. via names the wrapper ("" for a direct call).
func isReadAllCall(e ast.Expr, ioName, ioutilName string, wrappers map[string]bool) (via string, ok bool) {
	call, isCall := ast.Unparen(e).(*ast.CallExpr)
	if !isCall {
		return "", false
	}
	switch fn := call.Fun.(type) {
	case *ast.SelectorExpr:
		if id, isID := fn.X.(*ast.Ident); isID && fn.Sel.Name == "ReadAll" &&
			((ioName != "" && id.Name == ioName) || (ioutilName != "" && id.Name == ioutilName)) {
			return "", true
		}
	case *ast.Ident:
		if wrappers[fn.Name] {
			return fn.Name, true
		}
	}
	return "", false
}

// readAllWrappers names the package-level functions in pkg whose body returns a direct
// io.ReadAll (or ioutil.ReadAll) call, so a discarded error from one of them is reported too.
func readAllWrappers(pkg []*ast.File) map[string]bool {
	out := map[string]bool{}
	for _, f := range pkg {
		ioName, ioutilName := readAllNames(f)
		for _, d := range f.Decls {
			fd, ok := d.(*ast.FuncDecl)
			if !ok || fd.Recv != nil || fd.Body == nil {
				continue
			}
			ast.Inspect(fd.Body, func(n ast.Node) bool {
				if _, lit := n.(*ast.FuncLit); lit {
					return false
				}
				if r, ok := n.(*ast.ReturnStmt); ok && len(r.Results) == 1 {
					if _, ok := isReadAllCall(r.Results[0], ioName, ioutilName, nil); ok {
						out[fd.Name.Name] = true
					}
				}
				return true
			})
		}
	}
	return out
}

// readAllDiscardSites reports each unchecked io.ReadAll error in f.
func readAllDiscardSites(fset *token.FileSet, f *ast.File, wrappers map[string]bool) (sites []string) {
	ioName, ioutilName := readAllNames(f)
	report := func(n ast.Node, what, via string) {
		if via != "" {
			what += " (via " + via + ")"
		}
		sites = append(sites, fset.Position(n.Pos()).String()+": "+what)
	}
	// errVar classifies the error slot of a two-value io.ReadAll binding.
	errVar := func(n ast.Node, slot ast.Expr, via string, ft *ast.FuncType, body *ast.BlockStmt) {
		id, ok := slot.(*ast.Ident)
		if !ok {
			return // a field or index target: out of this guard's reach
		}
		if id.Name == "_" {
			report(n, "io.ReadAll error discarded", via)
			return
		}
		if body != nil && !errReadAfter(body, ft, id, n.End()) {
			report(n, "io.ReadAll error never checked", via)
		}
	}
	// Package-level declarations: `var x, _ = io.ReadAll(...)`.
	for _, d := range f.Decls {
		if gd, ok := d.(*ast.GenDecl); ok {
			for _, s := range gd.Specs {
				if vs, ok := s.(*ast.ValueSpec); ok && len(vs.Names) == 2 && len(vs.Values) == 1 {
					if via, ok := isReadAllCall(vs.Values[0], ioName, ioutilName, wrappers); ok {
						errVar(vs, vs.Names[1], via, nil, nil)
					}
				}
			}
		}
	}
	// Function bodies, each scanned against its own body (a nested literal is its own scope
	// for finding the binding, but a read inside it counts for the enclosing one).
	var scanFunc func(ft *ast.FuncType, body *ast.BlockStmt)
	scanFunc = func(ft *ast.FuncType, body *ast.BlockStmt) {
		ast.Inspect(body, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.FuncLit:
				scanFunc(x.Type, x.Body)
				return false
			case *ast.ExprStmt:
				if via, ok := isReadAllCall(x.X, ioName, ioutilName, wrappers); ok {
					report(x, "io.ReadAll result ignored", via)
				}
			case *ast.AssignStmt:
				if len(x.Lhs) == 2 && len(x.Rhs) == 1 {
					if via, ok := isReadAllCall(x.Rhs[0], ioName, ioutilName, wrappers); ok {
						errVar(x, x.Lhs[1], via, ft, body)
					}
				}
			case *ast.ValueSpec:
				if len(x.Names) == 2 && len(x.Values) == 1 {
					if via, ok := isReadAllCall(x.Values[0], ioName, ioutilName, wrappers); ok {
						errVar(x, x.Names[1], via, ft, body)
					}
				}
			}
			return true
		})
	}
	for _, d := range f.Decls {
		switch x := d.(type) {
		case *ast.FuncDecl:
			if x.Body != nil {
				scanFunc(x.Type, x.Body)
			}
		case *ast.GenDecl:
			// A function literal outside every function declaration: a package-level func
			// value, or a func field in a package-level composite literal.
			ast.Inspect(x, func(n ast.Node) bool {
				if lit, ok := n.(*ast.FuncLit); ok {
					scanFunc(lit.Type, lit.Body)
					return false
				}
				return true
			})
		}
	}
	return sites
}

// errReadAfter reports whether the variable id names is read in body after pos, before it is
// overwritten. A reference as the target of an assignment is a write; a reference whose only
// use is `_ = err` (or `var _ = err`) is a discard and does not count; any other reference is
// a read. A bare return in a function whose named results include the variable is a read.
func errReadAfter(body *ast.BlockStmt, ft *ast.FuncType, id *ast.Ident, pos token.Pos) bool {
	same := func(r *ast.Ident) bool {
		if r.Name != id.Name {
			return false
		}
		if r.Obj != nil && id.Obj != nil {
			return r.Obj == id.Obj
		}
		return true
	}
	named := false
	if ft != nil && ft.Results != nil {
		for _, fl := range ft.Results.List {
			for _, nm := range fl.Names {
				if nm.Name == id.Name {
					named = true
				}
			}
		}
	}
	const (
		read = iota
		write
		discard
	)
	type ref struct {
		pos  token.Pos
		kind int
	}
	kind := map[*ast.Ident]int{}
	var refs []ref
	ast.Inspect(body, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.AssignStmt:
			allBlank := true
			for _, l := range x.Lhs {
				if li, ok := l.(*ast.Ident); ok {
					kind[li] = write
					if li.Name != "_" {
						allBlank = false
					}
				} else {
					allBlank = false
				}
			}
			if allBlank {
				for _, r := range x.Rhs {
					if ri, ok := r.(*ast.Ident); ok {
						kind[ri] = discard
					}
				}
			}
		case *ast.ValueSpec:
			allBlank := true
			for _, nm := range x.Names {
				kind[nm] = write
				if nm.Name != "_" {
					allBlank = false
				}
			}
			if allBlank {
				for _, v := range x.Values {
					if vi, ok := v.(*ast.Ident); ok {
						kind[vi] = discard
					}
				}
			}
		case *ast.ReturnStmt:
			if named && len(x.Results) == 0 && x.Pos() > pos {
				refs = append(refs, ref{x.Pos(), read})
			}
		case *ast.Ident:
			if x.Pos() > pos && same(x) {
				k, ok := kind[x]
				if !ok {
					k = read
				}
				refs = append(refs, ref{x.Pos(), k})
			}
		}
		return true
	})
	sort.Slice(refs, func(i, j int) bool { return refs[i].pos < refs[j].pos })
	for _, r := range refs {
		switch r.kind {
		case read:
			return true
		case write:
			return false
		}
	}
	return false
}
