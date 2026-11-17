package deskkit

// forgebodycap_guard_test.go — the source guard for the forge body cap. It scans the shipped
// files of the roots below (forgeBodyGuardRoots) for a response body read with io.ReadAll,
// copied, or decoded (bare, or wrapped in an io.LimitReader that silently truncates) instead of
// going through ReadCappedForgeBody / ReadForgeBodyUpTo, for a body read whose error is
// DISCARDED, and for a truncating reader handed to a capped reader (the capped reader then sees
// a body that already ends at the wrapper's bound, so its over-cap refusal can never fire). The
// planted fixture proves the scanner sees each shape, so a scanner that sees nothing cannot
// certify everything. scanForgeBodyReads states exactly which functions are scanned and which
// forms are recognised.
//
// What it does NOT claim: the guard is syntactic (go/ast, no type information) and NOT
// exhaustive. A read in a file outside the roots is not scanned at all (forgeBodyGuardGaps
// lists the known ones), and some ways of reaching a body through indirection are not seen;
// those known blind spots are tracked outside this file. A clean run means "no shape this guard
// recognises", never "every body read in the tree is capped".

import (
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

// forgeBodyGuardRoots are the trees whose forge body reads must go through the cap: the forge
// library, the credential paths, the lock-holding write verbs, and the commands that read
// the forge directly.
var forgeBodyGuardRoots = []string{
	"internal/deskkit",
	"internal/credential",
	"cmd/deskevidence",
	"cmd/deskpost",
	"cmd/deskrelease",
	"cmd/deskadvisory",
	"cmd/deskinbox",
	"cmd/deskgitlab",
	"cmd/desktoken",
	"cmd/deskfleet",
	"cmd/deskapps",
	"internal/forgelabels",
	// The release-pin checker is its own module (tools/pairedversions) and cannot import
	// deskkit; it keeps a local readCapped with the same refuse-over-limit rule.
	"../pairedversions",
}

// forgeBodyNoTransportCap are the roots whose HTTP client has no transport-level body cap, so
// the guard's one blind spot there (a body handed to a helper that takes a plain io.Reader)
// is bounded by nothing. In these roots every response body handed to any call other than a
// capped reader is a hit (uncapped-body-handoff).
var forgeBodyNoTransportCap = map[string]string{
	"../pairedversions": "its http.Client is a plain client with a timeout; every read is a direct readCapped call",
}

// forgeBodyReadPermits are the readers that keep their own, narrower bound AND refuse an
// over-bound body (they read limit+1 and check). Key: "<root>/<file>:<func> <kind>" — the
// hit kind is part of the key, and a permit covers ONE hit: a second read added to the same
// function is still reported, whether it is of another kind or of the permitted kind.
var forgeBodyReadPermits = map[string]string{
	"internal/deskkit/compositionsource.go:httpFetchBounded uncapped-body-read": "reads limit+1 and refuses a body over its own limit",
	"internal/deskkit/forge_github.go:GitHubForge.runLog uncapped-body-copy": "streams into runLogSink, which refuses an archive past runLogArchiveCap; " +
		"the client transport caps the same body one byte past that bound",
	"cmd/deskfleet/pushrotate.go:pushRun.provePush uncapped-body-copy": "drains a status-only probe answer to io.Discard under maxResponseBytes; " +
		"the body is never read as an answer",
}

// bodyReadFuncs are the calls whose error result is a body read's error.
var bodyReadFuncs = map[string]bool{
	"ReadCappedForgeBody": true, "ReadForgeBodyUpTo": true, "readCapped": true,
	"readCappedResponse": true, "readCappedN": true, "ReadForgeBody": true,
}

// regression: #6730
func TestForgeBodyReadsCapped(t *testing.T) {
	var found []string
	for _, r := range forgeBodyGuardRoots {
		hits, err := scanForgeBodyReads(filepath.Join(deskTreeRoot, r), r)
		if err != nil {
			t.Fatalf("the forge-body guard could not scan %s: %v — could-not-check, NOT clean", r, err)
		}
		found = append(found, hits...)
	}
	bad, unused := unpermittedBodyReads(found, forgeBodyReadPermits)
	for _, h := range bad {
		t.Errorf("%s — read forge response bodies through deskkit.ReadCappedForgeBody (or ReadForgeBodyUpTo "+
			"for a narrower bound), hand it the response body itself (never a truncating wrapper), and never "+
			"discard the read error: an uncapped or truncating read buffers a hostile body whole or hands back "+
			"a prefix as the answer (#6730)", h)
	}
	for _, key := range unused {
		t.Errorf("permit %s matches no read any more — drop it", key)
	}
}

// unpermittedBodyReads splits the guard's hits into the ones no permit covers and the permits
// no hit used. A permit covers exactly one "<file>:<func> <kind>" hit — the first; every
// further hit with the same key is reported, so two same-kind reads under one permit fail.
func unpermittedBodyReads(hits []string, permits map[string]string) (bad, unused []string) {
	used := map[string]bool{}
	for _, h := range hits {
		if _, ok := permits[h]; ok && !used[h] {
			used[h] = true
			continue
		}
		bad = append(bad, h)
	}
	for key := range permits {
		if !used[key] {
			unused = append(unused, key)
		}
	}
	sort.Strings(unused)
	return bad, unused
}

// A permit granted for one read in a function does not cover a second read of another kind in
// the same function: planted.go's permitThenRead drains a status probe (the permitted copy)
// and then reads the body whole, and only the drain may pass.
//
// regression: #6755
func TestForgeBodyPermitIsPerKind(t *testing.T) {
	hits, err := scanForgeBodyReads(filepath.Join("testdata", "forgebodycap"), "planted")
	if err != nil {
		t.Fatal(err)
	}
	permits := map[string]string{"planted/planted.go:permitThenRead uncapped-body-copy": "the status-probe drain"}
	bad, unused := unpermittedBodyReads(hits, permits)
	if len(unused) != 0 {
		t.Fatalf("the permit matched no hit: %v", unused)
	}
	var seen bool
	for _, h := range bad {
		switch h {
		case "planted/planted.go:permitThenRead uncapped-body-read":
			seen = true
		case "planted/planted.go:permitThenRead uncapped-body-copy":
			t.Errorf("the permitted drain was still reported: %s", h)
		}
	}
	if !seen {
		t.Errorf("a second read in a permitted function passed under the drain's permit; reported:\n%s",
			strings.Join(bad, "\n"))
	}
}

// A permit covers ONE hit: planted.go's permitTwice drains the body twice under one permit,
// and the second drain must still be reported.
//
// regression: #6755
func TestForgeBodyPermitCoversOne(t *testing.T) {
	hits, err := scanForgeBodyReads(filepath.Join("testdata", "forgebodycap"), "planted")
	if err != nil {
		t.Fatal(err)
	}
	const key = "planted/planted.go:permitTwice uncapped-body-copy"
	bad, unused := unpermittedBodyReads(hits, map[string]string{key: "one drain"})
	if len(unused) != 0 {
		t.Fatalf("the permit matched no hit: %v", unused)
	}
	n := 0
	for _, h := range bad {
		if h == key {
			n++
		}
	}
	if n != 1 {
		t.Errorf("permitTwice's two drains under one permit reported %d times, want 1 (the second drain)", n)
	}
}

// The planted fixture holds one of each shape the guard must catch, and two it must not
// (cappedRead, wrappedCapped).
//
// regression: #6730
func TestForgeBodyGuardSeesPlanted(t *testing.T) {
	hits, err := scanForgeBodyReads(filepath.Join("testdata", "forgebodycap"), "planted")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"planted/planted.go:aliasedRead uncapped-body-read",
		"planted/planted.go:bodyAliasRead uncapped-body-read",
		"planted/planted.go:copyRead uncapped-body-copy",
		"planted/planted.go:decodeRead uncapped-body-decode",
		"planted/planted.go:discardedCapped discarded-read-error",
		"planted/planted.go:discardedRead discarded-read-error",
		"planted/planted.go:discardedRead uncapped-body-read",
		"planted/planted.go:permitThenRead uncapped-body-copy",
		"planted/planted.go:permitThenRead uncapped-body-read",
		"planted/planted.go:permitTwice uncapped-body-copy",
		"planted/planted.go:permitTwice uncapped-body-copy",
		"planted/planted.go:reassignedCapped truncated-capped-read",
		"planted/planted.go:teeTruncCapped truncated-capped-read",
		"planted/planted.go:truncatedAliasCapped truncated-capped-read",
		"planted/planted.go:truncatedCapped truncated-capped-read",
		"planted/planted.go:truncatedLimitedCapped truncated-capped-read",
		"planted/planted.go:truncatingRead uncapped-body-read",
		"planted/planted.go:wrappedAliasCapped truncated-capped-read",
		"planted/planted.go:wrappedTruncCapped truncated-capped-read",
		"planted/sub/nested.go:nestedRead uncapped-body-read",
	}
	sort.Strings(hits)
	if strings.Join(hits, "\n") != strings.Join(want, "\n") {
		t.Errorf("planted hits:\n%s\nwant:\n%s", strings.Join(hits, "\n"), strings.Join(want, "\n"))
	}
}

