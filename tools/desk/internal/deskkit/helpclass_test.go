package deskkit

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// helpclass_test.go — the CLASS guard for the tier-two help rule (helprequest.go).
//
// DEFECT CLASS. A desk verb whose subcommand parses flags with a `flag.ContinueOnError`
// FlagSet reports `--help` as flag.ErrHelp. Unless the parse site hands that error to
// IsHelpRequest, the verb wraps it as a refusal (exit 5 or 2, not 0), and where the verb has
// an audit finalizer that writes a row for any error, charges a `refused` row to a ledger the
// write budget counts and nothing rotates. The first fix landed verb by verb (deskpr,
// deskfile, deskwt) and a per-verb test only exists for verbs someone remembered to write one
// for: deskreply, desktoken and deskpost were missed, and sibling subcommands of an
// already-fixed verb (deskwt remove, deskwt prune) were missed the same way.
//
// THE GUARD walks every command package under cmd/ and applies two structural rules:
//
//  1. a package that declares an audit finalizer (a method named `finalize`) must have that
//     finalizer consult IsHelpRequest, so a help error writes no row; and
//  2. in such a package (or one named in helpClassNoFinalizer), every function that builds a
//     `flag.ContinueOnError` FlagSet must call IsHelpRequest — directly, or through a
//     same-package function that does — so the parse error is recognised, not wrapped.
//
// The only exemption is an entry in helpClassExempt, which names the site and the reason and
// goes stale (and fails) the moment the site is fixed, so the list can only shrink.
//
// POSITIVE CONTROL. helpClassViolations is a pure function over source text, so
// TestHelpClassGuardSeesPlanted feeds it planted instances and requires them to be flagged. A
// matcher that silently stopped matching fails that test instead of reporting the tree clean.

// helpClassExempt lists flag-parse sites KNOWN to lack tier-two recognition that sit outside
// the six verbs the help retrofit named (deskpr, deskfile, deskwt, deskreply, desktoken,
// deskpost). Key: "<pkg>/<Func>". Each is an open gap, not a ruling: the entry makes the
// guard a ratchet — a NEW site fails, and a fixed site's stale entry fails too.
var helpClassExempt = map[string]string{
	"deskdigest/finalize":      "read-only digest verb; not one of the six verbs of the help retrofit — open gap",
	"deskdigest/parseOpts":     "read-only digest verb; not one of the six verbs of the help retrofit — open gap",
	"deskevidence/finalize":    "verify-desk Evidence verb; not one of the six verbs of the help retrofit — open gap",
	"deskevidence/cmdEvidence": "verify-desk Evidence verb; not one of the six verbs of the help retrofit — open gap",
	"deskgit/finalize":         "git transport verb; not one of the six verbs of the help retrofit — open gap",
	"deskgit/cmdFetch":         "git transport verb; not one of the six verbs of the help retrofit — open gap",
	"deskgit/cmdPush":          "git transport verb; not one of the six verbs of the help retrofit — open gap",
	"desksupervise/finalize":   "supervisor verb with its own bare-help pre-check; not one of the six verbs of the help retrofit — open gap",
	"desksupervise/cmdRun":     "supervisor verb with its own bare-help pre-check; not one of the six verbs of the help retrofit — open gap",
	"desksupervise/cmdStatus":  "supervisor verb with its own bare-help pre-check; not one of the six verbs of the help retrofit — open gap",
	"desksupervise/cmdStop":    "supervisor verb with its own bare-help pre-check; not one of the six verbs of the help retrofit — open gap",
	"desksupervise/cmdTick":    "supervisor verb with its own bare-help pre-check; not one of the six verbs of the help retrofit — open gap",
}

// helpClassNoFinalizer names audited verbs that have no shared `finalize` method but still
// parse subcommand flags, so rule 2 applies to them too. deskpost writes its rows inside each
// run* verb, after the flags are parsed: a help request there is not an audit-row defect, it
// is the exit-code defect (a parse error returned exit 2 instead of ExitOK).
var helpClassNoFinalizer = map[string]bool{"deskpost": true}

