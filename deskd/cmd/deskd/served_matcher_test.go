package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"sort"
	"strings"
)

// The choke-point matcher behind TestServedMuxIsTheOnlyMux
// (the authorization contract, reviews on #248). It is a syntax match over the
// package's non-test files: no type information, so it works offline and fast.
//
// The class it closes is "a route reaches the wire without passing through
// buildServedMux's recorder and the identity wrap". The ways to do that, and
// the rule that closes each:
//
//	mount on a mux       Handle / HandleFunc / NewServeMux / http.ServeMux (any
//	                     use, call or method value) only in the scopes of
//	                     mountScopeAllowed; http.DefaultServeMux nowhere
//	reach the inner mux  the servedMux.mux / .mounts fields only inside the
//	                     recorder methods and newServedMux — NOT buildServedMux
//	build a servedMux    a servedMux literal, new(servedMux), var x servedMux or
//	                     a conversion only inside newServedMux
//	change the recorder  the struct is pinned to its two fields, each method to
//	                     its statements (recorderShapeViolations)
//	serve another way    net/http Serve*/ListenAndServe*, any http.Server not
//	                     built as the one literal, a Handler that is not the
//	                     result of buildServedMux, and imports of the packages
//	                     that serve or register handlers (deniedImports)
//	rebind the result    any binding of the variable that holds the result of
//	                     buildServedMux other than that call
//	hide a selector      a dot import of net/http, or net/http imported twice
//	                     under different names: EVERY local name is tracked

// recvName returns the receiver's type name ("servedMux" for *servedMux), or "".
func recvName(fd *ast.FuncDecl) string {
	if fd.Recv == nil || len(fd.Recv.List) != 1 {
		return ""
	}
	t := fd.Recv.List[0].Type
	if star, ok := t.(*ast.StarExpr); ok {
		t = star.X
	}
	if id, ok := t.(*ast.Ident); ok {
		return id.Name
	}
	return ""
}

// mountScopeAllowed reports whether fd, in file name, is a place a pattern may
// be mounted: buildServedMux itself and the two servedMux methods that ARE the
// recorder. Nothing else in served.go gets the exemption — a helper beside
// buildServedMux that mounts on the returned mux is exactly the route the walk
// cannot see. newServedMux is NOT here: its body is pinned to a single return
// (constructorShapeViolations), so it has no Handle to exempt; the one name it
// owns, NewServeMux, is exempted separately through constructorScope.
func mountScopeAllowed(name string, fd *ast.FuncDecl) bool {
	if name != "served.go" {
		return false
	}
	if fd.Recv == nil {
		return fd.Name.Name == "buildServedMux"
	}
	return recvName(fd) == "servedMux" && (fd.Name.Name == "Handle" || fd.Name.Name == "HandleFunc")
}

// innerMuxScopeAllowed reports whether fd may touch servedMux's own fields
// (.mux, .mounts) or build a servedMux value: the constructor and the four
// recorder methods. buildServedMux is deliberately NOT here — inside it, only
// the recorder may reach the inner mux, or a mount there skips the record the
// walk reads.
func innerMuxScopeAllowed(name string, fd *ast.FuncDecl) bool {
	if name != "served.go" {
		return false
	}
	if fd.Recv == nil {
		return fd.Name.Name == "newServedMux"
	}
	return recvName(fd) == "servedMux"
}

// constructorScope reports whether fd is newServedMux in served.go: the one
// place a servedMux value is built and the one place besides the recorder that
// names http.ServeMux.
func constructorScope(name string, fd *ast.FuncDecl) bool {
	return name == "served.go" && fd.Recv == nil && fd.Name.Name == "newServedMux"
}

// deniedImports are packages whose presence in a non-test file of cmd/deskd
// means a second way to serve or to register a handler. The list is the known
// set; a package that serves through a mechanism not named here is outside what
// this check claims (see served.go).
var deniedImports = map[string]string{
	`"net/http/fcgi"`:              "serves handlers over FastCGI, outside the served mux",
	`"net/http/cgi"`:               "serves handlers over CGI, outside the served mux",
	`"net/http/httptest"`:          "starts servers (httptest.NewServer); tests only",
	`"net/http/pprof"`:             "registers /debug/pprof/ on http.DefaultServeMux",
	`"expvar"`:                     "registers /debug/vars on http.DefaultServeMux",
	`"golang.org/x/net/http2"`:     "serves connections itself (http2.Server.ServeConn)",
	`"golang.org/x/net/http2/h2c"`: "wraps a handler and serves h2c outside the reviewed path",
}

