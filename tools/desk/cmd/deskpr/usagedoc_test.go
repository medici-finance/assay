package main

// usagedoc_test.go — class guards for "the desk names a deskpr flag the help does not
// document for that verb, or one the verb does not have".
//
// Defect class: a flag that exists in a verb's flag set but is missing from that verb's
// USAGE line (so an author reading the help for that verb cannot find it), and its mirror,
// a remedy string elsewhere in the desk that tells an author to run `deskpr <verb> --flag`
// for a flag THAT verb never registered. Both leave an author following a refusal with no
// verb to run. The comparison is PER VERB throughout: a flag registered on one verb and
// documented only on another is a finding, not a pass.

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// flagNameArg maps every flag.FlagSet method that DEFINES a flag to the index of its name
// argument. Any other method called on a flag set must be in flagSetNonDefining below; a
// method in neither list fails the scan, so a new constructor cannot be silently skipped.
var flagNameArg = map[string]int{
	"String": 0, "Bool": 0, "Int": 0, "Int64": 0, "Uint": 0, "Uint64": 0,
	"Float64": 0, "Duration": 0, "Func": 0, "BoolFunc": 0,
	"StringVar": 1, "BoolVar": 1, "IntVar": 1, "Int64Var": 1, "UintVar": 1,
	"Uint64Var": 1, "Float64Var": 1, "DurationVar": 1, "Var": 1, "TextVar": 1,
}

var flagSetNonDefining = map[string]bool{
	"Parse": true, "Parsed": true, "Args": true, "Arg": true, "NArg": true, "NFlag": true,
	"SetOutput": true, "Output": true, "Name": true, "ErrorHandling": true, "Init": true,
	"Lookup": true, "Set": true, "Visit": true, "VisitAll": true, "PrintDefaults": true,
}

// flagNameConsts resolves the non-literal flag-name expressions the sources use.
var flagNameConsts = map[string]string{
	"deskkit.ScanOverrideFlag": deskkit.ScanOverrideFlag,
}

// verbFlagsInSource parses one Go source and returns, per verb, the flags registered on the
// flag set created by flag.NewFlagSet("<verb>", ...) inside a function, whether bound by
// `fs := ...`, `fs = ...` or `var fs = ...`. It returns an error string (never a silent
// skip) for any registration it cannot resolve: an unknown method on a flag set, a
// non-constant name, a flag set handed to another function that might register more, or a
// flag.NewFlagSet call it did not bind to a function-local identifier (a package-level
// var, a struct field, a call result used inline).
func verbFlagsInSource(name, src string) (map[string]map[string]bool, []string) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, name, src, 0)
	if err != nil {
		return nil, []string{err.Error()}
	}
	out := map[string]map[string]bool{}
	var problems []string
	// seen records every flag.NewFlagSet call pass 1 accounted for (bound, or already reported),
	// so the final sweep can report any call it never reached instead of skipping it.
	seen := map[*ast.CallExpr]bool{}
	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		// Pass 1: which identifiers in this function hold which verb's flag set.
		sets := map[string]string{}
		bind := func(lhs, rhs ast.Expr) {
			call, ok := rhs.(*ast.CallExpr)
			if !ok || exprString(call.Fun) != "flag.NewFlagSet" {
				return
			}
			seen[call] = true
			id, ok := lhs.(*ast.Ident)
			var lit *ast.BasicLit
			lok := false
			if len(call.Args) > 0 {
				lit, lok = call.Args[0].(*ast.BasicLit)
			}
			if !ok || !lok {
				problems = append(problems, fset.Position(call.Pos()).String()+": flag.NewFlagSet with a non-literal verb or target")
				return
			}
			verb, _ := strconv.Unquote(lit.Value)
			sets[id.Name] = verb
			if out[verb] == nil {
				out[verb] = map[string]bool{}
			}
		}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.AssignStmt:
				if len(x.Lhs) == len(x.Rhs) {
					for i := range x.Lhs {
						bind(x.Lhs[i], x.Rhs[i])
					}
				}
			case *ast.ValueSpec:
				if len(x.Names) == len(x.Values) {
					for i := range x.Names {
						bind(x.Names[i], x.Values[i])
					}
				}
			}
			return true
		})
		if len(sets) == 0 {
			continue
		}
		// Pass 2: every call on, or with, one of those identifiers.
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			pos := fset.Position(call.Pos()).String()
			for _, a := range call.Args {
				if id, ok := a.(*ast.Ident); ok && sets[id.Name] != "" {
					problems = append(problems, pos+": flag set "+id.Name+" passed to "+exprString(call.Fun)+" — registrations there are invisible to this scan")
				}
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			recv, ok := sel.X.(*ast.Ident)
			if !ok || sets[recv.Name] == "" {
				return true
			}
			verb, method := sets[recv.Name], sel.Sel.Name
			idx, defines := flagNameArg[method]
			if !defines {
				if !flagSetNonDefining[method] {
					problems = append(problems, pos+": unknown flag-set method "+method+" — add it to flagNameArg or flagSetNonDefining")
				}
				return true
			}
			if idx >= len(call.Args) {
				problems = append(problems, pos+": "+method+" call with too few arguments")
				return true
			}
			switch a := call.Args[idx].(type) {
			case *ast.BasicLit:
				v, _ := strconv.Unquote(a.Value)
				out[verb][v] = true
			default:
				if v, ok := flagNameConsts[exprString(a)]; ok {
					out[verb][v] = true
				} else {
					problems = append(problems, pos+": flag name "+exprString(a)+" is not a literal or a known constant")
				}
			}
			return true
		})
	}
	// Final sweep: a flag.NewFlagSet call pass 1 never bound — a package-level var, a struct
	// field, an inline call result — has registrations this scan cannot attribute to a verb.
	ast.Inspect(f, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok && exprString(call.Fun) == "flag.NewFlagSet" && !seen[call] {
			problems = append(problems, fset.Position(call.Pos()).String()+": flag.NewFlagSet not bound to a function-local identifier — its registrations are invisible to this scan")
		}
		return true
	})
	return out, problems
}

