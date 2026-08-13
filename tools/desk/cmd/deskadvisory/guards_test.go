package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// This file holds the guards that keep the rest of the suite from passing vacuously.
// deskadvisory's whole value is not overstating what it verified, and a test suite has
// the same failure mode as the tool: a limited view is never evidence about the world.
// If the suite enumerates something, it must be able to prove it could have seen an
// item it does not report.

// --- Positive control ---
//
// Every absence assertion in this package ("the guard found nothing") is worthless
// unless the same guard, over a tree that DOES contain the defect, reports it. This
// test plants the exact defect class the tool was built for — a Flux-evaluated bash
// default expansion in a ConfigMap-shipped script — and drives the SHIPPED example-k8s
// checkdef over it. If it does not fail, every clean-tree pass in this package is
// vacuous and the tool is decoration.

func TestPositiveControl_ShippedGrepGuardCatchesPlantedDefect(t *testing.T) {
	cl := mustLoadShippedCheckdef(t)

	guard := findCheck(t, cl, "ck-bash-default-expansion")
	only := &checkList{Version: 1, Checks: []checkSpec{*guard}}

	dir := t.TempDir()
	mustMkdir(t, filepath.Join(dir, "deploy", "scripts"))
	mustMkdir(t, filepath.Join(dir, "admin", "scripts"))
	mustWrite(t, filepath.Join(dir, "admin", "scripts", "other.sh"), "echo fine\n")
	// The planted defect: Flux's envsubst evaluates ${SRC:-…} and silently applies
	// the default in-cluster.
	mustWrite(t, filepath.Join(dir, "deploy", "scripts", "realm-import.sh"),
		"#!/bin/sh\nenvsubst < \"${SRC:-/dev/null}\" > out\n")

	if _, err := runChecks(only, dir); err == nil {
		t.Fatal("POSITIVE CONTROL FAILED: the shipped bash-default-expansion guard did not " +
			"detect a planted ${VAR:-default} in deploy/scripts. Every 'clean tree' pass in " +
			"this package is therefore vacuous.")
	}

	// And the same guard over the same tree with the defect removed must pass, or the
	// control proves only that the check always fails.
	mustWrite(t, filepath.Join(dir, "deploy", "scripts", "realm-import.sh"),
		"#!/bin/sh\nenvsubst < \"$SRC\" > out\n")
	if _, err := runChecks(only, dir); err != nil {
		t.Fatalf("the guard must PASS once the defect is removed, else it discriminates nothing: %v", err)
	}
}

func TestPositiveControl_ShippedCredentialGuardCatchesPlantedToken(t *testing.T) {
	cl := mustLoadShippedCheckdef(t)
	guard := findCheck(t, cl, "ck-credential-shaped")
	only := &checkList{Version: 1, Checks: []checkSpec{*guard}}

	dir := t.TempDir()
	for i := 0; i < guard.MinFiles; i++ {
		mustWrite(t, filepath.Join(dir, "f"+strconv.Itoa(i)+".txt"), "nothing here\n")
	}
	if _, err := runChecks(only, dir); err != nil {
		t.Fatalf("clean tree must pass the credential guard: %v", err)
	}

	// AKIA + 16 upper-alphanumerics: an AWS access-key id shape. Assembled at runtime
	// so this source file does not itself carry something credential-shaped.
	mustWrite(t, filepath.Join(dir, "leak.txt"), "AKIA"+strings.Repeat("A", 16)+"\n")
	if _, err := runChecks(only, dir); err == nil {
		t.Fatal("POSITIVE CONTROL FAILED: the shipped credential guard did not detect a " +
			"planted access-key-shaped string")
	}
}

// --- Inventory: every dispatcher case is classified somewhere ---
//
// Parses main.go's subcommand dispatcher with go/ast and requires every `case` label to
// be exercised by a `run([]string{…})` call in a test. A subcommand added without a test
// is a branch nobody has ever executed; enumerating them by hand in a slice would go
// stale the moment one is added, which is the defect this test exists to prevent.