// paramNames returns the names of fd's parameters in order, "" for an unnamed one.
func paramNames(fd *ast.FuncDecl) []string {
	var out []string
	if fd.Type.Params == nil {
		return out
	}
	for _, f := range fd.Type.Params.List {
		if len(f.Names) == 0 {
			out = append(out, "")
		}
		for _, n := range f.Names {
			out = append(out, n.Name)
		}
	}
	return out
}

// recvIdent returns the receiver variable's name, or "".
func recvIdent(fd *ast.FuncDecl) string {
	if fd.Recv == nil || len(fd.Recv.List) != 1 || len(fd.Recv.List[0].Names) != 1 {
		return ""
	}
	return fd.Recv.List[0].Names[0].Name
}

func isIdent(e ast.Expr, name string) bool {
	id, ok := e.(*ast.Ident)
	return ok && id.Name == name
}

// isSelPath reports whether e is the selector chain base.f1.f2... exactly.
func isSelPath(e ast.Expr, base string, fields ...string) bool {
	for i := len(fields) - 1; i >= 0; i-- {
		sel, ok := e.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != fields[i] {
			return false
		}
		e = sel.X
	}
	return isIdent(e, base)
}

// stmtCall returns the call of an expression statement, or nil.
func stmtCall(s ast.Stmt) *ast.CallExpr {
	if es, ok := s.(*ast.ExprStmt); ok {
		if c, ok := es.X.(*ast.CallExpr); ok {
			return c
		}
	}
	return nil
}

