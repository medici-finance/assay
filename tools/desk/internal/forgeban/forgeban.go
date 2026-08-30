// Package forgeban is the shell-exec ban: the structural check that no desk tool reaches a
// forge by invoking a forge command-line client (`gh`, `glab`, …).
//
// WHY IT EXISTS. The `Forge` interface (deskkit/forge.go) is a CLOSED surface — the dozen-odd
// operations a shipping desk tool consumes, each with a typed signature and a library-backed
// implementation on both backends. Shelling `gh` re-opens the entire CLI in one line and
// defeats that fence: the argv is unenumerated, the identity is whatever ambient credential
// the runner happens to carry, and the operation set becomes "whatever the installed binary
// version supports". The enumerated surface is only a control if reaching around it FAILS.
//
// HOW IT CHECKS. Two independent mechanisms, because a single-pattern scan is one novel
// spelling away from a false clean:
//
//	Layer 1 (structural). Every `os/exec` launch site is located through the file's own
//	import set — an aliased or dot-imported `os/exec` is resolved, not missed — and its
//	argv[0] is resolved to a compile-time constant where one exists. A constant that names a
//	forge CLI is an INVOCATION. A non-constant argv[0] is UNRESOLVED: reported as itself,
//	never rounded to clean, and carried in its own register so that hiding a `gh` behind a
//	variable still requires an in-tree edit.
//
//	Layer 2 (lexical, position-aware). A bare forge-CLI string literal anywhere in shipped
//	source is an invocation UNLESS it sits in a position that cannot launch anything — a map
//	or struct key, a switch case, an equality comparison, or an index expression. That is what
//	catches the launch shapes layer 1 cannot resolve: a wrapper called as `runCmd("gh", …)`,
//	a method value `g.run("gh", …)`, an argv assembled as `[]string{"gh", "api", …}` and then
//	splatted. It is deliberately NOT a naive grep: askassay's read-only guard vocabulary
//	(`"gh": true`, `case "gh":`, `f[0] != "gh"`) is a non-invocation reference and stays clean
//	without an allowance, exactly as the closed-forge-surface brief requires.
//
// Neither layer can be satisfied by weakening the other: layer 1 fires on the exec site,
// layer 2 on the name, and a callsite must evade BOTH to land unnoticed.
//
// Test files are excluded by construction (a fixture that spells a banned argv is the ban's
// own evidence, not a violation of it), as are vendor and testdata trees.
package forgeban

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// ForgeCLIs is the closed set of forge command-line clients desk tools must never invoke.
// It is a set of BINARY names, matched on the basename of a resolved argv[0], so an absolute
// path (`/opt/homebrew/bin/gh`) is the same finding as a bare name.
//
// It is an allow-nothing list rather than a deny-list of known-bad verbs: the ban is on
// reaching a forge through a CLI at all, not on particular subcommands, because the whole
// point of the enumerated surface is that the operation set is the interface's and not the
// binary's.
var ForgeCLIs = map[string]string{
	"gh":   "the GitHub CLI",
	"glab": "the GitLab CLI",
	"hub":  "the legacy GitHub CLI",
	"tea":  "the Gitea CLI",
	"bb":   "the Bitbucket CLI",
}

// Kind classifies a finding. The three-state rule applies: an exec site whose argv[0] could
// not be resolved is reported AS could-not-check (KindUnresolved), never rounded up to clean
// and never reported as an invocation it was not observed to be.
type Kind string

const (
	// KindInvocation is a launch whose argv[0] resolves to a forge CLI (layer 1), or a bare
	// forge-CLI name in a position that can launch one (layer 2). This is the ban.
	KindInvocation Kind = "invocation"
	// KindUnresolved is an exec site whose argv[0] is not a compile-time constant. It is not
	// an invocation of a forge CLI — it is a site where this checker CANNOT SEE what is
	// launched, which is the only shape a re-introduced callsite could hide in.
	KindUnresolved Kind = "unresolved-argv"
)

