package deskkit

// redactguard_test.go — the CLASS guard for the per-package userinfo redactor copies.
//
// Defect class: a desk verb rendering a remote URL through its OWN userinfo redactor. Each
// copy re-derived the userinfo span its own way (cut at the first '/', kept the user half,
// skipped scp-like forms), so a fix to one never reached the others and a credential survived
// in whichever copy a message happened to use. RedactRemoteUserinfo (remoterepo.go) is the one
// engine; RedactRemoteURL and RedactRemoteConfigKey are its sanctioned renderings. This test
// walks every shipping .go file of the module and fails, naming the site, on
//   - a function whose name reads as a URL / userinfo / remote redactor, outside the home
//     files, and
//   - a string literal carrying the "<redacted>@" splice a hand-rolled copy writes.
// Its positive control is testdata/redactguard_plant.go.txt: a planted copy the guard MUST
// flag on both counts, so a matcher that silently stopped matching fails instead of reporting
// clean.

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// redactorHomes are the files allowed to define a userinfo redactor, relative to the module.
var redactorHomes = map[string]bool{
	"internal/deskkit/remoterepo.go":    true,
	"internal/deskkit/pushtransport.go": true,
}

// redactorNameRe matches a function name that reads as a remote / URL / userinfo redactor.
var redactorNameRe = regexp.MustCompile(`(?i)redact.*(url|userinfo|remote|cred)|(url|userinfo|remote|cred).*redact`)

// redactorOutOfClass are same-named helpers of a DIFFERENT class: they strip a URL out of a
// *url.Error's text (a request URL with a query), not a git remote's userinfo.
var redactorOutOfClass = map[string]bool{
	"cmd/deskfleet/client.go:redactURLError":          true,
	"internal/deskkit/forge_github.go:redactURLQuery": true,
}

// redactorSites parses one Go source and returns "file:line what" for every second-redactor
// site in it. rel is the file's module-relative path (it keys the homes and the exemptions).
func redactorSites(t *testing.T, path, rel string) []string {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	if redactorHomes[rel] {
		return nil
	}
	var out []string
	at := func(n ast.Node, what string) {
		out = append(out, fmt.Sprintf("%s:%d %s", rel, fset.Position(n.Pos()).Line, what))
	}
	ast.Inspect(f, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.FuncDecl:
			if redactorNameRe.MatchString(x.Name.Name) && !redactorOutOfClass[rel+":"+x.Name.Name] {
				at(x, "func "+x.Name.Name)
			}
		case *ast.BasicLit:
			if x.Kind == token.STRING && strings.Contains(x.Value, "<redacted>@") {
				at(x, "literal "+x.Value)
			}
		}
		return true
	})
	return out
}

// regression: #6696
func TestOneUserinfoRedactor(t *testing.T) {
	// Positive control first: a guard that cannot see a plant proves nothing below.
	planted := redactorSites(t, filepath.Join("testdata", "redactguard_plant.go.txt"), "cmd/example/plant.go")
	if len(planted) != 2 {
		t.Fatalf("positive control: want the planted func AND its splice literal flagged, got %d: %v", len(planted), planted)
	}

	root, err := filepath.Abs("../..") // internal/deskkit -> tools/desk
	if err != nil {
		t.Fatal(err)
	}
	var found []string
	err = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if path != root && (d.Name() == "testdata" || d.Name() == "vendor" || strings.HasPrefix(d.Name(), ".")) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		rel, rerr := filepath.Rel(root, path)
		if rerr != nil {
			return rerr
		}
		found = append(found, redactorSites(t, path, filepath.ToSlash(rel))...)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range found {
		t.Errorf("second userinfo redactor at %s — render a remote through deskkit.RedactRemoteUserinfo "+
			"(or RedactRemoteURL / RedactRemoteConfigKey), never a per-package copy", s)
	}
}
