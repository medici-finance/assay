package main

// Mutant schemata (#2635): one compile per worker instead of one per mutant.
//
// The cost. Profiling the internal/deskkit sweep put ~72% of every mutant's CPU
// in ONE step: recompiling the mutated package's test variant (~8-9 CPU-s, ~1 GB
// RSS each). Copying the tree is once per worker, linking ~0.4 CPU-s, vet ~0.9,
// running the tests ~2. On a 2-CPU runner the sweep is bound by that compile.
//
// The idea. Compile every eligible mutant into the package ONCE, each behind a
// runtime switch, then pick the live mutant per suite run with an environment
// variable. The build is then identical for every run and the Go build cache
// serves it; a mutant costs a link and a test run, not a compile.
//
// How, without touching the tree. The schemata source is never written into
// the tree: it is a copy of each mutated file, handed to the go command through
// `-overlay` (in GOFLAGS), so source-reading tests still see the pristine file
// on disk. For a function holding mutant k the copy reads
//
//	func F(params) results {if muharSel0__ == "k" {/*line :L:C*/<mutated body>
//	};/*line :L:C*/<original body>}
//
// so with the selector unset the function runs its original body, and with it
// set to k the function runs exactly the body the classic mutant would have,
// inside the same function (same name on the stack, same defers, same named
// results), with every position mapped back to the line and column the classic
// mutated file would report.
//
// Why the verdicts cannot get worse. Schemata never decides a NOT CAUGHT. A
// schemata run that stays green is re-run the classic way (mutation written to
// disk, proven landed, suite rebuilt) and the classic verdict is the one
// reported. Every way schemata can differ from a classic mutant (a source-reading
// test, a subprocess that drops the environment, a file the suite never builds)
// makes it see LESS of the mutation, so those mutants fall through to the classic
// run. Schemata is only trusted for CAUGHT. Anything the shape check below cannot
// vouch for runs classically from the start, and a schemata build that does not
// pass the pristine baseline is narrowed (bisected) until it does, or dropped.

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

const (
	// schemaEnv names the variable a schemata build reads its live mutant from.
	schemaEnv = "MUHAR_SCHEMATA_MUTANT"
	// schemaNone is the selector for "no mutant": the pristine behaviour.
	schemaNone = "-"
	// schemaControl is the selector of the positive control.
	schemaControl = "control"
	// schemaPrefix starts every identifier schemata adds; a file that already
	// declares one is not eligible (the names could collide).
	schemaPrefix = "muhar"
)

// schemaID is the selector value for the mutation at spec index i (-1 = control).
func schemaID(i int) string {
	if i < 0 {
		return schemaControl
	}
	return strconv.Itoa(i)
}

// schemaSite is one eligible mutant: where its function's body sits in the
// pristine file and the body it is to run instead.
type schemaSite struct {
	id      string
	lbrace  int // offset of the function body's '{' in the pristine file
	line    int // position of the first byte after '{'
	col     int
	void    bool   // no results: the mutated body may fall off its end
	mutBody string // text between the braces, mutation applied
}