// helpClassViolations returns one line per violation in the given package source files
// (file name -> source). pkg is the command package's directory name.
func helpClassViolations(t *testing.T, pkg string, srcs map[string]string) []string {
	t.Helper()
	fset := token.NewFileSet()
	names := make([]string, 0, len(srcs))
	for n := range srcs {
		names = append(names, n)
	}
	sort.Strings(names)
	var files []*ast.File
	for _, n := range names {
		f, err := parser.ParseFile(fset, n, srcs[n], 0)
		if err != nil {
			t.Fatalf("parse %s/%s: %v", pkg, n, err)
		}
		files = append(files, f)
	}

	var out []string
	hasFinalize := false
	ctors := map[string]bool{}       // functions returning *flag.FlagSet (constructors)
	recognisers := map[string]bool{} // plain functions that themselves call IsHelpRequest
	for _, f := range files {
		for _, d := range f.Decls {
			fd, ok := d.(*ast.FuncDecl)
			if !ok || fd.Body == nil {
				continue
			}
			if fd.Recv != nil {
				if fd.Name.Name == "finalize" {
					hasFinalize = true
					if !callsIsHelpRequest(fd.Body) {
						out = append(out, pkg+"/finalize: the audit finalizer never consults deskkit.IsHelpRequest, so a help request is logged as a refusal")
					}
				}
				continue
			}
			if returnsFlagSet(fd) {
				ctors[fd.Name.Name] = true
			}
			if callsIsHelpRequest(fd.Body) {
				recognisers[fd.Name.Name] = true
			}
		}
	}
	if !hasFinalize && !helpClassNoFinalizer[pkg] {
		return out // an unaudited package writes no per-subcommand row here
	}

	for _, f := range files {
		for _, d := range f.Decls {
			fd, ok := d.(*ast.FuncDecl)
			if !ok || fd.Body == nil || ctors[fd.Name.Name] || !buildsContinueOnErrorFlagSet(fd.Body, ctors) {
				continue
			}
			if !callsIsHelpRequest(fd.Body) && !callsAny(fd.Body, recognisers) {
				out = append(out, pkg+"/"+fd.Name.Name+": builds a flag.ContinueOnError FlagSet but never calls deskkit.IsHelpRequest on the parse error")
			}
		}
	}
	return out
}

// callsIsHelpRequest reports whether body contains a call to IsHelpRequest (qualified or not).
func callsIsHelpRequest(body ast.Node) bool {
	found := false
	ast.Inspect(body, func(n ast.Node) bool {
		if ce, ok := n.(*ast.CallExpr); ok {
			switch fn := ce.Fun.(type) {
			case *ast.SelectorExpr:
				found = found || fn.Sel.Name == "IsHelpRequest"
			case *ast.Ident:
				found = found || fn.Name == "IsHelpRequest"
			}
		}
		return !found
	})
	return found
}

// callsAny reports whether body calls a plain (unqualified) function named in names.
func callsAny(body ast.Node, names map[string]bool) bool {
	found := false
	ast.Inspect(body, func(n ast.Node) bool {
		if ce, ok := n.(*ast.CallExpr); ok {
			if id, ok := ce.Fun.(*ast.Ident); ok && names[id.Name] {
				found = true
			}
		}
		return !found
	})
	return found
}

// returnsFlagSet reports whether fd's result list includes *flag.FlagSet: such a function is
// a constructor (deskfile's newFlagSet) that builds but never parses, so its CALLER is the
// parse site.
func returnsFlagSet(fd *ast.FuncDecl) bool {
	if fd.Type.Results == nil {
		return false
	}
	for _, r := range fd.Type.Results.List {
		if st, ok := r.Type.(*ast.StarExpr); ok {
			if se, ok := st.X.(*ast.SelectorExpr); ok && se.Sel.Name == "FlagSet" {
				return true
			}
		}
	}
	return false
}

// buildsContinueOnErrorFlagSet reports whether body calls flag.NewFlagSet(_, flag.ContinueOnError)
// directly, or calls one of the package's FlagSet constructors (ctors).
func buildsContinueOnErrorFlagSet(body ast.Node, ctors map[string]bool) bool {
	found := false
	ast.Inspect(body, func(n ast.Node) bool {
		ce, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		if id, ok := ce.Fun.(*ast.Ident); ok && ctors[id.Name] {
			found = true
			return false
		}
		if sel, ok := ce.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "NewFlagSet" && len(ce.Args) == 2 {
			if a, ok := ce.Args[1].(*ast.SelectorExpr); ok && a.Sel.Name == "ContinueOnError" {
				found = true
			}
		}
		return !found
	})
	return found
}