// recorderShapeViolations pins the servedMux methods to their reviewed
// CONTENT, not just their statement count. The walk probes only the patterns
// the recorder holds, so a method that mounts without recording, records
// selectively (if p != "/v9/" { append }) or dispatches a path itself would
// serve a route the walk never sees.
//
//	Handle      — m.mux.Handle(p, h), then m.mounts = append(m.mounts, p)
//	HandleFunc  — m.Handle(p, http.HandlerFunc(f))
//	ServeHTTP   — m.mux.ServeHTTP(w, r)
//	Mounts      — return append([]string(nil), m.mounts...)  (a plain copy)
//
// where m, p, h, f, w, r are the receiver and parameters as declared. Any other
// method on servedMux is flagged.
func recorderShapeViolations(fset *token.FileSet, fd *ast.FuncDecl) []string {
	if recvName(fd) != "servedMux" {
		return nil
	}
	here := fset.Position(fd.Pos())
	bad := func(format string, a ...any) []string {
		return []string{fmt.Sprintf("%s: servedMux.%s %s", here, fd.Name.Name, fmt.Sprintf(format, a...))}
	}
	m := recvIdent(fd)
	ps := paramNames(fd)
	var body []ast.Stmt
	if fd.Body != nil {
		body = fd.Body.List
	}
	if m == "" {
		return bad("needs a named receiver so its body can be checked")
	}
	for _, p := range ps {
		if p == "" || p == "_" {
			return bad("needs named parameters so its body can be checked")
		}
	}
	switch fd.Name.Name {
	case "Handle":
		if len(ps) != 2 || len(body) != 2 {
			return bad("must be exactly: %s.mux.Handle(p, h); %s.mounts = append(%s.mounts, p)", m, m, m)
		}
		c := stmtCall(body[0])
		if c == nil || !isSelPath(c.Fun, m, "mux", "Handle") || len(c.Args) != 2 || !isIdent(c.Args[0], ps[0]) || !isIdent(c.Args[1], ps[1]) {
			return bad("statement 1 must be %s.mux.Handle(%s, %s)", m, ps[0], ps[1])
		}
		as, ok := body[1].(*ast.AssignStmt)
		if !ok || as.Tok != token.ASSIGN || len(as.Lhs) != 1 || len(as.Rhs) != 1 || !isSelPath(as.Lhs[0], m, "mounts") {
			return bad("statement 2 must be %s.mounts = append(%s.mounts, %s)", m, m, ps[0])
		}
		ap, ok := as.Rhs[0].(*ast.CallExpr)
		if !ok || !isIdent(ap.Fun, "append") || ap.Ellipsis != token.NoPos || len(ap.Args) != 2 || !isSelPath(ap.Args[0], m, "mounts") || !isIdent(ap.Args[1], ps[0]) {
			return bad("statement 2 must be %s.mounts = append(%s.mounts, %s) — every mount is recorded, unconditionally", m, m, ps[0])
		}
	case "HandleFunc":
		if len(ps) != 2 || len(body) != 1 {
			return bad("must be exactly: %s.Handle(p, http.HandlerFunc(f))", m)
		}
		c := stmtCall(body[0])
		if c == nil || !isSelPath(c.Fun, m, "Handle") || len(c.Args) != 2 || !isIdent(c.Args[0], ps[0]) {
			return bad("must delegate to %s.Handle(%s, ...)", m, ps[0])
		}
		conv, ok := c.Args[1].(*ast.CallExpr)
		if !ok || len(conv.Args) != 1 || !isIdent(conv.Args[0], ps[1]) {
			return bad("must pass http.HandlerFunc(%s) to %s.Handle", ps[1], m)
		}
		if sel, ok := conv.Fun.(*ast.SelectorExpr); !ok || sel.Sel.Name != "HandlerFunc" {
			return bad("must pass http.HandlerFunc(%s) to %s.Handle", ps[1], m)
		}
	case "ServeHTTP":
		if len(ps) != 2 || len(body) != 1 {
			return bad("must only delegate to %s.mux.ServeHTTP(w, r)", m)
		}
		c := stmtCall(body[0])
		if c == nil || !isSelPath(c.Fun, m, "mux", "ServeHTTP") || len(c.Args) != 2 || !isIdent(c.Args[0], ps[0]) || !isIdent(c.Args[1], ps[1]) {
			return bad("must only delegate to %s.mux.ServeHTTP(%s, %s)", m, ps[0], ps[1])
		}
	case "Mounts":
		const want = "must be exactly: return append([]string(nil), %s.mounts...) — a filtered or substituted list hides a mount from the walk"
		if len(ps) != 0 || len(body) != 1 {
			return bad(want, m)
		}
		rs, ok := body[0].(*ast.ReturnStmt)
		if !ok || len(rs.Results) != 1 {
			return bad(want, m)
		}
		ap, ok := rs.Results[0].(*ast.CallExpr)
		if !ok || !isIdent(ap.Fun, "append") || ap.Ellipsis == token.NoPos || len(ap.Args) != 2 || !isSelPath(ap.Args[1], m, "mounts") {
			return bad(want, m)
		}
		conv, ok := ap.Args[0].(*ast.CallExpr)
		if !ok || len(conv.Args) != 1 || !isIdent(conv.Args[0], "nil") {
			return bad(want, m)
		}
		if at, ok := conv.Fun.(*ast.ArrayType); !ok || at.Len != nil || !isIdent(at.Elt, "string") {
			return bad(want, m)
		}
	default:
		return bad("is not one of the reviewed recorder methods (Handle, HandleFunc, ServeHTTP, Mounts)")
	}
	return nil
}