func TestInventory_EveryDispatcherCaseIsExercised(t *testing.T) {
	cases := dispatcherCaseLabels(t)
	if len(cases) == 0 {
		t.Fatal("found no case labels in run()'s subcommand switch — the inventory guard " +
			"cannot see the dispatcher, so it would pass no matter what is missing")
	}

	exercised := subcommandsExercisedByTests(t)
	for _, c := range cases {
		if !exercised[c] {
			t.Errorf("subcommand %q is dispatched by run() but no test calls run() with it — "+
				"it is classified nowhere", c)
		}
	}
}

// dispatcherCaseLabels returns the string case labels of the `switch sub` in run().
func dispatcherCaseLabels(t *testing.T) []string {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "main.go", nil, 0)
	if err != nil {
		t.Fatalf("parse main.go: %v", err)
	}

	var labels []string
	ast.Inspect(f, func(n ast.Node) bool {
		fn, ok := n.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "run" {
			return true
		}
		ast.Inspect(fn, func(m ast.Node) bool {
			sw, ok := m.(*ast.SwitchStmt)
			if !ok {
				return true
			}
			id, ok := sw.Tag.(*ast.Ident)
			if !ok || id.Name != "sub" {
				return true
			}
			for _, stmt := range sw.Body.List {
				cc, ok := stmt.(*ast.CaseClause)
				if !ok {
					continue
				}
				for _, e := range cc.List {
					if s, ok := stringLit(e); ok {
						labels = append(labels, s)
					}
				}
			}
			return true
		})
		return true
	})
	sort.Strings(labels)
	return labels
}

// subcommandsExercisedByTests scans every _test.go file for `run([]string{"x", …})`
// and returns the set of first elements.
func subcommandsExercisedByTests(t *testing.T) map[string]bool {
	t.Helper()
	out := map[string]bool{}
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}
	fset := token.NewFileSet()
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		f, perr := parser.ParseFile(fset, e.Name(), nil, 0)
		if perr != nil {
			t.Fatalf("parse %s: %v", e.Name(), perr)
		}
		ast.Inspect(f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			id, ok := call.Fun.(*ast.Ident)
			if !ok || id.Name != "run" || len(call.Args) != 1 {
				return true
			}
			cl, ok := call.Args[0].(*ast.CompositeLit)
			if !ok || len(cl.Elts) == 0 {
				return true
			}
			if s, ok := stringLit(cl.Elts[0]); ok {
				out[s] = true
			}
			return true
		})
	}
	return out
}

func stringLit(e ast.Expr) (string, bool) {
	bl, ok := e.(*ast.BasicLit)
	if !ok || bl.Kind != token.STRING {
		return "", false
	}
	s, err := strconv.Unquote(bl.Value)
	if err != nil {
		return "", false
	}
	return s, true
}

// --- Inventory: every checkSpec field is documented ---
//
// Reflects over checkSpec's JSON-tagged fields and requires each to appear in README.md's
// field table. A field added to the schema without documentation is a knob whose
// semantics live only in the reader's guess.

func TestInventory_EveryCheckSpecFieldIsDocumented(t *testing.T) {
	readme := mustReadREADME(t)
	typ := reflect.TypeOf(checkSpec{})
	seen := 0
	for i := 0; i < typ.NumField(); i++ {
		tag := typ.Field(i).Tag.Get("json")
		if tag == "" || tag == "-" {
			continue // unexported / runtime-only, e.g. the compiled regexp
		}
		name := strings.Split(tag, ",")[0]
		seen++
		if !strings.Contains(readme, "`"+name+"`") {
			t.Errorf("checkSpec field %q is not documented in README.md's field table", name)
		}
	}
	if seen == 0 {
		t.Fatal("reflected zero JSON fields off checkSpec — the guard cannot see the schema")
	}
}

// --- Doc parity: the README's enumerated check list matches what ships ---

func TestDocParity_READMEListsExactlyTheShippedChecks(t *testing.T) {
	cl := mustLoadShippedCheckdef(t)

	shipped := map[string]bool{}
	for _, c := range cl.Checks {
		shipped[c.Name] = true
	}

	documented := map[string]bool{}
	// Rows of the "Shipped check list" table: | `name` | tool | … |
	row := regexp.MustCompile("(?m)^\\|\\s*`(ck-[a-z0-9-]+|kubeconform-[a-z0-9-]+)`\\s*\\|")
	for _, m := range row.FindAllStringSubmatch(mustReadREADME(t), -1) {
		documented[m[1]] = true
	}

	if len(documented) == 0 {
		t.Fatal("README.md's shipped-check table matched no rows — the parity guard is blind " +
			"and would pass however far the docs had drifted")
	}
	for name := range shipped {
		if !documented[name] {
			t.Errorf("check %q ships in the embedded checkdef but is absent from README.md", name)
		}
	}
	for name := range documented {
		if !shipped[name] {
			t.Errorf("check %q is documented in README.md but no longer ships", name)
		}
	}
}