// regression: #6775
func TestForgeBodyGuardSeesGaps(t *testing.T) {
	hits, err := scanForgeBodyReads(filepath.Join("testdata", "forgebodygaps"), "gaps")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"gaps/gaps.go:a.drain uncapped-body-copy",
		// aliasByName reads a byte slice; the name-not-scope match reports it (fail-closed).
		"gaps/gaps.go:aliasByName uncapped-body-read",
		"gaps/gaps.go:b.drain uncapped-body-copy",
		"gaps/gaps.go:bodyReassignCapped truncated-capped-read",
		"gaps/gaps.go:helper2TruncCapped truncated-capped-read",
		"gaps/gaps.go:helperTruncCapped truncated-capped-read",
		"gaps/gaps.go:loopHopCapped truncated-capped-read",
		"gaps/gaps.go:pkgLitRead uncapped-body-read",
		"gaps/gaps.go:pkgTable uncapped-body-read",
		"gaps/gaps.go:twoHopCapped truncated-capped-read",
	}
	sort.Strings(hits)
	if strings.Join(hits, "\n") != strings.Join(want, "\n") {
		t.Errorf("gap plant hits:\n%s\nwant:\n%s", strings.Join(hits, "\n"), strings.Join(want, "\n"))
	}
}

// regression: #6775
func TestForgeBodyHandoffStrict(t *testing.T) {
	dir := filepath.Join("testdata", "forgebodyhandoff")
	hits, err := scanForgeBodyReadsIn(dir, "handoff", true)
	if err != nil {
		t.Fatal(err)
	}
	want := "handoff/handoff.go:handedOff uncapped-body-handoff"
	if strings.Join(hits, "\n") != want {
		t.Errorf("strict handoff hits:\n%s\nwant:\n%s", strings.Join(hits, "\n"), want)
	}
	// Outside a no-transport-cap root the handoff shape is off.
	if hits, err := scanForgeBodyReadsIn(dir, "handoff", false); err != nil || len(hits) != 0 {
		t.Errorf("non-strict handoff hits = %v, %v; want none", hits, err)
	}
	for root := range forgeBodyNoTransportCap {
		if !slicesContains(forgeBodyGuardRoots, root) {
			t.Errorf("forgeBodyNoTransportCap names %s, which is not a guard root", root)
		}
	}
}