// Finding is one located site. Key() deliberately omits the line number: a key that moves
// every time an unrelated line is inserted above it turns the allowlist into a source of
// spurious CI reds, and a list nobody trusts is a list that gets deleted.
type Finding struct {
	File string // path relative to the scanned root
	Line int
	Func string // enclosing function, method (`recv.Name`) or var declaration
	Bin  string // resolved argv[0] basename; empty for KindUnresolved
	Via  string // the call expression that launches, e.g. "exec.CommandContext" or "runCmd"
	Kind Kind
}

// Key identifies a finding for allowlisting: file, enclosing declaration, binary. Stable
// across edits that shift line numbers; distinct for two different call sites in two
// different functions of one file.
func (f Finding) Key() string {
	bin := f.Bin
	if bin == "" {
		bin = "<unresolved>"
	}
	return f.File + "::" + f.Func + "::" + bin
}

func (f Finding) String() string {
	return fmt.Sprintf("%s:%d (%s) %s via %s: %s", f.File, f.Line, f.Func, f.Kind, f.Via, f.Key())
}

// Scan walks root (a directory tree of Go source — in practice `tools/desk`) and returns
// every finding, sorted by file then line.
//
// It returns an error rather than an empty slice when it cannot read or parse the tree: a
// scanner that silently read nothing passes everything, which is the failure mode this whole
// package exists to prevent.
func Scan(root string) ([]Finding, error) {
	root = filepath.Clean(root)
	var files []string
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case "vendor", "testdata", ".git", "node_modules":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".go") || strings.HasSuffix(d.Name(), "_test.go") {
			return nil
		}
		files = append(files, p)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walking %s: %w", root, err)
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("no shipped Go source found under %s — a scan that reads nothing clears everything", root)
	}
	sort.Strings(files)

	fset := token.NewFileSet()
	var out []Finding
	for _, p := range files {
		src, rerr := os.ReadFile(p) //nolint:gosec // path comes from the walk of the scanned root
		if rerr != nil {
			return nil, fmt.Errorf("reading %s: %w", p, rerr)
		}
		f, perr := parser.ParseFile(fset, p, src, parser.ParseComments)
		if perr != nil {
			return nil, fmt.Errorf("parsing %s: %w", p, perr)
		}
		rel, relErr := filepath.Rel(root, p)
		if relErr != nil {
			rel = p
		}
		out = append(out, scanFile(fset, f, filepath.ToSlash(rel))...)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].File != out[j].File {
			return out[i].File < out[j].File
		}
		if out[i].Line != out[j].Line {
			return out[i].Line < out[j].Line
		}
		return out[i].Kind < out[j].Kind
	})
	return out, nil
}

// scanFile runs both layers over one parsed file and de-duplicates by (line, kind): a direct
// `exec.Command("gh", …)` is seen by both layers and is one finding, not two.
func scanFile(fset *token.FileSet, f *ast.File, rel string) []Finding {
	execNames, dotImported := execImportNames(f)
	consts := packageStringConsts(f)

	seen := map[string]bool{}
	var out []Finding
	add := func(fnd Finding) {
		k := fmt.Sprintf("%d/%s", fnd.Line, fnd.Kind)
		if seen[k] {
			return
		}
		seen[k] = true
		out = append(out, fnd)
	}

	walk(f, func(n ast.Node, stack []ast.Node) {
		switch node := n.(type) {
		case *ast.CallExpr:
			// --- Layer 1: os/exec launch sites ---
			if via, argIdx, ok := execLaunch(node, execNames, dotImported); ok {
				bin, resolved := resolveArg(node.Args, argIdx, consts)
				line := fset.Position(node.Pos()).Line
				switch {
				case !resolved:
					add(Finding{File: rel, Line: line, Func: enclosing(stack), Via: via, Kind: KindUnresolved})
				case ForgeCLIs[bin] != "":
					add(Finding{File: rel, Line: line, Func: enclosing(stack), Bin: bin, Via: via, Kind: KindInvocation})
				}
			}
		case *ast.BasicLit:
			// --- Layer 2: a bare forge-CLI name in a launchable position ---
			if node.Kind != token.STRING {
				return
			}
			v, uerr := strconv.Unquote(node.Value)
			if uerr != nil || ForgeCLIs[v] == "" {
				return
			}
			if nonLaunchingPosition(node, stack) {
				return
			}
			add(Finding{
				File: rel, Line: fset.Position(node.Pos()).Line, Func: enclosing(stack),
				Bin: v, Via: launchVia(stack), Kind: KindInvocation,
			})
		}
	})
	return out
}