// schemaSiteFor decides whether m can run as a schema mutant of the pristine
// file content src at path, and returns its site when it can. It refuses (ok
// false, with the reason) whenever it cannot show the schemata copy behaves as
// the classic mutated file would; those mutants simply run classically.
func schemaSiteFor(path, src string, m Mutation, id string) (schemaSite, string) {
	if !strings.HasSuffix(path, ".go") {
		return schemaSite{}, "not a .go file"
	}
	mutated, err := applyEdit(src, m)
	if err != nil {
		return schemaSite{}, "edit does not apply"
	}
	if strings.Contains(src, "//line ") || strings.Contains(src, "/*line ") {
		return schemaSite{}, "file carries line directives"
	}
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, src, parser.ParseComments)
	if err != nil {
		return schemaSite{}, "file does not parse"
	}
	for _, imp := range f.Imports {
		if imp.Path.Value == `"C"` {
			return schemaSite{}, "cgo file"
		}
	}
	if declaresPrefix(f) {
		return schemaSite{}, "file already uses the muhar identifier prefix"
	}
	off := strings.Index(src, m.Old)
	end := off + len(m.Old)
	idx := -1
	var fd *ast.FuncDecl
	for i, d := range f.Decls {
		d, ok := d.(*ast.FuncDecl)
		if !ok || d.Body == nil {
			continue
		}
		lb, rb := fset.Position(d.Body.Lbrace).Offset, fset.Position(d.Body.Rbrace).Offset
		if off > lb && end <= rb {
			idx, fd = i, d
			break
		}
	}
	if fd == nil {
		return schemaSite{}, "edit is not inside a function body"
	}
	lb := fset.Position(fd.Body.Lbrace).Offset
	rb := fset.Position(fd.Body.Rbrace).Offset
	if hasLabel(fd.Body) {
		return schemaSite{}, "function uses labels"
	}

	// The mutated file must still parse with the SAME declaration at the same
	// place, its body grown or shrunk by exactly the edit: anything else (an edit
	// that closes the function, or opens another) is not a body swap.
	mset := token.NewFileSet()
	mf, err := parser.ParseFile(mset, path, mutated, parser.ParseComments)
	if err != nil {
		return schemaSite{}, "mutated file does not parse"
	}
	if len(mf.Decls) != len(f.Decls) {
		return schemaSite{}, "edit changes the declarations"
	}
	md, ok := mf.Decls[idx].(*ast.FuncDecl)
	delta := len(m.New) - len(m.Old)
	if !ok || md.Body == nil || md.Name.Name != fd.Name.Name ||
		mset.Position(md.Body.Lbrace).Offset != lb || mset.Position(md.Body.Rbrace).Offset != rb+delta {
		return schemaSite{}, "edit changes the function, not only its body"
	}
	if hasLabel(md.Body) {
		return schemaSite{}, "mutated function uses labels"
	}
	// Nesting the body one block deeper changes one thing: a top-level `:=`
	// that re-uses a parameter or result name declares a NEW variable inside the
	// block where the classic body would assign the existing one.
	if redeclaresParam(md) {
		return schemaSite{}, "mutated body redeclares a parameter or result with :="
	}
	p := fset.Position(fd.Body.Lbrace)
	return schemaSite{
		id:      id,
		lbrace:  lb,
		line:    p.Line,
		col:     p.Column + 1,
		void:    fd.Type.Results == nil || len(fd.Type.Results.List) == 0,
		mutBody: mutated[lb+1 : rb+delta],
	}, ""
}

// declaresPrefix reports whether any identifier in f starts with schemaPrefix.
func declaresPrefix(f *ast.File) bool {
	found := false
	ast.Inspect(f, func(n ast.Node) bool {
		if id, ok := n.(*ast.Ident); ok && strings.HasPrefix(id.Name, schemaPrefix) {
			found = true
		}
		return !found
	})
	return found
}

func hasLabel(b *ast.BlockStmt) bool {
	found := false
	ast.Inspect(b, func(n ast.Node) bool {
		if _, ok := n.(*ast.LabeledStmt); ok {
			found = true
		}
		return !found
	})
	return found
}

// redeclaresParam reports a top-level `a, b := ...` in fd's body whose left side
// names a receiver, parameter or result of fd.
func redeclaresParam(fd *ast.FuncDecl) bool {
	names := map[string]bool{}
	for _, fl := range []*ast.FieldList{fd.Recv, fd.Type.Params, fd.Type.Results} {
		if fl == nil {
			continue
		}
		for _, fld := range fl.List {
			for _, n := range fld.Names {
				names[n.Name] = true
			}
		}
	}
	for _, st := range fd.Body.List {
		as, ok := st.(*ast.AssignStmt)
		if !ok || as.Tok != token.DEFINE {
			continue
		}
		for _, l := range as.Lhs {
			if id, ok := l.(*ast.Ident); ok && names[id.Name] {
				return true
			}
		}
	}
	return false
}

