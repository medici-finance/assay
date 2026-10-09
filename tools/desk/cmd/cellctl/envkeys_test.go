package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// The key registry (envkeys.go) and the machine-wide defaults template generated from it.
//
// The first test is the completeness guard: it walks this package's syntax for the read shapes
// listed below and fails when the registry does not classify a key one of them reads. The rest
// hold the template, the refused list, `set`'s known keys and the documented lists to the same
// registry.
//
// What the walk recognises: a call to os.Getenv or os.LookupEnv written through an identifier
// named `os`; a call to one of Env's readers (envReaderMethods), matched by method name on any
// receiver; a call to a function that reads the key it is handed (envKeyParamFuncs); and one of
// those readers taken as a value instead of called.
//
// What it does not see, so a key read ONLY this way can be missing from the registry and from
// the template without this test failing: a scan of os.Environ() for a name or a prefix;
// syscall.Getenv; os.Getenv behind a renamed import or held in a function value;
// os.ExpandEnv and the $VAR expansion of a file value; a variable read by a child shell or
// process this package starts; and a direct read or range of Env's own maps.

// envRead is one place a key is read.
type envRead struct {
	// site is callee(argument)@file:func — the argument as written, so a renamed variable is a
	// new site.
	site string
	// key is the key read, when the source says which: a string literal, a constant of this
	// package, or one of the imported constants in envKeyConsts.
	key      string
	computed bool // the key is built at run time
	value    bool // a reader taken as a value (`c.Env.Get` handed to something), not called
	def      string
	hasDef   bool // GetOr/GetOrSet with a literal default
}

// envReaderMethods are Env's readers. They are matched by name on any receiver: this is a
// syntax walk, so `client.Get(url)` is found too and has to be accounted for in
// envKeyOtherReads. That is the price of not missing an Env held under a new name.
var envReaderMethods = map[string]bool{"Get": true, "GetOr": true, "GetOrSet": true, "IsSet": true, "Source": true}

// envKeyParamFuncs are functions that read the key they are handed; the value is which
// argument. A call to one is a read of that argument, so the keys are checked at the call sites.
var envKeyParamFuncs = map[string]int{"envFileValue": 1, "configuredPath": 1}

// envKeyReads walks one file for every reader call and every reader taken as a value.
func envKeyReads(name string, src any, consts map[string]string) ([]envRead, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, name, src, 0)
	if err != nil {
		return nil, err
	}
	text := func(n ast.Node) string {
		var b bytes.Buffer
		printer.Fprint(&b, fset, n)
		return b.String()
	}
	base := filepath.Base(name)
	var out []envRead
	for _, d := range f.Decls {
		fn := "(package)"
		if fd, ok := d.(*ast.FuncDecl); ok {
			fn = fd.Name.Name
		}
		called := map[ast.Expr]bool{}
		ast.Inspect(d, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.CallExpr:
				called[x.Fun] = true
				callee, arg := "", -1
				switch fun := x.Fun.(type) {
				case *ast.SelectorExpr:
					if pkg, ok := fun.X.(*ast.Ident); ok && pkg.Name == "os" && (fun.Sel.Name == "Getenv" || fun.Sel.Name == "LookupEnv") {
						callee, arg = "os."+fun.Sel.Name, 0
					} else if envReaderMethods[fun.Sel.Name] {
						callee, arg = fun.Sel.Name, 0
					}
				case *ast.Ident:
					if i, ok := envKeyParamFuncs[fun.Name]; ok {
						callee, arg = fun.Name, i
					}
				}
				if callee == "" || arg >= len(x.Args) {
					return true
				}
				r := envRead{site: callee + "(" + text(x.Args[arg]) + ")@" + base + ":" + fn}
				if k, ok := envKeyOf(x.Args[arg], consts); ok {
					r.key = k
				} else {
					r.computed = true
				}
				if (callee == "GetOr" || callee == "GetOrSet") && len(x.Args) == 2 {
					if bl, ok := x.Args[1].(*ast.BasicLit); ok && bl.Kind == token.STRING {
						r.def, _ = strconv.Unquote(bl.Value)
						r.hasDef = true
					}
				}
				out = append(out, r)
			case *ast.SelectorExpr:
				if envReaderMethods[x.Sel.Name] && !called[x] {
					out = append(out, envRead{value: true, site: text(x) + "@" + base + ":" + fn})
				}
			}
			return true
		})
	}
	return out, nil
}

// envKeyOf reads a key off an argument when the source states it.
func envKeyOf(arg ast.Expr, consts map[string]string) (string, bool) {
	switch a := arg.(type) {
	case *ast.ParenExpr:
		return envKeyOf(a.X, consts)
	case *ast.BasicLit:
		if a.Kind == token.STRING {
			s, err := strconv.Unquote(a.Value)
			return s, err == nil
		}
	case *ast.Ident:
		if a.Obj == nil { // not declared in this file: a constant of the package, or nothing known
			v, ok := consts[a.Name]
			return v, ok
		}
		if a.Obj.Kind != ast.Con {
			return "", false
		}
		if vs, ok := a.Obj.Decl.(*ast.ValueSpec); ok {
			for i, n := range vs.Names {
				if n.Name == a.Name && i < len(vs.Values) {
					return envKeyOf(vs.Values[i], consts)
				}
			}
		}
	case *ast.SelectorExpr:
		if pkg, ok := a.X.(*ast.Ident); ok {
			v, ok := consts[pkg.Name+"."+a.Sel.Name]
			return v, ok
		}
	}
	return "", false
}

// packageSources is every non-test Go file of this package. Build tags are not applied, so a
// file compiled only under a tag (parity_on.go) is walked too.
func packageSources(t *testing.T, dir string) []string {
	t.Helper()
	files, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, f := range files {
		if f.IsDir() || !strings.HasSuffix(f.Name(), ".go") || strings.HasSuffix(f.Name(), "_test.go") {
			continue
		}
		out = append(out, filepath.Join(dir, f.Name()))
	}
	return out
}