// execImportNames returns the local names under which os/exec is imported in this file, and
// whether it is dot-imported. Resolving through the file's own import set is what makes an
// alias (`import xc "os/exec"`) or a dot import indistinguishable from the ordinary spelling
// — a checker keyed to the literal token `exec.` is blind to both.
func execImportNames(f *ast.File) (map[string]bool, bool) {
	names := map[string]bool{}
	dot := false
	for _, imp := range f.Imports {
		p, err := strconv.Unquote(imp.Path.Value)
		if err != nil || p != "os/exec" {
			continue
		}
		switch {
		case imp.Name == nil:
			names["exec"] = true
		case imp.Name.Name == ".":
			dot = true
		case imp.Name.Name == "_":
			// blank import launches nothing
		default:
			names[imp.Name.Name] = true
		}
	}
	return names, dot
}

// execLaunchers maps an os/exec constructor to the index of its argv[0] argument.
// LookPath is included: it exists only to decide whether a binary can be invoked, so a
// LookPath of a forge CLI is a forge-CLI dependency even when the launch is elsewhere.
var execLaunchers = map[string]int{
	"Command":        0,
	"CommandContext": 1,
	"LookPath":       0,
}

// execLaunch reports whether call is an os/exec launch site, naming it and the index of its
// argv[0].
func execLaunch(call *ast.CallExpr, execNames map[string]bool, dotImported bool) (string, int, bool) {
	switch fn := call.Fun.(type) {
	case *ast.SelectorExpr:
		pkg, ok := fn.X.(*ast.Ident)
		if !ok || !execNames[pkg.Name] {
			return "", 0, false
		}
		if idx, ok := execLaunchers[fn.Sel.Name]; ok {
			return pkg.Name + "." + fn.Sel.Name, idx, true
		}
	case *ast.Ident:
		if !dotImported {
			return "", 0, false
		}
		if idx, ok := execLaunchers[fn.Name]; ok {
			return fn.Name, idx, true
		}
	}
	return "", 0, false
}

// packageStringConsts collects file-level string constants so an argv[0] spelled as a named
// constant (`const ghBin = "gh"; exec.Command(ghBin, …)`) resolves rather than reporting as
// unresolved — an indirection one identifier deep is the cheapest possible evasion.
func packageStringConsts(f *ast.File) map[string]string {
	out := map[string]string{}
	for _, d := range f.Decls {
		gd, ok := d.(*ast.GenDecl)
		if !ok || gd.Tok != token.CONST {
			continue
		}
		for _, spec := range gd.Specs {
			vs, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			for i, name := range vs.Names {
				if i >= len(vs.Values) {
					continue
				}
				if lit, ok := vs.Values[i].(*ast.BasicLit); ok && lit.Kind == token.STRING {
					if v, err := strconv.Unquote(lit.Value); err == nil {
						out[name.Name] = v
					}
				}
			}
		}
	}
	return out
}

