package deskkit

// publishidentity_class_test.go — the class guard for issue #1967.
//
// THE CLASS. A caller of the publish-identity gate decides WHICH commits are judged. #1967 was
// one such caller (deskpr update) judging commits the remote PR head already held, so a
// mixed-author PR could never be updated through the gate. The opposite failure is the
// dangerous one: a caller offering a remote tip that is not really what the remote holds,
// shrinking the range past a commit the push adds. Both are a property of the CALL SITE, not
// of the gate, so the gate's own tests cannot see a new one.
//
// THE GUARD. An AST walk of every shipped (non-test, non-testdata) Go file under tools/desk
// enumerates each site that reaches the gate — a PublishIdentityInput literal, a direct call
// of PublishIdentityMatchesRole or a publishIdentityGateFn seam, and a call of a
// publishIdentityGate wrapper — together with the remote-tip expression it passes. Every
// literal must key RemoteTip explicitly (no caller inherits the zero value by omission), and
// the set of sites must equal publishIdentitySites below, each entry carrying the reason its
// tip is safe. A new caller, or a changed tip expression, fails until it is reviewed and
// listed; a listed site that disappears fails too, so the list cannot go stale.
//
// POSITIVE CONTROL. testdata/publishidentityclass/planted.go is a second instance of the
// class (an unlisted wrapper call and a literal that omits RemoteTip); the guard must flag
// both, or the scanner is proven blind and the clean verdict on the real tree means nothing.

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// publishIdentitySites is the reviewed register: site key → why its remote tip is safe.
// Key: "<path from tools/desk>|<enclosing func>|<kind>|<tip expression>".
var publishIdentitySites = map[string]string{
	// The wrappers: the only literals. Each passes its tip through explicitly.
	"cmd/deskpr/exec.go|publishIdentityGate|literal|remoteTip": "deskpr wrapper; the tip is chosen per call site below",
	`cmd/deskevidence/github.go|publishIdentityGate|literal|""`: "deskevidence writes Evidence to the base branch itself; " +
		"origin/<base>..HEAD is already exactly what the remote lacks",
	"cmd/deskpr/exec.go|publishIdentityGate|direct|-":         "the deskpr wrapper calls its seam",
	"cmd/deskevidence/github.go|publishIdentityGate|direct|-": "the deskevidence wrapper calls its seam",

	// deskpr's callers.
	`cmd/deskpr/deskpr.go|cmdCreate|call|""`: "create opens a NEW change: no PR head exists to anchor on, and no forge " +
		"branch-head read exists to confirm a local ref — the whole range is judged",
	"cmd/deskpr/deskpr.go|cmdUpdate|call|offlineTip": "offline/--check stage: remoteTrackingTip(facts.branch), a local " +
		"estimate used only if it resolves and is an ancestor of HEAD, re-judged by the live stage before any push; " +
		"under `update --pr N` the PR's head ref is unknown offline, so offlineTip is \"\" and the whole range is judged " +
		"(stricter, never narrower)",
	"cmd/deskpr/deskpr.go|cmdUpdate|call|full.HeadSHA": "live stage: the forge's own PR head, judged immediately before " +
		"the plain (never forced) push; unknown or non-ancestor widens to the whole range",

	// deskevidence's callers: its wrapper takes no tip at all.
	"cmd/deskevidence/deskevidence.go|cmdEvidence|call|-":            "deskevidence wrapper fixes RemoteTip to \"\"",
	"cmd/deskevidence/outcomerecord.go|cmdOutcomeRecordWrite|call|-": "deskevidence wrapper fixes RemoteTip to \"\"",
}

// publishIdentitySite is one place the gate is reached.
type publishIdentitySite struct {
	key     string
	pos     string
	missing bool // a PublishIdentityInput literal that does not key RemoteTip
}

func (s publishIdentitySite) String() string { return s.pos + " " + s.key }

// scanPublishIdentitySites walks root's shipped Go files (skipping _test.go, testdata and
// vendor) and returns every site that reaches the gate. rel paths are relative to root.
func scanPublishIdentitySites(root string) ([]publishIdentitySite, int, error) {
	var sites []publishIdentitySite
	files := 0
	fset := token.NewFileSet()
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, werr error) error {
		if werr != nil {
			return werr
		}
		if d.IsDir() {
			if n := d.Name(); path != root && (n == "testdata" || n == "vendor" || strings.HasPrefix(n, ".")) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		f, perr := parser.ParseFile(fset, path, nil, 0)
		if perr != nil {
			return perr
		}
		files++
		rel, _ := filepath.Rel(root, path)
		sites = append(sites, publishIdentitySitesIn(fset, f, filepath.ToSlash(rel))...)
		return nil
	})
	return sites, files, err
}