// forgeBodyGuardGaps are trees, relative to deskTreeRoot, that read forge response bodies but
// are NOT guard roots: their reads are not held to the cap by this guard. Each is kept out by
// decision — widening a root to a client with no transport cap means rewriting its reads
// first — and listed so the gap is visible. TestForgeBodyGapsStillOpen fails when one no longer
// shows a hit, so a gap that has been closed moves into forgeBodyGuardRoots instead of staying
// listed here.
var forgeBodyGuardGaps = []string{
	"../../deskd/internal/adapter",
	"../../deskd/internal/answer",
	"../../deskd/internal/comment",
	"../../deskd/internal/contract",
	"../../deskd/internal/identity",
	"../../deskd/internal/mcp",
	"../../statusgen/ghfetch.go",
	"../../statusgen/claimdecay_github.go",
	"../../statusgen/claimdecay_gitlab.go",
	"cmd/deskinstall",
}

// regression: #6775
func TestForgeBodyGapsStillOpen(t *testing.T) {
	for _, g := range forgeBodyGuardGaps {
		hits, err := scanForgeBodyReads(filepath.Join(deskTreeRoot, g), g)
		if err != nil {
			t.Errorf("the forge-body guard could not scan gap %s: %v — could-not-check", g, err)
			continue
		}
		if len(hits) == 0 {
			t.Errorf("gap %s shows no uncapped body read any more — move it into forgeBodyGuardRoots "+
				"and drop it from forgeBodyGuardGaps", g)
		}
	}
}