// envKeyConsts is every string constant a key argument can name: this package's own, read off
// the source, and the imported ones, taken from the packages themselves.
func envKeyConsts(t *testing.T) map[string]string {
	t.Helper()
	consts := map[string]string{"deskkit.EnvRepairAdmission": deskkit.EnvRepairAdmission}
	for _, name := range packageSources(t, ".") {
		f, err := parser.ParseFile(token.NewFileSet(), name, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, d := range f.Decls {
			gd, ok := d.(*ast.GenDecl)
			if !ok || gd.Tok != token.CONST {
				continue
			}
			for _, s := range gd.Specs {
				vs := s.(*ast.ValueSpec)
				for i, n := range vs.Names {
					if i >= len(vs.Values) {
						continue
					}
					if bl, ok := vs.Values[i].(*ast.BasicLit); ok && bl.Kind == token.STRING {
						if v, err := strconv.Unquote(bl.Value); err == nil {
							consts[n.Name] = v
						}
					}
				}
			}
		}
	}
	return consts
}

// envOtherRead accounts for one site whose key the source does not state, or which is not an
// environment read at all. A site that does read configuration says which registry keys.
type envOtherRead struct {
	keys     []string // the registry keys it reads
	prefixes []string // or whole families of them
	provider bool     // or a provider's variables, built from the provider's name
	why      string
}

// envKeyOtherReads is the closed list. What it cannot prove: that a site reads ONLY what its
// entry declares. A new site, a renamed argument or a moved function is a new entry, so each one
// is looked at once by a person; after that the entry is taken at its word.
var envKeyOtherReads = map[string]envOtherRead{
	// ── keys built at run time, all of them in the registry ──
	"Get(rvar)@model.go:resolveRoleModel": {prefixes: []string{"DESK_MODEL_", "CODEX_MODEL_", "CURSOR_MODEL_"},
		why: "the per-role pin of the harness being resolved"},
	"Get(dvar)@model.go:resolveRoleModel": {keys: []string{"DESK_MODEL_DEFAULT", "CODEX_MODEL_default", "CURSOR_MODEL_default"},
		why: "the harness default of the harness being resolved"},
	"Get(tvar)@model.go:resolveRoleModel": {prefixes: []string{"TIER_MODEL_"},
		why: "the tier-map entry for the role's tier and harness"},
	"Get(mvar)@desk.go:cmdDesk": {prefixes: []string{"DESK_MODEL_", "CODEX_MODEL_", "CURSOR_MODEL_"},
		why: "the per-role pin, read to refuse an Opus pin on the-desk"},
	"GetOrSet(k)@cell.go:loadCell": {prefixes: []string{"TIER_MODEL_"},
		why: "the compiled tier map, applied from tierModelDefaults"},
	`Get("TIER_MODEL_MID_" + strings.ToUpper(pinHarness(key)))@modelreset.go:cheapDefault`: {prefixes: []string{"TIER_MODEL_"},
		why: "the MID tier entry of the pin's harness, the model an aged default pin is repinned to"},
	"IsSet(k)@modelreset.go:resetAgedDefaultPins": {prefixes: []string{"DESK_MODEL_", "CODEX_MODEL_", "CURSOR_MODEL_"},
		why: "whether cell.env still carries the model pin a pin record names"},
	"Get(k)@modelreset.go:resetAgedDefaultPins": {prefixes: []string{"DESK_MODEL_", "CODEX_MODEL_", "CURSOR_MODEL_"},
		why: "the model pin cell.env carries for a pin record"},
	"Get(providerVar(name, suffix))@provider.go:providerValue": {provider: true,
		why: "a provider's endpoint, token variable name or model"},
	"Source(providerVar(name, suffix))@provider.go:providerValue": {provider: true,
		why: "which file supplied the provider value just read"},

	// ── Env.Get handed to another package ──
	"c.Env.Get@cache.go:cachePolicy": {keys: []string{"CELL_GO_CACHE", "CELL_GO_CACHE_ROOT", "CELL_GO_CACHE_BYTES", "CELL_GO_CACHE_MIN_FREE"},
		why: "the managed Go cache resolver reads its keys through this getter; TestEnvKeyRegistryIsComplete holds this list to the literals in that package"},

	// ── forwarding: the key is a parameter, and the call sites are walked instead ──
	"Get(key)@provider_defaults.go:configuredPath": {why: "configuredPath reads the key it is handed; its callers are checked (envKeyParamFuncs)"},

	// ── not configuration ──
	"Get(p.TokenEnv)@provider.go:providerCredential":           {why: "the credential variable a provider's TOKEN_ENV names: the operator's own variable, read from the layered environment (process environment, defaults file, cell.env) like any other key"},
	"Get(pt)@show.go:cmdShow":                                  {why: "the same credential variable, read only to say set or unset"},
	"Get(tokenVar)@check.go:cmdCheck":                          {why: "the same credential variable, checked for presence"},
	"Get(tokVal)@check.go:cmdCheck":                            {why: "the same credential variable, checked for presence"},
	"Get(name)@cell.go:expandVar":                              {why: "a ${VAR} reference inside a file value, expanded as the shell would"},
	"Get(s[1:j])@cell.go:expandVar":                            {why: "a $VAR reference inside a file value, expanded as the shell would"},
	"Get(need)@set.go:validateKindChange":                      {why: "the one key the target kind requires, named by a literal a few lines above"},
	"Source(key)@show.go:cmdShow":                              {why: "where an already-resolved choice came from; the key is one show lists by name"},
	"envFileValue(key)@show.go:cmdShow":                        {why: "whether that same key is in cell.env, for display"},
	"Source(rm.Src)@show.go:cmdShow":                           {why: "where the model variable resolveRoleModel named came from"},
	"envFileValue(rm.Src)@show.go:cmdShow":                     {why: "whether that same variable is in cell.env, for display"},
	`Get("http://" + c.DeskdAddr + "/healthz")@git.go:deskdUp`: {why: "an HTTP client's Get, not an Env's"},
	"res.Source@deskd.go:deskdMintGitHub":                      {why: "a struct field named Source, not the Env reader"},
}

// checkEnvReads classifies a set of reads against the registry. It returns one line per
// problem, the registry keys the reads account for, and the envKeyOtherReads sites it met.
func checkEnvReads(reads []envRead, other map[string]envOtherRead) (problems []string, readKeys, met map[string]bool) {
	readKeys, met = map[string]bool{}, map[string]bool{}
	for _, r := range reads {
		if !r.computed && !r.value {
			if _, ok := envKeyLookup(r.key); ok {
				readKeys[r.key] = true
				continue
			}
		}
		o, ok := other[r.site]
		if ok {
			if !met[r.site] {
				met[r.site] = true
				for _, k := range o.keys {
					if _, known := envKeyLookup(k); !known {
						problems = append(problems, fmt.Sprintf("%s is declared to read %s, which envkeys.go does not classify", r.site, k))
					}
					readKeys[k] = true
				}
				for _, p := range o.prefixes {
					n := 0
					for _, k := range envKeys {
						if strings.HasPrefix(k.key, p) {
							readKeys[k.key] = true
							n++
						}
					}
					if n == 0 {
						problems = append(problems, fmt.Sprintf("%s is declared to read the %s* family, which has no key in envkeys.go", r.site, p))
					}
				}
				if o.provider {
					for _, k := range envKeys {
						if isProviderFamilyKey(k.key) {
							readKeys[k.key] = true
						}
					}
				}
			}
			continue
		}
		switch {
		case r.value:
			problems = append(problems, fmt.Sprintf("%s takes an Env reader as a value, so the keys read through it cannot be seen: list the site in envKeyOtherReads with the keys it reads", r.site))
		case r.computed:
			problems = append(problems, fmt.Sprintf("%s builds its key at run time: read a literal key, or list the site in envKeyOtherReads with the registry keys it reads", r.site))
		default:
			problems = append(problems, fmt.Sprintf("%s reads %s, which envkeys.go does not classify: add it to envKeys as settable machine-wide (envMachine), per-cell (envCell) or not a lever (envNotLever) — or, when the call is not an environment read, list the site in envKeyOtherReads", r.site, r.key))
		}
	}
	return problems, readKeys, met
}

// cellcacheKeys is every key the managed Go cache package reads through the getter cellctl hands
// it: the literal argument of each get(…) call there.
func cellcacheKeys(t *testing.T) []string {
	t.Helper()
	seen := map[string]bool{}
	for _, name := range packageSources(t, filepath.Join("..", "..", "internal", "cellcache")) {
		f, err := parser.ParseFile(token.NewFileSet(), name, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(f, func(n ast.Node) bool {
			c, ok := n.(*ast.CallExpr)
			if !ok || len(c.Args) != 1 {
				return true
			}
			if id, ok := c.Fun.(*ast.Ident); !ok || id.Name != "get" {
				return true
			}
			k, ok := envKeyOf(c.Args[0], nil)
			if !ok {
				t.Errorf("%s: get(…) is called with a key built at run time, which this guard cannot read", name)
				return true
			}
			seen[k] = true
			return true
		})
	}
	var out []string
	for k := range seen {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// TestEnvKeyRegistryIsComplete is the guard behind "every key is in the registry". It walks every
// non-test file of the package for a call to one of Env's readers (Get, GetOr, GetOrSet, IsSet,
// Source), to os.Getenv or os.LookupEnv, or to a function that reads the key it is handed, and
// requires the key to be classified in envkeys.go.
//
// A key the source does not state — one built at run time, or read through a reader handed to
// another package — cannot be classified by reading it, so its site must be in
// envKeyOtherReads, saying which registry keys it reads or why it reads none. The list is
// closed both ways: an unlisted site fails, and so does a listed site that is no longer there.
//
// Two reads are outside the walk and are stated here rather than found: newEnvFromProcess
// copies the whole process environment in (the loader, not a reader of any one key), and a
// library cellctl calls may read the process environment for itself, where no file reaches it.
func TestEnvKeyRegistryIsComplete(t *testing.T) {
	consts := envKeyConsts(t)
	var reads []envRead
	for _, name := range packageSources(t, ".") {
		rs, err := envKeyReads(name, nil, consts)
		if err != nil {
			t.Fatal(err)
		}
		reads = append(reads, rs...)
	}
	problems, readKeys, met := checkEnvReads(reads, envKeyOtherReads)
	sort.Strings(problems)
	for _, p := range problems {
		t.Error(p)
	}
	var stale []string
	for site := range envKeyOtherReads {
		if !met[site] {
			stale = append(stale, site)
		}
	}
	sort.Strings(stale)
	if len(stale) > 0 {
		t.Errorf("envKeyOtherReads lists a site that is not in the package (stale list or broken matcher): %v", stale)
	}

	// The one reader handed to another package: what that package reads through it is exactly
	// what the entry declares.
	want := append([]string(nil), envKeyOtherReads["c.Env.Get@cache.go:cachePolicy"].keys...)
	sort.Strings(want)
	if got := cellcacheKeys(t); !reflect.DeepEqual(got, want) {
		t.Errorf("the managed Go cache package reads %v through cellctl's getter; envKeyOtherReads declares %v — classify a new key in envkeys.go and declare it there", got, want)
	}

	// The other direction: a registry key nothing reads is a lever that does nothing.
	for _, k := range envKeys {
		switch {
		case k.unread == "" && !readKeys[k.key]:
			t.Errorf("envkeys.go lists %s, and nothing in the package reads it: remove it, or say in its unread field where it is consumed", k.key)
		case k.unread != "" && readKeys[k.key]:
			t.Errorf("envkeys.go says nothing reads %s (%s), and something does: drop the unread note", k.key, k.unread)
		}
	}

	// Positive control: the walk finds each shape, and an unclassified key is a failure.
	const planted = `package main
const plantedKey = "CELL_PLANTED_CONST"
func plantedReads(c *Cell, name string) {
	_ = c.Env.Get("CELL_PLANTED_LITERAL")
	_ = c.Env.GetOr(plantedKey, "x")
	_ = c.Env.IsSet(deskkit.EnvRepairAdmission)
	_ = c.Env.Source("CELL_" + name)
	_, _ = os.LookupEnv("CELL_PLANTED_OS")
	_, _ = configuredPath(c.Env, "CELL_PLANTED_FORWARDED", "", "")
	resolve(c.Env.GetOrSet)
}`
	rs, err := envKeyReads("planted.go", planted, consts)
	if err != nil {
		t.Fatal(err)
	}
	got, plantedKeys, _ := checkEnvReads(rs, nil)
	sort.Strings(got)
	wantProblems := []string{
		`Get("CELL_PLANTED_LITERAL")@planted.go:plantedReads reads CELL_PLANTED_LITERAL, which envkeys.go does not classify`,
		`GetOr(plantedKey)@planted.go:plantedReads reads CELL_PLANTED_CONST, which envkeys.go does not classify`,
		`Source("CELL_" + name)@planted.go:plantedReads builds its key at run time`,
		`c.Env.GetOrSet@planted.go:plantedReads takes an Env reader as a value`,
		`configuredPath("CELL_PLANTED_FORWARDED")@planted.go:plantedReads reads CELL_PLANTED_FORWARDED, which envkeys.go does not classify`,
		`os.LookupEnv("CELL_PLANTED_OS")@planted.go:plantedReads reads CELL_PLANTED_OS, which envkeys.go does not classify`,
	}
	if len(got) != len(wantProblems) {
		t.Fatalf("positive control: %d problem(s), want %d:\n%s", len(got), len(wantProblems), strings.Join(got, "\n"))
	}
	for i, w := range wantProblems {
		if !strings.HasPrefix(got[i], w) {
			t.Errorf("positive control: problem %d = %q, want it to start %q", i, got[i], w)
		}
	}
	if !plantedKeys[deskkit.EnvRepairAdmission] || len(plantedKeys) != 1 {
		t.Errorf("positive control: an imported constant should resolve to its one classified key, got %v", plantedKeys)
	}
}

// TestEnvKeyRegistryShape: every entry carries what its class is printed with, and the compiled
// default the registry states for a settable key is the one the code applies wherever the code
// states it as a literal.
func TestEnvKeyRegistryShape(t *testing.T) {
	groups := map[string]bool{}
	for _, g := range envKeyGroups {
		groups[g] = true
	}
	seen := map[string]bool{}
	count := map[envKeyClass]int{}
	for _, k := range envKeys {
		if seen[k.key] {
			t.Errorf("%s is listed twice", k.key)
		}
		seen[k.key] = true
		count[k.class]++
		if !validEnvKeyShape(k.key) {
			t.Errorf("%q is not a key shape", k.key)
		}
		switch k.class {
		case envMachine:
			if !groups[k.group] || k.desc == "" {
				t.Errorf("%s: a settable key needs a description and a group the template prints, got group %q", k.key, k.group)
			}
			if k.why != "" || k.from != 0 {
				t.Errorf("%s: a settable key carries no refusal reason and no other source", k.key)
			}
		case envCell:
			if k.desc == "" || k.why == "" {
				t.Errorf("%s: a per-cell key needs a description and the reason defaults.env refuses it", k.key)
			}
			if k.group != "" || k.def != "" || k.from != 0 {
				t.Errorf("%s: a per-cell key is not printed with a default or a group, and its source is cell.env", k.key)
			}
		case envNotLever:
			if k.why == "" {
				t.Errorf("%s: a key that is not a lever needs the reason defaults.env refuses it", k.key)
			}
			switch k.from {
			case envFromRun, envFromShell:
				if k.desc == "" {
					t.Errorf("%s: a key the template describes needs a description", k.key)
				}
			case envFromHost, envInternal:
				if k.desc != "" {
					t.Errorf("%s: the template does not describe a host variable or plumbing, and a description here is printed nowhere", k.key)
				}
			default:
				t.Errorf("%s: a key that is not a lever says where its value comes from instead", k.key)
			}
			if k.group != "" || k.def != "" {
				t.Errorf("%s: a key that is not a lever has no template group or default", k.key)
			}
		default:
			t.Errorf("%s: unknown class %d", k.key, k.class)
		}
		for _, s := range []string{k.desc, k.why} {
			if strings.ContainsAny(s, "\n\r") || strings.TrimSpace(s) != s {
				t.Errorf("%s: a description or reason is one trimmed line, got %q", k.key, s)
			}
		}
	}
	if count[envMachine] == 0 || count[envCell] == 0 || count[envNotLever] == 0 {
		t.Fatalf("a class is empty: %v", count)
	}

	// Literal defaults in the code. A default the code derives or applies some other way is not
	// seen here; the template tests below resolve a cell with every line uncommented for those.
	consts := envKeyConsts(t)
	checked := 0
	for _, name := range packageSources(t, ".") {
		rs, err := envKeyReads(name, nil, consts)
		if err != nil {
			t.Fatal(err)
		}
		for _, r := range rs {
			k, ok := envKeyLookup(r.key)
			if r.computed || r.value || !r.hasDef || r.def == "" || !ok || k.class != envMachine {
				continue
			}
			checked++
			if k.def != r.def && !(k.def == "" && strings.Contains(k.desc, r.def)) {
				t.Errorf("%s applies the default %q; envkeys.go states %q for %s — make them agree", r.site, r.def, k.def, r.key)
			}
		}
	}
	if checked < 5 {
		t.Errorf("only %d literal defaults were compared: the matcher is not finding the GetOr calls", checked)
	}
}

// templateLine is a commented-out assignment as the template writes one.
var templateLine = regexp.MustCompile(`^# ([A-Za-z_][A-Za-z0-9_]*)=(.*)$`)

// templateAssignments is every commented-out assignment in the template: the index of its line,
// its key and the value after `=`.
func templateAssignments(lines []string) (idx []int, keys, vals []string) {
	for i, l := range lines {
		if m := templateLine.FindStringSubmatch(l); m != nil {
			idx, keys, vals = append(idx, i), append(keys, m[1]), append(vals, m[2])
		}
	}
	return
}

// templateSections splits the template at its `# --- title ---` rules. Each section is the
// comment text with the `#` margin and the line wrapping removed, so a description can be looked
// for as written in the registry.
func templateSections(tpl string) map[string]string {
	out := map[string]string{}
	title := ""
	var words []string
	flush := func() { out[title] = strings.Join(words, " "); words = nil }
	for _, l := range strings.Split(tpl, "\n") {
		if strings.HasPrefix(l, "# --- ") && strings.HasSuffix(l, " ---") {
			flush()
			title = strings.TrimSuffix(strings.TrimPrefix(l, "# --- "), " ---")
			continue
		}
		words = append(words, strings.Fields(strings.TrimLeft(l, "#"))...)
	}
	flush()
	return out
}

func machineKeys() []envKey {
	var out []envKey
	for _, g := range envKeyGroups {
		for _, k := range envKeys {
			if k.class == envMachine && k.group == g {
				out = append(out, k)
			}
		}
	}
	return out
}

func emptyEnv() *Env { return &Env{vals: map[string]string{}, set: map[string]bool{}} }

// TestCellDefaultsTemplateSetsNothing: the file cellctl writes, unedited, parses under the strict
// reader on every host and assigns no key, and a cell under a root that holds it loads exactly
// as it does with no file.
func TestCellDefaultsTemplateSetsNothing(t *testing.T) {
	tpl := cellDefaultsTemplate()
	if !strings.HasSuffix(tpl, "\n") || strings.Contains(tpl, "\r") {
		t.Errorf("the template ends with one newline and carries no carriage return")
	}
	for i, l := range strings.Split(strings.TrimSuffix(tpl, "\n"), "\n") {
		if l != "" && !strings.HasPrefix(l, "#") {
			t.Errorf("template line %d is neither blank nor a comment: %q", i+1, l)
		}
		if strings.TrimRight(l, " \t") != l {
			t.Errorf("template line %d has trailing space", i+1)
		}
		for _, r := range l {
			if r > 126 || r < 32 {
				t.Errorf("template line %d carries a byte outside printable ASCII: %q", i+1, l)
				break
			}
		}
	}
	for _, goos := range []string{"linux", "darwin", "windows"} {
		for name, body := range map[string]string{"as written": tpl, "saved with CRLF": strings.ReplaceAll(tpl, "\n", "\r\n")} {
			e := emptyEnv()
			keys, err := overlayCellDefaultsText(goos, e, []byte(body), "defaults.env")
			if err != nil {
				t.Errorf("%s, %s: the unedited template is refused: %v", goos, name, err)
			}
			if len(keys) != 0 || len(e.vals) != 0 {
				t.Errorf("%s, %s: the unedited template sets %v", goos, name, keys)
			}
		}
	}

	root := defaultsRoot(t)
	writeCell(t, root, "demo", demoCell)
	ref := loadCell("demo")
	path := filepath.Join(root, cellDefaultsFile)
	if created, err := createCellDefaults(path); err != nil || !created {
		t.Fatalf("createCellDefaults = %v, %v", created, err)
	}
	c := loadCell("demo")
	if !c.DefaultsRead || len(c.DefaultsKeys) != 0 {
		t.Errorf("the template was read=%v and set %v, want read and no keys", c.DefaultsRead, c.DefaultsKeys)
	}
	if !reflect.DeepEqual(c.Env.vals, ref.Env.vals) || !reflect.DeepEqual(c.Env.set, ref.Env.set) || !reflect.DeepEqual(c.Env.src, ref.Env.src) {
		t.Errorf("a cell loads differently under a root holding the unedited template")
	}
}

// TestCellDefaultsTemplateListsEveryMachineKey: the template's commented-out assignments are
// exactly the registry's machine-wide keys, once each, in registry order, each with its
// description above it and its compiled default after the `=`.
func TestCellDefaultsTemplateListsEveryMachineKey(t *testing.T) {
	tpl := cellDefaultsTemplate()
	_, keys, vals := templateAssignments(strings.Split(tpl, "\n"))
	want := machineKeys()
	var wantKeys []string
	for _, k := range want {
		wantKeys = append(wantKeys, k.key)
	}
	if !reflect.DeepEqual(keys, wantKeys) {
		t.Fatalf("the template's assignment lines are\n  %v\nwant every settable key of the registry, once, in order\n  %v", keys, wantKeys)
	}
	sections := templateSections(tpl)
	for i, k := range want {
		if vals[i] != envTemplateValue(k.def) {
			t.Errorf("%s is printed with %q, want its compiled default %q", k.key, vals[i], envTemplateValue(k.def))
		}
		if !strings.Contains(sections[k.group], strings.Join(strings.Fields(k.desc), " ")) {
			t.Errorf("%s: its description is not in the %q section", k.key, k.group)
		}
	}
	if !strings.Contains(sections[envGroupProvider], envKeyProviderFamily) {
		t.Errorf("the provider section does not name the open family %s", envKeyProviderFamily)
	}
	for _, k := range envKeys {
		if k.class != envMachine && templateLine.MatchString("# "+k.key+"=") && valueIn(k.key, keys) {
			t.Errorf("%s is not settable machine-wide and is printed as an assignment", k.key)
		}
	}
}

// cellSnapshot is what a cell resolves each settable key to, through the code that applies the
// compiled default: the answer has to be the same whether a default comes from the code or from
// an uncommented template line.
func cellSnapshot(t *testing.T, c *Cell) map[string]string {
	t.Helper()
	m := map[string]string{
		"kind": c.Kind, "harness": c.Harness, "roles": strings.Join(c.Roles, " "), "session": c.Session,
		"deskd": c.Deskd, "forge": c.Forge, "forge api": c.ForgeAPIBase, "repo": c.Repo,
		"generated files":  strings.Join(c.cellctlGeneratedFiles(), "|"),
		"repair admission": c.repairAdmissionValue(),
		"desk tools":       deskToolsBin(c.Env),
	}
	m["cockpit"], _ = c.cockpitWant("")
	sp, err := c.scratchPolicy()
	m["scratch"] = fmt.Sprintf("%v %v", sp, err)
	for _, role := range strings.Fields(rolesDefault) {
		for _, harness := range harnessValues {
			m["model "+role+" on "+harness] = fmt.Sprintf("%+v", c.resolveRoleModel(role, harness))
		}
	}
	for _, name := range []string{"kimi", "glm"} {
		for _, suffix := range providerSuffixes {
			m["provider "+name+" "+suffix], _ = c.providerValue(name, suffix)
		}
	}
	return m
}

// TestCellDefaultsTemplateUncommentedLines: each commented-out line, uncommented on its own, is
// read by the strict reader and is not refused; a line that prints a value assigns its key the
// registry's default, and a line that prints none assigns nothing (an empty value in this file
// sets nothing). A cell under a file with EVERY line uncommented resolves to what it resolves to
// with no file — which is what makes the value after each `=` the compiled default rather than a
// claim — and that holds with the environment setting the keys the template prints empty, too:
// those lines do not displace what the launching shell exported. A line that prints a value DOES
// outrank the environment, by design, and the last part pins that as well.
func TestCellDefaultsTemplateUncommentedLines(t *testing.T) {
	tpl := cellDefaultsTemplate()
	lines := strings.Split(tpl, "\n")
	idx, keys, _ := templateAssignments(lines)
	if len(idx) == 0 {
		t.Fatal("the template has no commented-out assignment")
	}
	uncomment := func(only int) string {
		out := append([]string(nil), lines...)
		for n, i := range idx {
			if only < 0 || only == n {
				out[i] = strings.TrimPrefix(out[i], "# ")
			}
		}
		return strings.Join(out, "\n")
	}
	// valued is the keys whose line prints a value, in the template's order; the rest print none.
	var valued, valueless []string
	for _, key := range keys {
		if k, _ := envKeyLookup(key); k.def != "" {
			valued = append(valued, key)
		} else {
			valueless = append(valueless, key)
		}
	}
	if len(valued) == 0 || len(valueless) == 0 {
		t.Fatalf("the template prints %d line(s) with a value and %d without; this test needs both", len(valued), len(valueless))
	}
	for _, goos := range []string{"linux", "windows"} {
		for n, key := range keys {
			e := emptyEnv()
			got, err := overlayCellDefaultsText(goos, e, []byte(uncomment(n)), "defaults.env")
			if err != nil {
				t.Errorf("%s: uncommenting %s is refused: %v", goos, key, err)
				continue
			}
			k, _ := envKeyLookup(key)
			if k.def == "" {
				if len(got) != 0 || e.IsSet(key) {
					t.Errorf("%s: uncommenting %s, which prints no value, set %v (set=%v), want nothing set", goos, key, got, e.IsSet(key))
				}
			} else if !reflect.DeepEqual(got, []string{key}) || !e.IsSet(key) || e.Get(key) != k.def {
				t.Errorf("%s: uncommenting %s set %v to %q, want that one key set to %q", goos, key, got, e.Get(key), k.def)
			}
			// The same line over an environment that sets the key: a line with no value leaves
			// the environment's value and its source; a line with one replaces both.
			e = emptyEnv()
			e.putFrom(key, "from the shell", layerProcess)
			if _, err := overlayCellDefaultsText(goos, e, []byte(uncomment(n)), "defaults.env"); err != nil {
				t.Errorf("%s: uncommenting %s over an environment that sets it is refused: %v", goos, key, err)
				continue
			}
			wantValue, wantSource := k.def, layerDefaults
			if k.def == "" {
				wantValue, wantSource = "from the shell", layerProcess
			}
			if e.Get(key) != wantValue || e.Source(key) != wantSource {
				t.Errorf("%s: with the environment setting %s, its uncommented line leaves %q from %s, want %q from %s", goos, key, e.Get(key), e.Source(key), wantValue, wantSource)
			}
		}
		if got, err := overlayCellDefaultsText(goos, emptyEnv(), []byte(uncomment(-1)), "defaults.env"); err != nil || !reflect.DeepEqual(got, valued) {
			t.Errorf("%s: every line uncommented set %v (%v), want the keys that print a value, %v", goos, got, err, valued)
		}
	}

	root := defaultsRoot(t)
	for _, k := range machineKeys() {
		t.Setenv(k.key, "")
		os.Unsetenv(k.key)
	}
	// Two cells: one with CELL_GO_CACHE=on in its own file, so the cache budget and floor are
	// resolved rather than skipped (cell.env overrides the template's own line, which says off),
	// and one that leaves the cache to the layers under it.
	writeCell(t, root, "demo", demoCell+"CELL_GO_CACHE=on\n")
	writeCell(t, root, "plain", strings.Replace(demoCell, "CELL=demo", "CELL=plain", 1))
	cacheHost := runtime.GOOS == "darwin" || runtime.GOOS == "linux"
	resolve := func() (map[string]string, *Cell) {
		c := loadCell("demo")
		m := cellSnapshot(t, c)
		plain := loadCell("plain")
		if cacheHost {
			p, err := c.cachePolicy()
			if err != nil || p == nil {
				t.Fatalf("cachePolicy = %v, %v", p, err)
			}
			m["cache budget"], m["cache floor"] = fmt.Sprint(p.Budget), fmt.Sprint(p.Floor)
			off, err := plain.cachePolicy()
			m["cache"] = fmt.Sprintf("on=%v err=%v", off != nil, err)
		}
		return m, plain
	}
	want, ref := resolve()
	writeDefaults(t, root, uncomment(-1))
	got, c := resolve()
	if !reflect.DeepEqual(c.DefaultsKeys, valued) {
		t.Fatalf("the file with every line uncommented set %v, want the keys that print a value, %v", c.DefaultsKeys, valued)
	}
	for name, w := range want {
		if got[name] != w {
			t.Errorf("%s resolves to %q with every template line uncommented, %q with no file: a printed default is not the compiled one", name, got[name], w)
		}
	}
	// A key the template does NOT print must be untouched by a file made of it.
	inTemplate := map[string]bool{}
	for _, k := range keys {
		inTemplate[k] = true
	}
	for k, v := range ref.Env.vals {
		if !inTemplate[k] && c.Env.Get(k) != v {
			t.Errorf("%s = %q with every template line uncommented, %q with no file", k, c.Env.Get(k), v)
		}
	}
	for k, v := range c.Env.vals {
		if !inTemplate[k] && ref.Env.Get(k) != v {
			t.Errorf("%s = %q with every template line uncommented, %q with no file", k, v, ref.Env.Get(k))
		}
	}

	// Every line that prints a value is covered: loadCell stores the default itself, or the
	// snapshot above resolves it through the code that applies it, or the default is one
	// constant the registry and the read share. A new line with a value needs one of the three.
	for _, k := range machineKeys() {
		proof, listed := templateDefaultProof[k.key]
		stored := k.def != "" && ref.Env.Get(k.key) == k.def
		switch {
		case k.def == "":
			if listed {
				t.Errorf("%s prints no value and templateDefaultProof lists it", k.key)
			}
		case !listed && !stored:
			t.Errorf("%s is printed with %q, and nothing here shows that is the compiled default: loadCell does not store it, so resolve it in cellSnapshot or share one constant with the read, and say which in templateDefaultProof", k.key, k.def)
		case listed && strings.HasPrefix(proof, "resolved: "):
			found := false
			for name := range want {
				found = found || strings.HasPrefix(name, strings.TrimPrefix(proof, "resolved: "))
			}
			if !found && (cacheHost || !strings.Contains(proof, "cache")) {
				t.Errorf("%s: templateDefaultProof names %q, which the snapshot does not resolve", k.key, proof)
			}
		case listed && !strings.HasPrefix(proof, "shared constant: ") && !strings.HasPrefix(proof, "by reading: "):
			t.Errorf("%s: templateDefaultProof entry %q is not one of resolved / shared constant / by reading", k.key, proof)
		}
	}
	for key := range templateDefaultProof {
		if k, ok := envKeyLookup(key); !ok || k.class != envMachine {
			t.Errorf("templateDefaultProof lists %s, which is not a key the template prints", key)
		}
	}

	// The environment sets keys. Everything above ran with every machine-wide key unset, which
	// is the one arrangement in which a line that assigned the empty string could not be told
	// from a line that assigns nothing. So: export a value for EVERY key the template prints
	// with no value, and the file with every line uncommented must still resolve both cells as
	// no file does — each of those keys keeping the exported value and naming the environment
	// as its source.
	if err := os.Remove(filepath.Join(root, cellDefaultsFile)); err != nil {
		t.Fatal(err)
	}
	exported := map[string]string{}
	for _, key := range valueless {
		exported[key] = "shell-" + strings.ToLower(key)
	}
	for key, v := range map[string]string{
		"CELL_CADENCE":     "45m",
		"CELL_TICK_BUDGET": "7",
		"CELL_PROVIDER":    "kimi",
	} {
		if _, ok := exported[key]; !ok {
			t.Fatalf("%s is no longer a key the template prints with no value; this table is stale", key)
		}
		exported[key] = v
	}
	for key, v := range exported {
		t.Setenv(key, v)
	}
	want, ref = resolve()
	for _, key := range valueless {
		if ref.Env.Get(key) != exported[key] {
			t.Fatalf("with no file, %s = %q, want the exported %q", key, ref.Env.Get(key), exported[key])
		}
	}
	writeDefaults(t, root, uncomment(-1))
	got, c = resolve()
	if !reflect.DeepEqual(c.DefaultsKeys, valued) {
		t.Errorf("with the environment setting %d key(s), the file with every line uncommented set %v, want %v", len(valueless), c.DefaultsKeys, valued)
	}
	for _, key := range valueless {
		if c.Env.Get(key) != exported[key] || c.Env.Source(key) != layerProcess {
			t.Errorf("%s = %q from %s with every template line uncommented, want the exported %q from %s: a line with no value displaced the environment", key, c.Env.Get(key), c.Env.Source(key), exported[key], layerProcess)
		}
	}
	for name, w := range want {
		if got[name] != w {
			t.Errorf("with the environment setting the keys the template prints empty, %s resolves to %q with every template line uncommented, %q with no file", name, got[name], w)
		}
	}

	// And the other half, which is the design and not a defect: a line that prints a value
	// outranks the environment. CELL_GO_CACHE is exported `on`; the template's own line says
	// `off`; uncommented unchanged, every cell that does not set the key itself gets `off`.
	if err := os.Remove(filepath.Join(root, cellDefaultsFile)); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CELL_GO_CACHE", "on")
	if p := loadCell("plain"); p.Env.Get("CELL_GO_CACHE") != "on" || p.Env.Source("CELL_GO_CACHE") != layerProcess {
		t.Fatalf("with no file, CELL_GO_CACHE = %q from %s, want the exported on", p.Env.Get("CELL_GO_CACHE"), p.Env.Source("CELL_GO_CACHE"))
	}
	for n, key := range keys {
		if key == "CELL_GO_CACHE" {
			writeDefaults(t, root, uncomment(n))
		}
	}
	if p := loadCell("plain"); p.Env.Get("CELL_GO_CACHE") != "off" || p.Env.Source("CELL_GO_CACHE") != layerDefaults {
		t.Errorf("CELL_GO_CACHE is exported on and the template's line is uncommented unchanged: the cell reads %q from %s, want off from the %s — a line that prints a value outranks the environment", p.Env.Get("CELL_GO_CACHE"), p.Env.Source("CELL_GO_CACHE"), layerDefaults)
	}
}

// templateDefaultProof says, for each template line that prints a value loadCell does not store
// in the cell's environment, what makes that value the compiled default. "resolved: <name>" is a
// cellSnapshot entry (by prefix): the test resolves it with the line uncommented and with no
// file, and the two must agree. "shared constant" is a default the read site and the registry
// take from one Go constant, for a read that is inline in a launch and has no resolver to call.
// "by reading" is the one line whose printed value is not a constant the code holds at all.
var templateDefaultProof = map[string]string{
	"CELL_GO_CACHE":                   "resolved: cache",
	"CELL_GO_CACHE_BYTES":             "resolved: cache budget",
	"CELL_GO_CACHE_MIN_FREE":          "resolved: cache floor",
	"CELL_SCRATCH_MAX_AGE":            "resolved: scratch",
	"CELL_SCRATCH_MAX_BYTES":          "resolved: scratch",
	"CELL_HARNESS":                    "resolved: harness",
	"CELL_COCKPIT":                    "resolved: cockpit",
	"CELLCTL_GENERATED_FILES":         "resolved: generated files",
	"DESK_MODEL_DEFAULT":              "resolved: model ",
	"ASSAY_REPAIR_ADMISSION":          "resolved: repair admission",
	"CELL_PROVIDER_KIMI_BASE_URL":     "resolved: provider kimi BASE_URL",
	"CELL_PROVIDER_KIMI_TOKEN_ENV":    "resolved: provider kimi TOKEN_ENV",
	"CELL_PROVIDER_KIMI_MODEL":        "resolved: provider kimi MODEL",
	"CELL_PROVIDER_GLM_BASE_URL":      "resolved: provider glm BASE_URL",
	"CELL_PROVIDER_GLM_TOKEN_ENV":     "resolved: provider glm TOKEN_ENV",
	"CELL_PROVIDER_GLM_MODEL":         "resolved: provider glm MODEL",
	"CELL_PATH":                       "shared constant: scrubbedPathTail, the tail scrubbedComposeEnv composes when the key is empty",
	"CELLCTL_ORCA_TIMEOUT":            "shared constant: orcaProbeSeconds, the bound orcaReachable starts from",
	"CLAUDE_CODE_AUTO_COMPACT_WINDOW": "shared constant: autoCompactWindowDefault, the value the claude launch falls back to",
	"CELLCTL_DESKWT":                  "shared constant: deskwtDefault, the value worktreeViaDeskwt falls back to",
	"CELL_FF_ROOTS":                   "by reading: the one read compares the value with 1, so 0 is every value that is not 1, unset included",
}

// TestCellDefaultsTemplateNamesWhatItRefuses: every per-cell key is in the refused section with
// its reason; every per-run switch, every followed location and every host variable is named in
// the last section, under its own heading; and internal plumbing is in the template nowhere.
func TestCellDefaultsTemplateNamesWhatItRefuses(t *testing.T) {
	tpl := cellDefaultsTemplate()
	sections := templateSections(tpl)
	refused, unset := sections["Per-cell keys: refused in this file"], sections["Taken from the command's environment: refused in this file"]
	if refused == "" || unset == "" {
		t.Fatalf("the template lacks a section: %v", reflect.ValueOf(sections).MapKeys())
	}
	perRun, rest, cut := strings.Cut(unset, "Locations followed from the launching shell:")
	if !cut || !strings.Contains(perRun, "Switches for one run:") {
		t.Fatal("the last section does not separate the per-run switches from the followed locations")
	}
	followed, host, cut := strings.Cut(rest, "The host's own variables, refused the same way: ")
	if !cut {
		t.Fatal("the last section does not name the host's own variables")
	}
	hostListed := map[string]bool{}
	for _, k := range strings.Split(strings.TrimSuffix(strings.TrimSpace(host), "."), ", ") {
		hostListed[k] = true
	}
	words := map[string]bool{}
	for _, l := range strings.Split(tpl, "\n") {
		for _, w := range strings.FieldsFunc(l, func(r rune) bool {
			return !(r == '_' || r >= '0' && r <= '9' || r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z')
		}) {
			words[w] = true
		}
	}
	nRefused, nHost := 0, 0
	for _, k := range envKeys {
		entry := k.key + ": " + strings.Join(strings.Fields(k.desc), " ")
		switch {
		case k.class == envMachine:
			continue
		case k.class == envCell:
			if !strings.Contains(refused, entry+" refused here: it "+k.why+".") {
				t.Errorf("%s is refused in defaults.env and the refused section does not carry it with its reason", k.key)
			}
		case k.from == envFromRun:
			if !strings.Contains(perRun, entry) {
				t.Errorf("%s is a per-run switch and the template's list of them does not name it", k.key)
			}
		case k.from == envFromShell:
			if !strings.Contains(followed, entry) {
				t.Errorf("%s is followed from the launching shell and the template's list of those does not name it", k.key)
			}
		case k.from == envFromHost:
			nHost++
			if !hostListed[k.key] {
				t.Errorf("%s is one of the host's own variables and the template's line of them does not name it", k.key)
			}
		default:
			if words[k.key] {
				t.Errorf("%s is internal plumbing and the template names it", k.key)
			}
		}
		nRefused++
	}
	if nHost != len(hostListed) {
		t.Errorf("the template names %d host variable(s) %v, the registry has %d", len(hostListed), hostListed, nHost)
	}
	if nRefused != len(cellDefaultsRefused) {
		t.Errorf("%d key(s) accounted for, defaults.env refuses %d", nRefused, len(cellDefaultsRefused))
	}
	// Every refused key IS refused, by the reader, on the line that sets it.
	for _, r := range cellDefaultsRefused {
		_, err := overlayCellDefaultsText(runtime.GOOS, emptyEnv(), []byte("# a comment\n"+r.key+"=x\n"), "defaults.env")
		if err == nil || !strings.Contains(err.Error(), "line 2 sets "+r.key+", which "+r.why) {
			t.Errorf("%s: want a refusal naming line 2, the key and the reason, got %v", r.key, err)
		}
	}
}

// TestSetKnowsEveryLever: `cellctl set` accepts without --force exactly the registry's levers.
// The second list is the one `set` kept by hand before the registry existed: a key leaving it
// would be a key `set` stops recognising.
func TestSetKnowsEveryLever(t *testing.T) {
	for _, k := range envKeys {
		if got, want := knownCellEnvKey(k.key), k.class != envNotLever; got != want {
			t.Errorf("knownCellEnvKey(%s) = %v, want %v", k.key, got, want)
		}
	}
	for _, k := range strings.Fields(`CELL CELL_KIND CELL_CONTAINER_CONFIG CELL_CONTAINER_LAUNCHER CELL_ROOTS CELL_COCKPIT DESKD CELL_FORGE CELL_REPO CELLS_CONFIG
FORGE_API_BASE DESKD_ADDR DESKD_INDEX DESKD_APP_PEM DESKD_APP_ID_VAR ORGS GITLAB_GROUP
GITLAB_API_BASE GITLAB_TOKEN_STORE DESKD_GITLAB_TOKEN_FILE ROLES DESK_MODEL_DEFAULT CODEX_MODEL_default CURSOR_MODEL_default
CELL_CADENCE CELL_TICK_BUDGET CELL_HARNESS TMUX_SESSION CELL_PROVIDER CELL_REPO_SLUG CELL_PATH CELL_MODEL_POLICY CELL_PROVIDER_DEFAULTS CELL_PROVIDER_OVERRIDES ASSAY_REPAIR_ADMISSION CELL_COMMS_CONFIG
CELL_ROLE_CONTEXT
TIER_MODEL_TOP_CLAUDE TIER_MODEL_MID_CLAUDE TIER_MODEL_FAST_CLAUDE
TIER_MODEL_TOP_CODEX TIER_MODEL_MID_CODEX TIER_MODEL_FAST_CODEX
TIER_MODEL_TOP_CURSOR TIER_MODEL_MID_CURSOR TIER_MODEL_FAST_CURSOR
CURSOR_MODEL_verify_desk CELL_PROVIDER_EXAMPLE_BASE_URL CELL_PROVIDER_EXAMPLE_TOKEN_ENV CELL_PROVIDER_EXAMPLE_MODEL`) {
		if !knownCellEnvKey(k) {
			t.Errorf("`set` knew %s before the registry and does not now", k)
		}
	}
	for _, k := range []string{"NOT_A_REAL_KEY", "DESK_MODEL_woker_desk", "CELL_PROVIDER__MODEL", "CELL_PROVIDER_EXAMPLE_MODEL_TOP", "HOME", "DRY_RUN", "CELLS_ROOT"} {
		if knownCellEnvKey(k) {
			t.Errorf("knownCellEnvKey(%s) = true: `set` would write a key that is not a lever without --force", k)
		}
	}
}

// docKeyToken is a key as the documentation writes one: letters, digits, underscores, and the
// <role> / <NAME> placeholders of a family.
var docKeyToken = regexp.MustCompile("`([A-Za-z_][A-Za-z0-9_]*(?:<[A-Za-z]+>[A-Za-z0-9_]*)*)`")

// docTableKeys is every key named in the first column of the Markdown table that follows the
// line starting with heading, up to the first line that is not a table row. A cell written
// `A_B_C` / `_D_C` names A_B_C and A_D_C; a <role> family names every role's key and a <NAME>
// family an example provider's.
func docTableKeys(t *testing.T, doc, heading string) []string {
	t.Helper()
	_, rest, ok := strings.Cut(doc, "\n"+heading)
	if !ok {
		t.Fatalf("the documentation has no %q", heading)
	}
	var out []string
	inTable := false
	for _, l := range strings.Split(rest, "\n")[1:] {
		if !strings.HasPrefix(l, "|") {
			if inTable {
				break
			}
			continue
		}
		inTable = true
		cells := strings.Split(l, "|")
		if len(cells) < 3 || strings.HasPrefix(strings.TrimSpace(cells[1]), "---") {
			continue
		}
		first := ""
		for _, m := range docKeyToken.FindAllStringSubmatch(cells[1], -1) {
			k := m[1]
			if strings.HasPrefix(k, "_") && first != "" {
				head := strings.Split(first, "_")
				tail := strings.Split(strings.TrimPrefix(k, "_"), "_")
				k = strings.Join(append(head[:len(head)-len(tail)], tail...), "_")
			} else {
				first = k
			}
			if strings.Contains(k, "<role>") {
				for _, role := range strings.Fields(rolesDefault) {
					out = append(out, strings.ReplaceAll(k, "<role>", underscore(role)))
				}
				continue
			}
			out = append(out, strings.ReplaceAll(k, "<NAME>", "EXAMPLE"))
		}
	}
	if len(out) == 0 {
		t.Fatalf("no key found in the table under %q", heading)
	}
	return out
}

// usageKeyToken is a variable name in the usage text, in one of the namespaces cellctl's own
// keys live in. A name outside them (a harness's ANTHROPIC_* export, an operator's credential
// variable) is another tool's and is not held to the registry.
var usageKeyToken = regexp.MustCompile(`\b(?:CELLS?|CELLCTL|DESKD|DESK_MODEL|CODEX_MODEL|CURSOR_MODEL|TIER_MODEL)_[A-Za-z0-9_<>]*[A-Za-z0-9>]`)

// usageKeysNotInRegistry is every such name the usage text carries that the registry does not,
// with the reason. The usage text is shared word for word with the shell implementation
// (TestUsageMatchesOracle), so it also describes what only that implementation does.
var usageKeysNotInRegistry = map[string]string{
	"CELL_PROVIDER_<NAME>":              "the stem of the provider family, written before its three suffixes",
	"CELL_PROVIDER_<NAME>_MODEL_<TIER>": "the shell implementation's per-tier provider model; this cellctl has no --model-top/mid/fast and reads no such key",
	"CELL_TIER_MODEL_<TIER>":            "the variable the shell implementation threads those flags through; not read here",
	"CELLCTL_VERSION":                   "the shell script's version variable, named in a comment about the build stamp",
}

// TestEnvKeyListsAgree holds the hand-written key lists to the registry: the two tables in the
// cellctl documentation, the usage text, and the shell implementation's own known-key list.
func TestEnvKeyListsAgree(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "docs", "cellctl.md"))
	if err != nil {
		t.Fatal(err)
	}
	doc := string(raw)

	// The cell.env table is a narrative of the keys a cell's own file usually carries, not the
	// complete list (`cellctl defaults print` is). Every key it names must be a lever.
	for _, k := range docTableKeys(t, doc, "## `cell.env`") {
		if !envKeyIsLever(k) {
			t.Errorf("docs/cellctl.md's cell.env table names %s, which envkeys.go does not list as a lever", k)
		}
	}

	// The refused table is the same set defaults.env refuses, both ways.
	documented := docTableKeys(t, doc, "**Keys the file refuses.**")
	sort.Strings(documented)
	var refused []string
	for _, r := range cellDefaultsRefused {
		refused = append(refused, r.key)
	}
	sort.Strings(refused)
	if !reflect.DeepEqual(documented, refused) {
		t.Errorf("docs/cellctl.md's table of keys defaults.env refuses is\n  %v\nand defaults.env refuses\n  %v", documented, refused)
	}

	// The usage text.
	// One placeholder is written out in words; read it as the placeholder it is.
	usage := strings.ReplaceAll(usageText, "<role with - as _>", "<role>")
	met := map[string]bool{}
	for _, tok := range usageKeyToken.FindAllString(usage, -1) {
		probe := tok
		for placeholder, example := range map[string]string{"<role>": "the_desk", "<NAME>": "EXAMPLE", "<TIER>": "TOP", "<HARNESS>": "CLAUDE"} {
			probe = strings.ReplaceAll(probe, placeholder, example)
		}
		if _, ok := envKeyLookup(probe); ok && tok != "CELL_PROVIDER_<NAME>_MODEL_<TIER>" {
			continue
		}
		if _, ok := usageKeysNotInRegistry[tok]; ok {
			met[tok] = true
			continue
		}
		t.Errorf("the usage text names %s, which envkeys.go does not classify: classify it, or say in usageKeysNotInRegistry why cellctl does not read it", tok)
	}
	for tok := range usageKeysNotInRegistry {
		if !met[tok] {
			t.Errorf("usageKeysNotInRegistry lists %s, which the usage text no longer names", tok)
		}
	}

	// The shell implementation's known-key list is a subset: every key it lets `set` write is a
	// lever here. (This cellctl knows more — the cursor harness's keys, the managed cache and
	// scratch keys, the cadence keys — which is a difference in what the two implement.)
	oracle, err := os.ReadFile(filepath.Join("..", "..", "..", "cellctl", "testdata", "cellctl-shell-oracle.sh"))
	if err != nil {
		t.Fatal(err)
	}
	_, list, ok := strings.Cut(string(oracle), "\nCELL_ENV_KNOWN_KEYS=\"")
	if !ok {
		t.Fatal("the shell implementation has no CELL_ENV_KNOWN_KEYS")
	}
	list, _, _ = strings.Cut(list, "\"")
	known := strings.Fields(strings.ReplaceAll(list, "\\\n", " "))
	if len(known) < 20 {
		t.Fatalf("read only %d key(s) from CELL_ENV_KNOWN_KEYS", len(known))
	}
	for _, k := range known {
		if !envKeyIsLever(k) {
			t.Errorf("the shell implementation's `set` knows %s, which envkeys.go does not list as a lever", k)
		}
	}
}

// TestNewSeedsCellDefaults: `cellctl new` writes the template into a cells root that has no
// defaults file and says so in one line; it never touches a file that is there, whatever it
// holds; and a `new` that refuses writes nothing.
func TestNewSeedsCellDefaults(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	repo := t.TempDir()
	if out, err := exec.Command("git", "init", "-q", repo).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v %s", err, out)
	}
	scaffold := func(name string) string {
		return captureStdout(t, func() {
			cmdNew([]string{name, "--kind", "scrubbed", "--repo", repo, "--repo-slug", "o/r", "--roots", "o/r=" + repo})
		})
	}
	root := defaultsRoot(t)
	path := filepath.Join(root, cellDefaultsFile)

	refusal(t, "new with no --repo", func() { cmdNew([]string{"refused", "--kind", "scrubbed"}) })
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		t.Fatalf("a refused `new` left %s behind (%v)", path, err)
	}

	out := scaffold("one")
	line := fmt.Sprintf(cellDefaultsCreated, path)
	if strings.Count(out, line) != 1 || strings.Count(out, "[defaults]") != 1 {
		t.Errorf("`new` in a root with no defaults file should say once that it wrote one:\n%s", out)
	}
	got, err := os.ReadFile(path)
	if err != nil || string(got) != cellDefaultsTemplate() {
		t.Fatalf("`new` should have written the template to %s (%v)", path, err)
	}
	if st, err := os.Stat(path); err != nil || (runtime.GOOS != "windows" && st.Mode().Perm() != 0o600) {
		t.Errorf("the defaults file should be private to its owner, got %v %v", st.Mode(), err)
	}
	if keys, read, err := overlayCellDefaults(emptyEnv(), path); err != nil || !read || len(keys) != 0 {
		t.Errorf("the file `new` wrote reads as keys=%v read=%v err=%v, want read and empty", keys, read, err)
	}

	// The operator's edit, then a second cell: the file is theirs now.
	const edited = "# mine\nCELL_COCKPIT=tmux\n"
	writeDefaults(t, root, edited)
	if out := scaffold("two"); strings.Contains(out, "[defaults]") {
		t.Errorf("`new` in a root that has a defaults file should say nothing about it:\n%s", out)
	}
	if got, _ := os.ReadFile(path); string(got) != edited {
		t.Errorf("`new` changed an existing defaults file:\n%s", got)
	}

	// Something that is not a readable file at all is left alone too.
	if runtime.GOOS != "windows" {
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
		target := filepath.Join(root, "elsewhere.env")
		if err := os.Symlink(target, path); err != nil {
			t.Fatal(err)
		}
		if out := scaffold("three"); strings.Contains(out, "[defaults]") {
			t.Errorf("`new` should say nothing when a dangling link is at the path:\n%s", out)
		}
		if _, err := os.Lstat(target); !os.IsNotExist(err) {
			t.Errorf("`new` wrote through a dangling link to %s", target)
		}
		if l, err := os.Readlink(path); err != nil || l != target {
			t.Errorf("`new` replaced the link at %s (%q, %v)", path, l, err)
		}
	}
}

// TestSeedCellDefaultsFailureIsANotice: a cells root the file cannot be written under does not
// fail the `new` that already scaffolded its cell.
func TestSeedCellDefaultsFailureIsANotice(t *testing.T) {
	blocker := filepath.Join(t.TempDir(), "a-file")
	if err := os.WriteFile(blocker, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CELLS_ROOT", filepath.Join(blocker, "cells"))
	var out string
	errOut := captureStderr(t, func() { out = captureStdout(t, seedCellDefaults) })
	if !strings.Contains(errOut, "NOTICE: no machine-wide defaults file written") || !strings.Contains(errOut, "cellctl defaults init") {
		t.Errorf("want a notice naming the way to write it later, got %q", errOut)
	}
	if out != "" {
		t.Errorf("nothing was created, and stdout says %q", out)
	}
}

// TestDefaultsVerb: `cellctl defaults print` is the template on stdout and writes no file;
// `cellctl defaults init` creates the file once and refuses a second time without touching it.
func TestDefaultsVerb(t *testing.T) {
	root := defaultsRoot(t)
	path := filepath.Join(root, cellDefaultsFile)
	verb := func(args ...string) string {
		prev := os.Args
		defer func() { os.Args = prev }()
		os.Args = append([]string{"cellctl", "defaults"}, args...)
		return captureStdout(t, func() { run() })
	}

	if got := verb("print"); got != cellDefaultsTemplate() {
		t.Errorf("`defaults print` is not the template (%d bytes, want %d)", len(got), len(cellDefaultsTemplate()))
	}
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		t.Fatalf("`defaults print` wrote %s", path)
	}

	if got, want := verb("init"), fmt.Sprintf(cellDefaultsCreated, path); got != want {
		t.Errorf("`defaults init` said %q, want %q", got, want)
	}
	if got, err := os.ReadFile(path); err != nil || string(got) != cellDefaultsTemplate() {
		t.Fatalf("`defaults init` should have written the template (%v)", err)
	}

	const edited = "CELL_HARNESS=codex\n"
	writeDefaults(t, root, edited)
	msg := refusal(t, "a second defaults init", func() { cmdDefaults([]string{"init"}) })
	if !strings.Contains(msg, path) || !strings.Contains(msg, "never overwritten") || !strings.Contains(msg, "cellctl defaults print") {
		t.Errorf("the refusal should name the file, the rule and the way to compare, got %q", msg)
	}
	if got, _ := os.ReadFile(path); string(got) != edited {
		t.Errorf("a refused `defaults init` changed the file:\n%s", got)
	}

	// A cells root that is not there is `new`'s to create: init refuses and makes no directory.
	missing := filepath.Join(root, "no-such-root")
	t.Setenv("CELLS_ROOT", missing)
	msg = refusal(t, "defaults init with no cells root", func() { cmdDefaults([]string{"init"}) })
	if !strings.Contains(msg, "there is no cells root at "+missing) || !strings.Contains(msg, "cellctl new") {
		t.Errorf("the refusal should say the cells root is missing and what creates it, got %q", msg)
	}
	if _, err := os.Lstat(missing); !os.IsNotExist(err) {
		t.Errorf("`defaults init` created the cells root %s (%v)", missing, err)
	}
	t.Setenv("CELLS_ROOT", root)

	for _, args := range [][]string{nil, {"write"}, {"init", "--force"}, {"print", "extra"}} {
		msg := refusal(t, fmt.Sprintf("defaults %v", args), func() { cmdDefaults(args) })
		if !strings.Contains(msg, "usage: cellctl defaults init|print") {
			t.Errorf("defaults %v: want the usage refusal, got %q", args, msg)
		}
	}
	if got, _ := os.ReadFile(path); string(got) != edited {
		t.Errorf("a refused `defaults` changed the file:\n%s", got)
	}
}
