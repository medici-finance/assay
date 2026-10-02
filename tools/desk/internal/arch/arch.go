// Package arch holds the desk tools' architectural fitness functions: three shape rules
// over the tools/desk source tree, computed by pure functions over an fs.FS so the same
// code checks the real tree and a fixture (build-less-brittle brief 10).
//
// WHY. The weight ratchet (internal/weight) holds how much surface there is. It says
// nothing about shape: whether a helper crept into the wrong direction, whether the hub
// package quietly grew a new dependency, or whether a meaning the semantic index
// (docs/contracts.md, "Semantic owners") gives one owner is now computed somewhere else.
// These rules make those shape claims executable, and arch_test.go runs them in CI.
//
// THE THREE RULES, each named by its rule-register row in docs/contracts.md:
//
//	R-dep-direction       no package under internal/ imports a package under cmd/, and no
//	                      cmd/<x> package imports cmd/<y> for y != x (cmd/<x>/internal/...
//	                      belongs to x). Ceiling 0, nothing grandfathered.
//	R-hub-allowlist       every in-module package the hub (internal/deskkit) imports is on
//	                      hub-allow.txt, and every line on hub-allow.txt is still imported,
//	                      so a removed dependency cannot silently come back.
//	R-one-implementation  a function that computes a registered meaning carries the line
//	                      comment "semantic: S-<slug>" directly above its declaration; every
//	                      such marker sits in the row's owner path or a path the row's
//	                      duplicates column lists, the count per row stays at or under
//	                      markers.txt's ceiling, and an owner inside the walked tree carries
//	                      at least one marker. A listed file path is that one file; a listed
//	                      directory is the package in that directory, never its
//	                      subdirectories; and a path naming the walked root, its cmd/ or
//	                      internal/ directory, or anything above them names no site at all,
//	                      so a prose mention of the module in a cell cannot swallow the tree.
//
// WHAT THE COMPILER ALREADY DOES, and this package does not re-check: Go refuses import
// cycles and refuses an import of an internal/ package from outside the tree rooted at its
// parent. Those are the strongest shape rules and they are free.
//
// WHAT RULE 3 CANNOT SEE: an undeclared duplicate. A function that computes a registered
// meaning without carrying the marker is invisible here; finding it is the design-fit
// review stage's question at review time. This package holds what has been declared.
//
// Imports are read with go/parser in ImportsOnly mode over every non-test .go file,
// ignoring build constraints: the graph is the union over all platforms, which is never
// weaker than any one platform's `go list` view.
package arch