// schemaFile renders the schemata copy of the pristine file src holding sites
// (all in this file). k makes the added identifiers unique per file. A probe
// copy (probe true) carries no line directives, so a compiler error lands on
// the copy's own line, and brackets each mutant with markers so that line can
// be traced back to the mutant (see schemaTree.blame).
func schemaFile(src string, f *ast.File, fset *token.FileSet, sites []schemaSite, k int, probe bool) string {
	sel := fmt.Sprintf("%sSel%d__", schemaPrefix, k)
	osName := fmt.Sprintf("%sOs%d__", schemaPrefix, k)

	byBrace := map[int][]schemaSite{}
	for _, s := range sites {
		byBrace[s.lbrace] = append(byBrace[s.lbrace], s)
	}
	type ins struct {
		at   int
		text string
	}
	var inserts []ins
	// The import rides on the package clause's own line, so no line below it
	// moves: every position outside the mutated bodies stays where it was.
	inserts = append(inserts, ins{fset.Position(f.Name.End()).Offset, fmt.Sprintf("; import %s %q", osName, "os")})
	for lb, ss := range byBrace {
		var b strings.Builder
		for _, s := range ss {
			if probe {
				fmt.Fprintf(&b, "if %s == %q {/*%s:B:%s*/%s", sel, s.id, schemaPrefix, s.id, s.mutBody)
			} else {
				fmt.Fprintf(&b, "if %s == %q {/*line :%d:%d*/%s", sel, s.id, s.line, s.col, s.mutBody)
			}
			if s.void {
				b.WriteString("\nreturn")
			}
			if probe {
				fmt.Fprintf(&b, "/*%s:E:%s*/", schemaPrefix, s.id)
			}
			b.WriteString("\n};")
		}
		if !probe {
			fmt.Fprintf(&b, "/*line :%d:%d*/", ss[0].line, ss[0].col)
		}
		inserts = append(inserts, ins{lb + 1, b.String()})
	}
	sort.Slice(inserts, func(i, j int) bool { return inserts[i].at > inserts[j].at })
	out := src
	for _, in := range inserts {
		out = out[:in.at] + in.text + out[in.at:]
	}
	return out + fmt.Sprintf("\nvar %s = %s.Getenv(%q)\n", sel, osName, schemaEnv)
}

// schemaTree is one tree's schemata build: the overlay the go command reads it
// from and which mutants it carries.
type schemaTree struct {
	dir     string          // temp dir holding the overlay and the schemata copies
	overlay string          // the -overlay JSON
	ids     map[string]bool // selectors this build carries
	dirs    []string        // package directories of the replaced files
	// copies maps each schemata copy's base name to the selectors it carries,
	// and (probe builds only) to the copy-line span of each mutant's block.
	copies map[string]*schemaCopy
	// skipped counts the mutants left out, by the reason schemaSiteFor gave.
	skipped map[string]int
}

type schemaCopy struct {
	ids   []string
	spans map[string][2]int // selector -> first, last line of its block in the copy
}

func (t *schemaTree) cleanup() {
	if t != nil && t.dir != "" {
		_ = os.RemoveAll(t.dir)
	}
}