// TestHelpClassGuardSeesPlanted is the positive control: a planted second instance of the
// defect MUST be flagged, and the recognising shapes MUST NOT be. Without this a matcher that
// stopped matching would let TestEveryAuditedVerbRecognisesHelp pass over an unguarded tree.
func TestHelpClassGuardSeesPlanted(t *testing.T) {
	planted := map[string]string{"planted.go": `package main

import "flag"

type auditCtx struct{}

func (a *auditCtx) finalize(err error) {
	if IsHelpRequest(err) {
		return
	}
}

func cmdPlanted(args []string) (err error) {
	fs := flag.NewFlagSet("planted", flag.ContinueOnError)
	if perr := fs.Parse(args); perr != nil {
		return perr
	}
	return nil
}
`}
	got := helpClassViolations(t, "plantedpkg", planted)
	if len(got) != 1 || !strings.Contains(got[0], "plantedpkg/cmdPlanted") {
		t.Fatalf("the planted flag-parse site was not flagged by name; violations = %q", got)
	}

	plantedFinalizer := map[string]string{"planted.go": `package main

type auditCtx struct{}

func (a *auditCtx) finalize(err error) { _ = err }
`}
	got = helpClassViolations(t, "plantedpkg", plantedFinalizer)
	if len(got) != 1 || !strings.Contains(got[0], "plantedpkg/finalize") {
		t.Fatalf("a finalizer that ignores help was not flagged; violations = %q", got)
	}

	// deskpost's shape: no finalizer, flagged only because the package is named.
	plantedNoFinalizer := map[string]string{"planted.go": `package main

import "flag"

func cmdPlanted(args []string) int {
	fs := flag.NewFlagSet("planted", flag.ContinueOnError)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	return 0
}
`}
	if got := helpClassViolations(t, "unnamedpkg", plantedNoFinalizer); len(got) != 0 {
		t.Fatalf("an unaudited, unnamed package was flagged: %q", got)
	}
	if got := helpClassViolations(t, "deskpost", plantedNoFinalizer); len(got) != 1 || !strings.Contains(got[0], "deskpost/cmdPlanted") {
		t.Fatalf("a flag-parse site in a named no-finalizer package was not flagged; violations = %q", got)
	}

	clean := map[string]string{"clean.go": `package main

import "flag"

type auditCtx struct{}

func (a *auditCtx) finalize(err error) {
	if deskkit.IsHelpRequest(err) {
		return
	}
}

func newFlagSet(name string) *flag.FlagSet {
	return flag.NewFlagSet(name, flag.ContinueOnError)
}

func parseFailCode(err error) int {
	if deskkit.IsHelpRequest(err) {
		return 0
	}
	return 2
}

func cmdDirect(args []string) (err error) {
	fs := flag.NewFlagSet("clean", flag.ContinueOnError)
	if perr := fs.Parse(args); perr != nil {
		if deskkit.IsHelpRequest(perr) {
			return deskkit.ErrHelpRequested
		}
		return perr
	}
	return nil
}

func cmdViaCtor(args []string) int {
	fs := newFlagSet("clean")
	if perr := fs.Parse(args); perr != nil {
		return parseFailCode(perr)
	}
	return 0
}
`}
	if got := helpClassViolations(t, "cleanpkg", clean); len(got) != 0 {
		t.Fatalf("the recognising shapes were flagged: %q", got)
	}

	// The constructor route must still catch a caller that skips recognition.
	viaCtor := map[string]string{"c.go": `package main

import "flag"

type auditCtx struct{}

func (a *auditCtx) finalize(err error) {
	if IsHelpRequest(err) {
		return
	}
}

func newFlagSet(name string) *flag.FlagSet { return flag.NewFlagSet(name, flag.ContinueOnError) }

func cmdSkips(args []string) error {
	fs := newFlagSet("x")
	return fs.Parse(args)
}
`}
	if got := helpClassViolations(t, "ctorpkg", viaCtor); len(got) != 1 || !strings.Contains(got[0], "ctorpkg/cmdSkips") {
		t.Fatalf("a caller of a FlagSet constructor that skips recognition was not flagged; violations = %q", got)
	}
}

// TestEveryAuditedVerbRecognisesHelp walks the real command tree.
func TestEveryAuditedVerbRecognisesHelp(t *testing.T) {
	dirs, err := filepath.Glob(filepath.Join("..", "..", "cmd", "*"))
	if err != nil {
		t.Fatalf("glob the command tree: %v", err)
	}
	var all []string
	scanned, audited := 0, 0
	for _, dir := range dirs {
		if st, serr := os.Stat(dir); serr != nil || !st.IsDir() {
			continue
		}
		srcs := map[string]string{}
		entries, _ := os.ReadDir(dir)
		for _, e := range entries {
			n := e.Name()
			if e.IsDir() || !strings.HasSuffix(n, ".go") || strings.HasSuffix(n, "_test.go") {
				continue
			}
			b, rerr := os.ReadFile(filepath.Join(dir, n))
			if rerr != nil {
				t.Fatalf("read %s: %v", filepath.Join(dir, n), rerr)
			}
			srcs[n] = string(b)
		}
		if len(srcs) == 0 {
			continue
		}
		scanned++
		for _, s := range srcs {
			if strings.Contains(s, ") finalize(") {
				audited++
				break
			}
		}
		all = append(all, helpClassViolations(t, filepath.Base(dir), srcs)...)
	}
	if scanned < 20 || audited < 6 {
		t.Fatalf("scanned %d packages (%d with an audit finalizer) — the enumeration is too small to prove anything", scanned, audited)
	}

	seen := map[string]bool{}
	var fresh []string
	for _, v := range all {
		key := v[:strings.Index(v, ":")]
		if _, ok := helpClassExempt[key]; ok {
			seen[key] = true
			continue
		}
		fresh = append(fresh, v)
	}
	if len(fresh) > 0 {
		t.Errorf("tier-two help recognition is missing at %d site(s) — on `--help` these exit non-zero and, "+
			"where the verb audits, record a refusal row (deskkit/helprequest.go):\n  %s\nFix: on "+
			"`deskkit.IsHelpRequest(perr)` return deskkit.ErrHelpRequested, make finalize skip it, and "+
			"print usage + exit 0 at the top level.", len(fresh), strings.Join(fresh, "\n  "))
	}
	for key, why := range helpClassExempt {
		if !seen[key] {
			t.Errorf("helpClassExempt[%q] (%s) is stale — the site no longer violates the rule; delete the entry", key, why)
		}
	}
}
