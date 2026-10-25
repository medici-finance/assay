// Package purity implements the domain-purity lint (design §3.4, §11;
// decision-model spec §1.5).
//
// The rule: platform-shaped identifiers must not leak into the domain types,
// the tool names, or the UI templates. The domain says "escalation", not "the
// number GitHub filed it under" — one adapter, honest naming, so a second
// platform is a package boundary rather than a rewrite.
//
// This is a real AST lint, not a grep. A grep over a Go file cannot tell a
// field name from the word in a sentence, so it either misses violations
// inside struct tags or fires on prose and gets silenced. This walks
// declarations and reports STRUCTURE: field names, type names, method names,
// struct tags, and named constants.
//
// The pattern list is open by design (spec §1.5: "extends to any new
// platform-shaped field names"). Adding a pattern here is the intended way to
// extend it; there is no config file, because a lint whose severity is
// configurable is a lint that gets configured off.
package purity

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
)

// Pattern is one forbidden platform-shaped identifier.
type Pattern struct {
	// Ident is matched case-INSENSITIVELY against a normalised identifier
	// (underscores and hyphens removed), so `NodeID`, `node_id` and
	// `nodeId` are all the same violation.
	Ident string
	// Why explains what the leak would cost, so a reader hitting the
	// failure learns the rule rather than renaming around it.
	Why string
}

// Patterns is the forbidden list. Spec §1.5 enumerates three literals and
// then says the list keeps extending; these are those three plus the spellings
// the same concepts take in Go.
//
// `URL` alone is deliberately NOT here: a domain type may legitimately hold a
// link a human typed. What is forbidden is a field named for the PLATFORM's
// url of the platform's object.
var Patterns = []Pattern{
	{Ident: "issuenumber", Why: "the numeric key GitHub assigns an issue is platform-shaped; the domain carries an adapter-minted EscalationID (spec §1.5)"},
	{Ident: "issueid", Why: "same as the numeric key: a platform primary key by another name"},
	{Ident: "prnumber", Why: "the numeric key GitHub assigns a pull request; the domain carries a WorkItemRef"},
	{Ident: "pullrequestnumber", Why: "spelled-out form of the same platform key"},
	{Ident: "nodeid", Why: "GitHub's GraphQL global node id — explicitly named in spec §1.5"},
	{Ident: "htmlurl", Why: "the platform deep-link is presentation data attached at the serving edge as links.platform, never a domain field (spec §1.5)"},
	{Ident: "apiurl", Why: "a platform endpoint has no place in a platform-free type"},
	{Ident: "githubid", Why: "names the platform in the field itself"},
	{Ident: "githublogin", Why: "the domain has Actor; a login is the platform's spelling of it"},
	{Ident: "installationid", Why: "GitHub App installation identity is adapter/transport concern, not domain"},
	{Ident: "repositoryid", Why: "platform primary key for a repo; the domain names cells and repos by slug at the adapter edge"},
}

// Finding is one violation.
type Finding struct {
	File    string
	Line    int
	Kind    string // "field" | "type" | "method" | "func" | "const" | "struct-tag"
	Name    string
	Pattern Pattern
}

func (f Finding) String() string {
	return fmt.Sprintf("%s:%d: %s %q leaks the platform-shaped identifier %q — %s",
		f.File, f.Line, f.Kind, f.Name, f.Pattern.Ident, f.Pattern.Why)
}

// normalise strips the separators that let the same concept wear three
// spellings, and lowercases. `Node_ID` and `nodeId` both become `nodeid`.
func normalise(s string) string {
	r := strings.NewReplacer("_", "", "-", "", ".", "")
	return strings.ToLower(r.Replace(s))
}

// match returns the pattern an identifier violates, if any.
//
// Matching is on SUBSTRING of the normalised identifier, so `ParentNodeID`
// and `nodeIDs` are both caught. That is intentionally aggressive: a
// false positive is renamed in one line, a false negative is the leak the
// lint exists to prevent.
func match(ident string) (Pattern, bool) {
	n := normalise(ident)
	for _, p := range Patterns {
		if strings.Contains(n, p.Ident) {
			return p, true
		}
	}
	return Pattern{}, false
}

// CheckDir lints every non-test .go file under dir (recursively) and returns
// the findings, sorted for stable output.
//
// Test files are skipped: a test may legitimately construct a platform
// payload to prove the ADAPTER strips it. What must stay clean is the
// shipped declaration surface.
func CheckDir(dir string) ([]Finding, error) {
	var findings []Finding
	fset := token.NewFileSet()

	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		file, perr := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
		if perr != nil {
			return fmt.Errorf("parse %s: %w", path, perr)
		}
		findings = append(findings, checkFile(fset, path, file)...)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(findings, func(i, j int) bool {
		if findings[i].File != findings[j].File {
			return findings[i].File < findings[j].File
		}
		return findings[i].Line < findings[j].Line
	})
	return findings, nil
}

func checkFile(fset *token.FileSet, path string, file *ast.File) []Finding {
	var out []Finding
	report := func(pos token.Pos, kind, name string, p Pattern) {
		out = append(out, Finding{File: path, Line: fset.Position(pos).Line, Kind: kind, Name: name, Pattern: p})
	}

	ast.Inspect(file, func(n ast.Node) bool {
		switch v := n.(type) {
		case *ast.TypeSpec:
			if p, bad := match(v.Name.Name); bad {
				report(v.Pos(), "type", v.Name.Name, p)
			}
		case *ast.FuncDecl:
			kind := "func"
			if v.Recv != nil {
				kind = "method"
			}
			if p, bad := match(v.Name.Name); bad {
				report(v.Pos(), kind, v.Name.Name, p)
			}
		case *ast.ValueSpec:
			for _, name := range v.Names {
				if p, bad := match(name.Name); bad {
					report(name.Pos(), "const", name.Name, p)
				}
			}
		case *ast.Field:
			for _, name := range v.Names {
				if p, bad := match(name.Name); bad {
					report(name.Pos(), "field", name.Name, p)
				}
			}
			// Struct tags carry the JSON name a consumer actually sees,
			// so a field renamed to dodge the lint but tagged
			// `json:"html_url"` is still a leak — and is the exact shape
			// a careless rename takes.
			if v.Tag != nil {
				if raw, err := strconv.Unquote(v.Tag.Value); err == nil {
					for _, key := range []string{"json", "yaml", "db"} {
						tagVal := reflect.StructTag(raw).Get(key)
						name, _, _ := strings.Cut(tagVal, ",")
						if name == "" || name == "-" {
							continue
						}
						if p, bad := match(name); bad {
							report(v.Tag.Pos(), "struct-tag", key+":"+name, p)
						}
					}
				}
			}
		}
		return true
	})
	return out
}