func slicesContains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

// scanForgeBodyReads parses every shipped .go file under path, subdirectories included (no
// _test.go, no testdata), and returns one "<label>/<file>:<func> <kind>" per offending read.
// path may also name one .go file, which is scanned alone. <func> is the function's name, a
// method's receiver type and name (T.name), or, for a function literal outside every function
// declaration, the package-level name it is bound to.
//
// It is a syntactic scan without type information, and it sees exactly these shapes:
//   - io.ReadAll, json.NewDecoder or io.Copy/io.CopyN whose reader is a response body, named
//     directly (resp.Body) or through a local alias assigned from one (r := resp.Body);
//   - a discarded body-read error;
//   - a truncating reader handed to one of the capped readers (truncated-capped-read): the
//     capped reader then sees a body that already ends at the wrapper's bound, so its
//     over-cap refusal can never fire. See buildsTruncating for the forms it recognises;
//   - in a root listed in forgeBodyNoTransportCap, a response body handed to any call other
//     than a capped reader (uncapped-body-handoff).
//
// Every function declaration is scanned, with the function literals inside it, and so is
// every function literal outside all of them (a package-level func value, or a func field in
// a package-level composite literal).
//
// Names are matched by name, not by scope or order: a name assigned a body (or a truncating
// reader) anywhere in a function makes every use of that name in the function count, and a
// helper's name matches any call of that name in its package. That can only add hits, so it
// errs toward failing the build, and it is kept that way by decision.
//
// A body handed to a helper that takes a plain io.Reader is out of its sight in the other
// roots. The forge library's GitHub and GitLab clients bound those reads with their transport
// cap (forgebodytransport.go, guarded by TestForgeClientsCapped).
func scanForgeBodyReads(path, label string) ([]string, error) {
	_, strict := forgeBodyNoTransportCap[label]
	return scanForgeBodyReadsIn(path, label, strict)
}