func exprString(e ast.Expr) string {
	switch x := e.(type) {
	case *ast.Ident:
		return x.Name
	case *ast.SelectorExpr:
		return exprString(x.X) + "." + x.Sel.Name
	}
	return "?"
}

// verbFlags returns, per verb, every flag that verb registers, read from the non-test
// sources of this package.
func verbFlags(t *testing.T) map[string]map[string]bool {
	t.Helper()
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	all := map[string]map[string]bool{}
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		got, problems := verbFlagsInSource(f, string(b))
		for _, p := range problems {
			t.Errorf("flag scanner cannot resolve a registration: %s", p)
		}
		for verb, flags := range got {
			if all[verb] == nil {
				all[verb] = map[string]bool{}
			}
			for name := range flags {
				all[verb][name] = true
			}
		}
	}
	for _, verb := range []string{"create", "update", "edit"} {
		if len(all[verb]) < 2 {
			t.Fatalf("flag scanner found %d registrations for %q (%v) — its matcher no longer matches the source", len(all[verb]), verb, all[verb])
		}
	}
	return all
}

// usageSynopses returns each verb's USAGE line from the usage text, continuation lines
// (indented deeper than the `  deskpr <verb>` line) joined on.
func usageSynopses(text string) map[string]string {
	out := map[string]string{}
	start := strings.Index(text, "USAGE:\n")
	if start < 0 {
		return out
	}
	verb := ""
	for _, line := range strings.Split(text[start+len("USAGE:\n"):], "\n") {
		switch {
		case strings.TrimSpace(line) == "":
			return out
		case strings.HasPrefix(line, "  deskpr "):
			fields := strings.Fields(line)
			verb = fields[1]
			out[verb] = line
		case strings.HasPrefix(line, "    ") && verb != "":
			out[verb] += " " + strings.TrimSpace(line)
		}
	}
	return out
}

// hasFlagToken reports whether text names --name as a whole flag token (so --pr is not
// satisfied by --prefix).
func hasFlagToken(text, name string) bool {
	return regexp.MustCompile(`(^|[^a-z-])--` + regexp.QuoteMeta(name) + `($|[^a-z-])`).MatchString(text)
}