// constructorShapeViolations pins newServedMux to its reviewed body, exactly:
//
//	return &servedMux{mux: http.NewServeMux()}
//
// The matcher exempts the constructor from the inner-mux rule (it has to name
// .mux to build the value), so without this pin a statement added there —
// m.mux.Handle("/v9/", ...) before the return — mounts a route that is never
// recorded and never walked.
func constructorShapeViolations(fset *token.FileSet, fd *ast.FuncDecl) []string {
	if fd.Recv != nil || fd.Name.Name != "newServedMux" {
		return nil
	}
	const want = "must be exactly: return &servedMux{mux: http.NewServeMux()} — any other statement can mount a route the recorder never sees"
	bad := []string{fmt.Sprintf("%s: newServedMux %s", fset.Position(fd.Pos()), want)}
	if len(paramNames(fd)) != 0 || fd.Body == nil || len(fd.Body.List) != 1 {
		return bad
	}
	rs, ok := fd.Body.List[0].(*ast.ReturnStmt)
	if !ok || len(rs.Results) != 1 {
		return bad
	}
	u, ok := rs.Results[0].(*ast.UnaryExpr)
	if !ok || u.Op != token.AND {
		return bad
	}
	cl, ok := u.X.(*ast.CompositeLit)
	if !ok || !isIdent(cl.Type, "servedMux") || len(cl.Elts) != 1 {
		return bad
	}
	kv, ok := cl.Elts[0].(*ast.KeyValueExpr)
	if !ok || !isIdent(kv.Key, "mux") {
		return bad
	}
	c, ok := kv.Value.(*ast.CallExpr)
	if !ok || !isSelPath(c.Fun, "http", "NewServeMux") || len(c.Args) != 0 {
		return bad
	}
	return nil
}

// servedMuxTypeViolations pins the servedMux struct to exactly its two named
// fields. An embedded *http.ServeMux would promote Handle and ServeHTTP past
// the recorder without a single selector the matcher could flag.
func servedMuxTypeViolations(fset *token.FileSet, ts *ast.TypeSpec) []string {
	st, ok := ts.Type.(*ast.StructType)
	if !ok {
		return []string{fmt.Sprintf("%s: servedMux must be a struct", fset.Position(ts.Pos()))}
	}
	var names []string
	for _, f := range st.Fields.List {
		if len(f.Names) == 0 {
			return []string{fmt.Sprintf("%s: servedMux embeds a type — an embedded mux promotes Handle past the recorder", fset.Position(f.Pos()))}
		}
		for _, n := range f.Names {
			names = append(names, n.Name)
		}
	}
	if strings.Join(names, ",") != "mux,mounts" {
		return []string{fmt.Sprintf("%s: servedMux fields are %v, want exactly [mux mounts]", fset.Position(ts.Pos()), names)}
	}
	return nil
}

// httpImportNames returns EVERY local name file f gives net/http ("http" when
// unaliased), and whether f dot-imports it. A package may legally be imported
// twice under different names, so a single name is not enough: the matcher must
// treat a selector on any of them as net/http's. "_" is skipped (no selectors);
// a dot import is flagged by the caller.
func httpImportNames(f *ast.File) map[string]bool {
	names := map[string]bool{}
	for _, im := range f.Imports {
		if im.Path.Value != `"net/http"` {
			continue
		}
		switch {
		case im.Name == nil:
			names["http"] = true
		case im.Name.Name != "_" && im.Name.Name != ".":
			names[im.Name.Name] = true
		}
	}
	return names
}

// isBuildServedMuxCall reports whether e is a call of buildServedMux(...).
func isBuildServedMuxCall(e ast.Expr) bool {
	call, ok := e.(*ast.CallExpr)
	return ok && isIdent(call.Fun, "buildServedMux")
}