// buildSchemaTree writes the schemata copies of every file touched by the
// mutants in want (spec index -> mutation; -1 is the control) for the tree at
// root, and the overlay mapping each pristine path to its copy. Mutants that
// are not eligible are left out. Each copy is read back to prove it landed.
func buildSchemaTree(root string, want map[int]Mutation, probe bool) (*schemaTree, error) {
	type fileSites struct {
		src   string
		sites []schemaSite
	}
	files := map[string]*fileSites{}
	skipped := map[string]int{}
	keys := make([]int, 0, len(want))
	for i := range want {
		keys = append(keys, i)
	}
	sort.Ints(keys)
	for _, i := range keys {
		m := want[i]
		path, err := filepath.Abs(resolve(root, m.File))
		if err != nil {
			continue
		}
		fs := files[path]
		if fs == nil {
			raw, err := os.ReadFile(path)
			if err != nil {
				skipped["file unreadable"]++
				continue
			}
			fs = &fileSites{src: string(raw)}
			files[path] = fs
		}
		s, why := schemaSiteFor(path, fs.src, m, schemaID(i))
		if why != "" {
			skipped[why]++
			continue
		}
		fs.sites = append(fs.sites, s)
	}

	t := &schemaTree{ids: map[string]bool{}, copies: map[string]*schemaCopy{}, skipped: skipped}
	dir, err := os.MkdirTemp("", "muhar-schemata-")
	if err != nil {
		return nil, err
	}
	t.dir = dir
	paths := make([]string, 0, len(files))
	for p := range files {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	replace := map[string]string{}
	seenDir := map[string]bool{}
	for k, p := range paths {
		fs := files[p]
		if len(fs.sites) == 0 {
			continue
		}
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, p, fs.src, parser.ParseComments)
		if err != nil {
			continue
		}
		text := schemaFile(fs.src, f, fset, fs.sites, k, probe)
		name := fmt.Sprintf("%d_%s", k, filepath.Base(p))
		cp := filepath.Join(dir, name)
		if err := os.WriteFile(cp, []byte(text), 0o644); err != nil {
			t.cleanup()
			return nil, err
		}
		back, err := os.ReadFile(cp)
		if err != nil || string(back) != text {
			t.cleanup()
			return nil, fmt.Errorf("schemata copy of %s did not land", p)
		}
		c := &schemaCopy{spans: map[string][2]int{}}
		for _, s := range fs.sites {
			if !strings.Contains(text, s.mutBody) {
				t.cleanup()
				return nil, fmt.Errorf("schemata copy of %s is missing mutant %s", p, s.id)
			}
			t.ids[s.id] = true
			c.ids = append(c.ids, s.id)
			if probe {
				b := strings.Index(text, fmt.Sprintf("/*%s:B:%s*/", schemaPrefix, s.id))
				e := strings.Index(text, fmt.Sprintf("/*%s:E:%s*/", schemaPrefix, s.id))
				if b < 0 || e < b {
					t.cleanup()
					return nil, fmt.Errorf("probe copy of %s lost the markers of mutant %s", p, s.id)
				}
				c.spans[s.id] = [2]int{1 + strings.Count(text[:b], "\n"), 1 + strings.Count(text[:e], "\n")}
			}
		}
		t.copies[name] = c
		replace[p] = cp
		if d := filepath.Dir(p); !seenDir[d] {
			seenDir[d] = true
			t.dirs = append(t.dirs, d)
		}
	}
	raw, err := json.Marshal(map[string]any{"Replace": replace})
	if err != nil {
		t.cleanup()
		return nil, err
	}
	t.overlay = filepath.Join(dir, "overlay.json")
	if err := os.WriteFile(t.overlay, raw, 0o644); err != nil {
		t.cleanup()
		return nil, err
	}
	return t, nil
}

// compileErr matches a go compiler diagnostic: "path/file.go:LINE:COL: msg".
var compileErr = regexp.MustCompile(`([^\s:]+\.go):(\d+):\d+: `)

// blame maps the diagnostics of a failed probe build to the mutants whose
// blocks they fall in. A diagnostic elsewhere in a schemata copy blames every
// mutant of that copy. ok is false when some diagnostic cannot be traced to a
// copy at all (or none parsed): the build failed for a reason schemata cannot
// localise, and the caller must give schemata up.
func (t *schemaTree) blame(out string) (bad map[string]bool, ok bool) {
	bad = map[string]bool{}
	matches := compileErr.FindAllStringSubmatch(out, -1)
	if len(matches) == 0 {
		return nil, false
	}
	for _, mm := range matches {
		c := t.copies[filepath.Base(mm[1])]
		if c == nil {
			return nil, false
		}
		line, _ := strconv.Atoi(mm[2])
		hit := false
		for id, sp := range c.spans {
			if line >= sp[0] && line <= sp[1] {
				bad[id], hit = true, true
			}
		}
		if !hit {
			for _, id := range c.ids {
				bad[id] = true
			}
		}
	}
	return bad, true
}

// minSchemaMutants is the smallest number of eligible mutations worth a
// schemata build. Setting one up costs a probe compile, a schemata baseline
// per worker and a control run; a mutation through it saves one package
// compile. Below this the classic path is as fast, so it is kept.
const minSchemaMutants = 8

// schemaPlan is the set of mutants a run routes through schemata.
type schemaPlan struct {
	want map[int]Mutation // spec index -> mutation, the control at -1
}