// synopsisMismatches compares each verb's registered flags with its USAGE line, both ways.
func synopsisMismatches(flags map[string]map[string]bool, text string) []string {
	syn := usageSynopses(text)
	var out []string
	for verb, names := range flags {
		line, ok := syn[verb]
		if !ok {
			out = append(out, verb+": no USAGE line")
			continue
		}
		for name := range names {
			if !hasFlagToken(line, name) {
				out = append(out, verb+": registers --"+name+" but its USAGE line omits it")
			}
		}
		for _, m := range regexp.MustCompile(`--([a-z][a-z-]*)`).FindAllStringSubmatch(line, -1) {
			if !names[m[1]] {
				out = append(out, verb+": USAGE line names --"+m[1]+" which it does not register")
			}
		}
	}
	sort.Strings(out)
	return out
}

// TestUsageDocumentsEveryFlag: each verb's USAGE line in the text `deskpr --help` and
// `deskpr <verb> --help` print names exactly the flags that verb registers.
func TestUsageDocumentsEveryFlag(t *testing.T) {
	for _, m := range synopsisMismatches(verbFlags(t), usage) {
		t.Errorf("deskpr usage: %s", m)
	}
	// The desk-decided surface is the one the defect was filed on: pin its pieces by name.
	for _, want := range []string{"--decided", deskkit.DeskDecidedHeading, deskkit.DeskDecidedLabel, "decision:", "alternative:", "cost:"} {
		if !strings.Contains(usage, want) {
			t.Errorf("deskpr usage does not document %q", want)
		}
	}
}

// TestUsageGuardPositiveControl: the guard must flag a flag registered on one verb but
// documented only on another, and must see registrations made through constructors other
// than String/Bool/Int — so a matcher that silently stopped matching fails instead of
// reporting clean.
func TestUsageGuardPositiveControl(t *testing.T) {
	src := `package p
import "flag"
func a() {
	fs := flag.NewFlagSet("create", flag.ContinueOnError)
	var v string
	fs.Int("pr", 0, "")
	fs.Duration("timeout", 0, "")
	fs.StringVar(&v, "label", "", "")
	_ = fs.Parse(nil)
}
func b() {
	fs := flag.NewFlagSet("edit", flag.ContinueOnError)
	fs.Int("pr", 0, "")
}`
	flags, problems := verbFlagsInSource("planted.go", src)
	if len(problems) != 0 {
		t.Fatalf("planted source raised problems: %v", problems)
	}
	for _, name := range []string{"pr", "timeout", "label"} {
		if !flags["create"][name] {
			t.Errorf("scanner missed planted create flag %q: got %v", name, flags["create"])
		}
	}
	text := "USAGE:\n  deskpr create [--timeout D]\n      [--label L]\n  deskpr edit [--pr N]\n\n"
	got := synopsisMismatches(flags, text)
	want := []string{"create: registers --pr but its USAGE line omits it"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("per-verb mismatch not isolated: got %v, want %v", got, want)
	}
	// A registration the scanner cannot resolve is a problem, never a skip.
	_, problems = verbFlagsInSource("planted2.go", `package p
import "flag"
func c(name string) {
	fs := flag.NewFlagSet("update", flag.ContinueOnError)
	fs.String(name, "", "")
	fs.Frobnicate()
	register(fs)
}`)
	if len(problems) != 3 {
		t.Errorf("want 3 unresolvable-registration problems, got %d: %v", len(problems), problems)
	}
	// A `var fs = flag.NewFlagSet(...)` binding is read like `fs :=`; a flag set the scanner
	// cannot bind to a function-local identifier is a problem, never a silent skip.
	flags, problems = verbFlagsInSource("planted3.go", `package p
import "flag"
var pkgFS = flag.NewFlagSet("fourth", flag.ContinueOnError)
type holder struct{ fs *flag.FlagSet }
func d() {
	var fs = flag.NewFlagSet("update", flag.ContinueOnError)
	fs.Bool("dry-run", false, "")
	h := holder{}
	h.fs = flag.NewFlagSet("fifth", flag.ContinueOnError)
}`)
	if !flags["update"]["dry-run"] {
		t.Errorf("scanner missed a flag registered on a var-declared flag set: got %v", flags)
	}
	if len(problems) != 2 {
		t.Errorf("want 2 unbound-flag-set problems (package-level var, struct field), got %d: %v", len(problems), problems)
	}
}

// remedyStartRe finds a `deskpr <verb>` mention, including the `create|edit` and
// `create/update/edit` shorthands and a verb wrapped onto the next line.
var remedyStartRe = regexp.MustCompile(`\bdeskpr\s+((?:create|edit|update)(?:[|/](?:create|edit|update))*)\b`)