// scanForgeBodyReadsIn is scanForgeBodyReads with the handoff shape switched on or off.
func scanForgeBodyReadsIn(path, label string, strict bool) ([]string, error) {
	type parsed struct {
		f   *ast.File
		rel string
	}
	byDir := map[string][]parsed{}
	var dirs []string
	fset := token.NewFileSet()
	files := 0
	add := func(file, rel string) error {
		f, err := parser.ParseFile(fset, file, nil, parser.SkipObjectResolution)
		if err != nil {
			return err
		}
		files++
		d := filepath.Dir(file)
		if _, seen := byDir[d]; !seen {
			dirs = append(dirs, d)
		}
		byDir[d] = append(byDir[d], parsed{f, label + "/" + filepath.ToSlash(rel)})
		return nil
	}
	st, err := os.Stat(path)
	switch {
	case err != nil:
		return nil, err
	case !st.IsDir():
		err = add(path, filepath.Base(path))
	default:
		err = filepath.WalkDir(path, func(p string, e os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			name := e.Name()
			if e.IsDir() {
				if name == "testdata" || (p != path && strings.HasPrefix(name, ".")) {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
				return nil
			}
			rel, _ := filepath.Rel(path, p)
			return add(p, rel)
		})
	}
	if err != nil {
		return nil, err
	}
	if files == 0 {
		return nil, fmt.Errorf("no Go source under %s — a guard that scans nothing certifies nothing", path)
	}
	var out []string
	for _, d := range dirs {
		pkg := map[*ast.File]string{}
		for _, p := range byDir[d] {
			pkg[p.f] = p.rel
		}
		helpers := truncatingHelpers(pkg)
		for _, p := range byDir[d] {
			out = append(out, scanBodyReadFile(p.f, p.rel, helpers, strict)...)
		}
	}
	return out, nil
}

// funcBodies yields every function body in f the guard scans, with the name it reports it
// under: each function declaration (a method as T.name), and each function literal outside
// every declaration, named for the package-level name it is bound to.
func funcBodies(f *ast.File, yield func(name string, body *ast.BlockStmt)) {
	for _, d := range f.Decls {
		switch x := d.(type) {
		case *ast.FuncDecl:
			if x.Body != nil {
				yield(funcDeclName(x), x.Body)
			}
		case *ast.GenDecl:
			for _, s := range x.Specs {
				vs, ok := s.(*ast.ValueSpec)
				if !ok {
					continue
				}
				for i, v := range vs.Values {
					name := vs.Names[0].Name
					if i < len(vs.Names) {
						name = vs.Names[i].Name
					}
					ast.Inspect(v, func(n ast.Node) bool {
						lit, ok := n.(*ast.FuncLit)
						if !ok {
							return true
						}
						yield(name, lit.Body)
						return false // the outer literal's scan covers literals inside it
					})
				}
			}
		}
	}
}

// funcDeclName is a function's name, or T.name for a method on T or *T.
func funcDeclName(fn *ast.FuncDecl) string {
	if fn.Recv == nil || len(fn.Recv.List) == 0 {
		return fn.Name.Name
	}
	t := fn.Recv.List[0].Type
	for {
		switch x := t.(type) {
		case *ast.StarExpr:
			t = x.X
			continue
		case *ast.IndexExpr:
			t = x.X
			continue
		case *ast.IndexListExpr:
			t = x.X
			continue
		case *ast.ParenExpr:
			t = x.X
			continue
		}
		break
	}
	if id, ok := t.(*ast.Ident); ok {
		return id.Name + "." + fn.Name.Name
	}
	return fn.Name.Name
}

func scanBodyReadFile(f *ast.File, label string, helpers map[string]bool, strict bool) []string {
	ioName := importName(f, "io", "io")
	jsonName := importName(f, "encoding/json", "json")
	var out []string
	funcBodies(f, func(name string, body *ast.BlockStmt) {
		where := fmt.Sprintf("%s:%s", label, name)
		aliases := bodyAliases(body)
		truncating := truncatingAliases(body, ioName, helpers)
		isTruncating := func(e ast.Expr) bool { return buildsTruncating(e, ioName, truncating, helpers) }
		isBody := func(e ast.Expr) bool {
			if mentionsBody(e) {
				return true
			}
			id, ok := e.(*ast.Ident)
			return ok && aliases[id.Name]
		}
		ast.Inspect(body, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.CallExpr:
				switch {
				case ioName != "" && isPkgCall(x, ioName, "ReadAll") && len(x.Args) == 1 && isBody(x.Args[0]):
					out = append(out, where+" uncapped-body-read")
				case ioName != "" && (isPkgCall(x, ioName, "Copy") || isPkgCall(x, ioName, "CopyN")) && len(x.Args) >= 2 && isBody(x.Args[1]):
					out = append(out, where+" uncapped-body-copy")
				case jsonName != "" && isPkgCall(x, jsonName, "NewDecoder") && len(x.Args) == 1 && isBody(x.Args[0]):
					out = append(out, where+" uncapped-body-decode")
				case isCappedReadCall(x) && len(x.Args) >= 1 && isTruncating(x.Args[0]):
					out = append(out, where+" truncated-capped-read")
				case strict && !isCappedReadCall(x) && anyArg(x, isBody):
					out = append(out, where+" uncapped-body-handoff")
				}
			case *ast.AssignStmt:
				if len(x.Lhs) == 2 && len(x.Rhs) == 1 && isBlank(x.Lhs[1]) {
					if c, ok := x.Rhs[0].(*ast.CallExpr); ok && isBodyReadCall(c, ioName) {
						out = append(out, where+" discarded-read-error")
					}
				}
			}
			return true
		})
	})
	return out
}