import (
	"bufio"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"io/fs"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Register row ids, as they appear in docs/contracts.md's rule register. Every violation
// names one of them so a red test points at the row that explains the rule.
const (
	RuleDirection = "R-dep-direction"
	RuleHubAllow  = "R-hub-allowlist"
	RuleOneImpl   = "R-one-implementation"
)

// Hub is the module-relative package whose imports hub-allow.txt governs.
const Hub = "internal/deskkit"

// Violation is one rule failure at one source position.
type Violation struct {
	Rule string // a register row id (RuleDirection, RuleHubAllow, RuleOneImpl)
	File string // path relative to the fs.FS root; empty for a list-file finding
	Line int
	Msg  string
}

func (v Violation) String() string {
	if v.File == "" {
		return fmt.Sprintf("%s: %s", v.Rule, v.Msg)
	}
	return fmt.Sprintf("%s: %s:%d: %s", v.Rule, v.File, v.Line, v.Msg)
}

// Edge is one in-module import: the importing file and line, and the imported package
// as a module-relative directory (for example "internal/gitcore").
type Edge struct {
	To   string
	File string
	Line int
}

// Graph maps each module-relative package directory ("" for the module root) to its
// in-module imports. Imports of packages outside the module are dropped.
type Graph map[string][]Edge

// Imports reads the module rooted at modDir inside fsys: the module path from
// modDir/go.mod, then every non-test .go file below it. Directories named testdata or
// vendor, directories starting with "." or "_", and nested modules are skipped, as the go
// tool skips them. File paths in the result are fsys-relative.
func Imports(fsys fs.FS, modDir string) (Graph, error) {
	modPath, err := modulePath(fsys, modDir)
	if err != nil {
		return nil, err
	}
	g := Graph{}
	fset := token.NewFileSet()
	err = walkGo(fsys, modDir, func(p string, src []byte) error {
		f, perr := parser.ParseFile(fset, p, src, parser.ImportsOnly)
		if perr != nil {
			return fmt.Errorf("parsing %s: %w", p, perr)
		}
		pkg := relDir(modDir, path.Dir(p))
		if _, ok := g[pkg]; !ok {
			g[pkg] = nil
		}
		for _, is := range f.Imports {
			ip, uerr := strconv.Unquote(is.Path.Value)
			if uerr != nil {
				return fmt.Errorf("%s: unreadable import %s", p, is.Path.Value)
			}
			var to string
			switch {
			case ip == modPath:
				to = ""
			case strings.HasPrefix(ip, modPath+"/"):
				to = strings.TrimPrefix(ip, modPath+"/")
			default:
				continue
			}
			g[pkg] = append(g[pkg], Edge{To: to, File: p, Line: fset.Position(is.Pos()).Line})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return g, nil
}

// Direction applies R-dep-direction to g.
func Direction(g Graph) []Violation {
	var out []Violation
	for _, pkg := range g.pkgs() {
		for _, e := range g[pkg] {
			switch {
			case under(pkg, "internal") && under(e.To, "cmd"):
				out = append(out, Violation{Rule: RuleDirection, File: e.File, Line: e.Line,
					Msg: fmt.Sprintf("%s imports %s: a package under internal/ never imports a command", pkg, e.To)})
			case under(pkg, "cmd") && under(e.To, "cmd") && verbOf(pkg) != verbOf(e.To):
				out = append(out, Violation{Rule: RuleDirection, File: e.File, Line: e.Line,
					Msg: fmt.Sprintf("%s imports %s: a command never imports another command; move the shared code under internal/", pkg, e.To)})
			}
		}
	}
	return out
}

// HubAllow applies R-hub-allowlist: every in-module import of hub must be internal/<entry>
// for an entry on allow, and every entry on allow must still be imported. A hub package
// absent from g is itself a violation, so a moved hub cannot turn the rule vacuous.
func HubAllow(g Graph, hub string, allow []string) []Violation {
	edges, ok := g[hub]
	if !ok {
		return []Violation{{Rule: RuleHubAllow, Msg: fmt.Sprintf("hub package %s not found in the tree; the allow-list checks nothing", hub)}}
	}
	listed := map[string]bool{}
	for _, a := range allow {
		listed[a] = true
	}
	seen := map[string]bool{}
	var out []Violation
	for _, e := range edges {
		name := strings.TrimPrefix(e.To, "internal/")
		if under(e.To, "internal") && listed[name] {
			seen[name] = true
			continue
		}
		out = append(out, Violation{Rule: RuleHubAllow, File: e.File, Line: e.Line,
			Msg: fmt.Sprintf("%s imports %s, which is not on hub-allow.txt; everything the hub imports is a dependency of every package that imports it", hub, e.To)})
	}
	for _, a := range allow {
		if !seen[a] {
			out = append(out, Violation{Rule: RuleHubAllow,
				Msg: fmt.Sprintf("hub-allow.txt lists %s but %s no longer imports internal/%s; remove the line to lock in the gain", a, hub, a)})
		}
	}
	return out
}

// SRow is one row of the semantic index: its id, and every repository path its owner and
// duplicates cells name.
type SRow struct {
	ID         string
	Owners     []string
	Duplicates []string
}

// ParseIndex reads the "## Semantic owners" table from a docs/contracts.md. Columns are
// found by header name (id, owner, duplicates...), cells split on unescaped pipes, and a
// path is any backticked span shaped like a slash-separated path, with a trailing
// :line or :line-line suffix dropped. A missing section, a missing column or a table with
// no S- rows is an error: an index that parses to nothing must not read as clean.
func ParseIndex(r io.Reader) ([]SRow, error) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 4<<20)
	inSection := false
	var header []string
	idCol, ownerCol, dupCol := -1, -1, -1
	var rows []SRow
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "## ") {
			if inSection {
				break
			}
			inSection = strings.HasPrefix(line, "## Semantic owners")
			continue
		}
		if !inSection {
			continue
		}
		t := strings.TrimSpace(line)
		if !strings.HasPrefix(t, "|") {
			if header != nil {
				break // the table ended
			}
			continue
		}
		cells := splitRow(t)
		if header == nil {
			header = cells
			for i, h := range cells {
				h = strings.ToLower(strings.TrimSpace(h))
				switch {
				case h == "id":
					idCol = i
				case h == "owner":
					ownerCol = i
				case strings.HasPrefix(h, "duplicates"):
					dupCol = i
				}
			}
			if idCol < 0 || ownerCol < 0 || dupCol < 0 {
				return nil, fmt.Errorf("semantic index header %q lacks an id, owner or duplicates column", t)
			}
			continue
		}
		if isSeparator(cells) {
			continue
		}
		if len(cells) <= idCol || len(cells) <= ownerCol || len(cells) <= dupCol {
			return nil, fmt.Errorf("semantic index row has %d cells, fewer than the header: %.60q", len(cells), t)
		}
		id := strings.TrimSpace(cells[idCol])
		if !sIDRe.MatchString(id) {
			return nil, fmt.Errorf("semantic index row id %q is not an S-<slug>", id)
		}
		rows = append(rows, SRow{ID: id, Owners: cellPaths(cells[ownerCol]), Duplicates: cellPaths(cells[dupCol])})
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	if !inSection && header == nil {
		return nil, fmt.Errorf("no \"## Semantic owners\" section")
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("the \"## Semantic owners\" section has no S- rows")
	}
	return rows, nil
}

// Marker is one declared implementation of a registered meaning.
type Marker struct {
	ID   string
	File string
	Line int
}

// Markers walks dir inside fsys (the same skips as Imports) for marker comments and
// checks where each one sits: directly above a func declaration, naming a row of index,
// in that row's owner path or a listed duplicate, as sites resolves them against dir. File
// paths are fsys-relative, so fsys must be rooted where the index's paths are rooted (the
// repository root).
func Markers(fsys fs.FS, dir string, index []SRow) ([]Marker, []Violation, error) {
	rows := map[string]SRow{}
	for _, r := range index {
		rows[r.ID] = r
	}
	var ms []Marker
	var out []Violation
	fset := token.NewFileSet()
	err := walkGo(fsys, dir, func(p string, src []byte) error {
		f, perr := parser.ParseFile(fset, p, src, parser.ParseComments)
		if perr != nil {
			return fmt.Errorf("parsing %s: %w", p, perr)
		}
		funcLines := map[int]bool{}
		for _, d := range f.Decls {
			if fd, ok := d.(*ast.FuncDecl); ok {
				funcLines[fset.Position(fd.Pos()).Line] = true
			}
		}
		for _, cg := range f.Comments {
			for _, c := range cg.List {
				if !strings.HasPrefix(c.Text, markerPrefix) {
					continue
				}
				line := fset.Position(c.Pos()).Line
				id := strings.TrimSpace(strings.TrimPrefix(c.Text, markerPrefix))
				if !funcLines[line+1] {
					out = append(out, Violation{Rule: RuleOneImpl, File: p, Line: line,
						Msg: fmt.Sprintf("marker for %s is not directly above a func declaration", id)})
					continue
				}
				row, known := rows[id]
				if !known {
					out = append(out, Violation{Rule: RuleOneImpl, File: p, Line: line,
						Msg: fmt.Sprintf("marker names %s, which is not a row of the semantic index", id)})
					continue
				}
				ms = append(ms, Marker{ID: id, File: p, Line: line})
				if !coveredBy(p, sites(row.Owners, dir)) && !coveredBy(p, sites(row.Duplicates, dir)) {
					out = append(out, Violation{Rule: RuleOneImpl, File: p, Line: line,
						Msg: fmt.Sprintf("%s implemented outside its owner at %s:%d; add it to the row's duplicates with a design-fit finding, or move it to the owner", id, p, line)})
				}
			}
		}
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	return ms, out, nil
}

// Ratchet applies the counting half of R-one-implementation: per index row, the marker
// count is at or under its ceiling (an S- id with no ceiling line has ceiling 0), and a row
// whose owner path lies inside scope carries at least one marker there. A ceiling line
// naming no index row is also a violation. Rows whose owner lies outside scope (another
// module, or no declared owner) are returned in skipped, reported as could-not-check for
// the owner half rather than passed.
func Ratchet(ms []Marker, index []SRow, ceilings map[string]int, scope string) (out []Violation, skipped []string) {
	count := map[string]int{}
	for _, m := range ms {
		count[m.ID]++
	}
	known := map[string]bool{}
	for _, r := range index {
		known[r.ID] = true
		if n, c := count[r.ID], ceilings[r.ID]; n > c {
			out = append(out, Violation{Rule: RuleOneImpl,
				Msg: fmt.Sprintf("%s has %d declared implementations, over its markers.txt ceiling of %d", r.ID, n, c)})
		}
		var inScope []string
		for _, o := range sites(r.Owners, scope) {
			if under(o, scope) {
				inScope = append(inScope, o)
			}
		}
		if len(inScope) == 0 {
			skipped = append(skipped, r.ID)
			continue
		}
		declared := false
		for _, m := range ms {
			if m.ID == r.ID && coveredBy(m.File, inScope) {
				declared = true
				break
			}
		}
		if !declared {
			out = append(out, Violation{Rule: RuleOneImpl,
				Msg: fmt.Sprintf("%s's owner (%s) declares no implementation; mark the function that computes the meaning", r.ID, strings.Join(inScope, ", "))})
		}
	}
	ids := make([]string, 0, len(ceilings))
	for id := range ceilings {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		if !known[id] {
			out = append(out, Violation{Rule: RuleOneImpl,
				Msg: fmt.Sprintf("markers.txt has a ceiling for %s, which is not a row of the semantic index", id)})
		}
	}
	return out, skipped
}

// ParseList reads a list file: one entry per line, lines starting with "#" are comments.
// A blank line or an entry carrying whitespace is an error, so the file stays exactly
// comparable to `go list` output.
func ParseList(r io.Reader) ([]string, error) {
	var out []string
	sc := bufio.NewScanner(r)
	n := 0
	for sc.Scan() {
		n++
		l := strings.TrimSuffix(sc.Text(), "\r") // a CRLF checkout reads the same list
		if strings.HasPrefix(l, "#") {
			continue
		}
		if l == "" || strings.ContainsAny(l, " \t") {
			return nil, fmt.Errorf("line %d: %q is neither a comment nor a single entry", n, l)
		}
		out = append(out, l)
	}
	return out, sc.Err()
}

// ParseCeilings reads markers.txt: "<S-id> <ceiling>" per line, "#" comments.
func ParseCeilings(r io.Reader) (map[string]int, error) {
	out := map[string]int{}
	sc := bufio.NewScanner(r)
	n := 0
	for sc.Scan() {
		n++
		l := strings.TrimSpace(sc.Text())
		if l == "" || strings.HasPrefix(l, "#") {
			continue
		}
		f := strings.Fields(l)
		if len(f) != 2 || !sIDRe.MatchString(f[0]) {
			return nil, fmt.Errorf("line %d: %q is not \"<S-id> <ceiling>\"", n, l)
		}
		c, err := strconv.Atoi(f[1])
		if err != nil || c < 0 {
			return nil, fmt.Errorf("line %d: ceiling %q is not a non-negative integer", n, f[1])
		}
		if _, dup := out[f[0]]; dup {
			return nil, fmt.Errorf("line %d: %s listed twice", n, f[0])
		}
		out[f[0]] = c
	}
	return out, sc.Err()
}

// --- helpers ---

const markerPrefix = "// semantic: "

var (
	sIDRe   = regexp.MustCompile(`^S-[a-z0-9]+(-[a-z0-9]+)*$`)
	codeRe  = regexp.MustCompile("`([^`]+)`")
	pathRe  = regexp.MustCompile(`^[A-Za-z0-9_.-]+(/[A-Za-z0-9_.-]+)+/?$`)
	lineSfx = regexp.MustCompile(`:[0-9]+(-[0-9]+)?$`)
)

// cellPaths returns every backticked path-shaped span in a table cell.
func cellPaths(cell string) []string {
	var out []string
	for _, m := range codeRe.FindAllStringSubmatch(cell, -1) {
		s := lineSfx.ReplaceAllString(strings.TrimSpace(m[1]), "")
		if pathRe.MatchString(s) {
			out = append(out, strings.TrimSuffix(s, "/"))
		}
	}
	return out
}

// splitRow splits a markdown table row on pipes that are not backslash-escaped, dropping
// the empty cells outside the leading and trailing pipe.
func splitRow(t string) []string {
	var cells []string
	var cur strings.Builder
	for i := 0; i < len(t); i++ {
		switch {
		case t[i] == '\\' && i+1 < len(t) && t[i+1] == '|':
			cur.WriteByte('|')
			i++
		case t[i] == '|':
			cells = append(cells, cur.String())
			cur.Reset()
		default:
			cur.WriteByte(t[i])
		}
	}
	cells = append(cells, cur.String())
	if len(cells) > 0 && strings.TrimSpace(cells[0]) == "" {
		cells = cells[1:]
	}
	if len(cells) > 0 && strings.TrimSpace(cells[len(cells)-1]) == "" {
		cells = cells[:len(cells)-1]
	}
	return cells
}

func isSeparator(cells []string) bool {
	for _, c := range cells {
		if strings.Trim(strings.TrimSpace(c), ":-") != "" {
			return false
		}
	}
	return true
}

// under reports whether p is dir or inside it.
func under(p, dir string) bool {
	return p == dir || strings.HasPrefix(p, dir+"/")
}

// sites drops the index paths that name no marker site relative to the walked root: the
// root itself, its cmd/ and internal/ directories, and anything above the root. Those are
// containers of every package, so a cell that mentions one in prose (a search it ran, the
// module it looked in) would otherwise make every function in the tree a listed site.
func sites(paths []string, root string) []string {
	var out []string
	for _, p := range paths {
		if under(root, p) || p == root+"/cmd" || p == root+"/internal" {
			continue
		}
		out = append(out, p)
	}
	return out
}

// coveredBy reports whether file is one of paths, or sits directly in a directory one of
// paths names. A directory covers its own package only, never its subdirectories: a
// subpackage is a different package and needs its own entry.
func coveredBy(file string, paths []string) bool {
	for _, p := range paths {
		if file == p || path.Dir(file) == p {
			return true
		}
	}
	return false
}

// GrowAnnotated reports whether entry is a line of the list file lines and the run of
// comment lines directly above it carries a "# grow " line. A blank or non-comment line
// breaks the run, as it does for the weight ceiling's annotation.
func GrowAnnotated(lines []string, entry string) bool {
	for i, l := range lines {
		if strings.TrimSuffix(l, "\r") != entry {
			continue
		}
		for j := i - 1; j >= 0; j-- {
			c := strings.TrimSpace(lines[j])
			if !strings.HasPrefix(c, "#") {
				break
			}
			if strings.HasPrefix(c, "# grow ") {
				return true
			}
		}
		return false
	}
	return false
}

// verbOf returns the cmd/<x> prefix of a package under cmd/.
func verbOf(pkg string) string {
	parts := strings.SplitN(pkg, "/", 3)
	if len(parts) < 2 {
		return pkg
	}
	return parts[0] + "/" + parts[1]
}

func relDir(root, dir string) string {
	if dir == root {
		return ""
	}
	return strings.TrimPrefix(dir, root+"/")
}

func (g Graph) pkgs() []string {
	out := make([]string, 0, len(g))
	for p := range g {
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}

func modulePath(fsys fs.FS, modDir string) (string, error) {
	b, err := fs.ReadFile(fsys, path.Join(modDir, "go.mod"))
	if err != nil {
		return "", fmt.Errorf("reading the module path: %w", err)
	}
	for _, l := range strings.Split(string(b), "\n") {
		if f := strings.Fields(l); len(f) == 2 && f[0] == "module" {
			return strings.Trim(f[1], `"`), nil
		}
	}
	return "", fmt.Errorf("%s/go.mod declares no module path", modDir)
}

// walkGo calls fn for every non-test .go file under dir, in lexical order, with the go
// tool's directory skips. A symlinked file is refused rather than followed, so the walk
// never reads outside the tree it was given.
func walkGo(fsys fs.FS, dir string, fn func(p string, src []byte) error) error {
	return fs.WalkDir(fsys, dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if p == dir {
				return nil
			}
			n := d.Name()
			if n == "testdata" || n == "vendor" || strings.HasPrefix(n, ".") || strings.HasPrefix(n, "_") {
				return fs.SkipDir
			}
			if _, serr := fs.Stat(fsys, path.Join(p, "go.mod")); serr == nil {
				return fs.SkipDir // a nested module is not this module's package
			}
			return nil
		}
		if !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
			return nil
		}
		if !d.Type().IsRegular() {
			return fmt.Errorf("%s: not a regular file; the walk never follows links", p)
		}
		src, rerr := fs.ReadFile(fsys, p)
		if rerr != nil {
			return rerr
		}
		return fn(p, src)
	})
}