var remedyFlagRe = regexp.MustCompile(`(?:^|[\s\[(|])--([a-z][a-z-]*)`)

// remedy is one `deskpr <verb...> ... --flag` mention: the verbs it names and every flag
// in its span.
type remedy struct {
	verbs []string
	flags []string
}

// remediesIn extracts every remedy in text. A span runs from the verb to the next backtick,
// double quote or newline — except that a span inside an inline code span (an odd number
// of backticks earlier on its line) continues across ONE newline to the closing backtick,
// so a remedy wrapped across two Markdown lines is still read whole.
func remediesIn(text string) []remedy {
	var out []remedy
	for _, loc := range remedyStartRe.FindAllStringSubmatchIndex(text, -1) {
		lineStart := strings.LastIndex(text[:loc[0]], "\n") + 1
		inCode := strings.Count(text[lineStart:loc[0]], "`")%2 == 1
		rest := text[loc[1]:]
		end, newlines := len(rest), 0
		for i, r := range rest {
			if r == '`' || r == '"' {
				end = i
				break
			}
			if r == '\n' {
				newlines++
				if !inCode || newlines > 1 {
					end = i
					break
				}
			}
		}
		r := remedy{verbs: strings.FieldsFunc(text[loc[2]:loc[3]], func(c rune) bool { return c == '|' || c == '/' })}
		for _, m := range remedyFlagRe.FindAllStringSubmatch(rest[:end], -1) {
			r.flags = append(r.flags, m[1])
		}
		if len(r.flags) > 0 {
			out = append(out, r)
		}
	}
	return out
}

// TestNamedRemediesAreRealFlags: every `deskpr <verb> ... --flag` remedy named anywhere in
// the desk's non-test Go sources or the shipped skill/reference text names, for EACH verb
// it names, only flags THAT verb registers.
func TestNamedRemediesAreRealFlags(t *testing.T) {
	flags := verbFlags(t)
	// help/version are handled before flag parsing and are real spellings on every verb.
	for _, v := range flags {
		v["help"], v["version"] = true, true
	}

	roots := []string{"../../", "../../../../plugins/assay/skills", "../../../../plugins/assay/references"}
	checked := 0
	for _, root := range roots {
		if _, err := os.Stat(root); err != nil {
			t.Fatalf("remedy scan root %s: %v", root, err)
		}
		_ = filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}
			isGo := strings.HasSuffix(p, ".go") && !strings.HasSuffix(p, "_test.go")
			isDoc := strings.HasSuffix(p, ".md")
			if !isGo && !isDoc {
				return nil
			}
			b, rerr := os.ReadFile(p)
			if rerr != nil {
				return nil
			}
			for _, r := range remediesIn(string(b)) {
				for _, verb := range r.verbs {
					for _, f := range r.flags {
						checked++
						if !flags[verb][f] {
							t.Errorf("%s names remedy `deskpr %s ... --%s`, but deskpr %s registers no such flag", p, verb, f, verb)
						}
					}
				}
			}
			return nil
		})
	}
	if checked == 0 {
		t.Fatal("remedy scanner matched nothing — it must at least see the deskflip refusal's `deskpr edit ... --decided`")
	}
}

// TestRemedyGuardPositiveControl: planted remedies the guard must read whole — a bad flag
// AFTER a good one, a remedy wrapped across two Markdown lines, and a shorthand naming two
// verbs.
func TestRemedyGuardPositiveControl(t *testing.T) {
	got := remediesIn("run `deskpr edit --body-file F --no-such-flag` now\n" +
		"declare it with `deskpr create\n--decided F` or so\n" +
		"use `deskpr create|edit --decided` here\n")
	want := "[edit]:[body-file no-such-flag] [create]:[decided] [create edit]:[decided]"
	var parts []string
	for _, r := range got {
		parts = append(parts, "["+strings.Join(r.verbs, " ")+"]:["+strings.Join(r.flags, " ")+"]")
	}
	if strings.Join(parts, " ") != want {
		t.Errorf("planted remedies not extracted whole:\n got %s\nwant %s", strings.Join(parts, " "), want)
	}
	flags := verbFlags(t)
	if flags["edit"]["no-such-flag"] {
		t.Error("the planted flag must not be a registered flag")
	}
	if flags["update"]["decided"] {
		t.Error("update must not register --decided — the per-verb remedy check relies on it")
	}
}