// anyArg reports whether any argument of c satisfies pred.
func anyArg(c *ast.CallExpr, pred func(ast.Expr) bool) bool {
	for _, a := range c.Args {
		if pred(a) {
			return true
		}
	}
	return false
}

// bodyAliases: local names assigned a bare response body (r := resp.Body, var r io.Reader =
// resp.Body). A call result (a capped reader, a wrapper) is not an alias.
func bodyAliases(body *ast.BlockStmt) map[string]bool {
	out := map[string]bool{}
	bare := func(e ast.Expr) bool {
		s, ok := e.(*ast.SelectorExpr)
		return ok && s.Sel.Name == "Body"
	}
	ast.Inspect(body, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.AssignStmt:
			if len(x.Lhs) == len(x.Rhs) {
				for i, r := range x.Rhs {
					if id, ok := x.Lhs[i].(*ast.Ident); ok && bare(r) {
						out[id.Name] = true
					}
				}
			}
		case *ast.ValueSpec:
			if len(x.Names) == len(x.Values) {
				for i, v := range x.Values {
					if bare(v) {
						out[x.Names[i].Name] = true
					}
				}
			}
		}
		return true
	})
	return out
}

// truncatingReader reports whether e builds a reader that ENDS at a bound instead of
// refusing past it: io.LimitReader(...), an io.LimitedReader literal (or its address), or a
// call of a helper named in helpers (a function of the same package that returns one).
func truncatingReader(e ast.Expr, ioName string, helpers map[string]bool) bool {
	if u, ok := e.(*ast.UnaryExpr); ok && u.Op == token.AND {
		e = u.X
	}
	switch x := e.(type) {
	case *ast.CallExpr:
		if ioName != "" && isPkgCall(x, ioName, "LimitReader") {
			return true
		}
		switch f := x.Fun.(type) {
		case *ast.Ident:
			return helpers[f.Name]
		case *ast.SelectorExpr:
			return helpers[f.Sel.Name]
		}
	case *ast.CompositeLit:
		sel, ok := x.Type.(*ast.SelectorExpr)
		if ioName == "" || !ok || sel.Sel.Name != "LimitedReader" {
			return false
		}
		id, ok := sel.X.(*ast.Ident)
		return ok && id.Name == ioName
	}
	return false
}

// buildsTruncating reports whether e is, or holds anywhere inside it, a truncating reader
// (truncatingReader) or a name in aliases: a local (lr) or a field path (resp.Body). The
// recognised forms, as the capped reader's reader argument:
//   - an io.LimitReader(...) call or an io.LimitedReader literal (or its address), whether it
//     is the whole argument or sits anywhere inside it (bufio.NewReader(io.LimitReader(...)),
//     io.TeeReader(io.LimitReader(...), w));
//   - a call of a helper in the same package whose return statement returns such an
//     expression, followed through helpers that return a call of another (truncatingHelpers);
//   - a local name, or a field path such as resp.Body, assigned such an expression in the same
//     function by :=, var or a plain = (a reassignment such as r = io.LimitReader(r, n), or
//     resp.Body = io.NopCloser(io.LimitReader(resp.Body, n)), included), followed through any
//     number of further assignments (truncatingAliases), named alone or inside the argument.
//
// The Sel of a selector is a field or method name, never a local, so a selector that is not
// itself an alias has only its X searched.
func buildsTruncating(e ast.Expr, ioName string, aliases, helpers map[string]bool) bool {
	seen := false
	ast.Inspect(e, func(n ast.Node) bool {
		switch x := n.(type) {
		case nil:
			return false
		case *ast.SelectorExpr:
			if k := selectorKey(x); k != "" && aliases[k] {
				seen = true
			}
			seen = seen || buildsTruncating(x.X, ioName, aliases, helpers)
			return false
		case *ast.Ident:
			if aliases[x.Name] {
				seen = true
			}
		case ast.Expr:
			if truncatingReader(x, ioName, helpers) {
				seen = true
			}
		}
		return !seen
	})
	return seen
}