// mountSiteViolations is the choke-point check behind TestServedMuxIsTheOnlyMux.
// See the table at the top of this file for the class; each rule is exercised
// by a positive control in that test and a case in mutate-route-walk.sh.
func mountSiteViolations(files map[string]string) []string {
	var bad []string
	fset := token.NewFileSet()
	served := map[string]bool{} // identifiers assigned from buildServedMux(...)
	var servers []*ast.CompositeLit
	literalTypes := map[ast.Node]bool{} // the http.Server selector of each composite literal
	var serverRefs []*ast.SelectorExpr  // every http.Server selector
	type handlerAssign struct {
		pos token.Pos
		rhs ast.Expr // nil when the assignment is not one-to-one
	}
	var handlerAssigns []handlerAssign
	type binding struct {
		pos       token.Pos
		name      string
		fromBuild bool
	}
	var bindings []binding
	parsed := map[string]*ast.File{}
	for name, src := range files {
		f, err := parser.ParseFile(fset, name, src, 0)
		if err != nil {
			bad = append(bad, fmt.Sprintf("%s: could not parse: %v", name, err))
			continue
		}
		parsed[name] = f
	}
	flag := func(pos token.Pos, format string, a ...any) {
		bad = append(bad, fmt.Sprintf("%s: %s", fset.Position(pos), fmt.Sprintf(format, a...)))
	}
	for name, f := range parsed {
		httpNames := httpImportNames(f)
		for _, im := range f.Imports {
			if why, denied := deniedImports[im.Path.Value]; denied {
				flag(im.Pos(), "import of %s — %s", im.Path.Value, why)
			}
			if im.Path.Value == `"net/http"` && im.Name != nil && im.Name.Name == "." {
				flag(im.Pos(), "dot-import of net/http hides http.X selectors from the choke-point check")
			}
		}
		for _, decl := range f.Decls {
			scopeOK, innerOK, ctorOK, typeDeclOK := false, false, false, false
			switch d := decl.(type) {
			case *ast.FuncDecl:
				scopeOK = mountScopeAllowed(name, d)
				innerOK = innerMuxScopeAllowed(name, d)
				ctorOK = constructorScope(name, d)
				if name == "served.go" {
					bad = append(bad, recorderShapeViolations(fset, d)...)
					bad = append(bad, constructorShapeViolations(fset, d)...)
				} else if recvName(d) == "servedMux" {
					flag(d.Pos(), "servedMux method %s declared outside served.go", d.Name.Name)
				}
			case *ast.GenDecl:
				for _, sp := range d.Specs {
					if ts, ok := sp.(*ast.TypeSpec); ok && ts.Name.Name == "servedMux" {
						if name == "served.go" {
							typeDeclOK = true
							bad = append(bad, servedMuxTypeViolations(fset, ts)...)
						} else {
							flag(ts.Pos(), "servedMux declared outside served.go")
						}
					}
				}
			}
			ast.Inspect(decl, func(n ast.Node) bool {
				switch n := n.(type) {
				case *ast.AssignStmt:
					if len(n.Rhs) == 1 && isBuildServedMuxCall(n.Rhs[0]) {
						if id, ok := n.Lhs[0].(*ast.Ident); ok {
							served[id.Name] = true
						}
					}
					for i, l := range n.Lhs {
						if id, ok := l.(*ast.Ident); ok && id.Name != "_" {
							bindings = append(bindings, binding{id.Pos(), id.Name, i == 0 && len(n.Rhs) == 1 && isBuildServedMuxCall(n.Rhs[0])})
						}
						if sel, ok := l.(*ast.SelectorExpr); ok && sel.Sel.Name == "Handler" {
							var rhs ast.Expr
							if len(n.Lhs) == len(n.Rhs) {
								rhs = n.Rhs[i]
							}
							handlerAssigns = append(handlerAssigns, handlerAssign{pos: n.Pos(), rhs: rhs})
						}
					}
				case *ast.ValueSpec:
					for i, id := range n.Names {
						if id.Name == "_" {
							continue
						}
						from := len(n.Values) == 1 && i == 0 && isBuildServedMuxCall(n.Values[0])
						if from {
							served[id.Name] = true
						}
						bindings = append(bindings, binding{id.Pos(), id.Name, from})
					}
					if isIdent(n.Type, "servedMux") {
						flag(n.Pos(), "a servedMux value declared with var — a mux this check cannot tie to buildServedMux")
					}
				case *ast.RangeStmt:
					for _, e := range []ast.Expr{n.Key, n.Value} {
						if id, ok := e.(*ast.Ident); ok && id.Name != "_" {
							bindings = append(bindings, binding{id.Pos(), id.Name, false})
						}
					}
				case *ast.FuncType:
					for _, fl := range []*ast.FieldList{n.Params, n.Results} {
						if fl == nil {
							continue
						}
						for _, fld := range fl.List {
							for _, id := range fld.Names {
								bindings = append(bindings, binding{id.Pos(), id.Name, false})
							}
							if isIdent(fld.Type, "servedMux") {
								flag(fld.Pos(), "a servedMux passed or returned by value — a copy the recorder does not own")
							}
						}
					}
				case *ast.CallExpr:
					if isIdent(n.Fun, "servedMux") {
						flag(n.Pos(), "conversion to servedMux builds a mux outside newServedMux")
					}
					if isIdent(n.Fun, "new") && len(n.Args) == 1 && isIdent(n.Args[0], "servedMux") && !ctorOK {
						flag(n.Pos(), "new(servedMux) outside newServedMux — a mux with no recorder behind it")
					}
				case *ast.SelectorExpr:
					pkg, _ := n.X.(*ast.Ident)
					isHTTP := pkg != nil && httpNames[pkg.Name]
					switch {
					case isHTTP && (n.Sel.Name == "Serve" || n.Sel.Name == "ServeTLS" ||
						n.Sel.Name == "ListenAndServe" || n.Sel.Name == "ListenAndServeTLS"):
						flag(n.Pos(), "http.%s serves a handler other than buildServedMux's", n.Sel.Name)
					case isHTTP && n.Sel.Name == "Server":
						serverRefs = append(serverRefs, n)
					case isHTTP && n.Sel.Name == "DefaultServeMux":
						flag(n.Pos(), "http.DefaultServeMux — a mux the served handler can be swapped for, or that a blank import registers on")
					case isHTTP && n.Sel.Name == "ServeMux" && !ctorOK && !typeDeclOK:
						flag(n.Pos(), "http.ServeMux outside the servedMux type and its constructor — a mux built without the recorder")
					case (n.Sel.Name == "mux" || n.Sel.Name == "mounts") && !innerOK:
						flag(n.Pos(), ".%s outside newServedMux and the servedMux recorder methods — reaching the inner mux skips the record the walk reads", n.Sel.Name)
					case !scopeOK && (n.Sel.Name == "Handle" || n.Sel.Name == "HandleFunc" || (n.Sel.Name == "NewServeMux" && !ctorOK)):
						flag(n.Pos(), "%s outside buildServedMux — mount only there, where the route-walk sees it", n.Sel.Name)
					}
				case *ast.CompositeLit:
					if sel, ok := n.Type.(*ast.SelectorExpr); ok && sel.Sel.Name == "Server" {
						if pkg, ok := sel.X.(*ast.Ident); ok && httpNames[pkg.Name] {
							servers = append(servers, n)
							literalTypes[sel] = true
						}
					}
					if isIdent(n.Type, "servedMux") && !ctorOK {
						flag(n.Pos(), "a servedMux literal outside newServedMux — a mux with no recorder behind it")
					}
				}
				return true
			})
		}
	}
	if len(servers) == 0 {
		bad = append(bad, "no http.Server literal found — the check is not looking at the binary's server")
	}
	if len(servers) > 1 {
		for _, s := range servers[1:] {
			flag(s.Pos(), "a second http.Server literal — the binary serves exactly one, whose Handler is buildServedMux's result")
		}
	}
	for _, r := range serverRefs {
		if !literalTypes[r] {
			flag(r.Pos(), "http.Server used other than as a composite literal (new, var, field) — a server this check cannot tie to buildServedMux")
		}
	}
	for _, s := range servers {
		ok := false
		for _, e := range s.Elts {
			kv, isKV := e.(*ast.KeyValueExpr)
			if !isKV {
				continue
			}
			if k, _ := kv.Key.(*ast.Ident); k != nil && k.Name == "Handler" {
				if v, _ := kv.Value.(*ast.Ident); v != nil && served[v.Name] {
					ok = true
				}
			}
		}
		if !ok {
			flag(s.Pos(), "http.Server's Handler is not the result of buildServedMux")
		}
	}
	for _, a := range handlerAssigns {
		if v, _ := a.rhs.(*ast.Ident); v == nil || !served[v.Name] {
			flag(a.pos, "assignment to a Handler field that is not the result of buildServedMux")
		}
	}
	// The variable holding buildServedMux's result is tracked by NAME, so every
	// binding of that name must be that call. Without this, `served = v1API`
	// (or a shadowing `served := v1API` beside a second server) would satisfy
	// the Handler check above while serving something else.
	for _, b := range bindings {
		if served[b.name] && !b.fromBuild {
			flag(b.pos, "%q is bound to something other than buildServedMux's result — the variable the Handler checks trust", b.name)
		}
	}
	sort.Strings(bad)
	return bad
}