// publishIdentitySitesIn returns the sites in one parsed file.
func publishIdentitySitesIn(fset *token.FileSet, f *ast.File, rel string) []publishIdentitySite {
	var sites []publishIdentitySite
	src := func(e ast.Expr) string {
		var b bytes.Buffer
		_ = printer.Fprint(&b, fset, e)
		return b.String()
	}
	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			add := func(kind, tip string, missing bool) {
				sites = append(sites, publishIdentitySite{
					key:     rel + "|" + fn.Name.Name + "|" + kind + "|" + tip,
					pos:     fset.Position(n.Pos()).String(),
					missing: missing,
				})
			}
			switch x := n.(type) {
			case *ast.CompositeLit:
				if typeName(x.Type) != "PublishIdentityInput" {
					return true
				}
				tip, found := "", false
				for _, el := range x.Elts {
					kv, ok := el.(*ast.KeyValueExpr)
					if !ok {
						continue
					}
					if id, ok := kv.Key.(*ast.Ident); ok && id.Name == "RemoteTip" {
						tip, found = src(kv.Value), true
					}
				}
				if !found {
					tip = "<RemoteTip not keyed>"
				}
				add("literal", tip, !found)
			case *ast.CallExpr:
				switch typeName(x.Fun) {
				case "PublishIdentityMatchesRole", "publishIdentityGateFn", "productionPublishIdentityGateFn":
					add("direct", "-", false)
				case "publishIdentityGate":
					tip := "-"
					if len(x.Args) >= 3 {
						tip = src(x.Args[2])
					}
					add("call", tip, false)
				}
			}
			return true
		})
	}
	return sites
}

// typeName is the trailing identifier of an expression: Foo, pkg.Foo → "Foo".
func typeName(e ast.Expr) string {
	switch x := e.(type) {
	case *ast.Ident:
		return x.Name
	case *ast.SelectorExpr:
		return x.Sel.Name
	}
	return ""
}

// reconcilePublishIdentitySites returns the problems: unlisted sites, literals not keying
// RemoteTip, and listed sites no longer present.
func reconcilePublishIdentitySites(sites []publishIdentitySite, register map[string]string) []string {
	var problems []string
	seen := map[string]bool{}
	for _, s := range sites {
		seen[s.key] = true
		if s.missing {
			problems = append(problems, "PublishIdentityInput literal does not key RemoteTip (state it, even as \"\"): "+s.String())
		}
		if _, ok := register[s.key]; !ok {
			problems = append(problems, "UNLISTED publish-identity site — review its remote tip against the fail-closed "+
				"rules (publishidentity.go WHICH COMMITS) and add it to publishIdentitySites with the reason: "+s.String())
		}
	}
	for k := range register {
		if !seen[k] {
			problems = append(problems, "listed publish-identity site no longer present (remove it or restore the call): "+k)
		}
	}
	sort.Strings(problems)
	return problems
}

// TestPubIdentityCallersTip is the class guard over the real tree.
func TestPubIdentityCallersTip(t *testing.T) {
	sites, files, err := scanPublishIdentitySites(deskTreeRoot)
	if err != nil {
		t.Fatalf("could not scan the desk tree: %v — could-not-check, NOT clean", err)
	}
	if files == 0 || len(sites) == 0 {
		t.Fatalf("scanned %d file(s), found %d site(s) — a scanner that sees nothing certifies everything; re-point deskTreeRoot", files, len(sites))
	}
	if problems := reconcilePublishIdentitySites(sites, publishIdentitySites); len(problems) > 0 {
		t.Fatalf("publish-identity call sites do not match the reviewed register:\n  %s", strings.Join(problems, "\n  "))
	}
}

// TestPubClassGuardSeesPlant is the positive control: the planted second instance in
// testdata must be flagged, for both shapes of the class.
func TestPubClassGuardSeesPlant(t *testing.T) {
	dir := filepath.Join("testdata", "publishidentityclass")
	if _, err := os.Stat(filepath.Join(dir, "planted.go")); err != nil {
		t.Fatalf("positive-control fixture missing: %v", err)
	}
	sites, _, err := scanPublishIdentitySites(dir)
	if err != nil {
		t.Fatalf("scan the planted fixture: %v", err)
	}
	problems := strings.Join(reconcilePublishIdentitySites(sites, publishIdentitySites), "\n")
	for _, want := range []string{
		"UNLISTED publish-identity site", `planted.go|pushAgain|call|"refs/remotes/origin/HEAD"`,
		"does not key RemoteTip", "planted.go|quietGate|literal|<RemoteTip not keyed>",
	} {
		if !strings.Contains(problems, want) {
			t.Errorf("the guard did not flag the planted instance (%q); it reported:\n%s", want, problems)
		}
	}
}