// selectorKey spells a name or a field path (resp.Body, c.resp.Body) as written, or "" for any
// other expression.
func selectorKey(e ast.Expr) string {
	switch x := e.(type) {
	case *ast.Ident:
		return x.Name
	case *ast.SelectorExpr:
		if k := selectorKey(x.X); k != "" {
			return k + "." + x.Sel.Name
		}
	}
	return ""
}

// truncatingAliases: the names and field paths assigned, anywhere in body, an expression that
// is or holds a truncating reader (lr := io.LimitReader(...), br := bufio.NewReader(lr),
// resp.Body = io.NopCloser(io.LimitReader(resp.Body, n))). It repeats until nothing new is
// found, so a name assigned another such name (r2 := lr) is followed however many hops deep.
func truncatingAliases(body *ast.BlockStmt, ioName string, helpers map[string]bool) map[string]bool {
	out := map[string]bool{}
	for grew := true; grew; {
		grew = false
		mark := func(k string, v ast.Expr) {
			if k != "" && !out[k] && buildsTruncating(v, ioName, out, helpers) {
				out[k] = true
				grew = true
			}
		}
		ast.Inspect(body, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.AssignStmt:
				if len(x.Lhs) == len(x.Rhs) {
					for i, r := range x.Rhs {
						mark(selectorKey(x.Lhs[i]), r)
					}
				}
			case *ast.ValueSpec:
				if len(x.Names) == len(x.Values) {
					for i, v := range x.Values {
						mark(x.Names[i].Name, v)
					}
				}
			}
			return true
		})
	}
	return out
}

// truncatingHelpers names the functions of one package (files maps each parsed file to its
// label) that return a truncating reader: a return statement in the function's own body (not
// in a literal inside it) whose result is or holds one, by buildsTruncating with the function's
// own aliases. It repeats until nothing new is found, so a helper returning another helper's
// call is followed. A method is named by its method name alone, and a function literal bound
// at package level by the name it is bound to.
func truncatingHelpers(files map[*ast.File]string) map[string]bool {
	out := map[string]bool{}
	for grew := true; grew; {
		grew = false
		for f := range files {
			ioName := importName(f, "io", "io")
			funcBodies(f, func(name string, body *ast.BlockStmt) {
				name = name[strings.LastIndex(name, ".")+1:]
				if out[name] {
					return
				}
				local := truncatingAliases(body, ioName, out)
				ast.Inspect(body, func(n ast.Node) bool {
					switch x := n.(type) {
					case *ast.FuncLit:
						return false
					case *ast.ReturnStmt:
						for _, r := range x.Results {
							if !out[name] && buildsTruncating(r, ioName, local, out) {
								out[name] = true
								grew = true
							}
						}
					}
					return true
				})
			})
		}
	}
	return out
}

// isCappedReadCall: a call to one of the capped body readers (by name, as bodyReadFuncs lists
// them), whatever package qualifies it.
func isCappedReadCall(c *ast.CallExpr) bool {
	switch f := c.Fun.(type) {
	case *ast.Ident:
		return bodyReadFuncs[f.Name]
	case *ast.SelectorExpr:
		return bodyReadFuncs[f.Sel.Name]
	}
	return false
}

func isPkgCall(c *ast.CallExpr, pkg, fn string) bool {
	sel, ok := c.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != fn {
		return false
	}
	id, ok := sel.X.(*ast.Ident)
	return ok && id.Name == pkg
}

// isBodyReadCall: io.ReadAll(<anything with .Body>) or one of the capped body readers.
func isBodyReadCall(c *ast.CallExpr, ioName string) bool {
	if ioName != "" && isPkgCall(c, ioName, "ReadAll") && len(c.Args) == 1 && mentionsBody(c.Args[0]) {
		return true
	}
	return isCappedReadCall(c)
}

// mentionsBody reports whether e contains a selector ending in .Body (resp.Body, r.Body, …).
func mentionsBody(e ast.Expr) bool {
	seen := false
	ast.Inspect(e, func(n ast.Node) bool {
		if s, ok := n.(*ast.SelectorExpr); ok && s.Sel.Name == "Body" {
			seen = true
		}
		return !seen
	})
	return seen
}

func isBlank(e ast.Expr) bool {
	id, ok := e.(*ast.Ident)
	return ok && id.Name == "_"
}