// --- Every embedded checkdef is loadable and structurally valid ---

func TestEmbeddedCheckdefs_AllParse(t *testing.T) {
	found := 0
	err := fsWalkCheckdefs(func(path string, data []byte) error {
		found++
		cl, perr := parseCheckList(data)
		if perr != nil {
			t.Errorf("embedded checkdef %s does not parse: %v", path, perr)
			return nil
		}
		for _, c := range cl.Checks {
			if c.Note == "" {
				t.Errorf("%s: check %q has no note — a check whose rationale is not written "+
					"down cannot be audited against the base repo's CI", path, c.Name)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking embedded checkdefs: %v", err)
	}
	if found == 0 {
		t.Fatal("walked zero embedded checkdefs — the embed is not reaching this test, so it " +
			"would report every checkdef valid while shipping none")
	}
}

// --- helpers ---

func mustLoadShippedCheckdef(t *testing.T) *checkList {
	t.Helper()
	data, err := checkdefsFS.ReadFile("checkdefs/example-org/example-k8s.json")
	if err != nil {
		t.Fatalf("embedded example-k8s checkdef not readable: %v", err)
	}
	cl, perr := parseCheckList(data)
	if perr != nil {
		t.Fatalf("embedded example-k8s checkdef does not parse: %v", perr)
	}
	return cl
}

func findCheck(t *testing.T, cl *checkList, name string) *checkSpec {
	t.Helper()
	for i := range cl.Checks {
		if cl.Checks[i].Name == name {
			return &cl.Checks[i]
		}
	}
	t.Fatalf("shipped checkdef has no check named %q", name)
	return nil
}

func mustReadREADME(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile("README.md")
	if err != nil {
		t.Fatalf("read README.md: %v", err)
	}
	return string(b)
}

func fsWalkCheckdefs(fn func(path string, data []byte) error) error {
	var walk func(dir string) error
	walk = func(dir string) error {
		entries, err := checkdefsFS.ReadDir(dir)
		if err != nil {
			return err
		}
		for _, e := range entries {
			p := dir + "/" + e.Name()
			if e.IsDir() {
				if werr := walk(p); werr != nil {
					return werr
				}
				continue
			}
			if !strings.HasSuffix(e.Name(), ".json") {
				continue
			}
			data, rerr := checkdefsFS.ReadFile(p)
			if rerr != nil {
				return rerr
			}
			if ferr := fn(p, data); ferr != nil {
				return ferr
			}
		}
		return nil
	}
	return walk("checkdefs")
}

// --- The shipped check list must be able to PASS on the base repo ---
//
// A check list that cannot return PASS on the base repo's own `main` is not a gate,
// it is an outage: every advisory fix would report FAIL regardless of its content, and
// the verdict would carry no information. This ran green against a fresh clone of
// example-k8s main (see the brief's Evidence); it is env-gated because it needs that
// clone plus kubeconform on PATH, neither of which CI has.
//
//	DESKADVISORY_BASE_CLONE=/path/to/example-k8s go test -count=1 -run PassesOnRealBase ./cmd/deskadvisory/
func TestShippedCheckdef_PassesOnRealBase(t *testing.T) {
	clone := os.Getenv("DESKADVISORY_BASE_CLONE")
	if clone == "" {
		t.Skip("set DESKADVISORY_BASE_CLONE to a example-k8s clone to run this")
	}
	cl := mustLoadShippedCheckdef(t)
	cov, err := runChecks(cl, clone)
	if err != nil {
		t.Fatalf("the shipped check list must PASS on the base repo's own tree, else the tool "+
			"is inoperable on the only repo it configures: %v", err)
	}
	t.Logf("coverage: %s", cov.String())
	if cov.Checks != len(cl.Checks) {
		t.Fatalf("ran %d of %d checks", cov.Checks, len(cl.Checks))
	}
}