// schemataSetup decides which mutants run through schemata, in the first
// tree: the eligible ones (schemaSiteFor), minus any the compiler rejects (one
// probe build reports them all), narrowed until the schemata build passes the
// pristine baseline, and only if the positive control is CAUGHT through it
// when the control is eligible. It returns nil when schemata is off or not
// worth it, in which case the whole run is classic.
func (h *Harness) schemataSetup(root string) (*schemaPlan, *schemaTree) {
	if h.SchemaTest == nil {
		return nil, nil
	}
	want := map[int]Mutation{-1: h.Control}
	for i, m := range h.Mutations {
		want[i] = m
	}
	keep := h.eligible(root, want)
	if h.SchemaBuild != nil {
		keep = h.probe(root, keep)
	}
	if n := len(keep) - btoi(keep[-1].Name != ""); n < minSchemaMutants {
		h.logf("muhar: schemata: %d of %d mutations eligible, fewer than %d; every mutation rebuilds the suite\n", n, len(h.Mutations), minSchemaMutants)
		return nil, nil
	}
	keep, t := h.narrow(root, keep)
	if t == nil {
		h.logf("muhar: schemata: no schemata build passes the baseline; every mutation rebuilds the suite\n")
		return nil, nil
	}
	if t.ids[schemaControl] && !h.SchemaTest(root, t.overlay, schemaControl) {
		t.cleanup()
		h.logf("muhar: schemata: the positive control is not CAUGHT through schemata; every mutation rebuilds the suite\n")
		return nil, nil
	}
	n := len(t.ids) - btoi(t.ids[schemaControl])
	h.logf("muhar: schemata: %d of %d mutations share one build per worker; the rest rebuild per mutation, and every schemata NOT CAUGHT is re-run the classic way\n", n, len(h.Mutations))
	return &schemaPlan{want: keep}, t
}

func btoi(b bool) int {
	if b {
		return 1
	}
	return 0
}

// eligible returns the subset of want that schemaSiteFor accepts in the tree at root.
func (h *Harness) eligible(root string, want map[int]Mutation) map[int]Mutation {
	t, err := buildSchemaTree(root, want, false)
	if err != nil {
		h.logf("muhar: schemata off (%v)\n", err)
		return nil
	}
	defer t.cleanup()
	if len(t.skipped) > 0 {
		var why []string
		for r, n := range t.skipped {
			why = append(why, fmt.Sprintf("%d %s", n, r))
		}
		sort.Strings(why)
		h.logf("muhar: schemata: not eligible (they rebuild per mutation): %s\n", strings.Join(why, "; "))
	}
	keep := map[int]Mutation{}
	for i, m := range want {
		if t.ids[schemaID(i)] {
			keep[i] = m
		}
	}
	return keep
}

// probe compiles the schemata packages once, with every error reported, and
// drops each mutant the compiler rejects (a mutant that does not compile is
// CAUGHT on the classic path, where it still goes). It repeats until the build
// is clean, and returns nil if a failure cannot be traced to a mutant.
func (h *Harness) probe(root string, want map[int]Mutation) map[int]Mutation {
	for round := 0; round < 4 && len(want) > 0; round++ {
		t, err := buildSchemaTree(root, want, true)
		if err != nil {
			return nil
		}
		out, clean := h.SchemaBuild(root, t.overlay, t.dirs)
		if clean {
			t.cleanup()
			return want
		}
		bad, ok := t.blame(out)
		t.cleanup()
		if !ok || len(bad) == 0 {
			h.logf("muhar: schemata: the probe build failed outside any mutant; schemata off:\n%s\n", firstLines(out, 5))
			return nil
		}
		for i := range want {
			if bad[schemaID(i)] {
				delete(want, i)
			}
		}
		h.logf("muhar: schemata: %d mutation(s) do not compile; they rebuild per mutation\n", len(bad))
	}
	return nil
}