// resolveArg resolves the argument at idx to a binary basename. It reports resolved=false for
// anything that is not a compile-time string — which is the honest answer, and the one that
// lands the site in the unresolved register instead of clearing it.
func resolveArg(args []ast.Expr, idx int, consts map[string]string) (string, bool) {
	if idx >= len(args) {
		return "", false
	}
	switch a := args[idx].(type) {
	case *ast.BasicLit:
		if a.Kind != token.STRING {
			return "", false
		}
		v, err := strconv.Unquote(a.Value)
		if err != nil {
			return "", false
		}
		return path.Base(filepath.ToSlash(v)), true
	case *ast.Ident:
		if v, ok := consts[a.Name]; ok {
			return path.Base(filepath.ToSlash(v)), true
		}
	}
	return "", false
}

// nonLaunchingPosition reports whether a forge-CLI string literal sits somewhere that cannot
// launch a process. These four positions are the vocabulary of a GUARD that reasons about the
// name — askassay's read-only allow-list is the live example — and excluding them by POSITION
// rather than by file is what keeps the ban from being a naive grep that reviewers learn to
// suppress.
//
// Everything else — a call argument, a slice element, an assignment — is treated as launchable.
func nonLaunchingPosition(lit *ast.BasicLit, stack []ast.Node) bool {
	if len(stack) == 0 {
		return false
	}
	parent := stack[len(stack)-1]
	switch p := parent.(type) {
	case *ast.KeyValueExpr:
		// `"gh": true` in a map literal, or a keyed struct field. Only the KEY half is exempt:
		// a value position (`bin: "gh"`) still names a binary something will run.
		return p.Key == ast.Expr(lit)
	case *ast.CaseClause:
		return true
	case *ast.BinaryExpr:
		return p.Op == token.EQL || p.Op == token.NEQ
	case *ast.IndexExpr:
		return p.Index == ast.Expr(lit)
	}
	return false
}

// launchVia names the enclosing call for a layer-2 finding, so the report says which wrapper
// carries the name rather than only that the name is present.
func launchVia(stack []ast.Node) string {
	for i := len(stack) - 1; i >= 0; i-- {
		call, ok := stack[i].(*ast.CallExpr)
		if !ok {
			continue
		}
		return exprName(call.Fun)
	}
	return "(literal)"
}

func exprName(e ast.Expr) string {
	switch x := e.(type) {
	case *ast.Ident:
		return x.Name
	case *ast.SelectorExpr:
		return exprName(x.X) + "." + x.Sel.Name
	case *ast.CallExpr:
		return exprName(x.Fun) + "()"
	case *ast.IndexExpr:
		return exprName(x.X)
	}
	return "(expr)"
}

// enclosing names the declaration a node sits in: `Name` for a function, `recv.Name` for a
// method, the variable name for a func literal bound to a package-level var. It is the stable
// half of an allowlist key.
func enclosing(stack []ast.Node) string {
	name := ""
	for _, n := range stack {
		switch d := n.(type) {
		case *ast.FuncDecl:
			name = d.Name.Name
			if d.Recv != nil && len(d.Recv.List) > 0 {
				name = recvTypeName(d.Recv.List[0].Type) + "." + name
			}
		case *ast.ValueSpec:
			if name == "" && len(d.Names) > 0 {
				name = d.Names[0].Name
			}
		}
	}
	if name == "" {
		return "(file scope)"
	}
	return name
}

func recvTypeName(e ast.Expr) string {
	switch x := e.(type) {
	case *ast.StarExpr:
		return recvTypeName(x.X)
	case *ast.Ident:
		return x.Name
	case *ast.IndexExpr:
		return recvTypeName(x.X)
	}
	return "?"
}

// walk is ast.Inspect with a parent stack, which both layers need: layer 2's whole precision
// comes from knowing what a literal's parent node is.
func walk(root ast.Node, fn func(n ast.Node, stack []ast.Node)) {
	var stack []ast.Node
	ast.Inspect(root, func(n ast.Node) bool {
		if n == nil {
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
			return false
		}
		fn(n, stack)
		stack = append(stack, n)
		return true
	})
}
