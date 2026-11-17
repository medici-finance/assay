package deskkit

// forgebodycap_guard_test.go — the source guard for the forge body cap. A shipped file in the
// trees below that reads a response body with io.ReadAll (bare or wrapped, e.g. in an
// io.LimitReader that silently truncates) instead of ReadCappedForgeBody / ReadForgeBodyUpTo,
// or that DISCARDS the error of a body read, fails the build. The planted fixture proves the
// scanner sees both shapes, so a scanner that sees nothing cannot certify everything. A
// truncating reader handed to one of the capped readers is a third shape: the capped reader
// then sees a body that already ends at the wrapper's bound, so its over-cap refusal can never
// fire. The truncating forms it sees are exactly these, as the capped reader's reader argument:
//   - an io.LimitReader(...) call or an io.LimitedReader literal (or its address), whether it
//     is the whole argument or sits anywhere inside it (bufio.NewReader(io.LimitReader(...)),
//     io.TeeReader(io.LimitReader(...), w));
//   - a local name assigned such an expression in the same function, by :=, var, or a plain =
//     (a reassignment such as r = io.LimitReader(r, n) included), named alone or inside the
//     argument the same way.

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

// forgeBodyReadPermits are the readers that keep their own, narrower bound AND refuse an
// over-bound body (they read limit+1 and check). Key: "<root>/<file>:<func> <kind>" — the
// hit kind is part of the key, and a permit covers ONE hit: a second read added to the same
// function is still reported, whether it is of another kind or of the permitted kind.
var forgeBodyReadPermits = map[string]string{
	"internal/deskkit/compositionsource.go:httpFetchBounded uncapped-body-read": "reads limit+1 and refuses a body over its own limit",
	"internal/deskkit/forge_github.go:runLog uncapped-body-copy": "streams into runLogSink, which refuses an archive past runLogArchiveCap; " +
		"the client transport caps the same body one byte past that bound",
	"cmd/deskfleet/pushrotate.go:provePush uncapped-body-copy": "drains a status-only probe answer to io.Discard under maxResponseBytes; " +
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

// scanForgeBodyReads parses every shipped .go file under dir, subdirectories included (no
// _test.go, no testdata), and returns one "<label>/<path>:<func> <kind>" per offending read.
//
// The shapes it sees are the ones a source scan can see without type information: a literal
// io.ReadAll, json.NewDecoder or io.Copy/io.CopyN whose reader is a response body — named
// directly (resp.Body) or through a local alias assigned from one (r := resp.Body) — and a
// discarded body-read error. A body handed to a helper that takes a plain io.Reader is out of
// its sight; the forge clients' transport cap (forgebodytransport.go, guarded by
// TestForgeClientsCapped) is what bounds those reads.
func scanForgeBodyReads(dir, label string) ([]string, error) {
	var out []string
	fset := token.NewFileSet()
	files := 0
	werr := filepath.WalkDir(dir, func(path string, e os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := e.Name()
		if e.IsDir() {
			if name == "testdata" || (path != dir && strings.HasPrefix(name, ".")) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			return nil
		}
		f, perr := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
		if perr != nil {
			return perr
		}
		files++
		rel, _ := filepath.Rel(dir, path)
		out = append(out, scanBodyReadFile(f, label+"/"+filepath.ToSlash(rel))...)
		return nil
	})
	if werr != nil {
		return nil, werr
	}
	if files == 0 {
		return nil, fmt.Errorf("no Go source under %s — a guard that scans nothing certifies nothing", dir)
	}
	return out, nil
}

func scanBodyReadFile(f *ast.File, label string) []string {
	ioName := importName(f, "io", "io")
	jsonName := importName(f, "encoding/json", "json")
	var out []string
	for _, d := range f.Decls {
		fn, ok := d.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		where := fmt.Sprintf("%s:%s", label, fn.Name.Name)
		aliases := bodyAliases(fn.Body)
		truncating := truncatingAliases(fn.Body, ioName)
		isTruncating := func(e ast.Expr) bool { return buildsTruncating(e, ioName, truncating) }
		isBody := func(e ast.Expr) bool {
			if mentionsBody(e) {
				return true
			}
			id, ok := e.(*ast.Ident)
			return ok && aliases[id.Name]
		}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
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
	}
	return out
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
// refusing past it: io.LimitReader(...) or an io.LimitedReader literal (or its address).
func truncatingReader(e ast.Expr, ioName string) bool {
	if ioName == "" {
		return false
	}
	if u, ok := e.(*ast.UnaryExpr); ok && u.Op == token.AND {
		e = u.X
	}
	switch x := e.(type) {
	case *ast.CallExpr:
		return isPkgCall(x, ioName, "LimitReader")
	case *ast.CompositeLit:
		sel, ok := x.Type.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "LimitedReader" {
			return false
		}
		id, ok := sel.X.(*ast.Ident)
		return ok && id.Name == ioName
	}
	return false
}

// buildsTruncating reports whether e is, or holds anywhere inside it, a truncating reader
// (truncatingReader) or a name in aliases. The Sel of a selector is a field or method name,
// never a local, so only its X is searched.
func buildsTruncating(e ast.Expr, ioName string, aliases map[string]bool) bool {
	seen := false
	ast.Inspect(e, func(n ast.Node) bool {
		switch x := n.(type) {
		case nil:
			return false
		case *ast.SelectorExpr:
			seen = seen || buildsTruncating(x.X, ioName, aliases)
			return false
		case *ast.Ident:
			if aliases[x.Name] {
				seen = true
			}
		case ast.Expr:
			if truncatingReader(x, ioName) {
				seen = true
			}
		}
		return !seen
	})
	return seen
}

// truncatingAliases: local names assigned an expression that is or holds a truncating reader
// (lr := io.LimitReader(...), br := bufio.NewReader(io.LimitReader(...))). One hop: a name
// assigned another such name is not followed.
func truncatingAliases(body *ast.BlockStmt, ioName string) map[string]bool {
	out := map[string]bool{}
	ast.Inspect(body, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.AssignStmt:
			if len(x.Lhs) == len(x.Rhs) {
				for i, r := range x.Rhs {
					if id, ok := x.Lhs[i].(*ast.Ident); ok && buildsTruncating(r, ioName, nil) {
						out[id.Name] = true
					}
				}
			}
		case *ast.ValueSpec:
			if len(x.Names) == len(x.Values) {
				for i, v := range x.Values {
					if buildsTruncating(v, ioName, nil) {
						out[x.Names[i].Name] = true
					}
				}
			}
		}
		return true
	})
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