// narrow returns the largest subset of want (found by halving) whose schemata
// build keeps the pristine baseline green, and that build. Whatever it leaves
// out (a mutant vet rejects, say) runs classically, where it is judged as before.
func (h *Harness) narrow(root string, want map[int]Mutation) (map[int]Mutation, *schemaTree) {
	t, err := buildSchemaTree(root, want, false)
	if err == nil && len(t.ids) > 0 && !h.SchemaTest(root, t.overlay, schemaNone) {
		return want, t
	}
	t.cleanup()
	if err != nil || len(want) <= 1 {
		return nil, nil
	}
	keys := make([]int, 0, len(want))
	for i := range want {
		keys = append(keys, i)
	}
	sort.Ints(keys)
	lo, hi := map[int]Mutation{}, map[int]Mutation{}
	for j, i := range keys {
		if j < len(keys)/2 {
			lo[i] = want[i]
		} else {
			hi[i] = want[i]
		}
	}
	a, ta := h.narrow(root, lo)
	ta.cleanup()
	b, tb := h.narrow(root, hi)
	tb.cleanup()
	merged := map[int]Mutation{}
	for i, m := range a {
		merged[i] = m
	}
	for i, m := range b {
		merged[i] = m
	}
	if len(merged) == 0 {
		return nil, nil
	}
	// Two halves that each pass can still fail together; prove the union.
	t, err = buildSchemaTree(root, merged, false)
	if err == nil && !h.SchemaTest(root, t.overlay, schemaNone) {
		return merged, t
	}
	t.cleanup()
	if len(a) < len(b) {
		a = b
	}
	if len(a) == 0 {
		return nil, nil
	}
	t, err = buildSchemaTree(root, a, false)
	if err != nil {
		return nil, nil
	}
	return a, t
}

// schemataFor builds the plan's schemata for another worker's tree and proves
// its baseline green there; nil means that worker runs every mutation classically.
func (h *Harness) schemataFor(root string, p *schemaPlan) *schemaTree {
	if p == nil {
		return nil
	}
	t, err := buildSchemaTree(root, p.want, false)
	if err != nil {
		return nil
	}
	if h.SchemaTest(root, t.overlay, schemaNone) {
		t.cleanup()
		h.logf("muhar: schemata: baseline red in a worker copy; that worker rebuilds per mutation\n")
		return nil
	}
	return t
}

// runMutation runs spec mutation i in the tree at root: through the tree's
// schemata build when it carries i and the suite goes red there, otherwise the
// classic way (runOne). Only CAUGHT is ever taken from schemata.
func (h *Harness) runMutation(root string, t *schemaTree, i int) (Result, error) {
	m := h.Mutations[i]
	if t != nil && t.ids[schemaID(i)] && h.SchemaTest(root, t.overlay, schemaID(i)) {
		return Caught, nil
	}
	return h.runOne(root, m)
}

func (h *Harness) logf(format string, args ...any) {
	w := h.Log
	if w == nil {
		w = io.Discard
	}
	fmt.Fprintf(w, format, args...)
}

// schemaGoflags returns the GOFLAGS value a schemata suite run uses: the
// inherited value plus the overlay.
func schemaGoflags(inherited, overlay string) string {
	return strings.TrimSpace(inherited + " -overlay=" + overlay)
}

// schemataUsable reports whether a suite command can use schemata: it must be
// a `go test` that disables the test cache (the selector is read before the
// test cache starts tracking the environment, so a cached pass could be
// replayed for a different mutant), and nothing may already supply an overlay
// (GOFLAGS holds one -overlay at most).
func schemataUsable(test, goflags, tmp string) bool {
	if !strings.Contains(test, "go test") || !hasCountOne(test) {
		return false
	}
	if strings.Contains(test, "-overlay") || strings.Contains(goflags, "-overlay") {
		return false
	}
	// GOFLAGS splits on spaces, so the overlay's path must not hold one.
	return !strings.ContainsAny(tmp, " \t\n")
}

func hasCountOne(test string) bool {
	for _, f := range strings.Fields(test) {
		f = strings.Trim(f, `'"`)
		if f == "-count=1" || f == "--count=1" {
			return true
		}
	}
	return false
}

// firstLines returns at most n leading lines of s, for a log note.
func firstLines(s string, n int) string {
	lines := strings.SplitN(strings.TrimSpace(s), "\n", n+1)
	if len(lines) > n {
		lines = lines[:n]
	}
	return strings.Join(lines, "\n")
}
